// SPIKE ONLY (branch spike/graph, never merged). Fake data for the graph
// spike: 90 calendar days, 11 Aug – 8 Nov 2026, with every edge case from the
// plan placed on a known date. The arithmetic below copies docs/fam-rules.md
// §3.3–§3.5 so the numbers match §12's vectors; the real implementation is
// Slice 1's, in internal/fam. Nothing here is a rule.

export const FIXED_MINUTES = 7 * 60; // 07:00 throughout
export const LAST_DATE = "2026-11-08";
export const DAYS = 90;
export const CYCLE_STARTS = ["2026-08-20", "2026-09-17", "2026-10-29"];

// What the API would send for one calendar day, everything derived already
// computed (CLAUDE.md: derived on read, never stored).
export type Day = {
  date: string; // YYYY-MM-DD
  hasEntry: boolean;
  rawMilli: number | null; // thousandths of °C
  measured: string | null; // HH:MM
  disturbances: string[];
  cases: string[]; // which edge cases (E1…E18) this day carries
  // Derived:
  cycleDay: number | null; // null = before the first known start
  rawC: number | null; // display hundredths
  adjC: number | null;
  rawF: number | null;
  adjF: number | null;
  adjusted: boolean; // display adjusted ≠ display raw (§3.4)
  delta: number | null; // fixed − measured, minutes; positive = early
};

type Override = {
  missing?: true; // no entry at all
  rawMilli?: number | null; // null = an entry without a temperature
  measured?: string;
  disturbances?: string[];
  cases: string[];
};

// The hand-placed days. Everything else is a generated on-time reading.
const OVERRIDES: Record<string, Override> = {
  // E15: 11–19 Aug are before the first cycle start (20 Aug).
  "2026-08-11": { cases: ["E15"] },
  // E16: a fever, tagged illness, which stretches the Y range in the 90-day view.
  "2026-09-05": { rawMilli: 38900, disturbances: ["illness"], cases: ["E16"] },
  // E13: a cycle start on a day with no entry.
  "2026-09-17": { missing: true, cases: ["E13"] },
  // F2: 98.60 °F is 37000 milli.
  "2026-10-10": { rawMilli: 37000, cases: ["F2"] },
  // E2: normal, disturbed, disturbed, normal.
  "2026-10-14": { disturbances: ["alcohol"], cases: ["E2"] },
  "2026-10-15": { disturbances: ["travel"], cases: ["E2"] },
  // E4: a run of three missing days.
  "2026-10-18": { missing: true, cases: ["E4"] },
  "2026-10-19": { missing: true, cases: ["E4"] },
  "2026-10-20": { missing: true, cases: ["E4"] },
  // E8: first day of the 14-day window is disturbed; 25 Oct, outside the
  // window, is a normal reading (the line does not enter from the edge).
  "2026-10-26": { disturbances: ["poor_sleep"], cases: ["E8"] },
  // E14: 27 and 28 Oct are cycle days 41 and 42.
  "2026-10-27": { cases: ["E1", "E14"] },
  // E1: normal, disturbed, normal.
  "2026-10-28": { disturbances: ["alcohol"], cases: ["E1", "E14"] },
  "2026-10-29": { cases: ["E1"] },
  // E3: one missing day.
  "2026-10-30": { missing: true, cases: ["E3"] },
  // E10 = T1: 36.62 at 07:45 → 36.55, a ghost at 36.62.
  "2026-10-31": { rawMilli: 36620, measured: "07:45", cases: ["E10"] },
  // E11 = T3: 36.50 at 07:01 → 36.50, no ghost. Also E6's first normal day.
  "2026-11-01": { rawMilli: 36500, measured: "07:01", cases: ["E11", "E6"] },
  // E12 = T2: 36.40 at 06:30 → 36.45, disturbed: ring and ghost. E6 continues.
  "2026-11-02": {
    rawMilli: 36400,
    measured: "06:30",
    disturbances: ["illness"],
    cases: ["E12", "E6"],
  },
  // E6: …then a missing day, so the line must not reach 5 Nov.
  "2026-11-03": { missing: true, cases: ["E6"] },
  // E5 + E18: an entry without a temperature, with a disturbance tag.
  "2026-11-04": {
    rawMilli: null,
    disturbances: ["travel"],
    cases: ["E5", "E18"],
  },
  // E7: a lone reading between two gaps.
  "2026-11-05": { rawMilli: 36450, cases: ["E7"] },
  "2026-11-06": { missing: true, cases: ["E7"] },
  // T5: 36.62 at 08:30 → 36.47, a large late adjustment.
  "2026-11-07": { rawMilli: 36620, measured: "08:30", cases: ["T5"] },
  // E9 + F1: last day of every window; 36617 milli displays as 97.91 °F.
  "2026-11-08": { rawMilli: 36617, cases: ["E9", "F1"] },
};

function addDays(date: string, n: number): string {
  const d = new Date(`${date}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() + n);
  return d.toISOString().slice(0, 10);
}

function daysBetween(a: string, b: string): number {
  return Math.round(
    (Date.parse(`${b}T00:00:00Z`) - Date.parse(`${a}T00:00:00Z`)) / 86_400_000,
  );
}

function minutes(hhmm: string): number {
  const [h, m] = hhmm.split(":").map(Number);
  return h * 60 + m;
}

// fam-rules §5.1: date − greatest start ≤ date + 1; null before the first.
function cycleDayOf(date: string): number | null {
  const start = CYCLE_STARTS.filter((s) => s <= date).at(-1);
  return start === undefined ? null : daysBetween(start, date) + 1;
}

// A plausible two-phase pattern, no randomness: low before ~day 15, high after.
function generatedMilli(date: string, index: number): number {
  const cd = cycleDayOf(date) ?? 8;
  const base = cd <= 15 ? 36420 : 36880;
  const wiggle = ((index * 37) % 7) - 3; // −3…+3
  return base + wiggle * 30;
}

function buildDay(date: string, index: number): Day {
  const o: Override = OVERRIDES[date] ?? { cases: [] };
  const missing = o.missing === true;
  const rawMilli = missing
    ? null
    : o.rawMilli !== undefined
      ? o.rawMilli
      : generatedMilli(date, index);
  const measured = rawMilli === null ? null : (o.measured ?? "07:00");
  const base = {
    date,
    hasEntry: !missing,
    rawMilli,
    measured,
    disturbances: o.disturbances ?? [],
    cases: o.cases,
    cycleDay: cycleDayOf(date),
  };
  if (rawMilli === null || measured === null) {
    return {
      ...base,
      rawC: null,
      adjC: null,
      rawF: null,
      adjF: null,
      adjusted: false,
      delta: null,
    };
  }
  // fam-rules §3.3: the exact adjusted value × 3, rounded once, half-up.
  const delta = FIXED_MINUTES - minutes(measured);
  const numerator = 3 * rawMilli + 5 * delta;
  const adjC = Math.floor((numerator + 15) / 30);
  const rawC = Math.floor((rawMilli + 5) / 10); // §3.4
  // §3.5: °F from the exact value, never from rounded °C. In hundredths of
  // °F: (9 × milli / 5 + 32000) / 10 = (9 × milli + 160000) / 50, half-up.
  const rawF = Math.floor((9 * rawMilli + 160000 + 25) / 50);
  const adjF = Math.floor((3 * numerator + 160000 + 25) / 50);
  return { ...base, rawC, adjC, rawF, adjF, adjusted: adjC !== rawC, delta };
}

export const ALL_DAYS: Day[] = Array.from({ length: DAYS }, (_, i) =>
  buildDay(addDays(LAST_DATE, i - (DAYS - 1)), i),
);

// What each case must look like (the plan's table), shown on the page.
export const CHECKLIST: { id: string; where: string; expect: string }[] = [
  {
    id: "E1",
    where: "27–29 Oct",
    expect: "ring on 28 Oct; line joins 27 to 29",
  },
  {
    id: "E2",
    where: "13–16 Oct (30 d)",
    expect: "two rings; line bridges both",
  },
  { id: "E3", where: "30 Oct", expect: "line stops at 29, restarts 31" },
  { id: "E4", where: "18–20 Oct (30 d)", expect: "a wide gap" },
  {
    id: "E5",
    where: "4 Nov",
    expect: "entry without temperature: gap, no dot",
  },
  {
    id: "E6",
    where: "1–5 Nov",
    expect: "line ends at 1 Nov; ring on 2 Nov stands alone",
  },
  { id: "E7", where: "5 Nov", expect: "dot only, no line" },
  {
    id: "E8",
    where: "26 Oct (14 d)",
    expect: "ring at left edge; line starts 27 Oct",
  },
  { id: "E9", where: "8 Nov", expect: "dot at right edge, not clipped" },
  {
    id: "E10",
    where: "31 Oct",
    expect: "dot 36.55; ghost 36.62 only with ghosts on",
  },
  { id: "E11", where: "1 Nov", expect: "no ghost, even with ghosts on" },
  { id: "E12", where: "2 Nov", expect: "ring at 36.45, ghost at 36.40" },
  { id: "E13", where: "17 Sep (90 d)", expect: "cycle-start line on a gap" },
  { id: "E14", where: "27–28 Oct", expect: "ticks show cycle day 41, 42" },
  {
    id: "E15",
    where: "11–19 Aug (90 d)",
    expect: "cycle day blank (known) or ? (unknown)",
  },
  {
    id: "E16",
    where: "5 Sep (90 d)",
    expect: "ring at 38.90; Y range stretches",
  },
  {
    id: "E17",
    where: "°F",
    expect: "axis, dots, popup in °F; 8 Nov shows 97.91",
  },
  {
    id: "E18",
    where: "4 Nov",
    expect: "tag on a no-temperature day: gap, not a ring",
  },
];
