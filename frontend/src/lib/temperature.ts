// Temperatures as she types them and as the screen shows them
// (docs/design-brief.md §7, docs/fam-rules.md §9.3).
//
// A temperature here is a whole number of hundredths of a degree: 36.62 °C
// is 3662. That is how the API reads and writes one, and it keeps every
// step in integers. No function in this file goes through a fraction:
// 36.62 as a JavaScript number is 36.6199999…, and multiplying it by 100
// gives 3661.9999999999995.

// What she typed, read as a temperature.
//
//   empty    nothing typed (spaces don't count)
//   invalid  not a number written the way the box reads one
//   value    a number; whether it is a possible temperature is another
//            question (isPossibleCelsius)
export type TypedTemperature =
  | { kind: "empty" }
  | { kind: "invalid" }
  | { kind: "value"; hundredths: number };

// One to three digits, then optionally a dot or a comma and one or two
// more. A phone's decimal keyboard offers whichever of the two its language
// uses, so both are read. [0-9], not \d: only the ten digits an English
// keyboard types (see time.ts).
const TYPED = /^([0-9]{1,3})(?:[.,]([0-9]{1,2}))?$/;

// parseTemperature reads the temperature box.
//
//   "36.62"  3662
//   "36,62"  3662   a comma is a decimal point
//   "36.6"   3660
//   "36"     3600
//   " 36.6 " 3660   spaces around it are dropped
//   "36."    invalid: she is still typing
//   "36.625" invalid: thousandths are not read and not rounded away
//   "+36.6", "3 6.6", "36.6°", "thirty-six"   invalid
export function parseTemperature(typed: string): TypedTemperature {
  const trimmed = typed.trim();
  if (trimmed === "") {
    return { kind: "empty" };
  }
  const match = TYPED.exec(trimmed);
  if (!match) {
    return { kind: "invalid" };
  }
  const whole = Number(match[1]);
  // "6" after the point is six tenths: 60 hundredths.
  const fraction = Number((match[2] ?? "").padEnd(2, "0"));
  return { kind: "value", hundredths: whole * 100 + fraction };
}

// The Celsius values the app accepts, 20.00 to 50.00 °C (docs/fam-rules.md
// §9.3). The server holds the same two numbers (internal/fam) and refuses
// anything outside them; the page checks first so that she is told next to
// the box. Whether a value inside them is *unusual* is the server's to
// decide, and the page doesn't know that range.
export const LOWEST_CELSIUS = 2000;
export const HIGHEST_CELSIUS = 5000;

export function isPossibleCelsius(hundredths: number): boolean {
  return hundredths >= LOWEST_CELSIUS && hundredths <= HIGHEST_CELSIUS;
}

// formatTemperature(3662) is "36.62". Always two decimals
// (docs/design-brief.md §7): 3660 is "36.60", 3600 is "36.00". The unit is
// added by the caller.
//
// A value that isn't a whole number of hundredths, zero or more, is a bug
// in the caller, so it throws rather than display something wrong.
export function formatTemperature(hundredths: number): string {
  if (!Number.isInteger(hundredths) || hundredths < 0) {
    throw new RangeError(`not a whole number of hundredths: ${hundredths}`);
  }
  const whole = Math.trunc(hundredths / 100);
  return `${whole}.${String(hundredths % 100).padStart(2, "0")}`;
}
