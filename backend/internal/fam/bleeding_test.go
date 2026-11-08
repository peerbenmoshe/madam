package fam

import "testing"

// on builds a DayBleeding for test tables: on(t, "2026-03-01", Bleeding).
func on(t *testing.T, s string, level BleedingLevel) DayBleeding {
	t.Helper()
	return DayBleeding{Date: date(t, s), Level: level}
}

// TestSuggestCycleStart is fam-rules §12.4.
func TestSuggestCycleStart(t *testing.T) {
	tests := []struct {
		id      string
		history []DayBleeding
		day     DayBleeding
		isStart bool
		want    bool
	}{
		{"S1", []DayBleeding{on(t, "2026-03-01", NoBleeding)}, on(t, "2026-03-02", Bleeding), false, true}, // AC-5
		// S2 (AC-5): spotting never suggests, whatever came before.
		{"S2 after none", []DayBleeding{on(t, "2026-03-01", NoBleeding)}, on(t, "2026-03-02", Spotting), false, false},
		{"S2 after bleeding", []DayBleeding{on(t, "2026-03-01", Bleeding)}, on(t, "2026-03-02", Spotting), false, false},
		{"S2 with no earlier entry", nil, on(t, "2026-03-02", Spotting), false, false},
		// S3 (AC-6): a missing day mid-period is no reason to suggest.
		{"S3", []DayBleeding{on(t, "2026-03-01", Bleeding)}, on(t, "2026-03-03", Bleeding), false, false},
		// S4: the earlier entry alone would suggest, so only isStart decides.
		{"S4", []DayBleeding{on(t, "2026-03-01", NoBleeding)}, on(t, "2026-03-02", Bleeding), true, false},
		{"S5", nil, on(t, "2026-03-02", Bleeding), false, true},
	}
	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			if got := SuggestCycleStart(tc.day, tc.isStart, tc.history); got != tc.want {
				t.Errorf("SuggestCycleStart(%v, isStart=%v, %v) = %v, want %v", tc.day, tc.isStart, tc.history, got, tc.want)
			}
		})
	}
}

// TestAskBleedingStopped is fam-rules §12.5.
func TestAskBleedingStopped(t *testing.T) {
	tests := []struct {
		id      string
		history []DayBleeding
		day     DayBleeding
		want    bool
	}{
		{"B1", []DayBleeding{on(t, "2026-03-04", Bleeding)}, on(t, "2026-03-05", NoBleeding), true}, // AC-7
		{"B2", []DayBleeding{on(t, "2026-03-04", Bleeding)}, on(t, "2026-03-05", Spotting), true},
		{"B3", []DayBleeding{on(t, "2026-03-04", NoBleeding)}, on(t, "2026-03-05", NoBleeding), false},
		// B4: the gap on 4 Mar doesn't suppress the question.
		{"B4", []DayBleeding{on(t, "2026-03-03", Bleeding)}, on(t, "2026-03-05", NoBleeding), true},
		{"B5", nil, on(t, "2026-03-05", NoBleeding), false},
		// B6: no cut-off, by design.
		{"B6", []DayBleeding{on(t, "2026-03-03", Bleeding)}, on(t, "2026-04-01", NoBleeding), true},
		// Not a §12 vector, but §6.3's condition: saving bleeding itself never asks.
		{"bleeding after bleeding", []DayBleeding{on(t, "2026-03-04", Bleeding)}, on(t, "2026-03-05", Bleeding), false},
	}
	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			if got := AskBleedingStopped(tc.day, tc.history); got != tc.want {
				t.Errorf("AskBleedingStopped(%v, %v) = %v, want %v", tc.day, tc.history, got, tc.want)
			}
		})
	}
}

// TestLookback checks the lookback both bleeding rules share: the most
// recent entry *before* the date, whatever its date. The caller may pass
// more history than that, including the entry being edited and later ones.
func TestLookback(t *testing.T) {
	history := []DayBleeding{
		on(t, "2026-03-05", Bleeding),   // later: ignored
		on(t, "2026-03-01", NoBleeding), // the most recent earlier entry
		on(t, "2026-02-20", Bleeding),   // older: ignored
		on(t, "2026-03-02", Bleeding),   // the entry being edited, as stored before: ignored
	}
	if !SuggestCycleStart(on(t, "2026-03-02", Bleeding), false, history) {
		t.Errorf("SuggestCycleStart = false, want true: the most recent earlier entry (1 Mar) is none")
	}

	history = []DayBleeding{
		on(t, "2026-03-05", NoBleeding), // later: ignored
		on(t, "2026-03-01", Bleeding),   // the most recent earlier entry
		on(t, "2026-03-02", NoBleeding), // the entry being edited: ignored
	}
	if !AskBleedingStopped(on(t, "2026-03-02", NoBleeding), history) {
		t.Errorf("AskBleedingStopped = false, want true: the most recent earlier entry (1 Mar) is bleeding")
	}
}
