package db_test

import (
	"errors"
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
