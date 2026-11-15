package db

// Hand-written, like store.go. This is the one place where the database's
// types (pgx's pgtype.Date and pgtype.Time, which sqlc generates) meet the
// domain's (fam.Date and fam.Minutes). Handlers and internal/fam never see a
// pgtype date or time (docs/tech-stack.md §6).

import (
	"errors"
	"fmt"
	"math"

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
	// ErrHalfReading: an entry row with a temperature and no time, or a
	// time and no temperature. The table's CHECK refuses to store one.
	ErrHalfReading = errors.New("a temperature without a time, or a time without a temperature")
	// ErrDoesNotFit: a number too large for its column. Converting it
	// anyway would wrap around and store some other number.
	ErrDoesNotFit = errors.New("number does not fit its column")
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

// Reading is a temperature and the time it was measured at. An entry has
// both or neither (the table's CHECK), so the two travel as one value and
// "one without the other" can't be written.
type Reading struct {
	Temperature fam.MilliC
	MeasuredAt  fam.Minutes
}

// EntryValues is everything a person can record for one day, in plain Go:
// what handlers read and write in place of an Entry row or an
// UpsertEntryParams (docs/tech-stack.md §6). A nil pointer is "not said",
// which the database stores as NULL.
//
// The date isn't here. It is the entry's key, and the caller already holds
// it: it asked for the row by that date.
type EntryValues struct {
	// Reading is nil for an entry without a temperature.
	Reading  *Reading
	Bleeding BleedingLevel

	SecretionCategory    *SecretionCategory
	Sensation            *Sensation
	SecretionAmount      *int // 1 to 3
	SecretionDescription *string
	CervixFirmness       *CervixFirmness
	CervixPosition       *CervixPosition
	Intercourse          *IntercourseRecord
	Notes                *string

	// Never nil once converted: "none" is an empty list.
	Disturbances []string
	Symptoms     []string
}

// emptyNotNil returns list, or an empty list in place of nil. pgx writes a
// nil slice as NULL, which the NOT NULL array columns refuse, and JSON
// writes it as null where the page expects [].
func emptyNotNil(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}

// EntryFromRow converts an entries row read from the database. It refuses
// a row with only half a reading and a time that isn't a whole minute;
// the table's CHECKs refuse to store either, so seeing one means the schema
// changed, not that a user did something.
func EntryFromRow(row Entry) (EntryValues, error) {
	v := EntryValues{
		Bleeding:     row.Bleeding,
		Disturbances: emptyNotNil(row.Disturbances),
		Symptoms:     emptyNotNil(row.Symptoms),
	}

	if row.TemperatureMilliC.Valid != row.MeasuredAt.Valid {
		return EntryValues{}, ErrHalfReading
	}
	if row.TemperatureMilliC.Valid {
		at, err := MinutesFromPg(row.MeasuredAt)
		if err != nil {
			return EntryValues{}, fmt.Errorf("measured_at: %w", err)
		}
		v.Reading = &Reading{
			Temperature: fam.MilliC(row.TemperatureMilliC.Int32),
			MeasuredAt:  at,
		}
	}

	// row is this function's own copy, so pointing into it is safe.
	if row.SecretionCategory.Valid {
		v.SecretionCategory = &row.SecretionCategory.SecretionCategory
	}
	if row.Sensation.Valid {
		v.Sensation = &row.Sensation.Sensation
	}
	if row.SecretionAmount.Valid {
		amount := int(row.SecretionAmount.Int16)
		v.SecretionAmount = &amount
	}
	if row.SecretionDescription.Valid {
		v.SecretionDescription = &row.SecretionDescription.String
	}
	if row.CervixFirmness.Valid {
		v.CervixFirmness = &row.CervixFirmness.CervixFirmness
	}
	if row.CervixPosition.Valid {
		v.CervixPosition = &row.CervixPosition.CervixPosition
	}
	if row.Intercourse.Valid {
		v.Intercourse = &row.Intercourse.IntercourseRecord
	}
	if row.Notes.Valid {
		v.Notes = &row.Notes.String
	}
	return v, nil
}

// UpsertParams converts the values into UpsertEntry's parameters, for the
// user's entry on date.
//
// It applies no rule of the domain: validate first (internal/fam, and the
// handler). A value the rules refuse is then refused by the database's
// CHECKs, never stored as something else. The one thing it does check is
// that each number fits its column, because a number that doesn't would
// wrap around into one that might: 65537 into a SMALLINT is 1.
func (v EntryValues) UpsertParams(userID pgtype.UUID, date fam.Date) (UpsertEntryParams, error) {
	p := UpsertEntryParams{
		UserID:       userID,
		EntryDate:    DateToPg(date),
		Bleeding:     v.Bleeding,
		Disturbances: emptyNotNil(v.Disturbances),
		Symptoms:     emptyNotNil(v.Symptoms),
	}

	if v.Reading != nil {
		t := v.Reading.Temperature
		if t < math.MinInt32 || t > math.MaxInt32 {
			return UpsertEntryParams{}, fmt.Errorf("temperature: %w", ErrDoesNotFit)
		}
		p.TemperatureMilliC = pgtype.Int4{Int32: int32(t), Valid: true}
		p.MeasuredAt = MinutesToPg(v.Reading.MeasuredAt)
	}

	if v.SecretionCategory != nil {
		p.SecretionCategory = NullSecretionCategory{SecretionCategory: *v.SecretionCategory, Valid: true}
	}
	if v.Sensation != nil {
		p.Sensation = NullSensation{Sensation: *v.Sensation, Valid: true}
	}
	if v.SecretionAmount != nil {
		amount := *v.SecretionAmount
		if amount < math.MinInt16 || amount > math.MaxInt16 {
			return UpsertEntryParams{}, fmt.Errorf("secretion amount: %w", ErrDoesNotFit)
		}
		p.SecretionAmount = pgtype.Int2{Int16: int16(amount), Valid: true}
	}
	if v.SecretionDescription != nil {
		p.SecretionDescription = pgtype.Text{String: *v.SecretionDescription, Valid: true}
	}
	if v.CervixFirmness != nil {
		p.CervixFirmness = NullCervixFirmness{CervixFirmness: *v.CervixFirmness, Valid: true}
	}
	if v.CervixPosition != nil {
		p.CervixPosition = NullCervixPosition{CervixPosition: *v.CervixPosition, Valid: true}
	}
	if v.Intercourse != nil {
		p.Intercourse = NullIntercourseRecord{IntercourseRecord: *v.Intercourse, Valid: true}
	}
	if v.Notes != nil {
		p.Notes = pgtype.Text{String: *v.Notes, Valid: true}
	}
	return p, nil
}
