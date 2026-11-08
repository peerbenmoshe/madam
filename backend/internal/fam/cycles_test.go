package fam

import (
	"errors"
	"testing"
)

// cycles builds a Cycles for test tables, failing the test on an error.
func cycles(t *testing.T, f CycleFacts) Cycles {
	t.Helper()
	c, err := NewCycles(f)
	if err != nil {
		t.Fatalf("NewCycles: unexpected error %v", err)
	}
	return c
}

// TestCycles is fam-rules §12.3. Cycle starts 1 Jan, 29 Jan and 26 Feb 2026;
// 20 Dec is in 2025, the year before.
func TestCycles(t *testing.T) {
	starts := []Date{date(t, "2026-01-01"), date(t, "2026-01-29"), date(t, "2026-02-26")}
	known := CycleFacts{Starts: starts}
	// C11–C13: the same starts, but onboarding answered "I don't know" and
	// her first entry is 10 Dec 2025.
	unknown := CycleFacts{Starts: starts, StartUnknown: true, FirstEntry: date(t, "2025-12-10"), HasEntries: true}
	// C14: "I don't know", and no cycle start yet.
	noStarts := CycleFacts{StartUnknown: true, FirstEntry: date(t, "2025-12-10"), HasEntries: true}

	days := []struct {
		id    string
		facts CycleFacts
		date  string
		want  CycleDay
	}{
		{"C1", known, "2026-01-01", CycleDay{InCycle, 1, 1}},
		{"C2", known, "2026-01-15", CycleDay{InCycle, 15, 1}},
		{"C3", known, "2026-01-28", CycleDay{InCycle, 28, 1}},
		{"C4", known, "2026-01-29", CycleDay{InCycle, 1, 2}},
		{"C8", known, "2025-12-20", CycleDay{Kind: Outside}},     // blank, no cycle
		{"C9", known, "2026-04-11", CycleDay{InCycle, 45, 3}},    // day 45, not an error
		{"C11", unknown, "2025-12-20", CycleDay{Kind: Unknown}},  // "?"
		{"C12", unknown, "2025-12-05", CycleDay{Kind: Outside}},  // before her first entry
		{"C14", noStarts, "2026-01-15", CycleDay{Kind: Unknown}}, // AC-12
	}
	for _, tc := range days {
		t.Run(tc.id, func(t *testing.T) {
			if got := cycles(t, tc.facts).DayOf(date(t, tc.date)); got != tc.want {
				t.Errorf("DayOf(%s) = %+v, want %+v", tc.date, got, tc.want)
			}
		})
	}

	t.Run("C5", func(t *testing.T) {
		all := cycles(t, known).All()
		if len(all) != 3 {
			t.Fatalf("All() has %d cycles, want 3", len(all))
		}
		if all[0].Length != 28 || all[0].Ongoing {
			t.Errorf("cycle starting 1 Jan = %+v, want length 28, completed", all[0])
		}
	})

	t.Run("C6", func(t *testing.T) {
		all := cycles(t, known).All()
		if len(all) != 3 {
			t.Fatalf("All() has %d cycles, want 3", len(all))
		}
		if last := all[2]; last.Start != date(t, "2026-02-26") || !last.Ongoing || last.Length != 0 {
			t.Errorf("cycle starting 26 Feb = %+v, want ongoing with no length", last)
		}
	})

	t.Run("C7", func(t *testing.T) {
		all := cycles(t, known).All()
		if len(all) != 3 {
			t.Fatalf("All() has %d cycles, want 3", len(all))
		}
		if all[2].Number != 3 {
			t.Errorf("cycle starting 26 Feb has number %d, want 3", all[2].Number)
		}
	})

	// C10 (AC-8): inserting a start at 20 Dec renumbers everything. The
	// starts are given out of order on purpose: order mustn't matter.
	t.Run("C10", func(t *testing.T) {
		c := cycles(t, CycleFacts{Starts: append([]Date{date(t, "2025-12-20")}, starts...)})
		want := []Cycle{
			{1, date(t, "2025-12-20"), 12, false},
			{2, date(t, "2026-01-01"), 28, false},
			{3, date(t, "2026-01-29"), 28, false},
			{4, date(t, "2026-02-26"), 0, true},
		}
		all := c.All()
		if len(all) != len(want) {
			t.Fatalf("All() = %+v, want %+v", all, want)
		}
		for i := range want {
			if all[i] != want[i] {
				t.Errorf("cycle %d = %+v, want %+v", i+1, all[i], want[i])
			}
		}
		if got, want := c.DayOf(date(t, "2025-12-20")), (CycleDay{InCycle, 1, 1}); got != want {
			t.Errorf("DayOf(20 Dec) = %+v, want %+v", got, want)
		}
		if got, want := c.DayOf(date(t, "2026-01-15")), (CycleDay{InCycle, 15, 2}); got != want {
			t.Errorf("DayOf(15 Jan) = %+v, want %+v", got, want)
		}
	})

	t.Run("C13", func(t *testing.T) {
		c := cycles(t, unknown)
		all := c.All()
		if len(all) != 3 || all[0].Number != 1 || all[0].Start != date(t, "2026-01-01") {
			t.Errorf("All() = %+v, want 3 cycles, the first numbered 1 starting 1 Jan", all)
		}
		if got := c.DayOf(date(t, "2026-01-01")).Number; got != 1 {
			t.Errorf("DayOf(1 Jan).Number = %d, want 1", got)
		}
	})
}

// TestCyclesInputs checks what NewCycles accepts and refuses, and IsStart.
func TestCyclesInputs(t *testing.T) {
	t.Run("a duplicate start is refused", func(t *testing.T) {
		d := date(t, "2026-01-01")
		if _, err := NewCycles(CycleFacts{Starts: []Date{d, d}}); !errors.Is(err, ErrDuplicateDate) {
			t.Errorf("error = %v, want %v", err, ErrDuplicateDate)
		}
	})

	t.Run("the caller's slice is not reordered", func(t *testing.T) {
		in := []Date{date(t, "2026-02-26"), date(t, "2026-01-01")}
		cycles(t, CycleFacts{Starts: in})
		if in[0] != date(t, "2026-02-26") {
			t.Errorf("NewCycles sorted the caller's slice: %v", in)
		}
	})

	t.Run("IsStart", func(t *testing.T) {
		c := cycles(t, CycleFacts{Starts: []Date{date(t, "2026-01-01")}})
		if !c.IsStart(date(t, "2026-01-01")) || c.IsStart(date(t, "2026-01-02")) {
			t.Errorf("IsStart is wrong around 1 Jan")
		}
	})

	t.Run("no unknown-start cycle without entries", func(t *testing.T) {
		c := cycles(t, CycleFacts{StartUnknown: true})
		if got := c.DayOf(date(t, "2026-01-15")); got.Kind != Outside {
			t.Errorf("DayOf = %+v, want outside", got)
		}
	})

	t.Run("no unknown-start cycle when the first entry is on or after the first start", func(t *testing.T) {
		c := cycles(t, CycleFacts{
			Starts:       []Date{date(t, "2026-01-01")},
			StartUnknown: true, FirstEntry: date(t, "2026-01-01"), HasEntries: true,
		})
		if got := c.DayOf(date(t, "2025-12-31")); got.Kind != Outside {
			t.Errorf("DayOf(31 Dec) = %+v, want outside", got)
		}
	})
}
