package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

// stubAuthenticator is the dev-only stand-in for auth until Slice 2: it
// returns a fixed user id, or err. It lives in a test file on purpose: Go
// never compiles _test.go files into a binary, so it can't reach production.
type stubAuthenticator struct {
	id  pgtype.UUID
	err error
}

func (s stubAuthenticator) Authenticate(*http.Request) (pgtype.UUID, error) {
	return s.id, s.err
}

// stubUserID is a hardcoded id: 11111111-1111-1111-1111-111111111111.
var stubUserID = pgtype.UUID{
	Bytes: [16]byte{0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11},
	Valid: true,
}

func TestRequireUser(t *testing.T) {
	tests := []struct {
		name       string
		auth       stubAuthenticator
		wantStatus int
		wantCalled bool
	}{
		{name: "authenticated", auth: stubAuthenticator{id: stubUserID}, wantStatus: http.StatusOK, wantCalled: true},
		{name: "not authenticated", auth: stubAuthenticator{err: errors.New("no token")}, wantStatus: http.StatusUnauthorized},
		// An Authenticator bug that returns no id and no error must not let
		// the request through as nobody.
		{name: "no id and no error", auth: stubAuthenticator{}, wantStatus: http.StatusUnauthorized},
		// "We couldn't check" is an outage, not a refusal, even when wrapped.
		{name: "authentication unavailable", auth: stubAuthenticator{err: fmt.Errorf("verify token: %w", ErrAuthUnavailable)}, wantStatus: http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			var gotID pgtype.UUID
			var gotOK bool
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				gotID, gotOK = UserID(r.Context())
			})

			rec := httptest.NewRecorder()
			RequireUser(tt.auth)(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/anything", nil))

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if called != tt.wantCalled {
				t.Fatalf("next handler called = %v, want %v", called, tt.wantCalled)
			}
			if called && (!gotOK || gotID != stubUserID) {
				t.Errorf("UserID = %s, %v; want %s, true", gotID, gotOK, stubUserID)
			}
		})
	}
}

func TestUserIDWithoutRequireUser(t *testing.T) {
	if id, ok := UserID(httptest.NewRequest(http.MethodGet, "/", nil).Context()); ok {
		t.Errorf("UserID outside RequireUser = %s, true; want false", id)
	}
}
