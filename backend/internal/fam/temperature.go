package fam

import "errors"

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

// Adjust corrects a raw reading for the difference between the measured time
// and the fixed time in effect for its date (fam-rules §3.2–§3.4).
func Adjust(raw MilliC, measured, fixed Minutes) (Adjusted, error) {
	return Adjusted{}, nil // stub: tests first
}

// IsAdjusted reports whether the reading counts as adjusted: whether it gets
// a ghost dot and a late/early indicator.
func (a Adjusted) IsAdjusted() bool {
	return a.Timing != OnTime
}

// Celsius returns the raw and adjusted values as displayed, in hundredths.
func (a Adjusted) Celsius() (raw, adjusted HundredthsC) {
	return 0, 0 // stub: tests first
}
