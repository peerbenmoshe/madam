package fam

import "fmt"

// The accepted Fahrenheit input range, exactly 20.00–50.00 °C (fam-rules §3.5).
const (
	minHundredthsF HundredthsF = 6800
	maxHundredthsF HundredthsF = 12200
)

// FromFahrenheit converts a typed Fahrenheit value to the stored unit,
// rounding half-up once (fam-rules §3.5):
//
//	milli = half-up((f − 32.00) × 50/9) = (100 × x + 9) / 18,  x = f − 3200
//
// The range check comes first: it keeps x positive, which the truncating
// division needs to round half-up (the same constraint as §3.3).
func FromFahrenheit(f HundredthsF) (MilliC, error) {
	if f < minHundredthsF || f > maxHundredthsF {
		return 0, fmt.Errorf("%w: got %d hundredths of °F", ErrTemperatureOutOfRange, f)
	}
	x := int(f) - 3200
	return MilliC((100*x + 9) / 18), nil
}

// Celsius returns a raw value as displayed in Celsius, rounded half-up.
func (m MilliC) Celsius() HundredthsC {
	return HundredthsC((int(m) + 5) / 10)
}

// Fahrenheit returns a raw value as displayed in Fahrenheit. A raw value is
// an adjustment of zero, so it uses the same formula with numerator 3 × milli.
func (m MilliC) Fahrenheit() HundredthsF {
	return fahrenheitFromNumerator(3 * int(m))
}

// fahrenheitFromNumerator rounds an exact value (thousandths × 3, §3.3) once,
// half-up, to Fahrenheit hundredths: (3 × numerator + 25) / 50 + 3200. The
// numerator is positive for every value Adjust and FromFahrenheit admit.
func fahrenheitFromNumerator(numerator int) HundredthsF {
	return HundredthsF((3*numerator+25)/50 + 3200)
}

// Fahrenheit returns the raw and adjusted values as displayed in Fahrenheit.
// The adjusted value comes from the exact numerator, never from the rounded
// Celsius figure, so it is rounded only once (fam-rules §3.5). Whether the
// reading counts as adjusted is still decided in Celsius (§3.4): see Timing.
func (a Adjusted) Fahrenheit() (raw, adjusted HundredthsF) {
	return a.Raw.Fahrenheit(), fahrenheitFromNumerator(a.numerator)
}
