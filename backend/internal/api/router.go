// Package api is the HTTP layer: routing, handlers and middleware.
// It translates HTTP to and from the rest of the app and holds no domain logic.
package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter builds the application's HTTP handler.
func NewRouter() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)

	// Not the conventional /healthz: Cloud Run reserves some paths ending in
	// "z" and never forwards them to the container.
	r.Get("/api/health", handleHealth)

	return r
}
