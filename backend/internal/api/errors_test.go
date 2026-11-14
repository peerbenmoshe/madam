package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// decodeAPIError checks that rec is an error in the API's one shape
// (docs/tech-stack.md §3) and returns it: the given status, JSON, never
// cached, and a body holding "error" with a code, a message and at most a
// field. Any other key fails, so nothing can ride along unnoticed.
func decodeAPIError(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int) apiError {
	t.Helper()
	if rec.Code != wantStatus {
		t.Errorf("status = %d, want %d", rec.Code, wantStatus)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var body errorBody
	dec := json.NewDecoder(bytes.NewReader(rec.Body.Bytes()))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		t.Fatalf("body %q is not the API's error shape: %v", rec.Body.String(), err)
	}
	if body.Error.Code == "" || body.Error.Message == "" {
		t.Errorf("body %q: an error needs a code and a message", rec.Body.String())
	}
	return body.Error
}

// Every error the API itself answers with has the same shape, whichever
// layer refused: the router, the auth middleware or a handler.
func TestAPIErrorsShareOneShape(t *testing.T) {
	tests := []struct {
		name       string
		users      UserStore
		auth       Authenticator
		method     string
		path       string
		wantStatus int
		wantCode   string
	}{
		{"an unknown path", nil, noAuth, http.MethodGet, "/api/nope", http.StatusNotFound, codeNotFound},
		{"an unknown path, POST", nil, noAuth, http.MethodPost, "/api/nope", http.StatusNotFound, codeNotFound},
		{"no valid token", nil, noAuth, http.MethodGet, "/api/me", http.StatusUnauthorized, codeUnauthorized},
		{
			"the token couldn't be checked", nil,
			stubAuthenticator{err: fmt.Errorf("verify token: %w", ErrAuthUnavailable)},
			http.MethodGet, "/api/me", http.StatusServiceUnavailable, codeAuthUnavailable,
		},
		{
			"the database is down", failingStore{}, stubAuthenticator{id: stubUserID},
			http.MethodGet, "/api/me", http.StatusInternalServerError, codeInternal,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := NewRouter(fakeDB{}, tc.users, tc.auth, noHook, testConfig, fakeFrontend)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))

			got := decodeAPIError(t, rec, tc.wantStatus)
			if got.Code != tc.wantCode {
				t.Errorf("code = %q, want %q", got.Code, tc.wantCode)
			}
			if got.Field != "" {
				t.Errorf("field = %q, want none: this error is about no field", got.Field)
			}
			if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", cc)
			}
		})
	}
}

// A handler mounted without RequireUser answers in the same shape.
func TestHandlerErrorWithoutRequireUser(t *testing.T) {
	rec := httptest.NewRecorder()
	handleMe(failingStore{})(rec, httptest.NewRequest(http.MethodGet, "/api/me", nil))
	if got := decodeAPIError(t, rec, http.StatusInternalServerError); got.Code != codeInternal {
		t.Errorf("code = %q, want %q", got.Code, codeInternal)
	}
}

// field is left out of the JSON when an error has none, and present when
// it has one.
func TestWriteErrorField(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(t.Context(), rec, http.StatusUnprocessableEntity,
		apiError{Code: "date_in_future", Field: "cycle_start_date", Message: "cycle_start_date is after today"})
	want := `{"error":{"code":"date_in_future","field":"cycle_start_date","message":"cycle_start_date is after today"}}`
	if got := rec.Body.String(); got != want {
		t.Errorf("body = %s\nwant   %s", got, want)
	}

	rec = httptest.NewRecorder()
	writeError(t.Context(), rec, http.StatusNotFound, apiError{Code: codeNotFound, Message: "no such API path"})
	want = `{"error":{"code":"not_found","message":"no such API path"}}`
	if got := rec.Body.String(); got != want {
		t.Errorf("body = %s\nwant   %s", got, want)
	}
}

// A value that can't be encoded is our bug. The answer is still the API's
// error shape, not a half-written body.
func TestWriteJSONThatCannotEncode(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(t.Context(), rec, http.StatusOK, map[string]any{"bad": make(chan int)})
	if got := decodeAPIError(t, rec, http.StatusInternalServerError); got.Code != codeInternal {
		t.Errorf("code = %q, want %q", got.Code, codeInternal)
	}
}
