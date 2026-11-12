package api

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

const (
	// jwksMaxAge is how long fetched keys are used before asking again.
	// Supabase's edge caches the document for 10 minutes too, and tells
	// operators to wait 20 before using a new key or revoking an old one.
	jwksMaxAge = 10 * time.Minute

	// jwksRetry is the shortest time between two fetches while we hold keys.
	// A token naming an unknown key causes a fetch (the key may be new), so
	// without this anyone could make us call Supabase on every request.
	jwksRetry = time.Minute

	// jwksRetryNoKeys is the same while we hold none (Supabase was
	// unreachable at the first request): shorter, because until a fetch
	// succeeds nobody can be authenticated.
	jwksRetryNoKeys = 5 * time.Second

	// jwksTimeout caps one fetch.
	jwksTimeout = 3 * time.Second

	// maxJWKSBody caps the document. A real one is a few hundred bytes.
	maxJWKSBody = 64 << 10
)

// errUnknownKey means the token names a key the project doesn't publish. The
// token is refused (401); it is not an outage.
var errUnknownKey = errors.New("token signed by an unknown key")

// keySet is the project's public signing keys (its JWKS), fetched on first
// use and kept in memory. Nothing is fetched at startup, so the server
// starts even when Supabase is unreachable, as it does with the database.
type keySet struct {
	url    string
	client *http.Client
	now    func() time.Time

	// mu guards the fields below and is held during a fetch, so requests
	// arriving together wait for one fetch instead of each starting one.
	mu        sync.Mutex
	keys      map[string]*ecdsa.PublicKey // by key id; nil until a fetch succeeds
	fetchedAt time.Time                   // the last successful fetch
	triedAt   time.Time                   // the last fetch, successful or not
}

// key returns the public key with the id kid. It fetches the key set when
// there is none yet, when it is older than jwksMaxAge, or when kid isn't in
// it, but never more often than the retry intervals allow. If a refresh
// fails, the keys already held keep working.
//
// The error is ErrAuthUnavailable when no keys could be had at all, and
// errUnknownKey when the project's keys don't include kid.
func (s *keySet) key(ctx context.Context, kid string) (*ecdsa.PublicKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	key, found := s.keys[kid]
	if found && now.Sub(s.fetchedAt) < jwksMaxAge {
		return key, nil
	}

	retry := jwksRetry
	if s.keys == nil {
		retry = jwksRetryNoKeys
	}
	if s.triedAt.IsZero() || now.Sub(s.triedAt) >= retry {
		s.triedAt = now
		keys, err := s.fetch(ctx)
		if err != nil {
			slog.WarnContext(ctx, "jwks: fetch failed; keeping the keys already held", "err", err, "held", len(s.keys))
		} else {
			s.keys, s.fetchedAt = keys, now
			key, found = keys[kid]
		}
	}

	switch {
	case found:
		return key, nil
	case s.keys == nil:
		return nil, ErrAuthUnavailable
	default:
		return nil, errUnknownKey
	}
}

// jwk is one entry of a JWKS document: the fields of an elliptic-curve
// public key (RFC 7518 §6.2). x and y are the point's coordinates.
type jwk struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

// fetch downloads the key set. Keys that aren't ES256 signing keys are
// skipped; a document with no usable key is an error.
func (s *keySet) fetch(ctx context.Context) (map[string]*ecdsa.PublicKey, error) {
	// The result serves every request, so the one that happened to start
	// the fetch can't cancel it by disconnecting.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), jwksTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	res, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", s.url, err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get %s: status %d", s.url, res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxJWKSBody+1))
	if err != nil {
		return nil, fmt.Errorf("read key set: %w", err)
	}
	if len(body) > maxJWKSBody {
		return nil, fmt.Errorf("key set is larger than %d bytes", maxJWKSBody)
	}

	var doc struct {
		Keys []jwk `json:"keys"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("key set is not JSON: %w", err)
	}

	keys := make(map[string]*ecdsa.PublicKey, len(doc.Keys))
	for _, k := range doc.Keys {
		pub, err := k.publicKey()
		if err != nil {
			slog.WarnContext(ctx, "jwks: skipping a key", "kid", k.Kid, "err", err)
			continue
		}
		keys[k.Kid] = pub
	}
	if len(keys) == 0 {
		return nil, errors.New("key set has no usable ES256 key")
	}
	return keys, nil
}

// publicKey turns a JWK into a P-256 public key, or says why it isn't one
// we use.
func (k jwk) publicKey() (*ecdsa.PublicKey, error) {
	switch {
	case k.Kid == "":
		return nil, errors.New("no kid")
	case k.Kty != "EC" || k.Crv != "P-256":
		return nil, fmt.Errorf("not a P-256 key (kty %q, crv %q)", k.Kty, k.Crv)
	case k.Alg != "" && k.Alg != "ES256":
		return nil, fmt.Errorf("alg %q, not ES256", k.Alg)
	case k.Use != "" && k.Use != "sig":
		return nil, fmt.Errorf("use %q, not sig", k.Use)
	}

	// JWKs use base64url without padding. Each coordinate is exactly 32
	// bytes on P-256.
	x, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil || len(x) != 32 {
		return nil, errors.New("x is not 32 base64url bytes")
	}
	y, err := base64.RawURLEncoding.DecodeString(k.Y)
	if err != nil || len(y) != 32 {
		return nil, errors.New("y is not 32 base64url bytes")
	}

	// The "uncompressed" form of a point is the byte 4, then x, then y.
	// Parsing it also checks that the point is on the curve.
	point := make([]byte, 0, 65)
	point = append(point, 4)
	point = append(point, x...)
	point = append(point, y...)
	pub, err := ecdsa.ParseUncompressedPublicKey(elliptic.P256(), point)
	if err != nil {
		return nil, fmt.Errorf("not a point on P-256: %w", err)
	}
	return pub, nil
}
