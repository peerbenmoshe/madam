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
	return Stats{}, false // stub: tests first
}
