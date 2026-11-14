package db_test

import (
	"context"
	"crypto/rand"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/peerbenmoshe/madam/backend/internal/db"
)

// appRole is the role migration 00002 creates and the server connects as.
const appRole = "madam_app"

// TestIsolation proves that Row Level Security, not only each query's WHERE
// clause, keeps users apart (docs/fam-rules.md §12.7, I3). It must run as the
// application role: as the owner, or any role that bypasses RLS, every row
// would be visible and the test would prove nothing, so it refuses to.
func TestIsolation(t *testing.T) {
	pool := testPool(t)
	ctx := t.Context()
	requireAppRole(t, ctx, db.New(pool))

	store := db.NewStore(pool)
	a, b := newUserID(t), newUserID(t)

	// Each user creates their own row, as themselves. That also shows WITH
	// CHECK lets a request write its own id.
	for _, id := range []pgtype.UUID{a, b} {
		err := store.InUserTx(ctx, id, func(q *db.Queries) error {
			return q.CreateUser(ctx, id)
		})
		if err != nil {
			t.Fatalf("create user %s as itself: %v", id, err)
		}
	}

	// Without this, I3's zero rows could just mean B's row doesn't exist.
	t.Run("a user sees their own row", func(t *testing.T) {
		var got db.User
		err := store.InUserTx(ctx, b, func(q *db.Queries) error {
			var err error
			got, err = q.GetUser(ctx, b)
			return err
		})
		if err != nil {
			t.Fatalf("GetUser(B) as B: %v", err)
		}
		if got.ID != b {
			t.Errorf("GetUser(B) as B returned id %s", got.ID)
		}
	})

	// I3. The WHERE clause matches B's row; only the policy hides it.
	t.Run("I3: another user's row is invisible", func(t *testing.T) {
		err := store.InUserTx(ctx, a, func(q *db.Queries) error {
			_, err := q.GetUser(ctx, b)
			return err
		})
		if !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("GetUser(B) as A: got err %v, want pgx.ErrNoRows (zero rows)", err)
		}
	})

	// WITH CHECK: a request can't write a row under someone else's id.
	t.Run("a user cannot create a row for another id", func(t *testing.T) {
		other := newUserID(t)
		err := store.InUserTx(ctx, a, func(q *db.Queries) error {
			return q.CreateUser(ctx, other)
		})
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Fatalf("CreateUser(other) as A: got err %v, want SQLSTATE 42501 (row-level security violation)", err)
		}
	})

	// A query that skipped the wrapper, on a connection that never had the
	// setting: current_setting(..., true) is NULL, so no rows.
	t.Run("no user id on a fresh connection: zero rows", func(t *testing.T) {
		fresh := testPool(t)
		_, err := db.New(fresh).GetUser(ctx, b)
		if !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("GetUser(B) without a user id: got err %v, want pgx.ErrNoRows", err)
		}
	})

	// The same on a connection whose earlier transaction set the user id.
	// The setting is gone, but reads as '' rather than NULL; the policy's
	// NULLIF must turn that into zero rows, not an "invalid uuid" error. It
	// also shows the id doesn't leak into the connection's next use.
	t.Run("no user id after the connection's last transaction: zero rows", func(t *testing.T) {
		conn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatalf("acquire: %v", err)
		}
		defer conn.Release()

		err = pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
			return db.New(tx).SetUserID(ctx, a)
		})
		if err != nil {
			t.Fatalf("set the user id in a transaction: %v", err)
		}
		_, err = db.New(conn).GetUser(ctx, a)
		if !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("GetUser(A) after the transaction ended: got err %v, want pgx.ErrNoRows", err)
		}
	})
}

// requireAppRole stops the test unless it connects as the application role,
// which can't bypass Row Level Security.
func requireAppRole(t *testing.T, ctx context.Context, q *db.Queries) {
	t.Helper()
	role, err := q.CurrentRole(ctx)
	if err != nil {
		t.Fatalf("CurrentRole: %v", err)
	}
	if role.RoleName != appRole || role.IsSuperuser.Bool || role.BypassesRls.Bool {
		t.Fatalf("connected as %q (superuser %v, bypasses RLS %v): MADAM_TEST_DATABASE_URL must use the application role %s",
			role.RoleName, role.IsSuperuser.Bool, role.BypassesRls.Bool, appRole)
	}
}

// newUserID returns a random user id, so test runs never collide.
func newUserID(t *testing.T) pgtype.UUID {
	t.Helper()
	id := pgtype.UUID{Valid: true}
	if _, err := rand.Read(id.Bytes[:]); err != nil {
		t.Fatalf("random user id: %v", err)
	}
	return id
}
