package fam

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// TestArrivingTemperature is fam-rules §12.11: a temperature arrives in
// hundredths with its unit, is refused outside 20–50 °C, and otherwise the
// stored thousandths decide whether she is asked about it.
func TestArrivingTemperature(t *testing.T) {
	const (
		celsius    = "°C"
		fahrenheit = "°F"
	)
	// wantStored 0 means refused: no temperature is anywhere near zero.
	cases := []struct {
		id          string
		unit        string
		hundredths  int
		wantStored  MilliC
		wantUnusual bool
	}{
		{"L1", celsius, 3420, 34200, true}, // AC-14
		{"L2", celsius, 3662, 36620, false},
		{"L3", celsius, 3500, 35000, false}, // the edge itself is usual
		{"L4", celsius, 3499, 34990, true},
		{"L5", celsius, 3900, 39000, false},
		{"L6", celsius, 3901, 39010, true},
		{"L7", celsius, 2000, 20000, true},
		{"L8", celsius, 1999, 0, false}, // AC-17
		{"L9", celsius, 5000, 50000, true},
		{"L10", celsius, 5001, 0, false}, // AC-17
		{"L11", fahrenheit, 9500, 35000, false},
		{"L12", fahrenheit, 9499, 34994, true},
		{"L13", fahrenheit, 10220, 39000, false},
		{"L14", fahrenheit, 10221, 39006, true},
		{"L15", fahrenheit, 6800, 20000, true},
		{"L16", fahrenheit, 6799, 0, false},
		{"L17", fahrenheit, 12200, 50000, true},
		{"L18", fahrenheit, 12201, 0, false},
		{"L19", celsius, 9791, 0, false},    // a Fahrenheit number sent as Celsius
		{"L20", fahrenheit, 3662, 0, false}, // a Celsius number sent as Fahrenheit
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			var stored MilliC
			var err error
			if tc.unit == celsius {
				stored, err = FromCelsius(HundredthsC(tc.hundredths))
			} else {
				stored, err = FromFahrenheit(HundredthsF(tc.hundredths))
			}
			if tc.wantStored == 0 {
				if !errors.Is(err, ErrTemperatureOutOfRange) {
					t.Fatalf("%d hundredths of %s: stored %d, err %v; want ErrTemperatureOutOfRange", tc.hundredths, tc.unit, stored, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("%d hundredths of %s: unexpected error %v", tc.hundredths, tc.unit, err)
			}
			if stored != tc.wantStored {
				t.Errorf("stored = %d, want %d", stored, tc.wantStored)
			}
			if got := stored.IsUnusual(); got != tc.wantUnusual {
				t.Errorf("IsUnusual() = %v, want %v", got, tc.wantUnusual)
			}
		})
	}
}

// TestCelsiusRoundTrip: every Celsius value the app accepts is shown back
// exactly as it was typed (fam-rules §12.11), and is stored inside the
// accepted range.
func TestCelsiusRoundTrip(t *testing.T) {
	const lowest, highest HundredthsC = 2000, 5000
	for c := lowest; c <= highest; c++ {
		stored, err := FromCelsius(c)
		if err != nil {
			t.Fatalf("FromCelsius(%d): unexpected error %v", c, err)
		}
		if stored < MinMilliC || stored > MaxMilliC {
			t.Fatalf("FromCelsius(%d) = %d, outside %d–%d", c, stored, MinMilliC, MaxMilliC)
		}
		if back := stored.Celsius(); back != c {
			t.Fatalf("FromCelsius(%d) = %d, displayed back as %d", c, stored, back)
		}
	}
	for _, c := range []HundredthsC{lowest - 1, highest + 1, 0, -3662} {
		if _, err := FromCelsius(c); !errors.Is(err, ErrTemperatureOutOfRange) {
			t.Errorf("FromCelsius(%d): err = %v, want ErrTemperatureOutOfRange", c, err)
		}
	}
}

// TestTags is fam-rules §12.12.
func TestTags(t *testing.T) {
	allNine := []string{"alcohol", "poor_sleep", "illness", "medication", "travel", "vacation", "new_thermometer", "uti", "yeast_infection"}
	lastFirst := slices.Clone(allNine)
	slices.Reverse(lastFirst)

	cases := []struct {
		id    string
		check func([]string) ([]string, error)
		in    []string
		want  []string // nil when wantErr is set
		// wantErr is the error the list is refused with, or nil.
		wantErr error
	}{
		{"G1", CheckDisturbances, []string{}, []string{}, nil},
		{"G2", CheckDisturbances, []string{"poor_sleep", "alcohol"}, []string{"alcohol", "poor_sleep"}, nil},
		{"G3", CheckDisturbances, lastFirst, allNine, nil},
		{"G4", CheckDisturbances, []string{"alcohol", "alcohol"}, nil, ErrRepeatedTag},
		{"G5", CheckDisturbances, []string{"jet_lag"}, nil, ErrUnknownTag},
		{"G6", CheckDisturbances, []string{"pms"}, nil, ErrUnknownTag},
		{"G7", CheckDisturbances, []string{"Alcohol"}, nil, ErrUnknownTag},
		{"G8", CheckDisturbances, []string{""}, nil, ErrUnknownTag},
		{"G9", CheckSymptoms, []string{"pms", "breast_tenderness"}, []string{"breast_tenderness", "pms"}, nil},
		{"G10", CheckSymptoms, []string{"alcohol"}, nil, ErrUnknownTag},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			got, err := tc.check(tc.in)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("got %v, err %v; want %v", got, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}

	// "None" is an empty list, never nil: a nil slice is written to the
	// database as NULL, which the column refuses (docs/data-model.md §10).
	t.Run("no tags is an empty list, not nil", func(t *testing.T) {
		for _, in := range [][]string{nil, {}} {
			got, err := CheckDisturbances(in)
			if err != nil || got == nil || len(got) != 0 {
				t.Errorf("CheckDisturbances(%#v) = %#v, %v; want an empty list that isn't nil", in, got, err)
			}
		}
	})

	// An unknown tag is whatever was sent, so it must not travel on into a
	// log through the error.
	t.Run("an unknown tag isn't repeated in the error", func(t *testing.T) {
		_, err := CheckDisturbances([]string{"something-she-typed"})
		if err == nil || strings.Contains(err.Error(), "something-she-typed") {
			t.Errorf("err = %v; want an error that doesn't repeat the tag", err)
		}
	})

	t.Run("the list given isn't changed", func(t *testing.T) {
		in := []string{"poor_sleep", "alcohol"}
		if _, err := CheckDisturbances(in); err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		if !slices.Equal(in, []string{"poor_sleep", "alcohol"}) {
			t.Errorf("the caller's list is now %v", in)
		}
	})

	// The lists themselves, as fam-rules §7 gives them, in its order.
	t.Run("the two lists are fam-rules §7's", func(t *testing.T) {
		if got := Disturbances(); !slices.Equal(got, allNine) {
			t.Errorf("Disturbances() = %v", got)
		}
		if got, want := Symptoms(), []string{"breast_tenderness", "bloating", "pms"}; !slices.Equal(got, want) {
			t.Errorf("Symptoms() = %v, want %v", got, want)
		}
		// A caller that changes its copy must not change the rule.
		Disturbances()[0] = "changed"
		if got := Disturbances()[0]; got != "alcohol" {
			t.Errorf("after a caller changed its copy, Disturbances()[0] = %q", got)
		}
	})
}

// TestFreeText is fam-rules §12.13.
func TestFreeText(t *testing.T) {
	cases := []struct {
		id      string
		check   func(string) error
		text    string
		refused bool
	}{
		{"X1", CheckNotes, strings.Repeat("a", 2000), false},
		{"X2", CheckNotes, strings.Repeat("a", 2001), true},
		{"X3", CheckNotes, strings.Repeat("א", 2000), false}, // 4,000 bytes
		{"X4", CheckNotes, strings.Repeat("א", 2001), true},
		{"X5", CheckSecretionDescription, strings.Repeat("a", 200), false},
		{"X6", CheckSecretionDescription, strings.Repeat("a", 201), true},
		{"X7", CheckNotes, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			err := tc.check(tc.text)
			if tc.refused && !errors.Is(err, ErrTextTooLong) {
				t.Errorf("err = %v, want ErrTextTooLong", err)
			}
			if !tc.refused && err != nil {
				t.Errorf("err = %v, want none", err)
			}
		})
	}

	// X3 really is what it says: twice as many bytes as characters.
	if n := len(strings.Repeat("א", 2000)); n != 4000 {
		t.Errorf("2,000 Hebrew letters are %d bytes, want 4000", n)
	}

	// What a user typed doesn't belong in a log: the error must not carry it.
	secret := strings.Repeat("private ", 300)
	if err := CheckNotes(secret); err == nil || strings.Contains(err.Error(), "private") {
		t.Errorf("CheckNotes's error repeats the text: %v", err)
	}
}
