package fam

import (
	"errors"
	"testing"
)

// TestFixedTime is fam-rules §12.8. The onboarding row says 07:00
// (valid_from = -infinity); a change says 06:30 from 1 April 2026.
func TestFixedTime(t *testing.T) {
	newSchedule := func(t *testing.T, changes []TimeChange) Schedule {
		t.Helper()
		s, err := NewSchedule(hm(7, 0), changes)
		if err != nil {
			t.Fatalf("NewSchedule: unexpected error %v", err)
		}
		return s
	}
	changed := []TimeChange{{From: date(t, "2026-04-01"), Time: hm(6, 30)}}

	tests := []struct {
		id      string
		changes []TimeChange
		date    string
		want    Minutes
	}{
		{"M1", changed, "2026-03-15", hm(7, 0)}, // AC-9
		{"M2", changed, "2026-03-31", hm(7, 0)}, // the last day of the first period
		{"M3", changed, "2026-04-01", hm(6, 30)},
		{"M4", changed, "2020-01-01", hm(7, 0)}, // before the account existed
		{"M5", nil, "2030-01-01", hm(7, 0)},     // only the onboarding row
	}
	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			if got := newSchedule(t, tc.changes).FixedTime(date(t, tc.date)); got != tc.want {
				t.Errorf("FixedTime(%s) = %d, want %d", tc.date, got, tc.want)
			}
		})
	}
}

// TestScheduleInputs checks what NewSchedule accepts and refuses.
func TestScheduleInputs(t *testing.T) {
	t.Run("changes in any order", func(t *testing.T) {
		s, err := NewSchedule(hm(7, 0), []TimeChange{
			{From: date(t, "2026-06-01"), Time: hm(6, 0)},
			{From: date(t, "2026-04-01"), Time: hm(6, 30)},
		})
		if err != nil {
			t.Fatalf("NewSchedule: unexpected error %v", err)
		}
		for _, c := range []struct {
			date string
			want Minutes
		}{{"2026-03-31", hm(7, 0)}, {"2026-05-15", hm(6, 30)}, {"2026-06-01", hm(6, 0)}} {
			if got := s.FixedTime(date(t, c.date)); got != c.want {
				t.Errorf("FixedTime(%s) = %d, want %d", c.date, got, c.want)
			}
		}
	})

	t.Run("two changes on one date are refused", func(t *testing.T) {
		d := date(t, "2026-04-01")
		_, err := NewSchedule(hm(7, 0), []TimeChange{{d, hm(6, 30)}, {d, hm(6, 0)}})
		if !errors.Is(err, ErrDuplicateDate) {
			t.Errorf("error = %v, want %v", err, ErrDuplicateDate)
		}
	})

	t.Run("times outside 00:00–23:59 are refused", func(t *testing.T) {
		if _, err := NewSchedule(24*60, nil); !errors.Is(err, ErrTimeOutOfRange) {
			t.Errorf("initial 24:00: error = %v, want %v", err, ErrTimeOutOfRange)
		}
		_, err := NewSchedule(hm(7, 0), []TimeChange{{date(t, "2026-04-01"), -1}})
		if !errors.Is(err, ErrTimeOutOfRange) {
			t.Errorf("change at −1: error = %v, want %v", err, ErrTimeOutOfRange)
		}
	})
}
