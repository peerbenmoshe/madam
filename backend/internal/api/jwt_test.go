package api

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	testSupabaseURL = "https://project.example"
	testIssuer      = testSupabaseURL + "/auth/v1"
	testJWKSURL     = testIssuer + "/.well-known/jwks.json"
	testKid         = "test-key-1"
	testUserA       = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
)

// testNow is the tests' clock reading; a fixed time, so nothing depends on
// when the tests run.
var testNow = time.Date(2026, 11, 12, 7, 0, 0, 0, time.UTC)

// A token signed by OpenSSL, not by the library under test, and the public
// half of the throwaway key that signed it (the private half was deleted).
// As with the webhook vector (hook tests), this checks our verification
// against an independent implementation rather than against itself:
//
//	printf '%s' "$HEADER.$PAYLOAD" | openssl dgst -sha256 -sign key.pem -binary
//
// then the DER signature's two integers, 32 bytes each, joined and
// base64url-encoded. It expires in 2100.
const (
	vectorJWKKid = "vector-key-1"
	vectorJWKX   = "caDsh-mj9YkqxnGCdFscVp3tP9iHw9lMVLbZ7phlyxU"
	vectorJWKY   = "EVJefYFKLEDNlEbCpnMi45Sz0fqknazuHUTrd2NYENc"
	vectorJWT    = "eyJhbGciOiJFUzI1NiIsImtpZCI6InZlY3Rvci1rZXktMSIsInR5cCI6IkpXVCJ9." +
		"eyJpc3MiOiJodHRwczovL3Byb2plY3QuZXhhbXBsZS9hdXRoL3YxIiwiYXVkIjoiYXV0aGVudGljYXRlZCIsImV4cCI6NDEwMjQ0NDgwMCwic3ViIjoiMjIyMjIyMjItMjIyMi0yMjIyLTIyMjItMjIyMjIyMjIyMjIyIiwicm9sZSI6ImF1dGhlbnRpY2F0ZWQiLCJpc19hbm9ueW1vdXMiOmZhbHNlfQ." +
		"D_L-ZeZODktV6B6AgD2oYw-6JiJqLexBlxSMj8Rj3eKLTIaRj85cNlIwf2_URcYpLEWbIDAWHWS8a8t0-p3-Ug"
	vectorJWTSub = "22222222-2222-2222-2222-222222222222"
)

// fakeJWKS stands in for Supabase's key endpoint. It is the http.Client's
// transport, so no port is opened and the test controls every answer.
type fakeJWKS struct {
	keys []jwk
	down bool // answer 503, as an unreachable Supabase would
	hits int  // how many times the key set was fetched
}

func (f *fakeJWKS) RoundTrip(req *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	switch {
	case req.URL.String() != testJWKSURL:
		rec.WriteHeader(http.StatusNotFound)
	case f.down:
		f.hits++
		rec.WriteHeader(http.StatusServiceUnavailable)
	default:
		f.hits++
		_ = json.NewEncoder(rec).Encode(map[string]any{"keys": f.keys})
	}
	return rec.Result(), nil
}

// newKey makes a P-256 key pair and its public half as a JWK.
func newKey(t *testing.T, kid string) (*ecdsa.PrivateKey, jwk) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	point, err := priv.PublicKey.Bytes() // the byte 4, then x, then y
	if err != nil {
		t.Fatal(err)
	}
	return priv, jwk{
		Kty: "EC", Crv: "P-256", Kid: kid, Alg: "ES256", Use: "sig",
		X: base64.RawURLEncoding.EncodeToString(point[1:33]),
		Y: base64.RawURLEncoding.EncodeToString(point[33:]),
	}
}

// testVerifier is a JWTVerifier over a fake key endpoint and a clock the
// test can move.
type testVerifier struct {
	*JWTVerifier
	jwks *fakeJWKS
	now  time.Time
}

func newTestVerifier(t *testing.T, keys ...jwk) *testVerifier {
	t.Helper()
	tv := &testVerifier{jwks: &fakeJWKS{keys: keys}, now: testNow}
	v, err := NewJWTVerifier(testSupabaseURL, &http.Client{Transport: tv.jwks})
	if err != nil {
		t.Fatal(err)
	}
	v.keys.now = func() time.Time { return tv.now }
	tv.JWTVerifier = v
	return tv
}

// authenticate runs a request carrying token as its bearer token.
func (tv *testVerifier) authenticate(token string) (string, error) {
	r := httptest.NewRequest(http.MethodGet, "/api/anything", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	id, err := tv.Authenticate(r)
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

// goodClaims is what Supabase puts in a signed-in user's token, as far as
// we read it, plus user_metadata, which we must ignore.
func goodClaims() jwt.MapClaims {
	return jwt.MapClaims{
		"iss":           testIssuer,
		"aud":           "authenticated",
		"exp":           testNow.Add(time.Hour).Unix(),
		"iat":           testNow.Unix(),
		"sub":           testUserA,
		"role":          "authenticated",
		"is_anonymous":  false,
		"user_metadata": map[string]any{"invite_code": "x", "sub": "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"},
	}
}

// sign makes a token the way Supabase does: ES256, with the key's id in the
// header.
func sign(t *testing.T, key *ecdsa.PrivateKey, kid string, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	if kid != "" {
		tok.Header["kid"] = kid
	}
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestJWTVerifierOpenSSLVector(t *testing.T) {
	tv := newTestVerifier(t, jwk{Kty: "EC", Crv: "P-256", Kid: vectorJWKKid, X: vectorJWKX, Y: vectorJWKY})

	got, err := tv.authenticate(vectorJWT)
	if err != nil {
		t.Fatalf("the OpenSSL-signed token was refused: %v", err)
	}
	if got != vectorJWTSub {
		t.Errorf("user id = %s, want %s", got, vectorJWTSub)
	}

	// One changed character in the payload must break it.
	tampered := strings.Replace(vectorJWT, ".eyJpc3Mi", ".eyJpc3Nj", 1)
	if tampered == vectorJWT {
		t.Fatal("the tampering changed nothing")
	}
	if _, err := tv.authenticate(tampered); err == nil {
		t.Error("a tampered OpenSSL-signed token was accepted")
	}
}

func TestJWTVerifierAcceptsAGoodToken(t *testing.T) {
	key, pub := newKey(t, testKid)
	tv := newTestVerifier(t, pub)

	got, err := tv.authenticate(sign(t, key, testKid, goodClaims()))
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	// The id is sub, never anything from user_metadata.
	if got != testUserA {
		t.Errorf("user id = %s, want %s", got, testUserA)
	}
}

func TestJWTVerifierRefuses(t *testing.T) {
	key, pub := newKey(t, testKid)
	otherKey, _ := newKey(t, testKid)

	// with returns goodClaims after change has edited them.
	with := func(change func(jwt.MapClaims)) jwt.MapClaims {
		c := goodClaims()
		change(c)
		return c
	}
	good := sign(t, key, testKid, goodClaims())
	parts := strings.Split(good, ".")

	// The same header and signature around another user's id.
	otherPayload, err := json.Marshal(with(func(c jwt.MapClaims) { c["sub"] = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb" }))
	if err != nil {
		t.Fatal(err)
	}
	tampered := parts[0] + "." + base64.RawURLEncoding.EncodeToString(otherPayload) + "." + parts[2]

	// "alg": "none": no signature at all.
	noneTok := jwt.NewWithClaims(jwt.SigningMethodNone, goodClaims())
	noneTok.Header["kid"] = testKid
	none, err := noneTok.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}

	// HS256 with the public key as the "shared secret". The public key is
	// public, so a verifier that let the token choose its algorithm would
	// accept this forgery.
	point, err := key.PublicKey.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	hsTok := jwt.NewWithClaims(jwt.SigningMethodHS256, goodClaims())
	hsTok.Header["kid"] = testKid
	hs256, err := hsTok.SignedString(point)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		token string
		// noFetch: refused before any key is looked up, so the key set
		// must not have been fetched. That is how the algorithm allow-list
		// shows: the library would refuse these two for other reasons too.
		noFetch bool
	}{
		{name: "not a token", token: "not.a.token", noFetch: true},
		{name: "payload changed after signing", token: tampered},
		{name: "signed by another key under the same kid", token: sign(t, otherKey, testKid, goodClaims())},
		{name: "a kid the project doesn't publish", token: sign(t, key, "unknown-key", goodClaims())},
		{name: "no kid", token: sign(t, key, "", goodClaims()), noFetch: true},
		{name: "alg none", token: none, noFetch: true},
		{name: "HS256 with the public key as secret", token: hs256, noFetch: true},
		{name: "expired a minute ago", token: sign(t, key, testKid, with(func(c jwt.MapClaims) { c["exp"] = testNow.Add(-time.Minute).Unix() }))},
		{name: "no exp", token: sign(t, key, testKid, with(func(c jwt.MapClaims) { delete(c, "exp") }))},
		{name: "not valid yet", token: sign(t, key, testKid, with(func(c jwt.MapClaims) { c["nbf"] = testNow.Add(time.Hour).Unix() }))},
		{name: "another project's issuer", token: sign(t, key, testKid, with(func(c jwt.MapClaims) { c["iss"] = "https://other.example/auth/v1" }))},
		{name: "no issuer", token: sign(t, key, testKid, with(func(c jwt.MapClaims) { delete(c, "iss") }))},
		{name: "another audience", token: sign(t, key, testKid, with(func(c jwt.MapClaims) { c["aud"] = "anon" }))},
		{name: "no audience", token: sign(t, key, testKid, with(func(c jwt.MapClaims) { delete(c, "aud") }))},
		{name: "sub is not a uuid", token: sign(t, key, testKid, with(func(c jwt.MapClaims) { c["sub"] = "someone" }))},
		{name: "sub without dashes", token: sign(t, key, testKid, with(func(c jwt.MapClaims) { c["sub"] = strings.ReplaceAll(testUserA, "-", "") }))},
		{name: "no sub", token: sign(t, key, testKid, with(func(c jwt.MapClaims) { delete(c, "sub") }))},
		{name: "role anon", token: sign(t, key, testKid, with(func(c jwt.MapClaims) { c["role"] = "anon" }))},
		{name: "role service_role", token: sign(t, key, testKid, with(func(c jwt.MapClaims) { c["role"] = "service_role" }))},
		{name: "no role", token: sign(t, key, testKid, with(func(c jwt.MapClaims) { delete(c, "role") }))},
		{name: "anonymous user", token: sign(t, key, testKid, with(func(c jwt.MapClaims) { c["is_anonymous"] = true }))},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tv := newTestVerifier(t, pub)
			id, err := tv.authenticate(tc.token)
			if err == nil {
				t.Fatalf("accepted as user %s; want an error", id)
			}
			// Each of these is a refusal (401), not an outage (503).
			if errors.Is(err, ErrAuthUnavailable) {
				t.Errorf("err = %v; want a refusal, not ErrAuthUnavailable", err)
			}
			if tc.noFetch && tv.jwks.hits != 0 {
				t.Errorf("the key set was fetched %d times; want refused before any key lookup", tv.jwks.hits)
			}
		})
	}
}

func TestJWTVerifierLeeway(t *testing.T) {
	key, pub := newKey(t, testKid)
	tv := newTestVerifier(t, pub)

	// Expired 10 seconds ago by our clock: inside the 30-second leeway.
	c := goodClaims()
	c["exp"] = testNow.Add(-10 * time.Second).Unix()
	if _, err := tv.authenticate(sign(t, key, testKid, c)); err != nil {
		t.Errorf("a token 10 s past exp was refused: %v", err)
	}
}

func TestJWTVerifierReadsTheHeader(t *testing.T) {
	key, pub := newKey(t, testKid)
	tv := newTestVerifier(t, pub)
	token := sign(t, key, testKid, goodClaims())

	for name, header := range map[string]string{
		"no header":        "",
		"another scheme":   "Basic " + token,
		"scheme only":      "Bearer",
		"empty token":      "Bearer ",
		"token, no scheme": token,
	} {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/anything", nil)
			if header != "" {
				r.Header.Set("Authorization", header)
			}
			if id, err := tv.Authenticate(r); err == nil {
				t.Errorf("accepted as user %s; want an error", id)
			}
		})
	}
	if tv.jwks.hits != 0 {
		t.Errorf("the key set was fetched %d times for requests with no token; want 0", tv.jwks.hits)
	}

	// The scheme's case doesn't matter (RFC 9110).
	r := httptest.NewRequest(http.MethodGet, "/api/anything", nil)
	r.Header.Set("Authorization", "bearer "+token)
	if _, err := tv.Authenticate(r); err != nil {
		t.Errorf(`"bearer <token>" was refused: %v`, err)
	}
}

func TestKeySetFetchesOnce(t *testing.T) {
	key, pub := newKey(t, testKid)
	tv := newTestVerifier(t, pub)
	token := sign(t, key, testKid, goodClaims())

	if tv.jwks.hits != 0 {
		t.Fatalf("fetched %d times before any request; want 0", tv.jwks.hits)
	}
	for range 3 {
		if _, err := tv.authenticate(token); err != nil {
			t.Fatal(err)
		}
	}
	if tv.jwks.hits != 1 {
		t.Errorf("fetched %d times for 3 requests; want 1", tv.jwks.hits)
	}
}

func TestKeySetUnavailable(t *testing.T) {
	key, pub := newKey(t, testKid)
	tv := newTestVerifier(t, pub)
	tv.jwks.down = true
	token := sign(t, key, testKid, goodClaims())

	// No keys were ever fetched: an outage, which RequireUser answers 503.
	if _, err := tv.authenticate(token); !errors.Is(err, ErrAuthUnavailable) {
		t.Fatalf("err = %v, want ErrAuthUnavailable", err)
	}
	// A second request straight away doesn't hammer Supabase.
	if _, err := tv.authenticate(token); !errors.Is(err, ErrAuthUnavailable) {
		t.Fatalf("err = %v, want ErrAuthUnavailable", err)
	}
	if tv.jwks.hits != 1 {
		t.Errorf("fetched %d times within the retry interval; want 1", tv.jwks.hits)
	}

	// Supabase is back; after the short retry interval it works.
	tv.jwks.down = false
	tv.now = tv.now.Add(jwksRetryNoKeys)
	if _, err := tv.authenticate(token); err != nil {
		t.Errorf("after Supabase came back: %v", err)
	}
}

func TestKeySetRotation(t *testing.T) {
	oldKey, oldPub := newKey(t, "old-key")
	newPriv, newPub := newKey(t, "new-key")
	tv := newTestVerifier(t, oldPub)

	if _, err := tv.authenticate(sign(t, oldKey, "old-key", goodClaims())); err != nil {
		t.Fatal(err)
	}

	// Supabase starts signing with a new key. Its first token arrives
	// seconds after our fetch: too soon to fetch again, so it is refused
	// (as a refusal, not an outage).
	tv.jwks.keys = []jwk{oldPub, newPub}
	newToken := sign(t, newPriv, "new-key", goodClaims())
	tv.now = tv.now.Add(10 * time.Second)
	if _, err := tv.authenticate(newToken); err == nil || errors.Is(err, ErrAuthUnavailable) {
		t.Fatalf("err = %v, want a refusal", err)
	}
	if tv.jwks.hits != 1 {
		t.Fatalf("fetched %d times; an unknown kid inside the retry interval must not fetch", tv.jwks.hits)
	}

	// After the retry interval the unknown kid causes one fetch.
	tv.now = tv.now.Add(jwksRetry)
	if _, err := tv.authenticate(newToken); err != nil {
		t.Fatalf("the new key's token after a refetch: %v", err)
	}
	if tv.jwks.hits != 2 {
		t.Errorf("fetched %d times, want 2", tv.jwks.hits)
	}

	// The old key is revoked. Once the keys are refreshed, its tokens stop
	// working, even ones that haven't expired.
	tv.jwks.keys = []jwk{newPub}
	tv.now = tv.now.Add(jwksMaxAge)
	c := goodClaims()
	c["exp"] = tv.now.Add(time.Hour).Unix()
	if _, err := tv.authenticate(sign(t, oldKey, "old-key", c)); err == nil {
		t.Error("a token signed by a revoked key was accepted after the refresh")
	}
}

func TestKeySetKeepsKeysWhenRefreshFails(t *testing.T) {
	key, pub := newKey(t, testKid)
	tv := newTestVerifier(t, pub)

	c := goodClaims()
	c["exp"] = testNow.Add(24 * time.Hour).Unix()
	token := sign(t, key, testKid, c)
	if _, err := tv.authenticate(token); err != nil {
		t.Fatal(err)
	}

	// The keys are now old and Supabase is down. People stay logged in.
	tv.jwks.down = true
	tv.now = tv.now.Add(jwksMaxAge + time.Second)
	if _, err := tv.authenticate(token); err != nil {
		t.Fatalf("with old keys and Supabase down: %v", err)
	}
	if tv.jwks.hits != 2 {
		t.Fatalf("fetched %d times, want 2 (the first, and one failed refresh)", tv.jwks.hits)
	}
	// ...and the failed refresh isn't repeated on every request.
	if _, err := tv.authenticate(token); err != nil {
		t.Fatal(err)
	}
	if tv.jwks.hits != 2 {
		t.Errorf("fetched %d times, want still 2", tv.jwks.hits)
	}
}

func TestKeySetSkipsKeysItCannotUse(t *testing.T) {
	key, pub := newKey(t, testKid)
	_, encKey := newKey(t, "enc-key")
	encKey.Use = "enc"
	_, offCurve := newKey(t, "off-curve")
	offCurve.Y = offCurve.X

	unusable := []jwk{
		{Kty: "RSA", Kid: "rsa-key"},
		{Kty: "EC", Crv: "P-384", Kid: "p384-key", X: pub.X, Y: pub.Y},
		{Kty: "EC", Crv: "P-256", Kid: "short", X: "AAAA", Y: "AAAA"},
		{Kty: "EC", Crv: "P-256", X: pub.X, Y: pub.Y}, // no kid
		encKey,
		offCurve,
	}

	// Among unusable keys, the good one still works.
	tv := newTestVerifier(t, append(unusable, pub)...)
	if _, err := tv.authenticate(sign(t, key, testKid, goodClaims())); err != nil {
		t.Errorf("the usable key among unusable ones: %v", err)
	}

	// With only unusable keys there is nothing to verify against.
	tv = newTestVerifier(t, unusable...)
	if _, err := tv.authenticate(sign(t, key, testKid, goodClaims())); !errors.Is(err, ErrAuthUnavailable) {
		t.Errorf("err = %v, want ErrAuthUnavailable", err)
	}
}

func TestKeySetRefusesAHugeDocument(t *testing.T) {
	// The document holds the key that signed vectorJWT, so only its size
	// can be the reason to refuse it.
	huge := roundTripFunc(func(*http.Request) (*http.Response, error) {
		rec := httptest.NewRecorder()
		_, _ = io.WriteString(rec, `{"keys":[{"kty":"EC","crv":"P-256","kid":"`+vectorJWKKid+
			`","x":"`+vectorJWKX+`","y":"`+vectorJWKY+`"}],"padding":"`+strings.Repeat("x", maxJWKSBody)+`"}`)
		return rec.Result(), nil
	})
	v, err := NewJWTVerifier(testSupabaseURL, &http.Client{Transport: huge})
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, "/api/anything", nil)
	r.Header.Set("Authorization", "Bearer "+vectorJWT)
	if _, err := v.Authenticate(r); !errors.Is(err, ErrAuthUnavailable) {
		t.Errorf("err = %v, want ErrAuthUnavailable", err)
	}
}

// roundTripFunc lets a function be an http.Client's transport.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestNewJWTVerifierChecksTheURL(t *testing.T) {
	client := &http.Client{}
	for _, u := range []string{
		"",
		"project.example",
		"http://project.example", // not https
		"https://",
		"https://project.example/auth/v1", // the project's URL, not Auth's
		"https://project.example?x=1",
		"https://user:pass@project.example",
		"postgres://project.example",
	} {
		if _, err := NewJWTVerifier(u, client); err == nil {
			t.Errorf("NewJWTVerifier(%q) = nil error, want an error", u)
		}
	}
	for _, u := range []string{"https://project.example", "https://project.example/"} {
		v, err := NewJWTVerifier(u, client)
		if err != nil {
			t.Errorf("NewJWTVerifier(%q): %v", u, err)
			continue
		}
		if v.keys.url != testJWKSURL {
			t.Errorf("NewJWTVerifier(%q) fetches keys from %s, want %s", u, v.keys.url, testJWKSURL)
		}
	}
	if _, err := NewJWTVerifier(testSupabaseURL, nil); err == nil {
		t.Error("NewJWTVerifier with no http client = nil error, want an error")
	}
}
