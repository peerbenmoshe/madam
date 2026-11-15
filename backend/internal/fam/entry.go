package fam

import (
	"errors"
	"fmt"
	"slices"
	"unicode/utf8"
)

// What a day's entry may hold, apart from the arithmetic: a temperature as
// it arrives, the two lists of tags, and the free text (fam-rules §7 and
// §9.3–§9.5).

// The usual range, 35.00–39.00 °C (fam-rules §9). A temperature outside it
// is still accepted: it is probably a typo (34.20 for 36.20), so she is
// asked whether it is correct. The bounds are an assumption and can be
// tuned.
const (
	UsualMinMilliC MilliC = 35000
	UsualMaxMilliC MilliC = 39000
)

// The accepted Celsius input range, in the hundredths it arrives in. It is
// MinMilliC to MaxMilliC, exactly.
const (
	minHundredthsC HundredthsC = 2000
	maxHundredthsC HundredthsC = 5000
)

var (
	// ErrUnknownTag: a disturbance or symptom that isn't in its list.
	ErrUnknownTag = errors.New("tag is not in the list")
	// ErrRepeatedTag: the same tag given twice.
	ErrRepeatedTag = errors.New("tag given twice")
	// ErrTextTooLong: notes or a secretion description over its limit.
	ErrTextTooLong = errors.New("text is too long")
)

// FromCelsius converts a typed Celsius value to the stored unit
// (fam-rules §9.3). It is exact: one hundredth is ten thousandths, so
// nothing is rounded. Outside 20.00–50.00 °C it returns an error wrapping
// ErrTemperatureOutOfRange, like FromFahrenheit outside the same
// temperatures.
func FromCelsius(c HundredthsC) (MilliC, error) {
	if c < minHundredthsC || c > maxHundredthsC {
		return 0, fmt.Errorf("%w: got %d hundredths of °C", ErrTemperatureOutOfRange, c)
	}
	return MilliC(10 * int(c)), nil
}

// IsUnusual reports whether a stored temperature is outside the usual
// range, 35.00–39.00 °C, so that saving it needs her "yes, it's correct"
// (fam-rules §9.3). The two edges themselves are usual.
//
// It is asked of the stored thousandths, whatever unit she typed. In
// Fahrenheit the edges are then exactly 95.00 and 102.20 °F.
func (m MilliC) IsUnusual() bool {
	return m < UsualMinMilliC || m > UsualMaxMilliC
}

// The two lists of fam-rules §7, in its order, which is the order a list is
// stored in. They are arrays, so nothing can append to them, and they are
// not exported, so nothing outside this package can change one.
//
// The database holds the same lists in two CHECK constraints on entries
// (migration 00004). Changing a list means a new migration and this file.
var (
	disturbances = [...]string{
		"alcohol", "poor_sleep", "illness", "medication", "travel",
		"vacation", "new_thermometer", "uti", "yeast_infection",
	}
	symptoms = [...]string{"breast_tenderness", "bloating", "pms"}
)

// Disturbances returns the tags that mark a reading as disturbed, in
// fam-rules §7's order. The caller gets its own copy.
func Disturbances() []string {
	return slices.Clone(disturbances[:])
}

// Symptoms returns the tags that are only recorded, in fam-rules §7's
// order. The caller gets its own copy.
func Symptoms() []string {
	return slices.Clone(symptoms[:])
}

// CheckDisturbances checks a day's disturbance tags and returns them in
// fam-rules §7's order (§9.4). Any order is accepted. The result is never
// nil: no tags is an empty list. It returns an error wrapping
// ErrUnknownTag for a tag that isn't a disturbance, a symptom included,
// and ErrRepeatedTag for one given twice. tags itself isn't changed.
func CheckDisturbances(tags []string) ([]string, error) {
	return checkTags(tags, disturbances[:])
}

// CheckSymptoms is CheckDisturbances for the symptoms.
func CheckSymptoms(tags []string) ([]string, error) {
	return checkTags(tags, symptoms[:])
}

func checkTags(tags, allowed []string) ([]string, error) {
	given := make(map[string]bool, len(tags))
	for _, tag := range tags {
		if !slices.Contains(allowed, tag) {
			// The tag isn't repeated in the error: it is whatever was sent.
			return nil, ErrUnknownTag
		}
		if given[tag] {
			// This one is safe to name: it is one of ours.
			return nil, fmt.Errorf("%w: %s", ErrRepeatedTag, tag)
		}
		given[tag] = true
	}
	// make, not a nil slice: "none" must be an empty list (see the test).
	ordered := make([]string, 0, len(tags))
	for _, tag := range allowed {
		if given[tag] {
			ordered = append(ordered, tag)
		}
	}
	return ordered, nil
}

// The limits on free text, in characters (fam-rules §9.5). Assumptions,
// and tunable. They exist so that an over-long text is refused about its
// own field, not by the size limit on a whole request.
const (
	MaxNotesLength                = 2000
	MaxSecretionDescriptionLength = 200
)

// CheckNotes reports whether the notes fit: at most MaxNotesLength
// characters. Its error wraps ErrTextTooLong and never repeats the text.
func CheckNotes(text string) error {
	return checkLength(text, MaxNotesLength)
}

// CheckSecretionDescription is CheckNotes for the secretion description,
// with MaxSecretionDescriptionLength.
func CheckSecretionDescription(text string) error {
	return checkLength(text, MaxSecretionDescriptionLength)
}

// checkLength counts characters (Unicode code points), not bytes: a Hebrew
// letter is two bytes and one character.
func checkLength(text string, limit int) error {
	if utf8.RuneCountInString(text) > limit {
		return fmt.Errorf("%w: over %d characters", ErrTextTooLong, limit)
	}
	return nil
}
