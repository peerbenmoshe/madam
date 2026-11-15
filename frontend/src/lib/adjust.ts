// What a reading will count as once the server has adjusted it for the
// time it was taken at: the number in the day screen's live line.
//
// THIS IS A COPY of the server's arithmetic (backend/internal/fam,
// docs/fam-rules.md §3.3), and the one piece of domain logic in the page.
// docs/tech-stack.md §1.3 says why it is allowed: the line must follow her
// typing, and nothing it computes is saved or sent. Every value that is
// stored, drawn or shown anywhere else comes from the server.
//
// If the rate or the rounding ever changes, change both. Two tests hold the
// copies together: the vectors of fam-rules §12.1, and a checksum over
// seventeen million inputs whose expected value is written in this
// module's test and in backend/internal/fam/copy_test.go.

import { HIGHEST_CELSIUS, LOWEST_CELSIUS } from "./temperature.ts";

// 00:00 is 0 and 23:59 is 1439.
const MINUTES_PER_DAY = 24 * 60;

// countsAs returns the adjusted temperature, in hundredths of °C, of a raw
// reading (hundredths of °C) measured at one time of day when her usual
// time is another. Both times are minutes after midnight.
//
// Early adds and late subtracts, 0.05 °C per 30 minutes. The reading
// counts as adjusted only if the result differs from raw (§3.4): a minute
// or two usually rounds away.
//
// The arithmetic is fam-rules §3.3's, in whole numbers. One minute is 5/3
// of a thousandth, so the exact value is kept multiplied by 3 and rounded
// once, straight to hundredths. "+ 15, then divide by 30 and drop the
// remainder" rounds half up only while the numerator is positive, and it
// is for every input accepted here: at the least, 3 × 20000 − 5 × 1439 =
// 52805.
//
// Anything outside those inputs is a bug in the caller, so it throws
// rather than show a number.
export function countsAs(
  rawHundredths: number,
  measuredMinutes: number,
  fixedMinutes: number,
): number {
  if (
    !Number.isInteger(rawHundredths) ||
    rawHundredths < LOWEST_CELSIUS ||
    rawHundredths > HIGHEST_CELSIUS
  ) {
    throw new RangeError(`not an accepted temperature: ${rawHundredths}`);
  }
  for (const minutes of [measuredMinutes, fixedMinutes]) {
    if (
      !Number.isInteger(minutes) ||
      minutes < 0 ||
      minutes >= MINUTES_PER_DAY
    ) {
      throw new RangeError(`not a minute of the day: ${minutes}`);
    }
  }
  const rawMilli = rawHundredths * 10;
  const delta = fixedMinutes - measuredMinutes; // positive: measured early
  const numerator = 3 * rawMilli + 5 * delta;
  return Math.trunc((numerator + 15) / 30);
}
