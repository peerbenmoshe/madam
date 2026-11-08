package fam

import (
	"fmt"
	"slices"
)

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
	if err := checkTime("onboarding measurement time", initial); err != nil {
		return Schedule{}, err
	}
	sorted := slices.Clone(changes)
	slices.SortFunc(sorted, func(a, b TimeChange) int { return a.From.Compare(b.From) })
	for i, c := range sorted {
		if err := checkTime("measurement time from "+c.From.String(), c.Time); err != nil {
			return Schedule{}, err
		}
		if i > 0 && c.From == sorted[i-1].From {
			return Schedule{}, fmt.Errorf("%w: measurement time change on %v", ErrDuplicateDate, c.From)
		}
	}
	return Schedule{initial: initial, changes: sorted}, nil
}

// FixedTime returns the measurement time in effect on d: the one from the
// latest change on or before d, or the onboarding time if there is none.
func (s Schedule) FixedTime(d Date) Minutes {
	// The same search as Cycles.latestStartIndex: d's index if a change
	// starts on d, otherwise one past the latest change before d.
	i, found := slices.BinarySearchFunc(s.changes, d, func(c TimeChange, d Date) int {
		return c.From.Compare(d)
	})
	if !found {
		i--
	}
	if i < 0 {
		return s.initial // §4: the -infinity row covers everything before
	}
	return s.changes[i].Time
}
