package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeFrontend stands in for internal/web, so a test can tell whether a
// request reached the app.
const frontendBody = "FRONTEND"

var fakeFrontend = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte(frontendBody))
})

// noAuth authenticates nobody, for routers whose tests send no token. Their
// user store is nil: no request gets past RequireUser to use it.
var noAuth = stubAuthenticator{err: errors.New("no token")}

// testConfig is the public configuration of the tests' made-up project.
var testConfig = func() PublicConfig {
	cfg, err := NewPublicConfig(testSupabaseURL, "sb_publishable_test-key_1")
	if err != nil {
		panic(err)
	}
	return cfg
}()

// noHook stands in for the sign-up hook in routers whose tests don't call it.
var noHook = http.NotFoundHandler()

// No path under /api/ may ever reach the frontend.
func TestAPIPathsNeverReachFrontend(t *testing.T) {
	tests := []struct {
		method     string
		path       string
		wantStatus int
	}{
		{http.MethodGet, "/api", http.StatusNotFound},
		{http.MethodGet, "/api/", http.StatusNotFound},
		{http.MethodGet, "/api/nope", http.StatusNotFound},
		{http.MethodGet, "/api/health/extra", http.StatusNotFound},
		{http.MethodGet, "/api/../index.html", http.StatusNotFound},
		{http.MethodHead, "/api/nope", http.StatusNotFound},
		{http.MethodPost, "/api/nope", http.StatusNotFound},
		{http.MethodPost, "/api/health", http.StatusMethodNotAllowed},
		{http.MethodGet, "/api/health", http.StatusOK},
		{http.MethodGet, "/api/config", http.StatusOK},
		{http.MethodPost, "/api/config", http.StatusMethodNotAllowed},
		{http.MethodGet, "/api/config/extra", http.StatusNotFound},
		// Behind RequireUser: without a valid token, 401 and nothing else.
		{http.MethodGet, "/api/me", http.StatusUnauthorized},
		{http.MethodGet, "/api/me/extra", http.StatusNotFound},
		{http.MethodPost, "/api/me", http.StatusMethodNotAllowed},
	}

	router := NewRouter(fakeDB{}, nil, noAuth, noHook, testConfig, fakeFrontend)
	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))

			if rec.Body.String() == frontendBody {
				t.Fatalf("reached the frontend")
			}
			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if got := rec.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", got)
			}
		})
	}
}

func TestOtherPathsReachFrontend(t *testing.T) {
	tests := []struct {
		method     string
		path       string
		wantStatus int
		wantApp    bool
	}{
		{http.MethodGet, "/", http.StatusOK, true},
		{http.MethodGet, "/day/2026-11-06", http.StatusOK, true},
		{http.MethodHead, "/", http.StatusOK, true},
		// Starts with "api" but isn't under /api/.
		{http.MethodGet, "/apiary", http.StatusOK, true},
		// The frontend only reads.
		{http.MethodPost, "/", http.StatusMethodNotAllowed, false},
		{http.MethodDelete, "/day/2026-11-06", http.StatusMethodNotAllowed, false},
	}

	router := NewRouter(fakeDB{}, nil, noAuth, noHook, testConfig, fakeFrontend)
	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			// HEAD reaches the handler, but the recorder keeps what it wrote,
			// so the body check works for it too.
			if reached := rec.Body.String() == frontendBody; reached != tc.wantApp {
				t.Errorf("reached frontend = %v, want %v", reached, tc.wantApp)
			}
		})
	}
}

func TestSecurityHeadersEverywhere(t *testing.T) {
	router := NewRouter(fakeDB{}, nil, noAuth, noHook, testConfig, fakeFrontend)
	for _, path := range []string{"/", "/api/health", "/api/nope"} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

		for header, want := range map[string]string{
			"Content-Security-Policy":    contentSecurityPolicy(testSupabaseURL),
			"X-Content-Type-Options":     "nosniff",
			"Referrer-Policy":            "no-referrer",
			"Cross-Origin-Opener-Policy": "same-origin",
		} {
			if got := rec.Header().Get(header); got != want {
				t.Errorf("GET %s: %s = %q, want %q", path, header, got, want)
			}
		}
	}
}
