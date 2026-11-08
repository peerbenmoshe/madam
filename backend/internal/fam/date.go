package fam

import (
	"errors"
	"time"
)

// ErrInvalidDate: a calendar date that doesn't exist, such as 31 February.
var ErrInvalidDate = errors.New("no such calendar date")

// Date is a calendar date with no clock and no time zone: the date an entry
// is about, exactly as entered (spec-v1 §13 has no time-zone handling).
//
// It is not a time.Time on purpose. A time.Time is an instant, carries a
// location, and can't safely be compared with ==, and dividing the
// difference of two by 24 hours is off by one across a DST change. A Date is
// a count of days, so cycle-day arithmetic is subtraction. The field is
// unexported so that only meaningful operations exist: adding two dates
// doesn't compile.
//
// The zero Date is 1 January 1970.
type Date struct {
	days int // days since 1970-01-01
}

// NewDate returns the date, or ErrInvalidDate if it doesn't exist.
func NewDate(year int, month time.Month, day int) (Date, error) {
	return Date{}, nil // stub: tests first
}

// DateOf returns t's calendar date in t's own location. pgx reads a DATE
// column as midnight UTC, which gives the stored date back.
func DateOf(t time.Time) Date {
	return Date{} // stub: tests first
}

// Time returns the date as midnight UTC, the form pgx writes to a DATE column.
func (d Date) Time() time.Time {
	return time.Time{} // stub: tests first
}

// AddDays returns the date n days later (earlier, for negative n).
func (d Date) AddDays(n int) Date {
	return Date{} // stub: tests first
}

// DaysSince returns how many days d is after e (negative if before).
func (d Date) DaysSince(e Date) int {
	return 0 // stub: tests first
}

// Before reports whether d is earlier than e.
func (d Date) Before(e Date) bool {
	return false // stub: tests first
}

// Compare returns -1, 0 or +1 as d is before, equal to or after e, for
// slices.SortFunc and friends.
func (d Date) Compare(e Date) int {
	return 0 // stub: tests first
}

// String formats the date as ISO 8601 (2026-01-29), for logs and test
// messages. What the user sees, dd/mm/yyyy, is the frontend's job.
func (d Date) String() string {
	return "" // stub: tests first
}
