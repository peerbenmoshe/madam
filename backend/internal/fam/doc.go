// Package fam is Madam's domain logic: time adjustment, Fahrenheit
// conversion, cycle days, lengths and numbers, the two bleeding rules, and
// the statistics. docs/fam-rules.md is the source of truth for every rule
// here, and its §12 test vectors are this package's tests.
//
// The package takes plain values and returns plain values. It imports only
// the standard library: no HTTP, no SQL, nothing from api/ or db/ (the
// depguard rule in .golangci.yml enforces this). Callers convert database
// rows into these types at their own boundary.
//
// Units, because getting them wrong is wrong by a factor of 1000:
//   - MilliC: thousandths of a degree Celsius, the stored unit (36.62 °C is 36620).
//   - HundredthsC, HundredthsF: displayed values (36.55 °C is 3655).
//   - Minutes: minutes after midnight (07:00 is 420).
//   - Date: a calendar date with no clock and no time zone.
//
// Nothing here panics on any input. Input that the rules exclude returns an
// error that wraps one of the Err values, for errors.Is.
package fam
