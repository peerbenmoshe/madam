// Run by `npm test`. Plain JavaScript for the reason given in date.test.mjs.
import assert from "node:assert/strict";
import { test } from "node:test";

const { countsAs } = await import("./adjust.ts");

// 07:00, the fixed time of every vector in docs/fam-rules.md §12.1.
const SEVEN = 7 * 60;
const at = (hours, minutes) => hours * 60 + minutes;

// The same rows as docs/fam-rules.md §12.1, which the server's fam.Adjust
// is tested against. Raw and expected are hundredths of °C: the table's
// 36620 thousandths is 3662 here.
test("countsAs gives the vectors of fam-rules §12.1", () => {
  const vectors = [
    // id, raw, measured, counts as, adjusted?
    ["T1", 3662, at(7, 45), 3655, true], // AC-1: 36.545 shows as 36.55, half up
    ["T2", 3640, at(6, 30), 3645, true], // AC-2: early adds
    ["T3", 3650, at(7, 1), 3650, false], // AC-3: one minute rounds away
    ["T4", 3670, at(7, 0), 3670, false], // on time
    ["T5", 3662, at(8, 30), 3647, true], // 90 minutes late
    ["T6", 3640, at(5, 0), 3660, true], // two hours early
  ];
  for (const [id, raw, measured, want, adjusted] of vectors) {
    const got = countsAs(raw, measured, SEVEN);
    assert.equal(got, want, id);
    assert.equal(got !== raw, adjusted, `${id}: adjusted?`);
  }
});

// fam-rules §12.2, F3, the Celsius half: three minutes late at 35.00 °C
// doesn't change the displayed value.
test("countsAs leaves 35.00 °C alone three minutes late (F3)", () => {
  assert.equal(countsAs(3500, at(7, 3), SEVEN), 3500);
});

test("early adds and late subtracts, half a tenth per half hour", () => {
  assert.equal(countsAs(3650, at(6, 30), SEVEN), 3655);
  assert.equal(countsAs(3650, at(7, 30), SEVEN), 3645);
  // And it is the fixed time that decides, not 07:00.
  assert.equal(countsAs(3650, at(7, 0), at(6, 30)), 3645);
  assert.equal(countsAs(3650, at(6, 30), at(6, 30)), 3650);
});

// The corners where the numerator is smallest and largest: the lowest
// temperature measured as late as a day allows, and the highest as early.
test("countsAs holds at the corners of what it accepts", () => {
  // 3 × 20000 − 5 × 1439 = 52805; (52805 + 15) / 30 = 1760.67, so 17.60.
  assert.equal(countsAs(2000, at(23, 59), at(0, 0)), 1760);
  // 3 × 50000 + 5 × 1439 = 157195; (157195 + 15) / 30 = 5240.33, so 52.40.
  assert.equal(countsAs(5000, at(0, 0), at(23, 59)), 5240);
});

test("countsAs throws for anything it doesn't accept", () => {
  const bad = [
    [1999, SEVEN, SEVEN], // below 20.00 °C
    [5001, SEVEN, SEVEN], // above 50.00 °C
    [36620, SEVEN, SEVEN], // thousandths, not hundredths
    [36.62, SEVEN, SEVEN], // degrees
    [3662.5, SEVEN, SEVEN],
    [NaN, SEVEN, SEVEN],
    [3662, -1, SEVEN],
    [3662, 1440, SEVEN],
    [3662, SEVEN, 1440],
    [3662, 7.5, SEVEN],
    [3662, "07:45", SEVEN], // the text, not the minutes
    [3662, SEVEN, undefined],
  ];
  for (const [raw, measured, fixed] of bad) {
    assert.throws(
      () => countsAs(raw, measured, fixed),
      RangeError,
      JSON.stringify([raw, measured, fixed]),
    );
  }
});

// The same checksum as backend/internal/fam/copy_test.go computes with the
// server's fam.Adjust: every adjusted value for four fixed times, every
// accepted temperature and every minute of the day, 17,285,760 readings,
// folded into one number.
//
// If this fails, the copy no longer does what the server does. The number
// below must be the same number as adjustChecksum in that Go file. Never
// update one alone.
test("countsAs agrees with the server's arithmetic over every input", () => {
  const ADJUST_CHECKSUM = 863312314;
  let hash = 0;
  let readings = 0;
  for (const fixed of [0, 390, 420, 1439]) {
    for (let raw = 2000; raw <= 5000; raw++) {
      for (let measured = 0; measured < 1440; measured++) {
        // Below 2^53 at every step, so exact: at most 31 × 1,000,000,006
        // plus a temperature.
        hash = (hash * 31 + countsAs(raw, measured, fixed)) % 1_000_000_007;
        readings++;
      }
    }
  }
  assert.equal(readings, 17_285_760);
  assert.equal(hash, ADJUST_CHECKSUM);
});
