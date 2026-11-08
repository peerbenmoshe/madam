package fam

// The accepted Fahrenheit input range, exactly 20.00–50.00 °C (fam-rules §3.5).
const (
	minHundredthsF HundredthsF = 6800
	maxHundredthsF HundredthsF = 12200
)

// FromFahrenheit converts a typed Fahrenheit value to the stored unit,
// rounding half-up once (fam-rules §3.5).
func FromFahrenheit(f HundredthsF) (MilliC, error) {
	return 0, nil // stub: tests first
}

// Fahrenheit returns a raw value as displayed in Fahrenheit.
func (m MilliC) Fahrenheit() HundredthsF {
	return 0 // stub: tests first
}

// Fahrenheit returns the raw and adjusted values as displayed in Fahrenheit.
// The adjusted value comes from the exact numerator, never from the rounded
// Celsius figure, so it is rounded only once (fam-rules §3.5). Whether the
// reading counts as adjusted is still decided in Celsius (§3.4): see Timing.
func (a Adjusted) Fahrenheit() (raw, adjusted HundredthsF) {
	return 0, 0 // stub: tests first
}
