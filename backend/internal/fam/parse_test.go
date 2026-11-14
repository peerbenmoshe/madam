package fam

import (
	"errors"
	"testing"
)

// TestParseDate is the first table of fam-rules §12.10.
func TestParseDate(t *testing.T) {
	for _, c := range []struct {
		name, in string
		want     string // the date, when accepted
		wantErr  error
	}{
		{"P1", "2026-11-01", "2026-11-01", nil},
		{"P2: 29 February in a leap year", "2024-02-29", "2024-02-29", nil},
		{"P3: 30 February", "2026-02-30", "", ErrInvalidDate},
		{"P4: month 13", "2026-13-01", "", ErrInvalidDate},
		{"P5: one-digit month and day", "2026-2-3", "", ErrDateFormat},
		{"P6: dd/mm/yyyy", "01/11/2026", "", ErrDateFormat},
		{"P7: with a time", "2026-11-01T00:00:00Z", "", ErrDateFormat},
		{"P8: a space first", " 2026-11-01", "", ErrDateFormat},
		{"P9: full-width digits", "２０２６-11-01", "", ErrDateFormat},
		{"P10: nothing", "", "", ErrDateFormat},
		// Beyond the table: the same shapes from other directions.
		{"a space last", "2026-11-01 ", "", ErrDateFormat},
		{"slashes in place", "2026/11/01", "", ErrDateFormat},
		{"a sign for the year", "-026-11-01", "", ErrDateFormat},
		{"letters", "yyyy-mm-dd", "", ErrDateFormat},
		{"month 0", "2026-00-10", "", ErrInvalidDate},
		{"day 0", "2026-11-00", "", ErrInvalidDate},
		{"29 February in a common year", "2026-02-29", "", ErrInvalidDate},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseDate(c.in)
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("ParseDate(%q): err = %v, want %v", c.in, err, c.wantErr)
			}
			if c.wantErr == nil && got != date(t, c.want) {
				t.Errorf("ParseDate(%q) = %v, want %s", c.in, got, c.want)
			}
		})
	}

	// What String writes, ParseDate reads: before 1970 and across a leap day.
	t.Run("round trip", func(t *testing.T) {
		for _, s := range []string{"1969-12-31", "2000-01-01", "2024-02-29", "2026-11-14"} {
			d, err := ParseDate(s)
			if err != nil || d.String() != s {
				t.Errorf("ParseDate(%q) = %v, %v", s, d, err)
			}
		}
	})
}

// TestParseMinutes is the second table of fam-rules §12.10.
func TestParseMinutes(t *testing.T) {
	for _, c := range []struct {
		name, in string
		want     Minutes
		wantErr  error
	}{
		{"H1", "07:05", 425, nil},
		{"H2: midnight", "00:00", 0, nil},
		{"H3: the last minute", "23:59", 1439, nil},
		{"H4: 24:00 is not a time", "24:00", 0, ErrTimeOutOfRange},
		{"H5: minute 60", "07:60", 0, ErrTimeOutOfRange},
		{"H6: one-digit hour", "7:05", 0, ErrTimeFormat},
		{"H7: with seconds", "07:05:00", 0, ErrTimeFormat},
		{"H8: a dot", "07.05", 0, ErrTimeFormat},
		{"H9: no colon", "0705", 0, ErrTimeFormat},
		{"H10: 12-hour", "7:05 AM", 0, ErrTimeFormat},
		{"H11: a sign", "-7:05", 0, ErrTimeFormat},
		{"H12: nothing", "", 0, ErrTimeFormat},
		// Beyond the table.
		{"a space first", " 7:05", 0, ErrTimeFormat},
		{"one-digit minute", "07:5", 0, ErrTimeFormat},
		{"letters", "hh:mm", 0, ErrTimeFormat},
		{"full-width digits", "０7:05", 0, ErrTimeFormat},
		{"hour 99", "99:00", 0, ErrTimeOutOfRange},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseMinutes(c.in)
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("ParseMinutes(%q): err = %v, want %v", c.in, err, c.wantErr)
			}
			if c.wantErr == nil && got != c.want {
				t.Errorf("ParseMinutes(%q) = %d, want %d", c.in, got, c.want)
			}
		})
	}
}

// TestMinutesString: a time written out is HH:MM, and every minute of the
// day survives being written and read again (fam-rules §12.10).
func TestMinutesString(t *testing.T) {
	for m, want := range map[Minutes]string{0: "00:00", 425: "07:05", 420: "07:00", 1439: "23:59"} {
		if got := m.String(); got != want {
			t.Errorf("Minutes(%d).String() = %q, want %q", m, got, want)
		}
	}
	for m := Minutes(0); m < 24*60; m++ {
		back, err := ParseMinutes(m.String())
		if err != nil || back != m {
			t.Fatalf("ParseMinutes(%q) = %d, %v; want %d", m.String(), back, err, m)
		}
	}
}
