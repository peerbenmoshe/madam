// Run by `npm test`. Plain JavaScript for the reason given in date.test.mjs.
import assert from "node:assert/strict";
import { test } from "node:test";

const {
  HIGHEST_CELSIUS,
  LOWEST_CELSIUS,
  formatTemperature,
  isPossibleCelsius,
  parseTemperature,
} = await import("./temperature.ts");

const value = (hundredths) => ({ kind: "value", hundredths });

test("parseTemperature reads one or two decimals, with a dot or a comma", () => {
  const read = [
    ["36.62", 3662],
    ["36,62", 3662], // a phone set to Hebrew or German types a comma
    ["36.6", 3660], // six tenths, not six hundredths
    ["36,6", 3660],
    ["36.60", 3660],
    ["36.06", 3606],
    ["36.0", 3600],
    ["36", 3600],
    ["36.00", 3600],
    ["97.91", 9791],
    ["102.2", 10220], // three digits, for Fahrenheit later
    ["0.5", 50],
    ["5", 500],
    [" 36.62 ", 3662],
    ["\t36.62\n", 3662],
  ];
  for (const [typed, hundredths] of read) {
    assert.deepEqual(
      parseTemperature(typed),
      value(hundredths),
      JSON.stringify(typed),
    );
  }
});

// The numbers that go wrong as fractions: 36.62 × 100 is 3661.9999999999995
// in JavaScript. Every two-decimal value from 20.00 to 50.00 must come out
// as exactly its hundredths, and go back to exactly what was typed.
test("every value from 20.00 to 50.00 is read exactly and written back the same", () => {
  assert.notEqual(36.62 * 100, 3662, "the trap this test is about");
  for (let hundredths = 2000; hundredths <= 5000; hundredths++) {
    const typed = `${Math.trunc(hundredths / 100)}.${String(hundredths % 100).padStart(2, "0")}`;
    assert.deepEqual(parseTemperature(typed), value(hundredths), typed);
    assert.equal(formatTemperature(hundredths), typed);
  }
});

test("parseTemperature calls nothing typed empty", () => {
  for (const typed of ["", " ", "   ", "\t", "\n"]) {
    assert.deepEqual(
      parseTemperature(typed),
      { kind: "empty" },
      JSON.stringify(typed),
    );
  }
});

test("parseTemperature calls everything else invalid, and never tidies it", () => {
  const invalid = [
    "36.", // still typing
    ".5",
    ",5",
    "36.625", // thousandths: not read, and not rounded
    "36..6",
    "36.6.2",
    "36,6,2",
    "36 .6",
    "3 6.6",
    "+36.6",
    "-36.6",
    "36.6°",
    "36.6 °C",
    "36.6C",
    "thirty-six",
    "3e1",
    "0x24",
    "Infinity",
    "NaN",
    "1000", // four digits
    "1000.5",
    "٣٦.٦", // Arabic-Indic digits
    "３６.６", // full-width digits
    "36·6",
    "36/6",
  ];
  for (const typed of invalid) {
    assert.deepEqual(
      parseTemperature(typed),
      { kind: "invalid" },
      JSON.stringify(typed),
    );
  }
});

// The same two edges as docs/fam-rules.md §12.11: L7 and L8, L9 and L10.
test("isPossibleCelsius is 20.00 to 50.00 °C, edges included", () => {
  assert.equal(LOWEST_CELSIUS, 2000);
  assert.equal(HIGHEST_CELSIUS, 5000);
  assert.equal(isPossibleCelsius(2000), true); // L7
  assert.equal(isPossibleCelsius(1999), false); // L8
  assert.equal(isPossibleCelsius(5000), true); // L9
  assert.equal(isPossibleCelsius(5001), false); // L10
  assert.equal(isPossibleCelsius(3662), true);
  assert.equal(isPossibleCelsius(3420), true); // unusual, but possible: AC-14 is the server's
  assert.equal(isPossibleCelsius(9791), false); // a Fahrenheit number
  assert.equal(isPossibleCelsius(0), false);
});

test("formatTemperature always shows two decimals", () => {
  assert.equal(formatTemperature(3662), "36.62");
  assert.equal(formatTemperature(3660), "36.60");
  assert.equal(formatTemperature(3606), "36.06");
  assert.equal(formatTemperature(3600), "36.00");
  assert.equal(formatTemperature(3655), "36.55"); // AC-1's adjusted value
  assert.equal(formatTemperature(10220), "102.20");
  assert.equal(formatTemperature(50), "0.50");
  assert.equal(formatTemperature(0), "0.00");
});

test("formatTemperature throws for anything that isn't whole hundredths", () => {
  for (const bad of [
    36.62,
    3662.5,
    -3662,
    NaN,
    Infinity,
    "3662",
    null,
    undefined,
  ]) {
    assert.throws(() => formatTemperature(bad), RangeError, String(bad));
  }
});
