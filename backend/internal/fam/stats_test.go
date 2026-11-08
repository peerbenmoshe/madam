package fam

import "testing"

// startsWithLengths returns cycle starts from first, each the given number
// of days after the one before, plus one more start for the ongoing cycle.
func startsWithLengths(first Date, lengths ...int) []Date {
	starts := []Date{first}
	for _, n := range lengths {
		starts = append(starts, starts[len(starts)-1].AddDays(n))
	}
	return starts
}

// TestStats is fam-rules §12.6: completed lengths 28, 30, 27; one ongoing
// cycle; plus an unknown-start cycle (first entry 10 Dec 2025).
func TestStats(t *testing.T) {
	starts := startsWithLengths(date(t, "2026-01-01"), 28, 30, 27)
	withUnknown := CycleFacts{Starts: starts, StartUnknown: true, FirstEntry: date(t, "2025-12-10"), HasEntries: true}
	knownOnly := CycleFacts{Starts: starts}

	stats := func(t *testing.T, f CycleFacts) Stats {
		t.Helper()
		s, ok := cycles(t, f).Stats()
		if !ok {
			t.Fatalf("Stats() ok = false, want true")
		}
		return s
	}

	t.Run("A1", func(t *testing.T) {
		if s := stats(t, withUnknown); s.Latest != 27 || s.Count != 3 {
			t.Errorf("Latest = %d based on %d cycles, want 27 based on 3", s.Latest, s.Count)
		}
	})
	t.Run("A2", func(t *testing.T) {
		if s := stats(t, withUnknown); s.Shortest != 27 {
			t.Errorf("Shortest = %d, want 27", s.Shortest)
		}
	})
	t.Run("A3", func(t *testing.T) {
		if s := stats(t, withUnknown); s.Longest != 30 {
			t.Errorf("Longest = %d, want 30", s.Longest)
		}
	})
	t.Run("A4", func(t *testing.T) {
		if s := stats(t, withUnknown); s.AverageTenths != 283 {
			t.Errorf("AverageTenths = %d, want 283 (28.3)", s.AverageTenths)
		}
	})

	// A5: the ongoing cycle is a fourth cycle in All(), but not in the stats.
	t.Run("A5", func(t *testing.T) {
		c := cycles(t, withUnknown)
		if n := len(c.All()); n != 4 {
			t.Fatalf("All() has %d cycles, want 4 with the ongoing one", n)
		}
		if s := stats(t, withUnknown); s.Count != 3 {
			t.Errorf("Count = %d, want 3: the ongoing cycle is excluded", s.Count)
		}
	})

	// A6: the same starts give the same stats with or without the
	// unknown-start cycle.
	t.Run("A6", func(t *testing.T) {
		if with, without := stats(t, withUnknown), stats(t, knownOnly); with != without {
			t.Errorf("with the unknown-start cycle %+v, without %+v; want equal", with, without)
		}
	})

	t.Run("A7", func(t *testing.T) {
		for _, c := range []struct {
			name  string
			facts CycleFacts
		}{
			{"no cycle starts", CycleFacts{}},
			{"only an ongoing cycle", CycleFacts{Starts: []Date{date(t, "2026-01-01")}}},
			{"only the unknown-start cycle", CycleFacts{StartUnknown: true, FirstEntry: date(t, "2025-12-10"), HasEntries: true}},
			{"the unknown-start cycle and an ongoing one", CycleFacts{
				Starts: []Date{date(t, "2026-01-01")}, StartUnknown: true, FirstEntry: date(t, "2025-12-10"), HasEntries: true,
			}},
		} {
			if s, ok := cycles(t, c.facts).Stats(); ok {
				t.Errorf("%s: Stats() = %+v, ok = true; want not enough data", c.name, s)
			}
		}
	})
}

// TestStatsAverageRoundsHalfUp pins fam-rules §8's half-up rounding at an
// exact half, where it differs from banker's rounding: 28.25 shows as 28.3,
// not 28.2.
func TestStatsAverageRoundsHalfUp(t *testing.T) {
	s, ok := cycles(t, CycleFacts{Starts: startsWithLengths(date(t, "2026-01-01"), 28, 28, 29, 28)}).Stats()
	if !ok || s.AverageTenths != 283 {
		t.Errorf("Stats() = %+v, ok = %v; want AverageTenths 283", s, ok)
	}
}
