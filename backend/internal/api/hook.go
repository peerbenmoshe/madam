package api

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

// maxHookBody caps the request body. Supabase's own limit is 20 KB.
const maxHookBody = 64 << 10

// The two answers Supabase understands, both sent with HTTP 200
// (docs/tech-stack.md §4, measured in the invite-code spike). The hook's
// status says whether the hook worked; whether the sign-up may go ahead is
// in the body. The error's http_code and message become the status and
// message the person signing up sees. A 4xx from us would instead reach
// them as a 500 saying "Invalid payload sent to hook".
const (
	hookAllow  = `{}`
	hookRefuse = `{"error":{"http_code":400,"message":"Invalid invite code."}}`
)

var errBadSignature = errors.New("webhook signature invalid")

// BeforeUserCreatedHook answers Supabase Auth's "before user created" hook:
// a sign-up goes ahead only if it carries the invite code (docs/spec-v1.md
// §2). Supabase calls it for every new account, however it is started,
// except those made in the dashboard.
type BeforeUserCreatedHook struct {
	key      []byte            // the HMAC key, decoded from v1,whsec_<base64>
	codeHash [sha256.Size]byte // SHA-256 of the invite code
	now      func() time.Time
}

// NewBeforeUserCreatedHook takes the signing secret exactly as Supabase's
// dashboard shows it ("v1,whsec_<base64>") and the invite code.
func NewBeforeUserCreatedHook(secret, inviteCode string) (*BeforeUserCreatedHook, error) {
	key, err := decodeWebhookSecret(secret)
	if err != nil {
		return nil, err
	}
	if inviteCode == "" {
		return nil, errors.New("invite code is empty")
	}
	// The classic mistake when storing a secret: a trailing newline. The
	// comparison is exact, so every sign-up would then be refused, with
	// nothing to show why.
	if strings.TrimSpace(inviteCode) != inviteCode {
		return nil, errors.New("invite code starts or ends with a space or a newline")
	}
	return &BeforeUserCreatedHook{key: key, codeHash: sha256.Sum256([]byte(inviteCode)), now: time.Now}, nil
}

// decodeWebhookSecret strips Supabase's "v1," version prefix and the Standard
// Webhooks "whsec_" prefix, then decodes the standard base64 that remains.
// Go's decoder skips newlines, so a secret stored with a trailing one gives
// the same key. The errors never repeat the secret.
func decodeWebhookSecret(secret string) ([]byte, error) {
	s := strings.TrimPrefix(secret, "v1,")
	s, ok := strings.CutPrefix(s, "whsec_")
	if !ok {
		return nil, errors.New(`webhook secret must start with "v1,whsec_" or "whsec_"`)
	}
	key, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, errors.New("webhook secret is not base64 after its prefix (a stray space or newline?)")
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
		return fmt.Errorf("%w: timestamp is not a number", errBadSignature)
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

// hookEvent is the only part of the payload we read. The rest (the email,
// the signer-up's IP address) is personal data we have no use for here.
type hookEvent struct {
	User struct {
		UserMetadata map[string]any `json:"user_metadata"`
	} `json:"user"`
}

// ServeHTTP checks the signature first, so nothing about the invite code can
// be learned by anyone but Supabase, then answers allow or refuse.
//
// It never logs the code, the email, the IP address or the payload. A
// status other than 200 means "the hook failed", and Supabase then refuses
// the sign-up with its own 500: the safe direction for every failure here.
func (h *BeforeUserCreatedHook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := h.now()
	ctx := r.Context()

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxHookBody))
	if err != nil {
		slog.WarnContext(ctx, "hook: reading the body", "err", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if err := verifyWebhook(h.key, r.Header.Get("webhook-id"), r.Header.Get("webhook-timestamp"),
		r.Header.Get("webhook-signature"), body, start); err != nil {
		slog.WarnContext(ctx, "hook: refused a request that Supabase didn't sign", "err", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var ev hookEvent
	if err := json.Unmarshal(body, &ev); err != nil {
		// Not err itself: a JSON error can quote the payload.
		slog.WarnContext(ctx, "hook: a signed payload is not the JSON we expect")
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Compared as hashes, in constant time: neither the code's content nor
	// its length shows in how long this takes.
	code, _ := ev.User.UserMetadata["invite_code"].(string)
	got := sha256.Sum256([]byte(code))
	allowed := subtle.ConstantTimeCompare(got[:], h.codeHash[:]) == 1

	slog.InfoContext(ctx, "hook: before-user-created",
		"allowed", allowed,
		"webhook_id", r.Header.Get("webhook-id"),
		"took", h.now().Sub(start).String())

	answer := hookRefuse
	if allowed {
		answer = hookAllow
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, answer)
}
