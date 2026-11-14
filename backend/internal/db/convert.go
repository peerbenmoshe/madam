package db

// Hand-written, like store.go. This is the one place where the database's
// types (pgx's pgtype.Date and pgtype.Time, which sqlc generates) meet the
// domain's (fam.Date and fam.Minutes). Handlers and internal/fam never see a
// pgtype date or time (docs/tech-stack.md §6).

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/peerbenmoshe/madam/backend/internal/fam"
)

// microsPerMinute: a Postgres TIME travels as microseconds after midnight.
const microsPerMinute = 60 * 1_000_000

// minutesPerDay bounds a time of day: 00:00 is 0 and 23:59 is 1439.
const minutesPerDay = 24 * 60

var (
	// ErrNullValue: a NULL where the column is NOT NULL. Seeing it means a
	// query or the schema changed, not that a user did something.
	ErrNullValue = errors.New("unexpected NULL")
	// ErrInfiniteDate: 'infinity' or '-infinity' where a calendar date was
	// expected. Only measurement_time_periods.valid_from may hold
	// '-infinity', and ScheduleFromRows deals with that row itself.
	ErrInfiniteDate = errors.New("infinite date")
	// ErrNotAMinute: a TIME that isn't a whole minute from 00:00 to 23:59.
	// The table's CHECK refuses to store one.
	ErrNotAMinute = errors.New("time is not a whole minute of the day")
	// ErrNoFirstMeasurementTime: no row with valid_from = '-infinity'.
	// Onboarding writes it, so every onboarded user has one.
	ErrNoFirstMeasurementTime = errors.New("no measurement time from -infinity")
)

// DateToPg converts a date for a DATE column or parameter.
func DateToPg(d fam.Date) pgtype.Date {
	return pgtype.Date{Time: d.Time(), Valid: true}
}

// DateFromPg converts a DATE read from the database. It refuses NULL and
// both infinities, so '-infinity' can never become a fam.Date.
func DateFromPg(d pgtype.Date) (fam.Date, error) {
	if !d.Valid {
		return fam.Date{}, ErrNullValue
	}
	if d.InfinityModifier != pgtype.Finite {
		return fam.Date{}, ErrInfiniteDate
	}
	return fam.DateOf(d.Time), nil
}

// MinutesToPg converts a time of day for a TIME column or parameter. It
// checks nothing: validate with fam first. An out-of-range value is refused
// by the database, never stored as some other time.
func MinutesToPg(m fam.Minutes) pgtype.Time {
	return pgtype.Time{Microseconds: int64(m) * microsPerMinute, Valid: true}
}

// MinutesFromPg converts a TIME read from the database. It refuses NULL and
// anything that isn't a whole minute from 00:00 to 23:59.
func MinutesFromPg(t pgtype.Time) (fam.Minutes, error) {
	if !t.Valid {
		return 0, ErrNullValue
	}
	if t.Microseconds < 0 || t.Microseconds%microsPerMinute != 0 ||
		t.Microseconds/microsPerMinute >= minutesPerDay {
		return 0, fmt.Errorf("%w: %d microseconds after midnight", ErrNotAMinute, t.Microseconds)
	}
	return fam.Minutes(t.Microseconds / microsPerMinute), nil
}

// ScheduleFromRows turns a user's measurement_time_periods rows, in any
// order, into the schedule that says which fixed time applies to a date
// (docs/fam-rules.md §4). The row with valid_from = '-infinity' is
// onboarding's: its time is the schedule's first. Every other row is a
// change from its valid_from onward.
func ScheduleFromRows(rows []ListMeasurementTimePeriodsRow) (fam.Schedule, error) {
	var (
		initial  fam.Minutes
		hasFirst bool
		changes  []fam.TimeChange
	)
	for _, row := range rows {
		minutes, err := MinutesFromPg(row.MeasurementTime)
		if err != nil {
			return fam.Schedule{}, fmt.Errorf("measurement time: %w", err)
		}
		if row.ValidFrom.Valid && row.ValidFrom.InfinityModifier == pgtype.NegativeInfinity {
			// The primary key allows only one per user.
			initial, hasFirst = minutes, true
			continue
		}
		from, err := DateFromPg(row.ValidFrom)
		if err != nil {
			return fam.Schedule{}, fmt.Errorf("valid_from: %w", err)
		}
		changes = append(changes, fam.TimeChange{From: from, Time: minutes})
	}
	if !hasFirst {
		return fam.Schedule{}, ErrNoFirstMeasurementTime
	}
	return fam.NewSchedule(initial, changes)
}
