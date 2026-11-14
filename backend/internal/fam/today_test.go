package fam

import (
	"errors"
	"testing"
	"time"
)

// utc builds an instant in November 2026, UTC, for the tables below.
func utc(day, hour, minute int) time.Time {
	return time.Date(2026, time.November, day, hour, minute, 0, 0, time.UTC)
}

// TestCheckDate is fam-rules §12.9: which dates are accepted when the
// server doesn't know her time zone.
func TestCheckDate(t *testing.T) {
	for _, c := range []struct {
		name string
		now  time.Time
		date string
		want error // nil: accepted
	}{
		{"FD1: 23:59 in Israel, today", utc(13, 21, 59), "2026-11-13", nil},
		{"FD2: 00:30 in Israel, her today", utc(13, 22, 30), "2026-11-14", nil},
		{"FD3: the 14th hasn't begun anywhere", utc(13, 9, 59), "2026-11-14", ErrDateInFuture},
		{"FD4: the 14th has just begun at UTC+14", utc(13, 10, 0), "2026-11-14", nil},
		{"FD5: the day after her today", utc(13, 22, 30), "2026-11-15", ErrDateInFuture},
		{"FD6: any past date", utc(13, 22, 30), "2020-01-01", nil},
		{"FD7: the day before the floor", utc(13, 22, 30), "1999-12-31", ErrDateTooEarly},
		{"FD8: the floor itself", utc(13, 22, 30), "2000-01-01", nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := CheckDate(date(t, c.date), c.now)
			if !errors.Is(err, c.want) {
				t.Errorf("CheckDate(%s at %s) = %v, want %v", c.date, c.now.Format(time.RFC3339), err, c.want)
			}
		})
	}
}

// TestLatestToday checks the half of §9.1 the page never sees: the latest
// date that has begun anywhere on Earth.
func TestLatestToday(t *testing.T) {
	t.Run("one minute apart, a day apart", func(t *testing.T) {
		if got := LatestToday(utc(13, 9, 59)); got != date(t, "2026-11-13") {
			t.Errorf("at 09:59 UTC: %v, want 2026-11-13", got)
		}
		if got := LatestToday(utc(13, 10, 0)); got != date(t, "2026-11-14") {
			t.Errorf("at 10:00 UTC: %v, want 2026-11-14", got)
		}
	})

	// The answer depends on the instant, not on the zone the clock happens
	// to be read in: a server set to another zone must give the same date.
	t.Run("the clock's own zone doesn't matter", func(t *testing.T) {
		instant := utc(13, 22, 30)
		want := LatestToday(instant)
		for _, zone := range []*time.Location{
			time.FixedZone("Israel", 2*60*60),
			time.FixedZone("New York", -5*60*60),
			time.FixedZone("Kiritimati", 14*60*60),
		} {
			if got := LatestToday(instant.In(zone)); got != want {
				t.Errorf("read in %v: %v, want %v", zone, got, want)
			}
		}
	})

	t.Run("across a year's end", func(t *testing.T) {
		now := time.Date(2026, time.December, 31, 10, 0, 0, 0, time.UTC)
		if got := LatestToday(now); got != date(t, "2027-01-01") {
			t.Errorf("at 10:00 UTC on 31 December: %v, want 2027-01-01", got)
		}
	})
}

func TestEarliestDate(t *testing.T) {
	if got := EarliestDate(); got != date(t, "2000-01-01") {
		t.Errorf("EarliestDate() = %v, want 2000-01-01", got)
	}
}
