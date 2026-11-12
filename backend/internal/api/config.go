package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
)

// PublicConfig is what the page must know to talk to Supabase Auth: which
// project, and that project's publishable key. Both are public by design.
// The page is the same build in every environment, so it asks the server
// (GET /api/config) instead of having them compiled in
// (docs/tech-stack.md §4).
type PublicConfig struct {
	supabaseOrigin string // "https://<ref>.supabase.co", nothing after the host
	publishableKey string
}

// publishableKeyPattern is the shape of a Supabase publishable key. The
// prefix is the point: Supabase's secret keys start "sb_secret_", and this
// value is handed to every visitor.
var publishableKeyPattern = regexp.MustCompile(`^sb_publishable_[A-Za-z0-9_-]+$`)

// NewPublicConfig checks both values. It refuses anything that isn't a
// publishable key, so a secret key set by mistake stops the server from
// starting instead of being published. The errors never repeat the key.
func NewPublicConfig(supabaseURL, publishableKey string) (PublicConfig, error) {
	origin, err := supabaseOrigin(supabaseURL)
	if err != nil {
		return PublicConfig{}, err
	}
	if publishableKey == "" {
		return PublicConfig{}, errors.New("publishable key is empty")
	}
	if !publishableKeyPattern.MatchString(publishableKey) {
		return PublicConfig{}, errors.New(`publishable key must start with "sb_publishable_" and hold only letters, digits, "_" and "-": ` +
			`it is sent to every browser, so a secret key ("sb_secret_…") must never be put here`)
	}
	return PublicConfig{supabaseOrigin: origin, publishableKey: publishableKey}, nil
}

// supabaseOrigin checks a Supabase project URL and returns it as
// "https://<host>", without a trailing slash.
func supabaseOrigin(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("supabase url: %w", err)
	}
	if u.Scheme != "https" || u.Host == "" || u.User != nil ||
		(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf(`supabase url must look like "https://<ref>.supabase.co", got %q`, raw)
	}
	return "https://" + u.Host, nil
}

// configResponse is the body of GET /api/config.
type configResponse struct {
	SupabaseURL            string `json:"supabase_url"`
	SupabasePublishableKey string `json:"supabase_publishable_key"`
}

// handleConfig answers GET /api/config. It needs no token: the page asks
// before anyone is signed in, and nothing in the answer is secret.
func handleConfig(c PublicConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(r.Context(), w, http.StatusOK, configResponse{
			SupabaseURL:            c.supabaseOrigin,
			SupabasePublishableKey: c.publishableKey,
		})
	}
}
