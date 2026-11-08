package fam

import (
	"errors"
	"testing"
)

// hm builds a time of day for test tables: hm(7, 45) is 07:45.
func hm(h, m int) Minutes { return Minutes(h*60 + m) }

// TestAdjustment is fam-rules §12.1: fixed time 07:00 throughout.
func TestAdjustment(t *testing.T) {
	fixed := hm(7, 0)
	tests := []struct {
		id            string
		raw           MilliC
		measured      Minutes
		wantNumerator int
		wantRaw       HundredthsC
		wantDisplayed HundredthsC
		wantTiming    Timing // "Adjusted?" in §12.1: OnTime is "no"
	}{
		{"T1", 36620, hm(7, 45), 109635, 3662, 3655, Late},  // AC-1, the half-up case
		{"T2", 36400, hm(6, 30), 109350, 3640, 3645, Early}, // AC-2
		{"T3", 36500, hm(7, 1), 109495, 3650, 3650, OnTime}, // AC-3, cancels after rounding
		{"T4", 36700, hm(7, 0), 110100, 3670, 3670, OnTime}, // on time
		{"T5", 36620, hm(8, 30), 109410, 3662, 3647, Late},  // large late
		{"T6", 36400, hm(5, 0), 109800, 3640, 3660, Early},  // large early
	}
	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			a, err := Adjust(tc.raw, tc.measured, fixed)
			if err != nil {
				t.Fatalf("Adjust(%d, %d, %d): unexpected error %v", tc.raw, tc.measured, fixed, err)
			}
			if a.numerator != tc.wantNumerator {
				t.Errorf("numerator = %d, want %d", a.numerator, tc.wantNumerator)
			}
			raw, displayed := a.Celsius()
			if raw != tc.wantRaw {
				t.Errorf("raw displayed = %d, want %d", raw, tc.wantRaw)
			}
			if displayed != tc.wantDisplayed {
				t.Errorf("adjusted displayed = %d, want %d", displayed, tc.wantDisplayed)
			}
			if a.Timing != tc.wantTiming {
				t.Errorf("timing = %v, want %v", a.Timing, tc.wantTiming)
			}
			if a.IsAdjusted() != (tc.wantTiming != OnTime) {
				t.Errorf("IsAdjusted() = %v, want %v", a.IsAdjusted(), tc.wantTiming != OnTime)
			}
		})
	}
}

// TestNumeratorPositive asserts fam-rules §3.3's constraint: Go's integer
// division truncates toward zero, which is floor (and so half-up rounding)
// only while the numerator is positive. Adjust's guards are what keep it
// positive, so this tests both the extreme corners they admit and that
// everything just outside them is refused.
func TestNumeratorPositive(t *testing.T) {
	for _, raw := range []MilliC{MinMilliC, MaxMilliC} {
		for _, times := range [][2]Minutes{{hm(23, 59), hm(0, 0)}, {hm(0, 0), hm(23, 59)}} {
			measured, fixed := times[0], times[1]
			a, err := Adjust(raw, measured, fixed)
			if err != nil {
				t.Fatalf("Adjust(%d, %d, %d): unexpected error %v", raw, measured, fixed, err)
			}
			if a.numerator <= 0 {
				t.Errorf("Adjust(%d, %d, %d): numerator = %d, want > 0", raw, measured, fixed, a.numerator)
			}
		}
	}

	refused := []struct {
		name            string
		raw             MilliC
		measured, fixed Minutes
		want            error
	}{
		{"raw below 20.00 °C", MinMilliC - 1, hm(7, 0), hm(7, 0), ErrTemperatureOutOfRange},
		{"raw above 50.00 °C", MaxMilliC + 1, hm(7, 0), hm(7, 0), ErrTemperatureOutOfRange},
		{"measured before 00:00", 36500, -1, hm(7, 0), ErrTimeOutOfRange},
		{"measured at 24:00", 36500, 24 * 60, hm(7, 0), ErrTimeOutOfRange},
		{"fixed before 00:00", 36500, hm(7, 0), -1, ErrTimeOutOfRange},
		{"fixed at 24:00", 36500, hm(7, 0), 24 * 60, ErrTimeOutOfRange},
	}
	for _, tc := range refused {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Adjust(tc.raw, tc.measured, tc.fixed)
			if !errors.Is(err, tc.want) {
				t.Errorf("error = %v, want %v", err, tc.want)
			}
		})
	}
}
