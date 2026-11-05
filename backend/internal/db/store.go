package db

// Hand-written, unlike the rest of this package: sqlc generates the
// *.sql.go files, db.go and models.go, and never touches this one.

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store is the rest of the app's way to the database. Handlers are given a
// Store, never the pool, so the only path to user data is InUserTx.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore wraps a connection pool.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Ping is the health check's round trip. It touches no table, so it needs no
// user id and no transaction.
func (s *Store) Ping(ctx context.Context) (int32, error) {
	return New(s.pool).Ping(ctx)
}

// InUserTx runs fn inside one transaction in which Row Level Security sees
// userID as the requesting user (docs/data-model.md §9). The user id is set
// before fn runs, and only for this transaction, so it can't carry over to
// the next request that gets the same pooled connection.
//
// It commits if fn returns nil and rolls back otherwise, including when fn
// panics; fn's own error comes back unchanged, so callers can still match
// pgx.ErrNoRows. Call it before writing the response: a failed commit must
// become an error response, never a success for data that wasn't saved.
func (s *Store) InUserTx(ctx context.Context, userID pgtype.UUID, fn func(*Queries) error) error {
	if !userID.Valid {
		return errors.New("InUserTx: no user id")
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := New(tx)
		if err := q.SetUserID(ctx, userID); err != nil {
			return fmt.Errorf("set the request's user id: %w", err)
		}
		return fn(q)
	})
}
