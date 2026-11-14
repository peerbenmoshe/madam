package fam

import (
	"errors"
	"time"
)

var (
	// ErrDateInFuture: a date that hasn't begun anywhere on Earth yet
	// (fam-rules §9.1).
	ErrDateInFuture = errors.New("date is in the future")
	// ErrDateTooEarly: a date before EarliestDate.
	ErrDateTooEarly = errors.New("date is before 1 January 2000")
)

// furthestAhead is UTC+14, the time zone where each calendar date begins
// first.
const furthestAhead = 14 * time.Hour

// EarliestDate is the first date the app accepts: 1 January 2000. It exists
// to catch a mangled year (0002), not to judge what is plausible, and it
// can be tuned (fam-rules §9.1).
func EarliestDate() Date {
	return fromUTCMidnight(time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC))
}

// LatestToday returns the latest calendar date that has begun anywhere on
// Earth at the instant now: the date at UTC+14.
//
// The server uses it because it doesn't know the user's time zone. Her own
// today, whatever her zone, is never after this date, so refusing only
// later dates never refuses a real today. now is an argument, so this
// package never reads a clock.
func LatestToday(now time.Time) Date {
	return DateOf(now.UTC().Add(furthestAhead))
}

// CheckDate reports whether a date may be recorded at the instant now
// (fam-rules §9.1). It returns ErrDateTooEarly for a date before
// EarliestDate, ErrDateInFuture for one after LatestToday(now), and nil
// otherwise. The rule is the same for every date the app accepts.
func CheckDate(d Date, now time.Time) error {
	if d.Before(EarliestDate()) {
		return ErrDateTooEarly
	}
	if LatestToday(now).Before(d) {
		return ErrDateInFuture
	}
	return nil
}
