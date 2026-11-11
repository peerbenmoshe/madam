package api

// SPIKE (branch spike/invite-hook, never merged): Supabase Auth's "before user
// created" HTTP hook, to measure whether the invite code typed at sign-up
// reaches it (docs/build-order.md §3). It logs whole payloads, which is only
// acceptable because the spike signs up test addresses on madam-dev.

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// webhookTolerance is how far webhook-timestamp may be from our clock. The
// Standard Webhooks spec asks for "some allowable tolerance" against replays
// without naming a number; 5 minutes is the common choice.
const webhookTolerance = 5 * time.Minute

// maxHookBody caps the request body. Supabase's limit is 20 KB.
const maxHookBody = 64 << 10

var errBadSignature = errors.New("webhook signature invalid")

// BeforeUserCreatedHook answers Supabase's hook: {} to allow the sign-up, an
// error object to refuse it.
type BeforeUserCreatedHook struct {
	key          []byte // the HMAC key, decoded from v1,whsec_<base64>
	inviteCode   []byte
	rejectStatus int // HTTP status of a refusal; the spike measures 200 and 400
	now          func() time.Time
}

// NewBeforeUserCreatedHook takes the secret exactly as Supabase's dashboard
// shows it ("v1,whsec_<base64>").
func NewBeforeUserCreatedHook(secret, inviteCode string, rejectStatus int) (*BeforeUserCreatedHook, error) {
	key, err := decodeWebhookSecret(secret)
	if err != nil {
		return nil, err
	}
	if inviteCode == "" {
		return nil, errors.New("invite code is empty")
	}
	return &BeforeUserCreatedHook{key: key, inviteCode: []byte(inviteCode), rejectStatus: rejectStatus, now: time.Now}, nil
}

// decodeWebhookSecret strips Supabase's "v1," version prefix and the Standard
// Webhooks "whsec_" prefix, then decodes the standard base64 that remains.
func decodeWebhookSecret(secret string) ([]byte, error) {
	s := strings.TrimPrefix(secret, "v1,")
	s, ok := strings.CutPrefix(s, "whsec_")
	if !ok {
		return nil, errors.New(`webhook secret must start with "v1,whsec_" or "whsec_"`)
	}
	key, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("webhook secret: %w", err)
	}
	if len(key) == 0 {
		return nil, errors.New("webhook secret is empty")
	}
	return key, nil
}

// verifyWebhook checks a Standard Webhooks signature: HMAC-SHA256 over
// "id.timestamp.body", base64-encoded, matched against any "v1,<sig>" entry
// of the space-separated signature header.
func verifyWebhook(key []byte, id, timestamp, signatures string, body []byte, now time.Time) error {
	if id == "" || timestamp == "" || signatures == "" {
		return fmt.Errorf("%w: missing header", errBadSignature)
	}
	secs, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("%w: timestamp %q", errBadSignature, timestamp)
	}
	if d := now.Sub(time.Unix(secs, 0)); d > webhookTolerance || d < -webhookTolerance {
		return fmt.Errorf("%w: timestamp %s off", errBadSignature, d)
	}

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(id + "." + timestamp + ".")) // a hash.Hash's Write never fails
	mac.Write(body)
	want := mac.Sum(nil)

	for entry := range strings.FieldsSeq(signatures) {
		b64, ok := strings.CutPrefix(entry, "v1,")
		if !ok {
			continue // another scheme (e.g. v1a, asymmetric); not ours
		}
		got, err := base64.StdEncoding.DecodeString(b64)
		if err == nil && hmac.Equal(got, want) {
			return nil
		}
	}
	return fmt.Errorf("%w: no matching v1 signature", errBadSignature)
}

// hookEvent is the part of the payload the decision needs. The whole payload
// is logged too, because what else arrives is what the spike measures.
type hookEvent struct {
	User struct {
		Email        string          `json:"email"`
		AppMetadata  json.RawMessage `json:"app_metadata"`
		UserMetadata map[string]any  `json:"user_metadata"`
	} `json:"user"`
}

func (h *BeforeUserCreatedHook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := h.now()
	ctx := r.Context()

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxHookBody))
	if err != nil {
		slog.WarnContext(ctx, "hook: reading body", "err", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if err := verifyWebhook(h.key, r.Header.Get("webhook-id"), r.Header.Get("webhook-timestamp"),
		r.Header.Get("webhook-signature"), body, start); err != nil {
		slog.WarnContext(ctx, "hook: refused unsigned request", "err", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var ev hookEvent
	if err := json.Unmarshal(body, &ev); err != nil {
		slog.WarnContext(ctx, "hook: payload is not JSON", "err", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	code, _ := ev.User.UserMetadata["invite_code"].(string)
	allowed := subtle.ConstantTimeCompare([]byte(code), h.inviteCode) == 1

	slog.InfoContext(ctx, "hook: before-user-created",
		"allowed", allowed,
		"email", ev.User.Email,
		"app_metadata", string(ev.User.AppMetadata),
		"user_metadata", ev.User.UserMetadata,
		"webhook_id", r.Header.Get("webhook-id"),
		"payload", string(body),
		"took", time.Since(start).String())

	w.Header().Set("Content-Type", "application/json")
	if allowed {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
		return
	}
	w.WriteHeader(h.rejectStatus)
	_, _ = w.Write([]byte(`{"error":{"http_code":400,"message":"Invalid invite code."}}`))
}
