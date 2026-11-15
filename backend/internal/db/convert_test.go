package db_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/peerbenmoshe/madam/backend/internal/db"
	"github.com/peerbenmoshe/madam/backend/internal/fam"
)

// These tests need no database: they check the conversions between pgx's
// types and fam's (convert.go).

func mustDate(t *testing.T, year int, month time.Month, day int) fam.Date {
	t.Helper()
	d, err := fam.NewDate(year, month, day)
	if err != nil {
		t.Fatalf("NewDate(%d, %v, %d): %v", year, month, day, err)
	}
	return d
}

// pgTime is a TIME as pgx reads it: microseconds after midnight.
func pgTime(hours, minutes, seconds, micros int64) pgtype.Time {
	return pgtype.Time{
		Microseconds: ((hours*60+minutes)*60+seconds)*1_000_000 + micros,
		Valid:        true,
	}
}

func TestDateConversion(t *testing.T) {
	// 29 February, and a date before 1970, where a careless division of a
	// negative Unix time would land on the wrong day.
	for _, d := range []fam.Date{
		mustDate(t, 2026, time.November, 14),
		mustDate(t, 2024, time.February, 29),
		mustDate(t, 1969, time.December, 31),
	} {
		pg := db.DateToPg(d)
		if !pg.Valid || pg.InfinityModifier != pgtype.Finite {
			t.Errorf("DateToPg(%v) = %+v, want a valid, finite date", d, pg)
		}
		// pgx writes a DATE from the time's year, month and day.
		if got := pg.Time.Format(time.DateOnly); got != d.String() {
			t.Errorf("DateToPg(%v) holds %s", d, got)
		}
		back, err := db.DateFromPg(pg)
		if err != nil || back != d {
			t.Errorf("DateFromPg(DateToPg(%v)) = %v, %v", d, back, err)
		}
	}

	refused := []struct {
		name string
		in   pgtype.Date
		want error
	}{
		{"NULL", pgtype.Date{}, db.ErrNullValue},
		{"-infinity", pgtype.Date{InfinityModifier: pgtype.NegativeInfinity, Valid: true}, db.ErrInfiniteDate},
		{"infinity", pgtype.Date{InfinityModifier: pgtype.Infinity, Valid: true}, db.ErrInfiniteDate},
	}
	for _, tc := range refused {
		if _, err := db.DateFromPg(tc.in); !errors.Is(err, tc.want) {
			t.Errorf("DateFromPg(%s): err = %v, want %v", tc.name, err, tc.want)
		}
	}
}

func TestMinutesConversion(t *testing.T) {
	// data-model.md §10 measured 06:30 as 23400000000 microseconds.
	if got := db.MinutesToPg(390); !got.Valid || got.Microseconds != 23_400_000_000 {
		t.Errorf("MinutesToPg(390) = %+v, want 23400000000 microseconds", got)
	}

	for _, m := range []fam.Minutes{0, 420, 1439} {
		back, err := db.MinutesFromPg(db.MinutesToPg(m))
		if err != nil || back != m {
			t.Errorf("MinutesFromPg(MinutesToPg(%d)) = %d, %v", m, back, err)
		}
	}

	refused := []struct {
		name string
		in   pgtype.Time
		want error
	}{
		{"NULL", pgtype.Time{}, db.ErrNullValue},
		{"07:00:30", pgTime(7, 0, 30, 0), db.ErrNotAMinute},
		{"07:00:00.000001", pgTime(7, 0, 0, 1), db.ErrNotAMinute},
		{"24:00", pgTime(24, 0, 0, 0), db.ErrNotAMinute},
		{"before midnight", pgtype.Time{Microseconds: -60_000_000, Valid: true}, db.ErrNotAMinute},
	}
	for _, tc := range refused {
		if _, err := db.MinutesFromPg(tc.in); !errors.Is(err, tc.want) {
			t.Errorf("MinutesFromPg(%s): err = %v, want %v", tc.name, err, tc.want)
		}
	}
}

// TestScheduleFromRows runs docs/fam-rules.md §12.8 from rows shaped as the
// database returns them: the onboarding row says 07:00 from -infinity, and
// a change says 06:30 from 1 April 2026.
func TestScheduleFromRows(t *testing.T) {
	first := db.ListMeasurementTimePeriodsRow{
		ValidFrom:       pgtype.Date{InfinityModifier: pgtype.NegativeInfinity, Valid: true},
		MeasurementTime: db.MinutesToPg(7 * 60),
	}
	change := db.ListMeasurementTimePeriodsRow{
		ValidFrom:       db.DateToPg(mustDate(t, 2026, time.April, 1)),
		MeasurementTime: db.MinutesToPg(6*60 + 30),
	}

	t.Run("M1 to M4: the onboarding row and one change", func(t *testing.T) {
		// The change first: the order of the rows must not matter.
		s, err := db.ScheduleFromRows([]db.ListMeasurementTimePeriodsRow{change, first})
		if err != nil {
			t.Fatalf("ScheduleFromRows: %v", err)
		}
		cases := []struct {
			name string
			date fam.Date
			want fam.Minutes
		}{
			{"M1: 15 Mar 2026", mustDate(t, 2026, time.March, 15), 420},
			{"M2: 31 Mar 2026", mustDate(t, 2026, time.March, 31), 420},
			{"M3: 1 Apr 2026", mustDate(t, 2026, time.April, 1), 390},
			{"M4: 1 Jan 2020", mustDate(t, 2020, time.January, 1), 420},
		}
		for _, tc := range cases {
			if got := s.FixedTime(tc.date); got != tc.want {
				t.Errorf("%s: FixedTime = %d, want %d", tc.name, got, tc.want)
			}
		}
	})

	t.Run("M5: only the onboarding row", func(t *testing.T) {
		s, err := db.ScheduleFromRows([]db.ListMeasurementTimePeriodsRow{first})
		if err != nil {
			t.Fatalf("ScheduleFromRows: %v", err)
		}
		if got := s.FixedTime(mustDate(t, 2030, time.January, 1)); got != 420 {
			t.Errorf("FixedTime(1 Jan 2030) = %d, want 420", got)
		}
	})

	t.Run("no onboarding row is an error, not a guess", func(t *testing.T) {
		for _, rows := range [][]db.ListMeasurementTimePeriodsRow{nil, {change}} {
			if _, err := db.ScheduleFromRows(rows); !errors.Is(err, db.ErrNoFirstMeasurementTime) {
				t.Errorf("ScheduleFromRows(%d rows, none from -infinity): err = %v, want ErrNoFirstMeasurementTime", len(rows), err)
			}
		}
	})

	t.Run("a row the table could never hold is refused", func(t *testing.T) {
		bad := []struct {
			name string
			row  db.ListMeasurementTimePeriodsRow
			want error
		}{
			{"a time with seconds", db.ListMeasurementTimePeriodsRow{ValidFrom: first.ValidFrom, MeasurementTime: pgTime(7, 0, 30, 0)}, db.ErrNotAMinute},
			{"valid_from = infinity", db.ListMeasurementTimePeriodsRow{ValidFrom: pgtype.Date{InfinityModifier: pgtype.Infinity, Valid: true}, MeasurementTime: first.MeasurementTime}, db.ErrInfiniteDate},
			{"valid_from NULL", db.ListMeasurementTimePeriodsRow{MeasurementTime: first.MeasurementTime}, db.ErrNullValue},
		}
		for _, tc := range bad {
			_, err := db.ScheduleFromRows([]db.ListMeasurementTimePeriodsRow{first, tc.row})
			if !errors.Is(err, tc.want) {
				t.Errorf("%s: err = %v, want %v", tc.name, err, tc.want)
			}
		}
	})
}

// ptr returns a pointer to v, for the optional fields of db.EntryValues.
func ptr[T any](v T) *T {
	return &v
}

// A day with every field filled in, as the database returns it and as
// handlers see it. 36.62 °C at 07:45.
func fullEntryRow(user pgtype.UUID, date fam.Date) db.Entry {
	return db.Entry{
		UserID:               user,
		EntryDate:            db.DateToPg(date),
		TemperatureMilliC:    pgtype.Int4{Int32: 36620, Valid: true},
		MeasuredAt:           pgTime(7, 45, 0, 0),
		Bleeding:             db.BleedingLevelSpotting,
		SecretionCategory:    db.NullSecretionCategory{SecretionCategory: db.SecretionCategoryClearStretchy, Valid: true},
		Sensation:            db.NullSensation{Sensation: db.SensationWet, Valid: true},
		SecretionAmount:      pgtype.Int2{Int16: 2, Valid: true},
		SecretionDescription: pgtype.Text{String: "clear, stretches", Valid: true},
		CervixFirmness:       db.NullCervixFirmness{CervixFirmness: db.CervixFirmnessSoft, Valid: true},
		CervixPosition:       db.NullCervixPosition{CervixPosition: db.CervixPositionHighOpen, Valid: true},
		Disturbances:         []string{"alcohol", "poor_sleep"},
		Symptoms:             []string{"bloating"},
		Intercourse:          db.NullIntercourseRecord{IntercourseRecord: db.IntercourseRecordProtected, Valid: true},
		Notes:                pgtype.Text{String: "line one\nline two", Valid: true},
	}
}

func fullEntryValues() db.EntryValues {
	return db.EntryValues{
		Reading:              &db.Reading{Temperature: 36620, MeasuredAt: 7*60 + 45},
		Bleeding:             db.BleedingLevelSpotting,
		SecretionCategory:    ptr(db.SecretionCategoryClearStretchy),
		Sensation:            ptr(db.SensationWet),
		SecretionAmount:      ptr(2),
		SecretionDescription: ptr("clear, stretches"),
		CervixFirmness:       ptr(db.CervixFirmnessSoft),
		CervixPosition:       ptr(db.CervixPositionHighOpen),
		Intercourse:          ptr(db.IntercourseRecordProtected),
		Notes:                ptr("line one\nline two"),
		Disturbances:         []string{"alcohol", "poor_sleep"},
		Symptoms:             []string{"bloating"},
	}
}

// paramsOf is the row's own columns as UpsertEntry's parameters: what
// writing that row takes.
func paramsOf(row db.Entry) db.UpsertEntryParams {
	return db.UpsertEntryParams{
		UserID:               row.UserID,
		EntryDate:            row.EntryDate,
		TemperatureMilliC:    row.TemperatureMilliC,
		MeasuredAt:           row.MeasuredAt,
		Bleeding:             row.Bleeding,
		SecretionCategory:    row.SecretionCategory,
		Sensation:            row.Sensation,
		SecretionAmount:      row.SecretionAmount,
		SecretionDescription: row.SecretionDescription,
		CervixFirmness:       row.CervixFirmness,
		CervixPosition:       row.CervixPosition,
		Disturbances:         row.Disturbances,
		Symptoms:             row.Symptoms,
		Intercourse:          row.Intercourse,
		Notes:                row.Notes,
	}
}

func TestEntryConversion(t *testing.T) {
	user := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	date := mustDate(t, 2026, time.November, 15)

	t.Run("every field filled in, both ways", func(t *testing.T) {
		row := fullEntryRow(user, date)
		got, err := db.EntryFromRow(row)
		if err != nil {
			t.Fatalf("EntryFromRow: %v", err)
		}
		if want := fullEntryValues(); !reflect.DeepEqual(got, want) {
			t.Errorf("EntryFromRow:\n got %+v\nwant %+v", got, want)
		}
		params, err := fullEntryValues().UpsertParams(user, date)
		if err != nil {
			t.Fatalf("UpsertParams: %v", err)
		}
		if want := paramsOf(row); !reflect.DeepEqual(params, want) {
			t.Errorf("UpsertParams:\n got %+v\nwant %+v", params, want)
		}
	})

	t.Run("nothing filled in, both ways", func(t *testing.T) {
		// As the database returns a day with only its defaults.
		row := db.Entry{
			UserID: user, EntryDate: db.DateToPg(date),
			Bleeding: db.BleedingLevelNone, Disturbances: []string{}, Symptoms: []string{},
		}
		got, err := db.EntryFromRow(row)
		if err != nil {
			t.Fatalf("EntryFromRow: %v", err)
		}
		want := db.EntryValues{Bleeding: db.BleedingLevelNone, Disturbances: []string{}, Symptoms: []string{}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("EntryFromRow:\n got %+v\nwant %+v", got, want)
		}
		params, err := want.UpsertParams(user, date)
		if err != nil {
			t.Fatalf("UpsertParams: %v", err)
		}
		if !reflect.DeepEqual(params, paramsOf(row)) {
			t.Errorf("UpsertParams:\n got %+v\nwant %+v", params, paramsOf(row))
		}
	})

	t.Run("a nil list becomes an empty one, never NULL", func(t *testing.T) {
		// pgx writes a nil slice as NULL, which the columns refuse.
		params, err := db.EntryValues{Bleeding: db.BleedingLevelNone}.UpsertParams(user, date)
		if err != nil {
			t.Fatalf("UpsertParams: %v", err)
		}
		if params.Disturbances == nil || params.Symptoms == nil {
			t.Errorf("UpsertParams: disturbances %#v, symptoms %#v; want empty, not nil", params.Disturbances, params.Symptoms)
		}
		got, err := db.EntryFromRow(db.Entry{Bleeding: db.BleedingLevelNone})
		if err != nil {
			t.Fatalf("EntryFromRow: %v", err)
		}
		if got.Disturbances == nil || got.Symptoms == nil {
			t.Errorf("EntryFromRow: disturbances %#v, symptoms %#v; want empty, not nil", got.Disturbances, got.Symptoms)
		}
	})

	t.Run("a row the table could never hold is refused", func(t *testing.T) {
		bad := []struct {
			name   string
			change func(*db.Entry)
			want   error
		}{
			{"a temperature without a time", func(e *db.Entry) { e.MeasuredAt = pgtype.Time{} }, db.ErrHalfReading},
			{"a time without a temperature", func(e *db.Entry) { e.TemperatureMilliC = pgtype.Int4{} }, db.ErrHalfReading},
			{"a time with seconds", func(e *db.Entry) { e.MeasuredAt = pgTime(7, 45, 30, 0) }, db.ErrNotAMinute},
			{"24:00", func(e *db.Entry) { e.MeasuredAt = pgTime(24, 0, 0, 0) }, db.ErrNotAMinute},
		}
		for _, tc := range bad {
			row := fullEntryRow(user, date)
			tc.change(&row)
			if _, err := db.EntryFromRow(row); !errors.Is(err, tc.want) {
				t.Errorf("%s: err = %v, want %v", tc.name, err, tc.want)
			}
		}
	})

	t.Run("a number that doesn't fit its column is refused, not wrapped", func(t *testing.T) {
		// 65537 as a 16-bit number is 1, which the CHECK (1 to 3) would
		// accept; 4294967296 + 36620 as a 32-bit number is 36620.
		tooBig := []struct {
			name   string
			change func(*db.EntryValues)
		}{
			{"secretion amount 65537", func(v *db.EntryValues) { v.SecretionAmount = ptr(65537) }},
			{"secretion amount -65535", func(v *db.EntryValues) { v.SecretionAmount = ptr(-65535) }},
			{"temperature 2^32 + 36620", func(v *db.EntryValues) { v.Reading.Temperature = 1<<32 + 36620 }},
			{"temperature -2^32 + 36620", func(v *db.EntryValues) { v.Reading.Temperature = -(1 << 32) + 36620 }},
		}
		for _, tc := range tooBig {
			values := fullEntryValues()
			tc.change(&values)
			if _, err := values.UpsertParams(user, date); !errors.Is(err, db.ErrDoesNotFit) {
				t.Errorf("%s: err = %v, want ErrDoesNotFit", tc.name, err)
			}
		}
	})
}
