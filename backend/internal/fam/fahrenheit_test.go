package fam

import (
	"errors"
	"testing"
)

// TestFahrenheit is fam-rules §12.2.
func TestFahrenheit(t *testing.T) {
	roundTrips := []struct {
		id         string
		input      HundredthsF
		wantStored MilliC
	}{
		{"F1", 9791, 36617}, // the case that justifies storing thousandths
		{"F2", 9860, 37000},
	}
	for _, tc := range roundTrips {
		t.Run(tc.id, func(t *testing.T) {
			stored, err := FromFahrenheit(tc.input)
			if err != nil {
				t.Fatalf("FromFahrenheit(%d): unexpected error %v", tc.input, err)
			}
			if stored != tc.wantStored {
				t.Errorf("stored = %d, want %d", stored, tc.wantStored)
			}
			if back := stored.Fahrenheit(); back != tc.input {
				t.Errorf("displayed back = %d, want %d", back, tc.input)
			}
		})
	}

	// F3: Celsius decides whether a reading is adjusted (§3.4), even when
	// the two Fahrenheit values differ.
	t.Run("F3", func(t *testing.T) {
		a, err := Adjust(35000, hm(7, 3), hm(7, 0))
		if err != nil {
			t.Fatalf("Adjust: unexpected error %v", err)
		}
		if rawC, adjC := a.Celsius(); rawC != 3500 || adjC != 3500 {
			t.Errorf("Celsius() = %d, %d; want 3500, 3500", rawC, adjC)
		}
		if rawF, adjF := a.Fahrenheit(); rawF != 9500 || adjF != 9499 {
			t.Errorf("Fahrenheit() = %d, %d; want 9500, 9499", rawF, adjF)
		}
		if a.IsAdjusted() {
			t.Errorf("IsAdjusted() = true, want false: Celsius decides")
		}
	})
}

// TestFahrenheitRoundTrip checks fam-rules §3.5's promise for every input
// the app accepts: what she types in °F is what she is shown back.
func TestFahrenheitRoundTrip(t *testing.T) {
	for f := minHundredthsF; f <= maxHundredthsF; f++ {
		stored, err := FromFahrenheit(f)
		if err != nil {
			t.Fatalf("FromFahrenheit(%d): unexpected error %v", f, err)
		}
		if stored < MinMilliC || stored > MaxMilliC {
			t.Fatalf("FromFahrenheit(%d) = %d, outside 20.00–50.00 °C", f, stored)
		}
		if back := stored.Fahrenheit(); back != f {
			t.Fatalf("FromFahrenheit(%d) = %d, displayed back as %d", f, stored, back)
		}
	}

	// Outside the accepted range is refused, like 20–50 °C in §9. 31.00 °F
	// is below freezing, where the half-up arithmetic would go negative.
	for _, f := range []HundredthsF{minHundredthsF - 1, maxHundredthsF + 1, 3100} {
		if _, err := FromFahrenheit(f); !errors.Is(err, ErrTemperatureOutOfRange) {
			t.Errorf("FromFahrenheit(%d): error = %v, want %v", f, err, ErrTemperatureOutOfRange)
		}
	}
}
