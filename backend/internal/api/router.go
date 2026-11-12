// Package api is the HTTP layer: routing, handlers and middleware.
// It translates HTTP to and from the rest of the app and holds no domain logic.
package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter builds the application's HTTP handler. Everything under /api/ is
// the API; every other GET or HEAD goes to frontend, the embedded app
// (internal/web).
//
// db answers the health check, users is the way to user data, auth says
// which user a request is for, signUpHook answers Supabase Auth's "before
// user created" hook, and cfg is what the page is told about Supabase. In
// the server, db and users are the same *db.Store.
func NewRouter(db Pinger, users UserStore, auth Authenticator, signUpHook http.Handler, cfg PublicConfig, frontend http.Handler) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(securityHeaders(cfg.supabaseOrigin))

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

		// Public: the page asks before anyone is signed in.
		r.Get("/config", handleConfig(cfg))

		// Called by Supabase Auth, not by a browser, so it can't be behind
		// RequireUser: the handler checks Supabase's signature itself, before
		// anything else (hook.go). It touches no user data.
		r.Post("/hooks/before-user-created", signUpHook.ServeHTTP)

		// Every route that touches user data goes in this group, behind
		// RequireUser: without a valid token nothing in it runs.
		r.Group(func(r chi.Router) {
			r.Use(RequireUser(auth))

			r.Get("/me", handleMe(users))
		})
	})

	r.Get("/*", frontend.ServeHTTP)
	r.Head("/*", frontend.ServeHTTP)

	return r
}
