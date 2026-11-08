// SPIKE ONLY. Splits the visible days into the runs the temperature line is
// drawn as (docs/fam-rules.md §7, spec-v1.md §9.2):
//   - a day with no temperature (no entry, or an entry without one) ends the
//     current run: the line stops there;
//   - a disturbed day is skipped but leaves the run open: the line steps
//     over it and joins its neighbours;
//   - any other day joins the current run.
// Recharts' connectNulls is one switch per line, so each run becomes its own
// <Line>; inside a run the only blanks are disturbed days, so connectNulls is
// safe there.

export type SegmentInput = { hasTemp: boolean; disturbed: boolean };

// Returns the runs as lists of indexes into `days`, in date order. A run of
// one index draws no line (E7); its dot is drawn by the dot layer anyway.
export function lineSegments(days: readonly SegmentInput[]): number[][] {
  const runs: number[][] = [];
  let current: number[] = [];
  days.forEach((day, i) => {
    if (!day.hasTemp) {
      if (current.length > 0) runs.push(current);
      current = [];
    } else if (!day.disturbed) {
      current.push(i);
    }
    // disturbed with a temperature: stepped over, the run stays open
  });
  if (current.length > 0) runs.push(current);
  return runs;
}
