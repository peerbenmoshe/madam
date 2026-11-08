package fam

// Stats are fam-rules §8's statistics over completed cycles with a known
// start, all time. The ongoing cycle and the unknown-start cycle are never
// counted.
type Stats struct {
	// Count is N in "based on N cycles": all four statistics count the same
	// completed cycles.
	Count int
	// Latest is the length of the most recently completed cycle.
	Latest   int
	Shortest int
	Longest  int
	// AverageTenths is the mean length in tenths of a day, rounded half-up:
	// 28.3 days is 283.
	AverageTenths int
}

// Stats computes the statistics. ok is false when no cycle has completed,
// which the screen shows as "Not enough data yet".
func (c Cycles) Stats() (s Stats, ok bool) {
	sum := 0
	// All() holds known-start cycles only, oldest first, so the unknown-start
	// cycle is never here and the last completed one seen is the latest.
	for _, cycle := range c.All() {
		if cycle.Ongoing {
			continue
		}
		if s.Count == 0 || cycle.Length < s.Shortest {
			s.Shortest = cycle.Length
		}
		if cycle.Length > s.Longest {
			s.Longest = cycle.Length
		}
		s.Latest = cycle.Length
		sum += cycle.Length
		s.Count++
	}
	if s.Count == 0 {
		return Stats{}, false
	}
	// Half-up to tenths: floor(10 × sum / n + 1/2) = (20 × sum + n) / (2n).
	// Lengths are at least 1 day, so the numerator is positive and Go's
	// truncating division is floor, as in fam-rules §3.3.
	s.AverageTenths = (20*sum + s.Count) / (2 * s.Count)
	return s, true
}
