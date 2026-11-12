package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConfigEndpoint(t *testing.T) {
	// No token: the page asks before anyone is signed in.
	router := NewRouter(fakeDB{}, nil, noAuth, noHook, testConfig, fakeFrontend)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/config", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body %q is not JSON: %v", rec.Body.String(), err)
	}
	want := map[string]string{
		"supabase_url":             testSupabaseURL,
		"supabase_publishable_key": "sb_publishable_test-key_1",
	}
	// Exactly these two: anything else in a public answer needs a decision.
	if len(got) != len(want) {
		t.Errorf("body = %v, want exactly %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
}

// The browser may talk to this project's Supabase and to nothing else, and
// may load nothing from it.
func TestCSPNamesTheSupabaseProject(t *testing.T) {
	router := NewRouter(fakeDB{}, nil, noAuth, noHook, testConfig, fakeFrontend)
	for _, path := range []string{"/", "/api/health", "/api/config", "/api/nope"} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		csp := rec.Header().Get("Content-Security-Policy")

		directives := map[string]string{}
		for _, d := range strings.Split(csp, ";") {
			name, value, _ := strings.Cut(strings.TrimSpace(d), " ")
			directives[name] = value
		}
		if got, want := directives["connect-src"], "'self' "+testSupabaseURL; got != want {
			t.Errorf("%s: connect-src = %q, want %q", path, got, want)
		}
		// Everywhere else the project must not appear: not as a source of
		// script, style, images or anything the default covers.
		for name, value := range directives {
			if name != "connect-src" && strings.Contains(value, "project.example") {
				t.Errorf("%s: %s names the Supabase project: %q", path, name, value)
			}
		}
		if directives["script-src"] != "'self'" || directives["default-src"] != "'self'" {
			t.Errorf("%s: script-src or default-src loosened: %q", path, csp)
		}
	}
}

func TestNewPublicConfig(t *testing.T) {
	const goodKey = "sb_publishable_Abc123_x-Y"

	if _, err := NewPublicConfig(testSupabaseURL, goodKey); err != nil {
		t.Errorf("a publishable key was refused: %v", err)
	}
	// A trailing slash on the URL is tidied away, so the CSP source and the
	// page's address for Supabase are the bare origin.
	cfg, err := NewPublicConfig(testSupabaseURL+"/", goodKey)
	if err != nil || cfg.supabaseOrigin != testSupabaseURL {
		t.Errorf("origin = %q, %v; want %q", cfg.supabaseOrigin, err, testSupabaseURL)
	}

	// The value is published to every visitor. Only a publishable key may
	// get through; above all, never a secret key.
	secretLooking := "sb_secret_Abc123SUPERSECRETvalue"
	for _, key := range []string{
		"",
		secretLooking,
		"sb_publishable_",
		"publishable_Abc123",
		" " + goodKey,
		goodKey + "\n",
		goodKey + " " + secretLooking,
		// A legacy key is a JWT. The legacy service_role key looks the same
		// as the legacy anon key, so neither is accepted.
		"eyJhbGciOiJIUzI1NiJ9.eyJyb2xlIjoic2VydmljZV9yb2xlIn0.c2ln",
	} {
		_, err := NewPublicConfig(testSupabaseURL, key)
		if err == nil {
			t.Errorf("NewPublicConfig accepted the key %q", key)
			continue
		}
		// The error goes to the startup log.
		if key != "" && strings.Contains(err.Error(), "SUPERSECRET") {
			t.Errorf("the error repeats the key: %v", err)
		}
	}

	for _, u := range []string{"", "http://project.example", "https://project.example/auth/v1", "project.example"} {
		if _, err := NewPublicConfig(u, goodKey); err == nil {
			t.Errorf("NewPublicConfig accepted the URL %q", u)
		}
	}
}
