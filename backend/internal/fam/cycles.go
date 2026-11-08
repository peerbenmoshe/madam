package fam

import (
	"errors"
	"fmt"
)

// ErrDuplicateDate: the same date given twice where each date must be
// unique (a cycle start, a measurement-time change).
var ErrDuplicateDate = errors.New("date given twice")

// CycleFacts are the stored facts every cycle value derives from
// (fam-rules §5). Nothing derived is stored.
type CycleFacts struct {
	// Starts are the user's cycle_starts dates, in any order.
	Starts []Date
	// StartUnknown is the onboarding answer "I don't know"
	// (users.cycle_start_unknown, fam-rules §5.2).
	StartUnknown bool
	// FirstEntry is the date of the user's earliest entry. It means nothing
	// unless HasEntries is true.
	FirstEntry Date
	HasEntries bool
}

// Cycles answers cycle questions for one user. Build it with NewCycles.
type Cycles struct {
	starts []Date // sorted, unique
}

// NewCycles validates the facts and prepares them for lookups. Its error
// wraps ErrDuplicateDate if a start date appears twice.
func NewCycles(f CycleFacts) (Cycles, error) {
	return Cycles{}, nil // stub: tests first
}

// DayKind says which of fam-rules §5's three states a date is in.
type DayKind int

const (
	// Outside: the date belongs to no cycle. Its cycle day is shown blank (§5.3).
	Outside DayKind = iota
	// Unknown: inside the unknown-start cycle. Its cycle day is shown "?" (§5.2).
	Unknown
	// InCycle: inside a cycle with a known start; Day and Number are set.
	InCycle
)

// String names the kind, so test failures say "unknown", not "1".
func (k DayKind) String() string {
	switch k {
	case Outside:
		return "outside"
	case Unknown:
		return "unknown"
	case InCycle:
		return "in cycle"
	}
	return fmt.Sprintf("DayKind(%d)", int(k))
}

// CycleDay is where a date falls. Day and Number are 0 unless Kind is InCycle.
type CycleDay struct {
	Kind   DayKind
	Day    int // 1 on the start date; no maximum (§5.1)
	Number int // the cycle's position among the starts, from 1
}

// DayOf returns where d falls (fam-rules §5.1–§5.3).
func (c Cycles) DayOf(d Date) CycleDay {
	return CycleDay{} // stub: tests first
}

// Cycle is one cycle with a known start.
type Cycle struct {
	Number int
	Start  Date
	// Length is the number of days until the next start; 0 while Ongoing.
	Length int
	// Ongoing marks the current cycle: the latest start, with nothing after
	// it. It has no length and is excluded from statistics (§5.1).
	Ongoing bool
}

// All returns every cycle with a known start, oldest first. The
// unknown-start cycle is not among them: it has no number and no length.
func (c Cycles) All() []Cycle {
	return nil // stub: tests first
}

// IsStart reports whether d is a cycle start.
func (c Cycles) IsStart(d Date) bool {
	return false // stub: tests first
}
