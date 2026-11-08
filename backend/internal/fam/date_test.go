package fam

import (
	"errors"
	"testing"
	"time"
)

// date builds a Date from "yyyy-mm-dd" for test tables, failing the test on
// a bad literal.
func date(t *testing.T, s string) Date {
	t.Helper()
	tm, err := time.Parse(time.DateOnly, s)
	if err != nil {
		t.Fatalf("bad date literal %q: %v", s, err)
	}
	d, err := NewDate(tm.Year(), tm.Month(), tm.Day())
	if err != nil {
		t.Fatalf("NewDate(%q): %v", s, err)
	}
	return d
}

// TestDate covers the calendar arithmetic everything else relies on.
func TestDate(t *testing.T) {
	t.Run("invalid dates are refused", func(t *testing.T) {
		for _, c := range []struct {
			y     int
			m     time.Month
			d     int
			label string
		}{
			{2026, time.February, 31, "31 Feb"},
			{2026, time.February, 29, "29 Feb in a common year"},
			{2026, 13, 1, "month 13"},
			{2026, time.March, 0, "day 0"},
		} {
			if _, err := NewDate(c.y, c.m, c.d); !errors.Is(err, ErrInvalidDate) {
				t.Errorf("%s: error = %v, want %v", c.label, err, ErrInvalidDate)
			}
		}
	})

	t.Run("29 Feb in a leap year exists", func(t *testing.T) {
		if _, err := NewDate(2024, time.February, 29); err != nil {
			t.Errorf("NewDate(2024-02-29): unexpected error %v", err)
		}
	})

	t.Run("days between", func(t *testing.T) {
		for _, c := range []struct {
			to, from string
			want     int
		}{
			{"2026-03-01", "2026-02-01", 28},
			{"2024-03-01", "2024-02-01", 29}, // leap year
			{"2026-01-01", "2025-12-20", 12}, // across a year
			{"2026-03-30", "2026-03-27", 3},  // across Israel's switch to summer time
			{"2025-12-20", "2026-01-01", -12},
		} {
			if got := date(t, c.to).DaysSince(date(t, c.from)); got != c.want {
				t.Errorf("%s since %s = %d, want %d", c.to, c.from, got, c.want)
			}
		}
	})

	t.Run("adding days", func(t *testing.T) {
		if got, want := date(t, "2026-02-26").AddDays(44), date(t, "2026-04-11"); got != want {
			t.Errorf("26 Feb + 44 days = %v, want %v", got, want)
		}
		if got, want := date(t, "2026-01-01").AddDays(-1), date(t, "2025-12-31"); got != want {
			t.Errorf("1 Jan − 1 day = %v, want %v", got, want)
		}
	})

	t.Run("ordering", func(t *testing.T) {
		a, b := date(t, "2026-01-01"), date(t, "2026-01-02")
		if !a.Before(b) || b.Before(a) || a.Before(a) {
			t.Errorf("Before is wrong for %v and %v", a, b)
		}
		if a.Compare(b) != -1 || b.Compare(a) != 1 || a.Compare(a) != 0 {
			t.Errorf("Compare is wrong for %v and %v", a, b)
		}
	})

	t.Run("time.Time at the boundary", func(t *testing.T) {
		d := date(t, "2026-01-29")
		if got := d.String(); got != "2026-01-29" {
			t.Errorf("String() = %q, want %q", got, "2026-01-29")
		}
		if got, want := d.Time(), time.Date(2026, time.January, 29, 0, 0, 0, 0, time.UTC); !got.Equal(want) || got.Location() != time.UTC {
			t.Errorf("Time() = %v, want %v", got, want)
		}
		if got := DateOf(d.Time()); got != d {
			t.Errorf("DateOf(Time()) = %v, want %v", got, d)
		}
		// 01:30 on 28 March at UTC+3 is still 27 March in UTC: DateOf keeps
		// the calendar date of t's own location.
		local := time.Date(2026, time.March, 28, 1, 30, 0, 0, time.FixedZone("UTC+3", 3*60*60))
		if got, want := DateOf(local), date(t, "2026-03-28"); got != want {
			t.Errorf("DateOf(%v) = %v, want %v", local, got, want)
		}
	})
}
