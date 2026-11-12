package api

import "net/http"

// contentSecurityPolicy lets the page load only its own files, and talk only
// to its own origin and to one other: the Supabase project, because the
// browser calls Supabase Auth itself to sign up, log in and refresh its
// token (docs/tech-stack.md §4). That is connect-src, which governs fetch;
// nothing may be loaded from there. The built page has no inline script or
// style, so none is allowed. style-src stays: the graph spike measured that
// Recharts sets styles through the DOM, which this policy allows
// (docs/tech-stack.md §8.3). The self-hosted font needs nothing: fonts fall
// under default-src.
func contentSecurityPolicy(supabaseOrigin string) string {
	return "default-src 'self'; script-src 'self'; style-src 'self'; " +
		"img-src 'self' data:; connect-src 'self' " + supabaseOrigin + "; object-src 'none'; base-uri 'none'; " +
		"frame-ancestors 'none'; form-action 'self'"
}

// securityHeaders sets the browser protections on every response, API and
// page alike. No HSTS: run.app is under .app, which browsers already force to
// HTTPS (it is on the HSTS preload list as a whole). Add it with a custom
// domain.
func securityHeaders(supabaseOrigin string) func(http.Handler) http.Handler {
	// Built once: the policy is the same for every response.
	csp := contentSecurityPolicy(supabaseOrigin)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("Content-Security-Policy", csp)
			// Never guess a Content-Type: a text file is never run as a script.
			h.Set("X-Content-Type-Options", "nosniff")
			// Links out of the app don't tell the other site where they came from.
			h.Set("Referrer-Policy", "no-referrer")
			h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
			// A page this app opens, or that opens it, gets no handle on its window.
			h.Set("Cross-Origin-Opener-Policy", "same-origin")
			next.ServeHTTP(w, r)
		})
	}
}

// noStore keeps API responses out of every cache, browser and proxy: they
// will carry health data.
func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
