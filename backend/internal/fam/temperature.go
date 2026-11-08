package fam

import (
	"errors"
	"fmt"
)

// MilliC is a temperature in thousandths of a degree Celsius, the unit the
// database stores: 36.62 °C is 36620.
type MilliC int

// HundredthsC is a displayed Celsius value in hundredths: 36.55 °C is 3655.
type HundredthsC int

// HundredthsF is a displayed or typed Fahrenheit value in hundredths:
// 97.91 °F is 9791.
type HundredthsF int

// Minutes is a time of day in whole minutes after midnight, 0 to 1439:
// 07:00 is 420.
type Minutes int

// The impossible-temperature bounds of fam-rules §9, 20.00–50.00 °C. They
// catch a unit bug in our own code, not a typo, so outside them is an error.
const (
	MinMilliC MilliC = 20000
	MaxMilliC MilliC = 50000
)

var (
	// ErrTemperatureOutOfRange: outside 20.00–50.00 °C (68.00–122.00 °F).
	ErrTemperatureOutOfRange = errors.New("temperature outside 20.00–50.00 °C")
	// ErrTimeOutOfRange: a time of day outside 00:00–23:59.
	ErrTimeOutOfRange = errors.New("time of day outside 00:00–23:59")
)

// Timing says whether a reading counts as taken early, late or on time.
type Timing int

const (
	// OnTime: the rounded adjusted value equals the rounded raw value
	// (fam-rules §3.4), even if the clock times differ.
	OnTime Timing = iota
	// Early: measured before the fixed time, so the correction added.
	Early
	// Late: measured after the fixed time, so the correction subtracted.
	Late
)

// Adjusted is a raw reading together with its time adjustment (fam-rules §3).
type Adjusted struct {
	Raw    MilliC
	Timing Timing
	// numerator is the exact adjusted value × 3, in thousandths (§3.3).
	numerator int
}

// String names the timing, so test failures and logs say "late", not "2".
func (t Timing) String() string {
	switch t {
	case OnTime:
		return "on time"
	case Early:
		return "early"
	case Late:
		return "late"
	}
	return fmt.Sprintf("Timing(%d)", int(t))
}

// minutesPerDay bounds a time of day: 00:00 is 0, 23:59 is 1439.
const minutesPerDay = 24 * 60

func checkTemperature(raw MilliC) error {
	if raw < MinMilliC || raw > MaxMilliC {
		return fmt.Errorf("%w: got %d thousandths", ErrTemperatureOutOfRange, raw)
	}
	return nil
}

func checkTime(name string, m Minutes) error {
	if m < 0 || m >= minutesPerDay {
		return fmt.Errorf("%w: %s is %d minutes after midnight", ErrTimeOutOfRange, name, m)
	}
	return nil
}

// Adjust corrects a raw reading for the difference between the measured time
// and the fixed time in effect for its date (fam-rules §3.2–§3.4): 0.05 °C
// per 30 minutes, early adds and late subtracts.
//
// The arithmetic is §3.3's, in integers. One minute is 5/3 of a thousandth,
// so the exact adjusted value is kept multiplied by 3 (the numerator) and
// rounded once, straight to displayed hundredths, in Celsius.
//
// Positive numerator: Go's integer division truncates toward zero, which is
// floor, and so makes "+ 15, then / 30" round half-up, only while the
// numerator is positive. The guards below admit raw ≥ 20000 and a delta of at
// most ±1439 minutes, so the smallest possible numerator is
// 3 × 20000 − 5 × 1439 = 52805 > 0. TestNumeratorPositive checks those
// corners. Loosen a guard and that proof goes with it.
func Adjust(raw MilliC, measured, fixed Minutes) (Adjusted, error) {
	if err := checkTemperature(raw); err != nil {
		return Adjusted{}, err
	}
	if err := checkTime("measured time", measured); err != nil {
		return Adjusted{}, err
	}
	if err := checkTime("fixed time", fixed); err != nil {
		return Adjusted{}, err
	}

	delta := int(fixed - measured) // positive: measured early
	a := Adjusted{Raw: raw, numerator: 3*int(raw) + 5*delta}

	// §3.4: adjusted only if the rounded values differ. A one-minute
	// difference usually rounds away, and then the reading counts as on time.
	rawShown, adjustedShown := a.Celsius()
	switch {
	case adjustedShown == rawShown:
		a.Timing = OnTime
	case delta > 0:
		a.Timing = Early
	default:
		a.Timing = Late
	}
	return a, nil
}

// IsAdjusted reports whether the reading counts as adjusted: whether it gets
// a ghost dot and a late/early indicator.
func (a Adjusted) IsAdjusted() bool {
	return a.Timing != OnTime
}

// Celsius returns the raw and adjusted values as displayed, in hundredths,
// each rounded half-up exactly once (fam-rules §3.3–§3.4).
func (a Adjusted) Celsius() (raw, adjusted HundredthsC) {
	return a.Raw.Celsius(), HundredthsC((a.numerator + 15) / 30)
}
