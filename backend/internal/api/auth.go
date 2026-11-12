package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
)

// Authenticator establishes which user a request is for. Slice 2's JWT
// verifier will implement it and be passed in from main.go. Until then only
// a test stub does (auth_test.go), and test files are never compiled into the
// server binary, so no build of the server can fake an identity.
type Authenticator interface {
	// Authenticate returns the requesting user's id, or an error if the
	// request carries no valid identity. The error is logged, never sent.
	Authenticate(r *http.Request) (pgtype.UUID, error)
}

// ErrAuthUnavailable is what an Authenticator returns (wrapped or not) when
// it couldn't check the request at all, for example because the identity
// provider's keys can't be fetched. It is an outage, not a refusal, so
// RequireUser answers 503 instead of 401.
var ErrAuthUnavailable = errors.New("authentication unavailable")

// userIDKey is the context key for the authenticated user id. An unexported
// type, so no other package can set or overwrite the value.
type userIDKey struct{}

// RequireUser is the middleware for every route that touches user data. It
// answers 401 unless a authenticates the request (503 if a couldn't check
// it), and otherwise puts the user id in the request's context, where UserID
// finds it. Everything downstream asks only "who is this request for?", so
// swapping the Authenticator in Slice 2 changes nothing else.
func RequireUser(a Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, err := a.Authenticate(r)
			if errors.Is(err, ErrAuthUnavailable) {
				slog.WarnContext(r.Context(), "request could not be authenticated", "err", err)
				http.Error(w, "authentication unavailable", http.StatusServiceUnavailable)
				return
			}
			if err != nil || !id.Valid {
				slog.InfoContext(r.Context(), "request not authenticated", "err", err)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), userIDKey{}, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserID returns the user id RequireUser put in ctx. ok is false outside
// RequireUser, which means the route was mounted without it: a programming
// error, to be answered with a 500, never by guessing a user.
func UserID(ctx context.Context) (id pgtype.UUID, ok bool) {
	id, ok = ctx.Value(userIDKey{}).(pgtype.UUID)
	return id, ok
}
