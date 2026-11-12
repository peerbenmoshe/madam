package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// A test-only key and a signature computed for it by OpenSSL, not by this
// package (no official Standard Webhooks vector exists), so the Go code is
// checked against an independent implementation:
//
//	printf '%s.%s.%s' "$ID" "$TS" "$PAYLOAD" |
//	  openssl dgst -sha256 -mac HMAC -macopt hexkey:<key as hex> -binary | openssl base64 -A
const (
	vectorKeyB64   = "eBs3/wYeajOET79mGzCO5XqVh9aJsUWKDiPKeaQ1hOI="
	vectorID       = "msg_spike_vector_1"
	vectorTS       = "1762862400"
	vectorPayload  = `{"metadata":{"name":"before-user-created"},"user":{"user_metadata":{"invite_code":"x"}}}`
	vectorSigEntry = "v1,VUsKntdOTEalA/wt+eN/K5tzhBsSSLiso/z7EdSNqr4="

	// testHookSecret is in the dashboard's format, split so that no secret
	// scanner takes the test key for a real one.
	testHookSecret = "v1,whsec_" + vectorKeyB64
	testInviteCode = "open-sesame"
)

var vectorTime = time.Unix(1762862400, 0)

func vectorKey(t *testing.T) []byte {
	t.Helper()
	key, err := decodeWebhookSecret(testHookSecret)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func TestVerifyWebhook(t *testing.T) {
	key := vectorKey(t)
	tests := []struct {
		name    string
		id, ts  string
		sigs    string
		body    string
		now     time.Time
		wantErr bool
	}{
		{name: "OpenSSL vector", id: vectorID, ts: vectorTS, sigs: vectorSigEntry, body: vectorPayload, now: vectorTime},
		{name: "one of several entries", id: vectorID, ts: vectorTS, sigs: "v1a,abc v1,AAAA " + vectorSigEntry, body: vectorPayload, now: vectorTime},
		{name: "4 minutes late is within tolerance", id: vectorID, ts: vectorTS, sigs: vectorSigEntry, body: vectorPayload, now: vectorTime.Add(4 * time.Minute)},
		{name: "tampered body", id: vectorID, ts: vectorTS, sigs: vectorSigEntry, body: strings.Replace(vectorPayload, `"x"`, `"y"`, 1), now: vectorTime, wantErr: true},
		{name: "other id", id: "msg_other", ts: vectorTS, sigs: vectorSigEntry, body: vectorPayload, now: vectorTime, wantErr: true},
		{name: "other timestamp", id: vectorID, ts: "1762862401", sigs: vectorSigEntry, body: vectorPayload, now: vectorTime, wantErr: true},
		{name: "6 minutes late", id: vectorID, ts: vectorTS, sigs: vectorSigEntry, body: vectorPayload, now: vectorTime.Add(6 * time.Minute), wantErr: true},
		{name: "6 minutes early", id: vectorID, ts: vectorTS, sigs: vectorSigEntry, body: vectorPayload, now: vectorTime.Add(-6 * time.Minute), wantErr: true},
		{name: "timestamp is not a number", id: vectorID, ts: "yesterday", sigs: vectorSigEntry, body: vectorPayload, now: vectorTime, wantErr: true},
		{name: "missing signature", id: vectorID, ts: vectorTS, body: vectorPayload, now: vectorTime, wantErr: true},
		{name: "missing id", ts: vectorTS, sigs: vectorSigEntry, body: vectorPayload, now: vectorTime, wantErr: true},
		{name: "unversioned signature", id: vectorID, ts: vectorTS, sigs: strings.TrimPrefix(vectorSigEntry, "v1,"), body: vectorPayload, now: vectorTime, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := verifyWebhook(key, tc.id, tc.ts, tc.sigs, []byte(tc.body), tc.now)
			if gotErr := err != nil; gotErr != tc.wantErr {
				t.Fatalf("err = %v, want error: %v", err, tc.wantErr)
			}
			if err != nil && !errors.Is(err, errBadSignature) {
				t.Errorf("err = %v, want errBadSignature", err)
			}
		})
	}

	// Signed with another key: the vector's signature no longer matches.
	otherKey := append([]byte{}, key...)
	otherKey[0] ^= 1
	if err := verifyWebhook(otherKey, vectorID, vectorTS, vectorSigEntry, []byte(vectorPayload), vectorTime); err == nil {
		t.Error("a signature made with another key was accepted")
	}
}

func TestDecodeWebhookSecret(t *testing.T) {
	for _, s := range []string{"", "v1,", "whsec", "v1,whsec_", "v1,whsec_not base64!", "sk_" + vectorKeyB64, " " + testHookSecret, testHookSecret + " "} {
		_, err := decodeWebhookSecret(s)
		if err == nil {
			t.Errorf("decodeWebhookSecret(%q) = nil error, want an error", s)
			continue
		}
		// The error goes to the startup log: it must not carry the secret.
		if s != "" && len(s) > 12 && strings.Contains(err.Error(), vectorKeyB64) {
			t.Errorf("decodeWebhookSecret(%q): the error repeats the secret: %v", s, err)
		}
	}
	// With and without Supabase's version prefix, and with the trailing
	// newline a careless `echo` adds: Go's base64 decoder skips newlines, so
	// the key is the same one.
	want := vectorKey(t)
	for _, s := range []string{testHookSecret, strings.TrimPrefix(testHookSecret, "v1,"), testHookSecret + "\n"} {
		got, err := decodeWebhookSecret(s)
		if err != nil {
			t.Errorf("decodeWebhookSecret(%q): %v", s, err)
		} else if !bytes.Equal(got, want) {
			t.Errorf("decodeWebhookSecret(%q) gave another key", s)
		}
	}
}

func TestNewBeforeUserCreatedHook(t *testing.T) {
	if _, err := NewBeforeUserCreatedHook(testHookSecret, ""); err == nil {
		t.Error("an empty invite code was accepted")
	}
	if _, err := NewBeforeUserCreatedHook("", testInviteCode); err == nil {
		t.Error("an empty signing secret was accepted")
	}
	// A code stored with a stray newline or space could never be typed.
	for _, code := range []string{testInviteCode + "\n", " " + testInviteCode, testInviteCode + " "} {
		_, err := NewBeforeUserCreatedHook(testHookSecret, code)
		if err == nil {
			t.Errorf("the invite code %q was accepted", code)
		} else if strings.Contains(err.Error(), testInviteCode) {
			t.Errorf("the error repeats the invite code: %v", err)
		}
	}
	// A space inside the code is the operator's choice.
	if _, err := NewBeforeUserCreatedHook(testHookSecret, "four random words here"); err != nil {
		t.Errorf("a code with spaces inside: %v", err)
	}
}

// newTestHook is the hook with the test secret and code, at a fixed time.
func newTestHook(t *testing.T) *BeforeUserCreatedHook {
	t.Helper()
	h, err := NewBeforeUserCreatedHook(testHookSecret, testInviteCode)
	if err != nil {
		t.Fatal(err)
	}
	h.now = func() time.Time { return vectorTime }
	return h
}

// signed builds a hook request signed with key at time now, the way Supabase
// sends one.
func signed(key []byte, body string, now time.Time) *http.Request {
	ts := strconv.FormatInt(now.Unix(), 10)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("msg_test." + ts + "." + body))
	req := httptest.NewRequest(http.MethodPost, "/api/hooks/before-user-created", strings.NewReader(body))
	req.Header.Set("webhook-id", "msg_test")
	req.Header.Set("webhook-timestamp", ts)
	req.Header.Set("webhook-signature", "v1,"+base64.StdEncoding.EncodeToString(mac.Sum(nil)))
	return req
}

// A payload shaped like Supabase's, with the personal data a real one
// carries, so the logging test has something to find.
func payload(metadata string) string {
	return `{"metadata":{"uuid":"8a4b3f0e-0000-4000-8000-000000000001","time":"2026-11-12T07:00:00Z",` +
		`"name":"before-user-created","ip_address":"203.0.113.77"},` +
		`"user":{"id":"8a4b3f0e-0000-4000-8000-000000000002","email":"someone@example.com",` +
		`"app_metadata":{"provider":"email"},"user_metadata":` + metadata + `}}`
}

func TestBeforeUserCreatedHook(t *testing.T) {
	const refusal = `{"error":{"http_code":400,"message":"Invalid invite code."}}`
	tests := []struct {
		name       string
		body       string
		tamper     func(*http.Request) // makes the request one Supabase didn't sign
		wantStatus int
		wantBody   string
	}{
		{name: "right code", body: payload(`{"invite_code":"open-sesame"}`), wantStatus: 200, wantBody: "{}"},
		{name: "right code among other metadata", body: payload(`{"name":"A","invite_code":"open-sesame"}`), wantStatus: 200, wantBody: "{}"},

		// Refusals are HTTP 200 too: a 4xx would reach the person as a 500.
		{name: "wrong code", body: payload(`{"invite_code":"nope"}`), wantStatus: 200, wantBody: refusal},
		{name: "a prefix of the code", body: payload(`{"invite_code":"open"}`), wantStatus: 200, wantBody: refusal},
		{name: "the code and more", body: payload(`{"invite_code":"open-sesame!"}`), wantStatus: 200, wantBody: refusal},
		{name: "other case", body: payload(`{"invite_code":"Open-Sesame"}`), wantStatus: 200, wantBody: refusal},
		{name: "space around the code", body: payload(`{"invite_code":" open-sesame "}`), wantStatus: 200, wantBody: refusal},
		{name: "empty code", body: payload(`{"invite_code":""}`), wantStatus: 200, wantBody: refusal},
		{name: "no code", body: payload(`{}`), wantStatus: 200, wantBody: refusal},
		{name: "metadata is null", body: payload(`null`), wantStatus: 200, wantBody: refusal},
		{name: "code is a number", body: payload(`{"invite_code":7}`), wantStatus: 200, wantBody: refusal},
		{name: "code is a list holding the code", body: payload(`{"invite_code":["open-sesame"]}`), wantStatus: 200, wantBody: refusal},
		{name: "code under another name", body: payload(`{"inviteCode":"open-sesame"}`), wantStatus: 200, wantBody: refusal},
		{name: "no user", body: `{}`, wantStatus: 200, wantBody: refusal},

		// Not from Supabase: 401 before the code is looked at, right or not.
		{name: "no signature", body: payload(`{"invite_code":"open-sesame"}`),
			tamper: func(r *http.Request) { r.Header.Del("webhook-signature") }, wantStatus: 401},
		{name: "forged signature", body: payload(`{"invite_code":"open-sesame"}`),
			tamper: func(r *http.Request) { r.Header.Set("webhook-signature", "v1,"+vectorKeyB64) }, wantStatus: 401},
		{name: "signed five minutes and a second ago", body: payload(`{"invite_code":"open-sesame"}`),
			tamper: func(r *http.Request) {
				*r = *signed(vectorKey(t), payload(`{"invite_code":"open-sesame"}`), vectorTime.Add(-webhookTolerance-time.Second))
			}, wantStatus: 401},

		// Signed, but not something we can read: the hook failed.
		{name: "not JSON", body: `nope`, wantStatus: 400},
		{name: "user_metadata is a string", body: payload(`"open-sesame"`), wantStatus: 400},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHook(t)
			req := signed(h.key, tc.body, vectorTime)
			if tc.tamper != nil {
				tc.tamper(req)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if rec.Code != http.StatusOK {
				// A failure says nothing about the code or the payload.
				if b := rec.Body.String(); strings.Contains(b, "invite") || strings.Contains(b, "error\":") {
					t.Errorf("a %d answer has body %q", rec.Code, b)
				}
				return
			}
			if got := rec.Body.String(); got != tc.wantBody {
				t.Errorf("body = %q, want %q", got, tc.wantBody)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json (Supabase requires it)", ct)
			}
		})
	}
}

func TestHookRefusesAHugeBody(t *testing.T) {
	h := newTestHook(t)
	body := payload(`{"invite_code":"open-sesame","padding":"` + strings.Repeat("x", maxHookBody) + `"}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, signed(h.key, body, vectorTime))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// The hook sees an email address, an IP address and the invite code. None of
// them may reach the log, whatever the outcome.
func TestHookLogsNoPersonalData(t *testing.T) {
	var logged bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(old) })

	h := newTestHook(t)
	requests := map[string]*http.Request{
		"allowed":  signed(h.key, payload(`{"invite_code":"open-sesame"}`), vectorTime),
		"refused":  signed(h.key, payload(`{"invite_code":"a-wrong-guess"}`), vectorTime),
		"unsigned": httptest.NewRequest(http.MethodPost, "/api/hooks/before-user-created", strings.NewReader(payload(`{"invite_code":"open-sesame"}`))),
		"bad JSON": signed(h.key, `{"user":{"email":"someone@example.com","user_metadata":"open-sesame"}}`, vectorTime),
	}
	for name, req := range requests {
		h.ServeHTTP(httptest.NewRecorder(), req)
		if !strings.Contains(logged.String(), "hook:") {
			t.Fatalf("%s: nothing was logged, so this test checks nothing", name)
		}
	}

	out := logged.String()
	for what, secret := range map[string]string{
		"the invite code":   "open-sesame",
		"a guessed code":    "a-wrong-guess",
		"the email address": "someone@example.com",
		"the IP address":    "203.0.113.77",
		"the user's id":     "8a4b3f0e-0000-4000-8000-000000000002",
		"the signing key":   vectorKeyB64,
	} {
		if strings.Contains(out, secret) {
			t.Errorf("the log contains %s (%q):\n%s", what, secret, out)
		}
	}
	// What it should say instead.
	if !strings.Contains(out, "allowed=true") || !strings.Contains(out, "allowed=false") || !strings.Contains(out, "webhook_id=msg_test") {
		t.Errorf("the log lacks the outcome or the webhook id:\n%s", out)
	}
}

// The route is public (Supabase can't send a user's token), inside /api, and
// answers only signed POSTs.
func TestHookRoute(t *testing.T) {
	h := newTestHook(t)
	router := NewRouter(fakeDB{}, nil, noAuth, h, testConfig, fakeFrontend)
	const path = "/api/hooks/before-user-created"

	do := func(req *http.Request) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Body.String() == frontendBody {
			t.Errorf("%s %s reached the frontend", req.Method, req.URL.Path)
		}
		return rec
	}

	if rec := do(signed(h.key, payload(`{"invite_code":"open-sesame"}`), vectorTime)); rec.Code != http.StatusOK || rec.Body.String() != "{}" {
		t.Errorf("signed, right code: got %d %q, want 200 {}", rec.Code, rec.Body.String())
	}
	if rec := do(httptest.NewRequest(http.MethodPost, path, strings.NewReader("{}"))); rec.Code != http.StatusUnauthorized {
		t.Errorf("unsigned POST: status = %d, want 401", rec.Code)
	}
	if rec := do(httptest.NewRequest(http.MethodGet, path, nil)); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET: status = %d, want 405", rec.Code)
	}
	if rec := do(httptest.NewRequest(http.MethodPost, "/api/hooks/other", nil)); rec.Code != http.StatusNotFound {
		t.Errorf("another hook path: status = %d, want 404", rec.Code)
	}
}
