package fam

import (
	"errors"
	"fmt"
	"slices"
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
	// hasUnknown is true when an unknown-start cycle exists; it runs from
	// unknownFrom (her first entry) to the day before the first start.
	hasUnknown  bool
	unknownFrom Date
}

// NewCycles validates the facts and prepares them for lookups. Its error
// wraps ErrDuplicateDate if a start date appears twice. It works on a copy,
// so the caller's slice is never reordered.
func NewCycles(f CycleFacts) (Cycles, error) {
	starts := slices.Clone(f.Starts)
	slices.SortFunc(starts, Date.Compare)
	for i := 1; i < len(starts); i++ {
		if starts[i] == starts[i-1] {
			return Cycles{}, fmt.Errorf("%w: cycle start %v", ErrDuplicateDate, starts[i])
		}
	}

	c := Cycles{starts: starts}
	// §5.2: an unknown-start cycle exists when she answered "I don't know"
	// and has entries that no known start covers.
	if f.StartUnknown && f.HasEntries && (len(starts) == 0 || f.FirstEntry.Before(starts[0])) {
		c.hasUnknown = true
		c.unknownFrom = f.FirstEntry
	}
	return c, nil
}

// latestStartIndex returns the index of the greatest start on or before d,
// or -1 if there is none.
func (c Cycles) latestStartIndex(d Date) int {
	// BinarySearchFunc returns d's index if it is a start, otherwise the
	// index of the first start after d, which is one past the one we want.
	i, found := slices.BinarySearchFunc(c.starts, d, Date.Compare)
	if !found {
		i--
	}
	return i
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
	if i := c.latestStartIndex(d); i >= 0 {
		// §5.1: cycle_day = date − (greatest start ≤ date) + 1
		return CycleDay{Kind: InCycle, Day: d.DaysSince(c.starts[i]) + 1, Number: i + 1}
	}
	if c.hasUnknown && !d.Before(c.unknownFrom) {
		return CycleDay{Kind: Unknown}
	}
	return CycleDay{Kind: Outside}
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
	all := make([]Cycle, len(c.starts))
	for i, start := range c.starts {
		all[i] = Cycle{Number: i + 1, Start: start}
		if i+1 < len(c.starts) {
			// §5.1: cycle_length = (next start after s) − s
			all[i].Length = c.starts[i+1].DaysSince(start)
		} else {
			all[i].Ongoing = true
		}
	}
	return all
}

// IsStart reports whether d is a cycle start.
func (c Cycles) IsStart(d Date) bool {
	_, found := slices.BinarySearchFunc(c.starts, d, Date.Compare)
	return found
}
