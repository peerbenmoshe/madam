package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
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
)

var vectorTime = time.Unix(1762862400, 0)

func vectorKey(t *testing.T) []byte {
	t.Helper()
	key, err := decodeWebhookSecret("v1,whsec_" + vectorKeyB64)
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
		{name: "6 minutes late", id: vectorID, ts: vectorTS, sigs: vectorSigEntry, body: vectorPayload, now: vectorTime.Add(6 * time.Minute), wantErr: true},
		{name: "6 minutes early", id: vectorID, ts: vectorTS, sigs: vectorSigEntry, body: vectorPayload, now: vectorTime.Add(-6 * time.Minute), wantErr: true},
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
}

func TestDecodeWebhookSecret(t *testing.T) {
	for _, s := range []string{"", "v1,", "whsec", "v1,whsec_", "v1,whsec_not base64!", "sk_" + vectorKeyB64} {
		if _, err := decodeWebhookSecret(s); err == nil {
			t.Errorf("decodeWebhookSecret(%q) = nil error, want an error", s)
		}
	}
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

func TestBeforeUserCreatedHook(t *testing.T) {
	now := time.Unix(1762862400, 0)
	tests := []struct {
		name         string
		body         string
		rejectStatus int
		unsigned     bool
		wantStatus   int
		wantBody     string
	}{
		{name: "right code", body: `{"user":{"user_metadata":{"invite_code":"open-sesame"}}}`, wantStatus: 200, wantBody: "{}"},
		{name: "wrong code", body: `{"user":{"user_metadata":{"invite_code":"open"}}}`, rejectStatus: 400, wantStatus: 400, wantBody: `"Invalid invite code."`},
		{name: "wrong code, refused with 200", body: `{"user":{"user_metadata":{"invite_code":"nope"}}}`, rejectStatus: 200, wantStatus: 200, wantBody: `"error"`},
		{name: "no metadata", body: `{"user":{"email":"a@example.com","user_metadata":{}}}`, rejectStatus: 400, wantStatus: 400, wantBody: `"error"`},
		{name: "code is not a string", body: `{"user":{"user_metadata":{"invite_code":7}}}`, rejectStatus: 400, wantStatus: 400, wantBody: `"error"`},
		{name: "unsigned", body: `{"user":{"user_metadata":{"invite_code":"open-sesame"}}}`, unsigned: true, wantStatus: 401},
		{name: "not JSON", body: `nope`, wantStatus: 400},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, err := NewBeforeUserCreatedHook("v1,whsec_"+vectorKeyB64, "open-sesame", tc.rejectStatus)
			if err != nil {
				t.Fatal(err)
			}
			h.now = func() time.Time { return now }

			req := signed(h.key, tc.body, now)
			if tc.unsigned {
				req.Header.Del("webhook-signature")
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Errorf("body = %q, want it to contain %q", rec.Body.String(), tc.wantBody)
			}
		})
	}
}

// The route is mounted inside /api only when main.go passes it.
func TestHookRoute(t *testing.T) {
	h, err := NewBeforeUserCreatedHook("v1,whsec_"+vectorKeyB64, "open-sesame", 400)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		router http.Handler
		want   int
	}{
		{"mounted", NewRouter(fakeDB{}, fakeFrontend, WithBeforeUserCreatedHook(h)), http.StatusUnauthorized},
		{"not mounted", NewRouter(fakeDB{}, fakeFrontend), http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/hooks/before-user-created", strings.NewReader("{}"))
			tc.router.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}
