package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/peerbenmoshe/madam/backend/internal/db"
)

// UserStore is the handlers' only way to user data: one transaction in which
// Row Level Security sees userID as the requesting user. *db.Store
// implements it.
type UserStore interface {
	InUserTx(ctx context.Context, userID pgtype.UUID, fn func(*db.Queries) error) error
}

// dbTimeout bounds a handler's database work. On a fresh instance it also
// covers opening the first connection. It is well under the server's 10 s
// write timeout, so a slow database becomes our error response, not a cut
// connection.
const dbTimeout = 5 * time.Second

// meResponse is the signed-in user's own account, as the page needs it to
// choose between onboarding and the app.
type meResponse struct {
	ID              string `json:"id"`
	TemperatureUnit string `json:"temperature_unit"`
	// OnboardedAt is null until onboarding is finished (Slice 3).
	OnboardedAt *time.Time `json:"onboarded_at"`
}

// handleMe answers GET /api/me with the requesting user's own row, creating
// it first if this is the account's first request (docs/tech-stack.md §4).
// Supabase Auth holds the identity; this row is where our data hangs from.
//
// The id comes from the verified token and nowhere else: nothing in the
// request can name another user.
func handleMe(store UserStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
		defer cancel()

		id, ok := UserID(ctx)
		if !ok {
			// The route was mounted without RequireUser: our bug.
			slog.ErrorContext(ctx, "me: no user id in the request's context")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		var user db.User
		err := store.InUserTx(ctx, id, func(q *db.Queries) error {
			if err := q.EnsureUser(ctx, id); err != nil {
				return fmt.Errorf("ensure the user's row: %w", err)
			}
			var err error
			if user, err = q.GetUser(ctx, id); err != nil {
				return fmt.Errorf("read the user's row: %w", err)
			}
			return nil
		})
		if err != nil {
			// The cause goes to the log only.
			slog.ErrorContext(ctx, "me: database", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		res := meResponse{ID: user.ID.String(), TemperatureUnit: string(user.TemperatureUnit)}
		if user.OnboardedAt.Valid {
			res.OnboardedAt = &user.OnboardedAt.Time
		}
		writeJSON(ctx, w, http.StatusOK, res)
	}
}

// writeJSON sends v as the response body.
func writeJSON(ctx context.Context, w http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		slog.ErrorContext(ctx, "encode response", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
