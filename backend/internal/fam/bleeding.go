package fam

// BleedingLevel is an entry's bleeding value. Its values are the database
// enum bleeding_level's own spellings, so a row converts with a plain type
// conversion: fam.BleedingLevel(row.Bleeding).
type BleedingLevel string

const (
	NoBleeding BleedingLevel = "none"
	Spotting   BleedingLevel = "spotting"
	Bleeding   BleedingLevel = "bleeding"
)

// DayBleeding is one entry's date and bleeding level: all the bleeding rules
// need to know about an entry.
type DayBleeding struct {
	Date  Date
	Level BleedingLevel
}

// SuggestCycleStart reports whether saving day should suggest starting a
// new cycle on its date (fam-rules §6.1). It is a question for the user,
// never an action. isStart says whether day's date is already a cycle start.
//
// history is any set of her other entries. Only the most recent one before
// day's date matters, whatever its date, so the caller may pass just that
// entry or many. Entries on or after day's date are ignored.
func SuggestCycleStart(day DayBleeding, isStart bool, history []DayBleeding) bool {
	return false // stub: tests first
}
