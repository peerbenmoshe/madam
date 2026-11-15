package fam

import "testing"

// adjustChecksum is one number that stands for what Adjust returns across
// seventeen million inputs (below). The page holds a copy of Adjust's
// arithmetic, for the day screen's live line (docs/tech-stack.md §1.3), and
// its test computes the same checksum with that copy and expects this same
// number: frontend/src/lib/adjust.test.mjs.
//
// So if this test fails, the arithmetic changed. If that was meant, the
// copy in frontend/src/lib/adjust.ts must change with it, and then both
// numbers together. Never update one alone.
const adjustChecksum = 863312314

// TestAdjustMatchesThePagesCopy folds every adjusted value, in hundredths
// of °C, into one number: for each of four fixed times, every accepted
// temperature from 20.00 to 50.00 °C, at every minute of the day. That is
// 4 × 3,001 × 1,440 = 17,285,760 readings.
func TestAdjustMatchesThePagesCopy(t *testing.T) {
	const modulus = 1_000_000_007
	hash, readings := 0, 0
	for _, fixed := range []Minutes{0, 390, 420, 1439} {
		for hundredths := 2000; hundredths <= 5000; hundredths++ {
			raw, err := FromCelsius(HundredthsC(hundredths))
			if err != nil {
				t.Fatalf("FromCelsius(%d): %v", hundredths, err)
			}
			for measured := Minutes(0); measured < minutesPerDay; measured++ {
				a, err := Adjust(raw, measured, fixed)
				if err != nil {
					t.Fatalf("Adjust(%d, %d, %d): %v", raw, measured, fixed, err)
				}
				_, adjusted := a.Celsius()
				hash = (hash*31 + int(adjusted)) % modulus
				readings++
			}
		}
	}
	if readings != 17_285_760 {
		t.Errorf("%d readings, want 17,285,760", readings)
	}
	if hash != adjustChecksum {
		t.Errorf("checksum = %d, want %d: Adjust's arithmetic changed. See adjustChecksum.", hash, adjustChecksum)
	}
}
