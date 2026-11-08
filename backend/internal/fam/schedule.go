package fam

// TimeChange is one measurement_time_periods row after onboarding's: Time
// applies from From (its valid_from) until the day before the next change.
type TimeChange struct {
	From Date
	Time Minutes
}

// Schedule says which fixed measurement time applies to a date
// (fam-rules §4). Build it with NewSchedule.
type Schedule struct {
	initial Minutes
	changes []TimeChange // sorted by From, unique
}

// NewSchedule validates the measurement-time rows. initial is the time of
// the row with valid_from = -infinity, written at onboarding. It is a
// required argument, so "no fixed time" can't be expressed (§4). The caller
// recognises that row by its pgtype.Date.InfinityModifier and passes every
// other row as a change, in any order.
//
// Overlapping periods can't be expressed either, since a change is only a
// start date. The one invalid shape left is two changes on one date, which
// returns an error wrapping ErrDuplicateDate; a time outside 00:00–23:59
// wraps ErrTimeOutOfRange.
func NewSchedule(initial Minutes, changes []TimeChange) (Schedule, error) {
	return Schedule{}, nil // stub: tests first
}

// FixedTime returns the measurement time in effect on d: the one from the
// latest change on or before d, or the onboarding time if there is none.
func (s Schedule) FixedTime(d Date) Minutes {
	return 0 // stub: tests first
}
