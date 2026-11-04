package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// Pinger checks that the database answers. *db.Queries implements it; tests
// use a fake.
type Pinger interface {
	Ping(ctx context.Context) (int32, error)
}

// healthTimeout bounds the database check. On a fresh instance it also covers
// opening the first connection: the dial, TLS and authentication with Supabase.
const healthTimeout = 3 * time.Second

// handleHealth reports whether the server can reach the database, using one
// query that touches no table (internal/db/queries/health.sql).
//
// Don't use it as a Cloud Run liveness probe: a database outage would then
// restart containers that are working fine.
func handleHealth(db Pinger, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if _, err := db.Ping(ctx); err != nil {
			// The cause goes to the log only; this endpoint is public.
			slog.ErrorContext(ctx, "health check: database unavailable", "err", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("database unavailable"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
}
