package api

import "net/http"

// contentSecurityPolicy lets the page load only its own files and talk only
// to its own origin. The built page has no inline script or style, so none is
// allowed. Expected to change (docs/tech-stack.md §8.4): connect-src gains
// Supabase if the browser calls Supabase Auth itself (Slice 2). style-src
// stays: the graph spike measured that Recharts sets styles through the DOM,
// which this policy allows (docs/tech-stack.md §8.3).
const contentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self'; " +
	"img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; " +
	"frame-ancestors 'none'; form-action 'self'"

// securityHeaders sets the browser protections on every response, API and
// page alike. No HSTS: run.app is under .app, which browsers already force to
// HTTPS (it is on the HSTS preload list as a whole). Add it with a custom
// domain.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", contentSecurityPolicy)
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

// noStore keeps API responses out of every cache, browser and proxy: they
// will carry health data.
func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
