package db_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/peerbenmoshe/madam/backend/internal/db"
	"github.com/peerbenmoshe/madam/backend/internal/fam"
)

// The entries table as the application role uses it: what it stores, what
// its constraints refuse, its trigger, and isolation by date
// (docs/data-model.md §4.2, docs/fam-rules.md §12.7). The checklist, the
// cascade and case I3 are in user_tables_test.go, where entries is one of
// the userTables.

// upsertAs saves an entry inside the transaction of the user `as`.
func upsertAs(ctx context.Context, store *db.Store, as pgtype.UUID, params db.UpsertEntryParams) (db.Entry, error) {
	var row db.Entry
	err := store.InUserTx(ctx, as, func(q *db.Queries) error {
		var err error
		row, err = q.UpsertEntry(ctx, params)
		return err
	})
	return row, err
}

// entryAs reads owner's entry for date as the user `as` sees it. found is
// false when there is no such row to see.
func entryAs(t *testing.T, store *db.Store, as, owner pgtype.UUID, date fam.Date) (row db.Entry, found bool) {
	t.Helper()
	err := store.InUserTx(t.Context(), as, func(q *db.Queries) error {
		var err error
		row, err = q.GetEntry(t.Context(), db.GetEntryParams{UserID: owner, EntryDate: db.DateToPg(date)})
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.Entry{}, false
	}
	if err != nil {
		t.Fatalf("GetEntry(%s, %v) as %s: %v", owner, date, as, err)
	}
	return row, true
}

// refusedBy returns the SQLSTATE of err and, for a constraint violation,
// the name of the constraint that refused.
func refusedBy(err error) (code, constraint string) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code, pgErr.ConstraintName
	}
	return "", ""
}

// sameData reports whether two rows hold the same entry, leaving out the
// two timestamps the database sets.
func sameData(a, b db.Entry) bool {
	a.CreatedAt, a.UpdatedAt = pgtype.Timestamptz{}, pgtype.Timestamptz{}
	b.CreatedAt, b.UpdatedAt = pgtype.Timestamptz{}, pgtype.Timestamptz{}
	return reflect.DeepEqual(a, b)
}

// TestEntryRoundTrip stores an entry and reads it back, with every column
// filled in and with every optional column empty.
func TestEntryRoundTrip(t *testing.T) {
	pool := testPool(t)
	ctx := t.Context()
	requireAppRole(t, ctx, db.New(pool))
	store := db.NewStore(pool)
	user := newStoredUser(t, store)
	date := mustDate(t, 2026, time.November, 15)

	cases := []struct {
		name   string
		values db.EntryValues
	}{
		{"every field filled in", fullEntryValues()},
		{"nothing but the defaults", db.EntryValues{Bleeding: db.BleedingLevelNone, Disturbances: []string{}, Symptoms: []string{}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			params, err := tc.values.UpsertParams(user, date)
			if err != nil {
				t.Fatalf("UpsertParams: %v", err)
			}
			saved, err := upsertAs(ctx, store, user, params)
			if err != nil {
				t.Fatalf("UpsertEntry: %v", err)
			}
			read, found := entryAs(t, store, user, user, date)
			if !found {
				t.Fatal("the entry just saved isn't there")
			}
			if !reflect.DeepEqual(saved, read) {
				t.Errorf("UpsertEntry returned\n %+v\nand GetEntry reads\n %+v", saved, read)
			}
			got, err := db.EntryFromRow(read)
			if err != nil {
				t.Fatalf("EntryFromRow: %v", err)
			}
			if !reflect.DeepEqual(got, tc.values) {
				t.Errorf("read back\n %+v\nwant\n %+v", got, tc.values)
			}
		})
	}

	// The second case saved the same date as the first: it replaced every
	// field of it, the filled-in ones included.
	t.Run("a second save of a date replaces the first", func(t *testing.T) {
		read, found := entryAs(t, store, user, user, date)
		if !found {
			t.Fatal("no entry")
		}
		if read.TemperatureMilliC.Valid || read.Notes.Valid || read.Intercourse.Valid || len(read.Disturbances) != 0 {
			t.Errorf("the first save's values are still there: %+v", read)
		}
	})
}

// TestEntriesAreByDate: one user, three dates. Reading, saving or deleting
// the middle one must leave the day before and the day after alone. Every
// other test here gives a user a single date, where a query that got its
// date wrong would still look right.
func TestEntriesAreByDate(t *testing.T) {
	pool := testPool(t)
	ctx := t.Context()
	requireAppRole(t, ctx, db.New(pool))
	store := db.NewStore(pool)
	user := newStoredUser(t, store)
	before := mustDate(t, 2026, time.November, 13)
	middle := mustDate(t, 2026, time.November, 14)
	after := mustDate(t, 2026, time.November, 15)

	save := func(date fam.Date, notes string) {
		t.Helper()
		params, err := db.EntryValues{Bleeding: db.BleedingLevelNone, Notes: &notes}.UpsertParams(user, date)
		if err != nil {
			t.Fatalf("UpsertParams: %v", err)
		}
		if _, err := upsertAs(ctx, store, user, params); err != nil {
			t.Fatalf("save %v: %v", date, err)
		}
	}
	// want is the notes each date should hold; "" for no entry.
	check := func(when string, want map[fam.Date]string) {
		t.Helper()
		for date, notes := range want {
			row, found := entryAs(t, store, user, user, date)
			if found != (notes != "") || row.Notes.String != notes {
				t.Errorf("%s, the entry for %v: found %v, notes %q; want %q", when, date, found, row.Notes.String, notes)
			}
		}
	}

	// Saved out of order, so the order of saving can't stand in for the date.
	save(after, "the 15th")
	save(before, "the 13th")
	save(middle, "the 14th")
	check("after three saves", map[fam.Date]string{before: "the 13th", middle: "the 14th", after: "the 15th"})

	save(middle, "the 14th, changed")
	check("after changing the middle one", map[fam.Date]string{before: "the 13th", middle: "the 14th, changed", after: "the 15th"})

	err := store.InUserTx(ctx, user, func(q *db.Queries) error {
		return q.DeleteEntry(ctx, db.DeleteEntryParams{UserID: user, EntryDate: db.DateToPg(middle)})
	})
	if err != nil {
		t.Fatalf("delete %v: %v", middle, err)
	}
	check("after deleting the middle one", map[fam.Date]string{before: "the 13th", middle: "", after: "the 15th"})
}

// TestEntryConstraints sends rows the handlers would never build, straight
// to the query, and expects the database to refuse each one by the named
// CHECK: the backstop for a bug in our own code (docs/data-model.md §7).
func TestEntryConstraints(t *testing.T) {
	pool := testPool(t)
	ctx := t.Context()
	requireAppRole(t, ctx, db.New(pool))
	store := db.NewStore(pool)
	date := mustDate(t, 2026, time.November, 15)

	// Each case starts from a valid entry with a reading and changes one
	// thing. A user per case, so no case sees another's row.
	valid := func(user pgtype.UUID) db.UpsertEntryParams {
		return db.UpsertEntryParams{
			UserID:            user,
			EntryDate:         db.DateToPg(date),
			TemperatureMilliC: pgtype.Int4{Int32: 36620, Valid: true},
			MeasuredAt:        pgTime(7, 45, 0, 0),
			Bleeding:          db.BleedingLevelNone,
			Disturbances:      []string{},
			Symptoms:          []string{},
		}
	}

	const (
		checkViolation   = "23514"
		notNullViolation = "23502"
		invalidEnumText  = "22P02"
	)
	refused := []struct {
		name       string
		change     func(*db.UpsertEntryParams)
		code       string
		constraint string
	}{
		{"a temperature without a time", func(p *db.UpsertEntryParams) { p.MeasuredAt = pgtype.Time{} },
			checkViolation, "reading_has_a_temperature_and_a_time"},
		{"a time without a temperature", func(p *db.UpsertEntryParams) { p.TemperatureMilliC = pgtype.Int4{} },
			checkViolation, "reading_has_a_temperature_and_a_time"},
		{"19.999 °C", func(p *db.UpsertEntryParams) { p.TemperatureMilliC.Int32 = 19999 },
			checkViolation, "temperature_is_possible"},
		{"50.001 °C", func(p *db.UpsertEntryParams) { p.TemperatureMilliC.Int32 = 50001 },
			checkViolation, "temperature_is_possible"},
		// The unit bug the CHECK exists for: hundredths where thousandths belong.
		{"3662, a reading in the wrong unit", func(p *db.UpsertEntryParams) { p.TemperatureMilliC.Int32 = 3662 },
			checkViolation, "temperature_is_possible"},
		{"measured at 24:00", func(p *db.UpsertEntryParams) { p.MeasuredAt = pgTime(24, 0, 0, 0) },
			checkViolation, "measured_at_is_a_minute_of_the_day"},
		{"measured at 07:45:30", func(p *db.UpsertEntryParams) { p.MeasuredAt = pgTime(7, 45, 30, 0) },
			checkViolation, "measured_at_is_a_minute_of_the_day"},
		{"measured at 07:45:00.000001", func(p *db.UpsertEntryParams) { p.MeasuredAt = pgTime(7, 45, 0, 1) },
			checkViolation, "measured_at_is_a_minute_of_the_day"},
		{"secretion amount 0", func(p *db.UpsertEntryParams) { p.SecretionAmount = pgtype.Int2{Int16: 0, Valid: true} },
			checkViolation, "secretion_amount_is_1_to_3"},
		{"secretion amount 4", func(p *db.UpsertEntryParams) { p.SecretionAmount = pgtype.Int2{Int16: 4, Valid: true} },
			checkViolation, "secretion_amount_is_1_to_3"},
		{"an unknown disturbance", func(p *db.UpsertEntryParams) { p.Disturbances = []string{"alcohol", "jet_lag"} },
			checkViolation, "disturbances_are_known"},
		// A symptom isn't a disturbance: the two lists are separate.
		{"a symptom among the disturbances", func(p *db.UpsertEntryParams) { p.Disturbances = []string{"pms"} },
			checkViolation, "disturbances_are_known"},
		{"an unknown symptom", func(p *db.UpsertEntryParams) { p.Symptoms = []string{"headache"} },
			checkViolation, "symptoms_are_known"},
		// Why EntryValues.UpsertParams never sends a nil list.
		{"disturbances NULL (a nil slice)", func(p *db.UpsertEntryParams) { p.Disturbances = nil },
			notNullViolation, ""},
		{"symptoms NULL (a nil slice)", func(p *db.UpsertEntryParams) { p.Symptoms = nil },
			notNullViolation, ""},
		{"a bleeding level that isn't one", func(p *db.UpsertEntryParams) { p.Bleeding = "heavy" },
			invalidEnumText, ""},
		{"a sensation that isn't one", func(p *db.UpsertEntryParams) {
			p.Sensation = db.NullSensation{Sensation: "slippery", Valid: true}
		}, invalidEnumText, ""},
	}
	for _, tc := range refused {
		t.Run(tc.name+" is refused", func(t *testing.T) {
			user := newStoredUser(t, store)
			params := valid(user)
			tc.change(&params)
			_, err := upsertAs(ctx, store, user, params)
			code, constraint := refusedBy(err)
			if code != tc.code || constraint != tc.constraint {
				t.Errorf("err = %v\n got SQLSTATE %q by constraint %q, want %q by %q", err, code, constraint, tc.code, tc.constraint)
			}
			if _, found := entryAs(t, store, user, user, date); found {
				t.Error("a row was stored")
			}
		})
	}

	accepted := []struct {
		name   string
		change func(*db.UpsertEntryParams)
	}{
		{"the entry the cases start from", func(*db.UpsertEntryParams) {}},
		{"20.000 °C, the lowest", func(p *db.UpsertEntryParams) { p.TemperatureMilliC.Int32 = 20000 }},
		{"50.000 °C, the highest", func(p *db.UpsertEntryParams) { p.TemperatureMilliC.Int32 = 50000 }},
		{"measured at 00:00", func(p *db.UpsertEntryParams) { p.MeasuredAt = pgTime(0, 0, 0, 0) }},
		{"measured at 23:59", func(p *db.UpsertEntryParams) { p.MeasuredAt = pgTime(23, 59, 0, 0) }},
		{"secretion amount 1", func(p *db.UpsertEntryParams) { p.SecretionAmount = pgtype.Int2{Int16: 1, Valid: true} }},
		{"secretion amount 3", func(p *db.UpsertEntryParams) { p.SecretionAmount = pgtype.Int2{Int16: 3, Valid: true} }},
		{"no reading at all", func(p *db.UpsertEntryParams) {
			p.TemperatureMilliC, p.MeasuredAt = pgtype.Int4{}, pgtype.Time{}
		}},
		{"all nine disturbances", func(p *db.UpsertEntryParams) {
			p.Disturbances = []string{"alcohol", "poor_sleep", "illness", "medication", "travel", "vacation", "new_thermometer", "uti", "yeast_infection"}
		}},
		{"every symptom", func(p *db.UpsertEntryParams) {
			p.Symptoms = []string{"breast_tenderness", "bloating", "pms"}
		}},
	}
	for _, tc := range accepted {
		t.Run(tc.name+" is accepted", func(t *testing.T) {
			user := newStoredUser(t, store)
			params := valid(user)
			tc.change(&params)
			if _, err := upsertAs(ctx, store, user, params); err != nil {
				t.Errorf("err = %v, want none", err)
			}
		})
	}
}

// TestEntryTimestamps: created_at is the first save's time and never moves;
// updated_at moves on every later save. No query names either column, so
// what moves updated_at is the table's trigger (docs/data-model.md §4.2).
func TestEntryTimestamps(t *testing.T) {
	pool := testPool(t)
	ctx := t.Context()
	requireAppRole(t, ctx, db.New(pool))
	store := db.NewStore(pool)
	user := newStoredUser(t, store)
	date := mustDate(t, 2026, time.November, 15)

	// The role fires the trigger without being allowed to run its function
	// or to create triggers: neither privilege is needed for that. If this
	// stops being true, the migration's REVOKE must go.
	var mayExecute bool
	err := pool.QueryRow(ctx,
		`SELECT has_function_privilege(current_user, 'public.set_updated_at()', 'EXECUTE')`).Scan(&mayExecute)
	if err != nil {
		t.Fatalf("has_function_privilege: %v", err)
	}
	if mayExecute {
		t.Errorf("%s may EXECUTE set_updated_at(); the migration revokes it from everyone", appRole)
	}

	save := func(notes string) db.Entry {
		t.Helper()
		params, err := db.EntryValues{Bleeding: db.BleedingLevelNone, Notes: &notes}.UpsertParams(user, date)
		if err != nil {
			t.Fatalf("UpsertParams: %v", err)
		}
		row, err := upsertAs(ctx, store, user, params)
		if err != nil {
			t.Fatalf("UpsertEntry: %v", err)
		}
		return row
	}

	first := save("first")
	if !first.CreatedAt.Valid || !first.UpdatedAt.Valid {
		t.Fatalf("timestamps after the first save: created %+v, updated %+v", first.CreatedAt, first.UpdatedAt)
	}
	if !first.UpdatedAt.Time.Equal(first.CreatedAt.Time) {
		t.Errorf("a new row: updated_at %v, created_at %v; want the same instant", first.UpdatedAt.Time, first.CreatedAt.Time)
	}

	second := save("second")
	if !second.CreatedAt.Time.Equal(first.CreatedAt.Time) {
		t.Errorf("created_at moved on the second save: %v, was %v", second.CreatedAt.Time, first.CreatedAt.Time)
	}
	if !second.UpdatedAt.Time.After(first.UpdatedAt.Time) {
		t.Errorf("updated_at didn't move on the second save: %v, was %v", second.UpdatedAt.Time, first.UpdatedAt.Time)
	}
	if second.Notes.String != "second" {
		t.Errorf("notes = %q, want the second save's", second.Notes.String)
	}
}

// TestEntryIsolationByDate is cases I1 and I2 of docs/fam-rules.md §12.7,
// asked of the database as the application role. The same cases through
// the API are in internal/api.
func TestEntryIsolationByDate(t *testing.T) {
	pool := testPool(t)
	ctx := t.Context()
	requireAppRole(t, ctx, db.New(pool))
	store := db.NewStore(pool)
	a, b := newStoredUser(t, store), newStoredUser(t, store)
	date := mustDate(t, 2026, time.November, 15)

	bParams, err := fullEntryValues().UpsertParams(b, date)
	if err != nil {
		t.Fatalf("UpsertParams: %v", err)
	}
	bSaved, err := upsertAs(ctx, store, b, bParams)
	if err != nil {
		t.Fatalf("B saves her entry: %v", err)
	}

	// I1. A asks for B's entry by its date, naming B. Not found.
	if row, found := entryAs(t, store, a, b, date); found {
		t.Errorf("I1: A reads B's entry: %+v", row)
	}
	// And A has none of her own on that date, so nothing of B's leaked in.
	if row, found := entryAs(t, store, a, a, date); found {
		t.Errorf("I1: A has an entry on B's date before saving one: %+v", row)
	}

	// I2. A saves that date. It creates A's own entry; B's is untouched.
	aNotes := "A's own"
	aParams, err := db.EntryValues{Bleeding: db.BleedingLevelBleeding, Notes: &aNotes}.UpsertParams(a, date)
	if err != nil {
		t.Fatalf("UpsertParams: %v", err)
	}
	if _, err := upsertAs(ctx, store, a, aParams); err != nil {
		t.Fatalf("I2: A saves the date: %v", err)
	}
	aRow, found := entryAs(t, store, a, a, date)
	if !found || aRow.Notes.String != aNotes || aRow.Bleeding != db.BleedingLevelBleeding {
		t.Errorf("I2: A's own entry: found %v, %+v", found, aRow)
	}
	bNow, found := entryAs(t, store, b, b, date)
	if !found || !reflect.DeepEqual(bNow, bSaved) {
		t.Errorf("I2: B's entry changed when A saved the same date:\n was %+v\n now %+v (found %v)", bSaved, bNow, found)
	}

	// A names B in an update: the policy's WITH CHECK refuses the row.
	if _, err := upsertAs(ctx, store, a, bParams); sqlState(err) != "42501" {
		t.Errorf("A saves an entry under B's id: err = %v, want SQLSTATE 42501 (row-level security)", err)
	}

	// A deletes B's date, naming B. No error, and nothing is deleted.
	err = store.InUserTx(ctx, a, func(q *db.Queries) error {
		return q.DeleteEntry(ctx, db.DeleteEntryParams{UserID: b, EntryDate: db.DateToPg(date)})
	})
	if err != nil {
		t.Fatalf("A deletes B's entry: %v", err)
	}
	if bNow, found := entryAs(t, store, b, b, date); !found || !sameData(bNow, bSaved) {
		t.Errorf("B's entry after A's delete: found %v, %+v", found, bNow)
	}

	// A deletes her own, and only hers goes.
	err = store.InUserTx(ctx, a, func(q *db.Queries) error {
		return q.DeleteEntry(ctx, db.DeleteEntryParams{UserID: a, EntryDate: db.DateToPg(date)})
	})
	if err != nil {
		t.Fatalf("A deletes her own entry: %v", err)
	}
	if _, found := entryAs(t, store, a, a, date); found {
		t.Error("A's entry is still there after she deleted it")
	}
	if _, found := entryAs(t, store, b, b, date); !found {
		t.Error("B's entry went when A deleted her own")
	}

	// Deleting a date that has no entry is not an error.
	err = store.InUserTx(ctx, a, func(q *db.Queries) error {
		return q.DeleteEntry(ctx, db.DeleteEntryParams{UserID: a, EntryDate: db.DateToPg(date)})
	})
	if err != nil {
		t.Errorf("A deletes a date with no entry: err = %v, want none", err)
	}
}
