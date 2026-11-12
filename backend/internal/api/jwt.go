package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// jwtLeeway is how far our clock may differ from Supabase's when checking a
// token's times.
const jwtLeeway = 30 * time.Second

// jwtAudience is the aud claim of every signed-in user's token.
const jwtAudience = "authenticated"

// JWTVerifier is the Authenticator for Supabase Auth's access tokens
// (docs/tech-stack.md §4). A token is accepted only if it was signed with
// ES256 by one of the project's published keys and its claims say it is a
// signed-in user's token for this project that hasn't expired.
type JWTVerifier struct {
	keys   *keySet
	parser *jwt.Parser
}

// supabaseClaims is what we read from a token. user_metadata is left out on
// purpose: its own user can edit it, so it is never evidence of anything.
type supabaseClaims struct {
	jwt.RegisteredClaims        // iss, aud, exp, sub and the other standard ones
	Role                 string `json:"role"`
	IsAnonymous          bool   `json:"is_anonymous"`
}

// NewJWTVerifier takes the Supabase project's URL
// ("https://<ref>.supabase.co") and the HTTP client used to fetch its public
// keys. No request is made here: the keys are fetched on first use.
func NewJWTVerifier(supabaseURL string, client *http.Client) (*JWTVerifier, error) {
	u, err := url.Parse(supabaseURL)
	if err != nil {
		return nil, fmt.Errorf("supabase url: %w", err)
	}
	if u.Scheme != "https" || u.Host == "" || u.User != nil ||
		(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf(`supabase url must look like "https://<ref>.supabase.co", got %q`, supabaseURL)
	}
	if client == nil {
		return nil, errors.New("jwt verifier: no http client")
	}

	// Supabase Auth lives under /auth/v1, and that address is the token's
	// iss claim.
	issuer := "https://" + u.Host + "/auth/v1"
	keys := &keySet{
		url:    issuer + "/.well-known/jwks.json",
		client: client,
		now:    time.Now,
	}
	return &JWTVerifier{
		keys: keys,
		parser: jwt.NewParser(
			// The only algorithm accepted, whatever the token's header
			// says. This is what refuses "none", and HS256 "signed" with
			// the public key as if it were a shared secret.
			jwt.WithValidMethods([]string{"ES256"}),
			jwt.WithIssuer(issuer),
			jwt.WithAudience(jwtAudience),
			jwt.WithExpirationRequired(),
			jwt.WithLeeway(jwtLeeway),
			// One clock for the key cache and the token's times, so tests
			// can move both.
			jwt.WithTimeFunc(func() time.Time { return keys.now() }),
		),
	}, nil
}

// Authenticate returns the user id in the request's bearer token. The error
// wraps ErrAuthUnavailable when the project's keys couldn't be fetched;
// every other error means the request carries no valid identity.
func (v *JWTVerifier) Authenticate(r *http.Request) (pgtype.UUID, error) {
	raw, err := bearerToken(r)
	if err != nil {
		return pgtype.UUID{}, err
	}

	var claims supabaseClaims
	// The parser checks the algorithm first, then calls this function for
	// the key, then checks the signature and the claims. kid is the id of
	// the key that signed the token.
	_, err = v.parser.ParseWithClaims(raw, &claims, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("token names no key (kid)")
		}
		return v.keys.key(r.Context(), kid)
	})
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("verify token: %w", err)
	}

	// Signed and current, but is it a signed-in person's token? Supabase
	// signs others with the same keys.
	if claims.Role != "authenticated" {
		return pgtype.UUID{}, fmt.Errorf("token role is %q, not authenticated", claims.Role)
	}
	if claims.IsAnonymous {
		return pgtype.UUID{}, errors.New("token is for an anonymous user")
	}

	// sub is the user's id. Only the canonical 36-character form: Scan
	// alone would also take 32 hex digits.
	var id pgtype.UUID
	if len(claims.Subject) != 36 {
		return pgtype.UUID{}, errors.New("token sub is not a uuid")
	}
	if err := id.Scan(claims.Subject); err != nil {
		return pgtype.UUID{}, errors.New("token sub is not a uuid")
	}
	return id, nil
}

// bearerToken reads "Authorization: Bearer <token>".
func bearerToken(r *http.Request) (string, error) {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return "", errors.New("no bearer token")
	}
	return token, nil
}
