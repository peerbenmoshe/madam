package db_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/peerbenmoshe/madam/backend/internal/db"
)

// The tables that hang from users: isolation (AC-16, part two), the
// new-table checklist of CLAUDE.md, and the cascade. All of it runs as the
// application role, like the server. TestIsolation (isolation_test.go) does
// the same for the users table itself.

// userTable is one table whose rows belong to a user, reduced to the two
// things these tests do to it. A new table adds one element to userTables.
type userTable struct {
	name string
	// write adds one row owned by owner.
	write func(ctx context.Context, q *db.Queries, owner pgtype.UUID) error
	// count asks for owner's rows, by owner's user_id, and counts them.
	count func(ctx context.Context, q *db.Queries, owner pgtype.UUID) (int, error)
}

var userTables = []userTable{
	{
		name: "cycle_starts",
		write: func(ctx context.Context, q *db.Queries, owner pgtype.UUID) error {
			return q.CreateCycleStart(ctx, db.CreateCycleStartParams{
				UserID:    owner,
				StartDate: pgtype.Date{Time: time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC), Valid: true},
			})
		},
		count: func(ctx context.Context, q *db.Queries, owner pgtype.UUID) (int, error) {
			rows, err := q.ListCycleStarts(ctx, owner)
			return len(rows), err
		},
	},
	{
		name: "measurement_time_periods",
		write: func(ctx context.Context, q *db.Queries, owner pgtype.UUID) error {
			return q.CreateFirstMeasurementTime(ctx, db.CreateFirstMeasurementTimeParams{
				UserID:          owner,
				MeasurementTime: db.MinutesToPg(7 * 60),
			})
		},
		count: func(ctx context.Context, q *db.Queries, owner pgtype.UUID) (int, error) {
			rows, err := q.ListMeasurementTimePeriods(ctx, owner)
			return len(rows), err
		},
	},
	{
		name: "entries",
		write: func(ctx context.Context, q *db.Queries, owner pgtype.UUID) error {
			_, err := q.UpsertEntry(ctx, db.UpsertEntryParams{
				UserID:       owner,
				EntryDate:    pgtype.Date{Time: time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC), Valid: true},
				Bleeding:     db.BleedingLevelNone,
				Disturbances: []string{},
				Symptoms:     []string{},
			})
			return err
		},
		count: func(ctx context.Context, q *db.Queries, owner pgtype.UUID) (int, error) {
			// One date, so one row or none.
			_, err := q.GetEntry(ctx, db.GetEntryParams{
				UserID:    owner,
				EntryDate: pgtype.Date{Time: time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC), Valid: true},
			})
			if errors.Is(err, pgx.ErrNoRows) {
				return 0, nil
			}
			if err != nil {
				return 0, err
			}
			return 1, nil
		},
	},
}

// newStoredUser returns a new user id whose users row exists, created by the
// user itself.
func newStoredUser(t *testing.T, store *db.Store) pgtype.UUID {
	t.Helper()
	id := newUserID(t)
	err := store.InUserTx(t.Context(), id, func(q *db.Queries) error {
		return q.CreateUser(t.Context(), id)
	})
	if err != nil {
		t.Fatalf("create user %s as itself: %v", id, err)
	}
	return id
}

// sqlState returns err's PostgreSQL error code, or "" if it isn't one.
func sqlState(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

// TestIsolationOfUserTables is TestIsolation's cases for every table in
// userTables (docs/fam-rules.md §12.7, I3).
func TestIsolationOfUserTables(t *testing.T) {
	pool := testPool(t)
	ctx := t.Context()
	requireAppRole(t, ctx, db.New(pool))
	store := db.NewStore(pool)

	for _, table := range userTables {
		t.Run(table.name, func(t *testing.T) {
			// A reads, B owns a row, C exists but owns nothing here.
			a, b, c := newStoredUser(t, store), newStoredUser(t, store), newStoredUser(t, store)

			// As seen by `as`, how many rows does owner have?
			countAs := func(as, owner pgtype.UUID) int {
				t.Helper()
				var n int
				err := store.InUserTx(ctx, as, func(q *db.Queries) error {
					var err error
					n, err = table.count(ctx, q, owner)
					return err
				})
				if err != nil {
					t.Fatalf("count %s's rows as %s: %v", owner, as, err)
				}
				return n
			}

			for _, owner := range []pgtype.UUID{a, b} {
				err := store.InUserTx(ctx, owner, func(q *db.Queries) error {
					return table.write(ctx, q, owner)
				})
				if err != nil {
					t.Fatalf("write a row as its owner %s: %v", owner, err)
				}
			}

			// Without this, I3's zero rows could just mean B has none.
			if got := countAs(b, b); got != 1 {
				t.Fatalf("B sees %d of B's rows, want 1", got)
			}

			// I3. The WHERE clause names B; only the policy hides the row.
			if got := countAs(a, b); got != 0 {
				t.Errorf("I3: A sees %d of B's rows, want 0", got)
			}

			// WITH CHECK. C has a users row, so the foreign key isn't what
			// refuses, and no row here, so the primary key isn't either.
			err := store.InUserTx(ctx, a, func(q *db.Queries) error {
				return table.write(ctx, q, c)
			})
			if sqlState(err) != "42501" {
				t.Errorf("A writes a row for C: err = %v, want SQLSTATE 42501 (row-level security)", err)
			}
			if got := countAs(c, c); got != 0 {
				t.Errorf("C has %d rows after A's refused write, want 0", got)
			}

			// A query that skipped the wrapper, on a connection that never
			// had the setting.
			fresh := testPool(t)
			if got, err := table.count(ctx, db.New(fresh), b); err != nil || got != 0 {
				t.Errorf("no user id, fresh connection: %d rows, err %v; want 0 rows", got, err)
			}

			// The same on a connection whose last transaction set a user id:
			// the setting then reads as '', which the policy's NULLIF covers.
			conn, err := pool.Acquire(ctx)
			if err != nil {
				t.Fatalf("acquire: %v", err)
			}
			defer conn.Release()
			err = pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
				return db.New(tx).SetUserID(ctx, b)
			})
			if err != nil {
				t.Fatalf("set the user id in a transaction: %v", err)
			}
			if got, err := table.count(ctx, db.New(conn), b); err != nil || got != 0 {
				t.Errorf("no user id, used connection: %d rows, err %v; want 0 rows", got, err)
			}
		})
	}
}

// TestDeletingAUserDeletesHerRows is the checklist's "does it cascade?",
// asked of the database rather than of the migration's text: account
// deletion must leave nothing behind (docs/data-model.md §8.8).
func TestDeletingAUserDeletesHerRows(t *testing.T) {
	pool := testPool(t)
	ctx := t.Context()
	requireAppRole(t, ctx, db.New(pool))
	store := db.NewStore(pool)
	a, b := newStoredUser(t, store), newStoredUser(t, store)

	counts := func(owner pgtype.UUID) map[string]int {
		t.Helper()
		got := map[string]int{}
		err := store.InUserTx(ctx, owner, func(q *db.Queries) error {
			for _, table := range userTables {
				n, err := table.count(ctx, q, owner)
				if err != nil {
					return err
				}
				got[table.name] = n
			}
			return nil
		})
		if err != nil {
			t.Fatalf("count %s's rows: %v", owner, err)
		}
		return got
	}

	for _, owner := range []pgtype.UUID{a, b} {
		err := store.InUserTx(ctx, owner, func(q *db.Queries) error {
			for _, table := range userTables {
				if err := table.write(ctx, q, owner); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("write %s's rows: %v", owner, err)
		}
	}
	for name, n := range counts(a) {
		if n != 1 {
			t.Fatalf("before the delete A has %d rows in %s, want 1", n, name)
		}
	}

	// A may not delete from measurement_time_periods herself (no DELETE
	// grant). The cascade must clear it anyway.
	err := store.InUserTx(ctx, a, func(q *db.Queries) error {
		return q.DeleteUser(ctx, a)
	})
	if err != nil {
		t.Fatalf("DeleteUser(A) as A: %v", err)
	}

	err = store.InUserTx(ctx, a, func(q *db.Queries) error {
		_, err := q.GetUser(ctx, a)
		return err
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("GetUser(A) after the delete: err = %v, want pgx.ErrNoRows", err)
	}
	for name, n := range counts(a) {
		if n != 0 {
			t.Errorf("after the delete A still has %d rows in %s", n, name)
		}
	}
	for name, n := range counts(b) {
		if n != 1 {
			t.Errorf("after A's delete B has %d rows in %s, want 1", n, name)
		}
	}

	// And A can't delete B.
	err = store.InUserTx(ctx, a, func(q *db.Queries) error {
		return q.DeleteUser(ctx, b)
	})
	if err != nil {
		t.Fatalf("DeleteUser(B) as A: %v", err)
	}
	for name, n := range counts(b) {
		if n != 1 {
			t.Errorf("after A tried to delete B, B has %d rows in %s, want 1", n, name)
		}
	}
}

// TestFirstMeasurementTime writes onboarding's row and reads it back: the
// '-infinity' in the query's text arrives as pgx's NegativeInfinity, and the
// schedule built from it answers docs/fam-rules.md §12.8's M4 and M5.
func TestFirstMeasurementTime(t *testing.T) {
	pool := testPool(t)
	ctx := t.Context()
	store := db.NewStore(pool)
	a := newStoredUser(t, store)

	write := func(user pgtype.UUID, at pgtype.Time) error {
		return store.InUserTx(ctx, user, func(q *db.Queries) error {
			return q.CreateFirstMeasurementTime(ctx, db.CreateFirstMeasurementTimeParams{
				UserID: user, MeasurementTime: at,
			})
		})
	}
	if err := write(a, db.MinutesToPg(7*60)); err != nil {
		t.Fatalf("write 07:00 from -infinity: %v", err)
	}

	var rows []db.ListMeasurementTimePeriodsRow
	err := store.InUserTx(ctx, a, func(q *db.Queries) error {
		var err error
		rows, err = q.ListMeasurementTimePeriods(ctx, a)
		return err
	})
	if err != nil {
		t.Fatalf("ListMeasurementTimePeriods: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if from := rows[0].ValidFrom; !from.Valid || from.InfinityModifier != pgtype.NegativeInfinity {
		t.Errorf("valid_from = %+v, want -infinity", from)
	}

	schedule, err := db.ScheduleFromRows(rows)
	if err != nil {
		t.Fatalf("ScheduleFromRows: %v", err)
	}
	if got := schedule.FixedTime(mustDate(t, 2020, time.January, 1)); got != 420 {
		t.Errorf("M4: FixedTime(1 Jan 2020) = %d, want 420", got)
	}
	if got := schedule.FixedTime(mustDate(t, 2030, time.January, 1)); got != 420 {
		t.Errorf("M5: FixedTime(1 Jan 2030) = %d, want 420", got)
	}

	t.Run("a second row from -infinity is refused", func(t *testing.T) {
		if err := write(a, db.MinutesToPg(6*60)); sqlState(err) != "23505" {
			t.Errorf("err = %v, want SQLSTATE 23505 (unique violation)", err)
		}
	})

	// The CHECK: a fixed time is a whole minute, 00:00 to 23:59.
	refused := []struct {
		name string
		at   pgtype.Time
	}{
		{"24:00", pgTime(24, 0, 0, 0)},
		{"07:00:30", pgTime(7, 0, 30, 0)},
		{"07:00:00.000001", pgTime(7, 0, 0, 1)},
	}
	for _, tc := range refused {
		t.Run(tc.name+" is refused", func(t *testing.T) {
			if err := write(newStoredUser(t, store), tc.at); sqlState(err) != "23514" {
				t.Errorf("err = %v, want SQLSTATE 23514 (check violation)", err)
			}
		})
	}
	t.Run("23:59 is the latest time", func(t *testing.T) {
		if err := write(newStoredUser(t, store), pgTime(23, 59, 0, 0)); err != nil {
			t.Errorf("err = %v, want none", err)
		}
	})
}

// tableGrants is every table in the public schema, with exactly what the
// application role may do to it. TestSchemaChecklist fails for a table that
// isn't listed, so a new table can't arrive without its line here.
var tableGrants = map[string][]string{
	// goose's bookkeeping. The app has no business in it.
	"goose_db_version":         nil,
	"users":                    {"SELECT", "INSERT", "UPDATE", "DELETE"},
	"cycle_starts":             {"SELECT", "INSERT", "DELETE"},
	"measurement_time_periods": {"SELECT", "INSERT"},
	"entries":                  {"SELECT", "INSERT", "UPDATE", "DELETE"},
}

// TestBothListsNameTheSameTables: a table of ours in tableGrants must also
// be one of the userTables, or the isolation and cascade tests would pass
// without ever looking at it. TestSchemaChecklist makes sure tableGrants
// itself is complete, so between them a new table can't miss either list.
// It needs no database.
func TestBothListsNameTheSameTables(t *testing.T) {
	inUserTables := map[string]bool{}
	for _, table := range userTables {
		inUserTables[table.name] = true
		if tableGrants[table.name] == nil {
			t.Errorf("userTables has %s, which tableGrants doesn't list with any privilege", table.name)
		}
	}
	for name, grants := range tableGrants {
		// Not a table of ours, or the table the others hang from, which
		// TestIsolation covers.
		if grants == nil || name == "users" {
			continue
		}
		if !inUserTables[name] {
			t.Errorf("tableGrants has %s, which isn't in userTables: add it there, so its isolation and its cascade are tested", name)
		}
	}
}

// Every privilege a PostgreSQL 17 table has.
var tablePrivileges = []string{
	"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER", "MAINTAIN",
}

// TestSchemaChecklist is CLAUDE.md's new-table checklist, read from the
// database's own catalog: Row Level Security enabled and forced, a policy
// for madam_app with USING and WITH CHECK, a cascade from users, and the
// exact grants.
//
// The SQL is written here, not in a .sql file. That rule is for the app's
// queries; these read the catalog, which the app never does, and ask about
// statements (TRUNCATE) the app must have no query for.
func TestSchemaChecklist(t *testing.T) {
	pool := testPool(t)
	ctx := t.Context()
	requireAppRole(t, ctx, db.New(pool))

	type tableFlags struct{ rls, forced bool }
	tables := map[string]tableFlags{}
	rows, err := pool.Query(ctx, `
		SELECT c.relname::text, c.relrowsecurity, c.relforcerowsecurity
		FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relkind = 'r'`)
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	for rows.Next() {
		var name string
		var f tableFlags
		if err := rows.Scan(&name, &f.rls, &f.forced); err != nil {
			t.Fatalf("scan: %v", err)
		}
		tables[name] = f
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("list tables: %v", err)
	}

	for name := range tables {
		if _, listed := tableGrants[name]; !listed {
			t.Errorf("table %s isn't in tableGrants: add it, with exactly what madam_app may do", name)
		}
	}

	for name, want := range tableGrants {
		t.Run(name, func(t *testing.T) {
			flags, exists := tables[name]
			if !exists {
				t.Fatalf("tableGrants lists %s, which isn't in the public schema", name)
			}

			var got []string
			for _, privilege := range tablePrivileges {
				var has bool
				err := pool.QueryRow(ctx,
					`SELECT has_table_privilege(current_user, ('public.' || $1)::regclass, $2)`,
					name, privilege).Scan(&has)
				if err != nil {
					t.Fatalf("has_table_privilege(%s): %v", privilege, err)
				}
				if has {
					got = append(got, privilege)
				}
			}
			if !slices.Equal(got, want) {
				t.Errorf("madam_app may %v, want exactly %v", got, want)
			}

			if want == nil {
				return // not a table of ours: nothing more to ask of it
			}

			if !flags.rls || !flags.forced {
				t.Errorf("row level security: enabled %v, forced %v; want both", flags.rls, flags.forced)
			}

			// qual is the USING expression and with_check the WITH CHECK
			// one, each as text, or NULL when the policy has none.
			policies, err := pool.Query(ctx, `
				SELECT policyname::text, cmd, roles::text[], qual, with_check
				FROM pg_catalog.pg_policies
				WHERE schemaname = 'public' AND tablename = $1`, name)
			if err != nil {
				t.Fatalf("list policies: %v", err)
			}
			count := 0
			for policies.Next() {
				var policy, cmd string
				var roles []string
				var using, withCheck *string
				if err := policies.Scan(&policy, &cmd, &roles, &using, &withCheck); err != nil {
					t.Fatalf("scan: %v", err)
				}
				count++
				if cmd != "ALL" || !slices.Equal(roles, []string{appRole}) || using == nil || withCheck == nil {
					t.Errorf("policy %s: FOR %s TO %v, USING %v, WITH CHECK %v; want FOR ALL TO [%s] with both",
						policy, cmd, roles, using != nil, withCheck != nil, appRole)
					continue
				}
				// The two must be one rule. The isolation tests prove USING
				// by behaviour (I3); this carries that proof to WITH CHECK,
				// which behaviour can't always reach: Postgres also applies
				// USING to a new row that the statement can read back
				// (RETURNING, ON CONFLICT DO UPDATE), so an upsert is still
				// refused under WITH CHECK (true). A plain INSERT isn't.
				if *using != *withCheck {
					t.Errorf("policy %s: USING and WITH CHECK differ; they must be the same rule\n     USING %s\nWITH CHECK %s",
						policy, *using, *withCheck)
				}
			}
			if err := policies.Err(); err != nil {
				t.Fatalf("list policies: %v", err)
			}
			if count != 1 {
				t.Errorf("%d policies, want 1", count)
			}

			if name == "users" {
				return // the table the others cascade from
			}
			var cascades int
			err = pool.QueryRow(ctx, `
				SELECT count(*)
				FROM pg_catalog.pg_constraint
				WHERE conrelid = ('public.' || $1)::regclass
				  AND contype = 'f'
				  AND confrelid = 'public.users'::regclass
				  AND confdeltype = 'c'`, name).Scan(&cascades)
			if err != nil {
				t.Fatalf("read foreign keys: %v", err)
			}
			if cascades != 1 {
				t.Errorf("%d foreign keys to users with ON DELETE CASCADE, want 1", cascades)
			}
		})
	}
}
