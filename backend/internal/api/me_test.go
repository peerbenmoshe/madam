package api

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/peerbenmoshe/madam/backend/internal/db"
)

// testStore connects to the database in MADAM_TEST_DATABASE_URL as the
// server does, or skips the test when it is unset (plain `make test`). Set
// but unusable is a failure, not a skip. The same rule as internal/db's
// tests (ping_test.go), repeated here because test helpers can't be shared
// across packages without a package of their own.
func testStore(t *testing.T) *db.Store {
	t.Helper()
	url := os.Getenv("MADAM_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("MADAM_TEST_DATABASE_URL is not set; run make test-integration")
	}
	pool, err := pgxpool.New(t.Context(), url)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	t.Cleanup(pool.Close)

	// As the owner, or any role that skips Row Level Security, this would
	// not be the server's situation.
	role, err := db.New(pool).CurrentRole(t.Context())
	if err != nil {
		t.Fatalf("CurrentRole: %v", err)
	}
	if role.RoleName != "madam_app" || role.IsSuperuser.Bool || role.BypassesRls.Bool {
		t.Fatalf("connected as %q: MADAM_TEST_DATABASE_URL must use the application role madam_app", role.RoleName)
	}
	return db.NewStore(pool)
}

// newUser returns a random user id, so test runs never collide.
func newUser(t *testing.T) pgtype.UUID {
	t.Helper()
	id := pgtype.UUID{Valid: true}
	if _, err := rand.Read(id.Bytes[:]); err != nil {
		t.Fatal(err)
	}
	return id
}

// tokenFor signs a Supabase-style access token for the user id.
func tokenFor(t *testing.T, key *ecdsa.PrivateKey, id pgtype.UUID) string {
	t.Helper()
	c := goodClaims()
	c["sub"] = id.String()
	return sign(t, key, testKid, c)
}

// getMe sends GET path with the bearer token (none if empty) and the extra
// headers, and returns the status and the decoded body of a 200.
func getMe(t *testing.T, router http.Handler, path, token string, headers map[string]string) (int, meResponse) {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, r)

	var body meResponse
	if rec.Code == http.StatusOK {
		if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
			t.Errorf("Cache-Control = %q, want no-store", cc)
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("body %q is not JSON: %v", rec.Body.String(), err)
		}
	}
	return rec.Code, body
}

// userExists reports whether id has a users row, asked as that user (anyone
// else would see nothing, whatever exists).
func userExists(t *testing.T, store *db.Store, id pgtype.UUID) bool {
	t.Helper()
	err := store.InUserTx(t.Context(), id, func(q *db.Queries) error {
		_, err := q.GetUser(t.Context(), id)
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false
	}
	if err != nil {
		t.Fatalf("GetUser(%s) as itself: %v", id, err)
	}
	return true
}

// TestIsolationThroughTheAPI is AC-16, part one (docs/spec-v1.md §12): given
// two accounts, no API request from user A can read or change user B's
// data. It runs the real router, the real token verifier and a real
// database, as the application role. Part two, that Row Level Security
// holds even when a query asks for another user's row, is internal/db's
// TestIsolation.
//
// The API offers no way to name a user: the id comes from the verified
// token only. So the test tries every place a request could smuggle
// another id in, and checks that the answer is still about the token's
// owner or a refusal, and that B's data is untouched.
func TestIsolationThroughTheAPI(t *testing.T) {
	store := testStore(t)
	key, pub := newKey(t, testKid)
	tv := newTestVerifier(t, pub)
	router := NewRouter(store, store, tv.JWTVerifier, fakeFrontend)

	a, b := newUser(t), newUser(t)
	tokenA, tokenB := tokenFor(t, key, a), tokenFor(t, key, b)

	t.Run("the first request creates the caller's own row, and only that", func(t *testing.T) {
		if userExists(t, store, a) || userExists(t, store, b) {
			t.Fatal("a test user already has a row")
		}
		status, me := getMe(t, router, "/api/me", tokenA, nil)
		if status != http.StatusOK {
			t.Fatalf("status = %d, want 200", status)
		}
		if me.ID != a.String() {
			t.Errorf("id = %s, want A (%s)", me.ID, a)
		}
		if me.TemperatureUnit != "celsius" || me.OnboardedAt != nil {
			t.Errorf("a new account = %+v, want celsius and not onboarded", me)
		}
		if !userExists(t, store, a) {
			t.Error("A has no row after A's request")
		}
		if userExists(t, store, b) {
			t.Error("B has a row after A's request")
		}
	})

	t.Run("a second request changes nothing", func(t *testing.T) {
		status, me := getMe(t, router, "/api/me", tokenA, nil)
		if status != http.StatusOK || me.ID != a.String() {
			t.Errorf("got %d, id %s; want 200, A", status, me.ID)
		}
	})

	t.Run("each user gets their own row", func(t *testing.T) {
		status, me := getMe(t, router, "/api/me", tokenB, nil)
		if status != http.StatusOK || me.ID != b.String() {
			t.Errorf("B: got %d, id %s; want 200, B (%s)", status, me.ID, b)
		}
		status, me = getMe(t, router, "/api/me", tokenA, nil)
		if status != http.StatusOK || me.ID != a.String() {
			t.Errorf("A: got %d, id %s; want 200, A (%s)", status, me.ID, a)
		}
	})

	// A is properly signed in and names B wherever a request has room for
	// a name. The answer stays A's.
	t.Run("A names B in the request and still gets A", func(t *testing.T) {
		tries := []struct {
			name    string
			path    string
			headers map[string]string
		}{
			{name: "query id", path: "/api/me?id=" + b.String()},
			{name: "query user_id", path: "/api/me?user_id=" + b.String()},
			{name: "query sub", path: "/api/me?sub=" + b.String()},
			{name: "header X-User-Id", path: "/api/me", headers: map[string]string{"X-User-Id": b.String()}},
			{name: "header X-Forwarded-User", path: "/api/me", headers: map[string]string{"X-Forwarded-User": b.String()}},
			{name: "cookie", path: "/api/me", headers: map[string]string{"Cookie": "user_id=" + b.String()}},
		}
		for _, tc := range tries {
			t.Run(tc.name, func(t *testing.T) {
				status, me := getMe(t, router, tc.path, tokenA, tc.headers)
				if status != http.StatusOK || me.ID != a.String() {
					t.Errorf("got %d, id %s; want 200 and A (%s)", status, me.ID, a)
				}
			})
		}
	})

	t.Run("no path leads to another user", func(t *testing.T) {
		for _, path := range []string{"/api/me/" + b.String(), "/api/users/" + b.String(), "/api/users"} {
			if status, _ := getMe(t, router, path, tokenA, nil); status != http.StatusNotFound {
				t.Errorf("%s: status = %d, want 404", path, status)
			}
		}
	})

	// A edits A's own token to say B. Only Supabase's key can sign for B.
	t.Run("A cannot turn A's token into B's", func(t *testing.T) {
		parts := strings.Split(tokenA, ".")
		claimsB := goodClaims()
		claimsB["sub"] = b.String()
		payload, err := json.Marshal(claimsB)
		if err != nil {
			t.Fatal(err)
		}
		forged := parts[0] + "." + base64.RawURLEncoding.EncodeToString(payload) + "." + parts[2]

		ownKey, _ := newKey(t, testKid)
		selfSigned := tokenFor(t, ownKey, b)

		// B's id only in user_metadata, which its own user can edit.
		claimsMeta := goodClaims()
		claimsMeta["sub"] = a.String()
		claimsMeta["user_metadata"] = map[string]any{"sub": b.String(), "id": b.String(), "user_id": b.String()}
		viaMetadata := sign(t, key, testKid, claimsMeta)

		for name, token := range map[string]string{
			"payload rewritten to B":        forged,
			"signed for B with A's own key": selfSigned,
		} {
			if status, me := getMe(t, router, "/api/me", token, nil); status != http.StatusUnauthorized {
				t.Errorf("%s: status = %d (id %s), want 401", name, status, me.ID)
			}
		}
		if status, me := getMe(t, router, "/api/me", viaMetadata, nil); status != http.StatusOK || me.ID != a.String() {
			t.Errorf("B's id in user_metadata: got %d, id %s; want 200 and A", status, me.ID)
		}
	})

	t.Run("no token, no data", func(t *testing.T) {
		if status, _ := getMe(t, router, "/api/me", "", nil); status != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", status)
		}
	})

	// After everything A tried, B's account is as B left it.
	t.Run("B's row is untouched", func(t *testing.T) {
		status, me := getMe(t, router, "/api/me", tokenB, nil)
		if status != http.StatusOK || me.ID != b.String() || me.TemperatureUnit != "celsius" || me.OnboardedAt != nil {
			t.Errorf("B: got %d, %+v", status, me)
		}
	})
}

// failingStore is a UserStore whose database is down.
type failingStore struct{}

func (failingStore) InUserTx(context.Context, pgtype.UUID, func(*db.Queries) error) error {
	return errors.New("connection refused: secret-host.example:5432")
}

func TestMeHidesDatabaseErrors(t *testing.T) {
	router := NewRouter(fakeDB{}, failingStore{}, stubAuthenticator{id: stubUserID}, fakeFrontend)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/me", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	// The cause is for the log, never for the response.
	if body := rec.Body.String(); strings.Contains(body, "secret-host") || strings.Contains(body, "refused") {
		t.Errorf("the response leaks the database error: %q", body)
	}
}

// A handler mounted without RequireUser must fail, never guess a user.
func TestMeWithoutRequireUser(t *testing.T) {
	rec := httptest.NewRecorder()
	handleMe(failingStore{})(rec, httptest.NewRequest(http.MethodGet, "/api/me", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}
