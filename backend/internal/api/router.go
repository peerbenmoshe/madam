// Package api is the HTTP layer: routing, handlers and middleware.
// It translates HTTP to and from the rest of the app and holds no domain logic.
package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Option adds optional routes inside /api.
type Option func(r chi.Router)

// WithBeforeUserCreatedHook mounts Supabase Auth's sign-up hook (SPIKE).
func WithBeforeUserCreatedHook(h http.Handler) Option {
	return func(r chi.Router) { r.Post("/hooks/before-user-created", h.ServeHTTP) }
}

// NewRouter builds the application's HTTP handler. Everything under /api/ is
// the API; every other GET or HEAD goes to frontend, the embedded app
// (internal/web).
func NewRouter(db Pinger, frontend http.Handler, opts ...Option) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(securityHeaders)

	r.Route("/api", func(r chi.Router) {
		r.Use(noStore)

		// An unknown API path is a plain 404, never the app's page: a client
		// expecting JSON must not get "200 text/html", and a mistyped route
		// must not look like success. chi gives a mounted router its parent's
		// NotFound handler when it has none of its own, so this one is set
		// explicitly, whatever the parent ever does.
		r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "not found", http.StatusNotFound)
		})

		// Not the conventional /healthz: Cloud Run reserves some paths ending
		// in "z" and never forwards them to the container.
		r.Get("/health", handleHealth(db, healthTimeout))

		for _, opt := range opts {
			opt(r)
		}
	})

	r.Get("/*", frontend.ServeHTTP)
	r.Head("/*", frontend.ServeHTTP)

	return r
}
