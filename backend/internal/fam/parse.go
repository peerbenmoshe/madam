package fam

import (
	"errors"
	"fmt"
	"time"
)

// How a date and a time of day are written, for everything the API reads
// and writes (fam-rules §9.2). There is one spelling of each, and anything
// else is refused, not tidied up. The parsing is done by hand because the
// standard library is lenient: time.Parse reads "7:05" for the layout
// "15:04".

var (
	// ErrDateFormat: a date that isn't written YYYY-MM-DD. A date that is,
	// but doesn't exist on the calendar, is ErrInvalidDate.
	ErrDateFormat = errors.New("date is not written YYYY-MM-DD")
	// ErrTimeFormat: a time of day that isn't written HH:MM. One that is,
	// but lies outside 00:00–23:59, is ErrTimeOutOfRange.
	ErrTimeFormat = errors.New("time is not written HH:MM")
)

// number reads s as a decimal number written only with the digits 0 to 9:
// no sign, no space, no digit from another script.
func number(s string) (n int, ok bool) {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}

// ParseDate reads a date written YYYY-MM-DD, exactly ten characters. Its
// error wraps ErrDateFormat for any other spelling and ErrInvalidDate for a
// date the calendar doesn't have. It says nothing about whether the date is
// acceptable now: that is CheckDate.
//
// The errors never repeat s: what a user typed doesn't belong in a log.
func ParseDate(s string) (Date, error) {
	if len(s) != len("2006-01-02") || s[4] != '-' || s[7] != '-' {
		return Date{}, ErrDateFormat
	}
	year, okYear := number(s[0:4])
	month, okMonth := number(s[5:7])
	day, okDay := number(s[8:10])
	if !okYear || !okMonth || !okDay {
		return Date{}, ErrDateFormat
	}
	return NewDate(year, time.Month(month), day)
}

// ParseMinutes reads a time of day written HH:MM, 24-hour, exactly five
// characters. Its error wraps ErrTimeFormat for any other spelling and
// ErrTimeOutOfRange for an hour over 23 or a minute over 59.
func ParseMinutes(s string) (Minutes, error) {
	if len(s) != len("15:04") || s[2] != ':' {
		return 0, ErrTimeFormat
	}
	hour, okHour := number(s[0:2])
	minute, okMinute := number(s[3:5])
	if !okHour || !okMinute {
		return 0, ErrTimeFormat
	}
	if hour > 23 || minute > 59 {
		return 0, fmt.Errorf("%w: %02d:%02d", ErrTimeOutOfRange, hour, minute)
	}
	return Minutes(hour*60 + minute), nil
}

// String writes the time of day as HH:MM, the spelling ParseMinutes reads:
// 425 is "07:05". It is meant for a valid time, 0 to 1439.
func (m Minutes) String() string {
	return fmt.Sprintf("%02d:%02d", int(m)/60, int(m)%60)
}
