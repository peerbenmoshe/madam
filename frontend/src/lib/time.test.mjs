// Run by `npm test`. Plain JavaScript for the reason given in date.test.mjs.
import assert from "node:assert/strict";
import { test } from "node:test";

const { isTimeOfDay, timeFromInput } = await import("./time.ts");

// The same rows as docs/fam-rules.md §12.10 (H1 to H12), which the server's
// fam.ParseMinutes is tested against: the page and the server must agree on
// what a time is.
test("isTimeOfDay accepts HH:MM from 00:00 to 23:59, and nothing else", () => {
  for (const good of ["07:05", "00:00", "23:59"]) {
    assert.equal(isTimeOfDay(good), true, good);
  }
  for (const bad of [
    "24:00",
    "07:60",
    "7:05",
    "07:05:00",
    "07.05",
    "0705",
    "7:05 AM",
    "-7:05",
    "",
    // Beyond the table.
    " 7:05",
    "07:5",
    "hh:mm",
    "０7:05",
    "99:00",
    "07:05\n",
  ]) {
    assert.equal(isTimeOfDay(bad), false, JSON.stringify(bad));
  }
});

test("timeFromInput keeps a native time input's HH:MM", () => {
  assert.equal(timeFromInput("07:05"), "07:05");
  assert.equal(timeFromInput("00:00"), "00:00");
  assert.equal(timeFromInput("23:59"), "23:59");
});

test("timeFromInput keeps an empty input empty: she cleared it", () => {
  assert.equal(timeFromInput(""), "");
});

test("timeFromInput drops seconds, which some browsers add", () => {
  assert.equal(timeFromInput("07:05:00"), "07:05");
  assert.equal(timeFromInput("07:05:30"), "07:05");
  assert.equal(timeFromInput("07:05:30.250"), "07:05");
});

test("timeFromInput answers null for anything else, so the box keeps what it had", () => {
  for (const bad of ["7:05", "24:00", "07:60", "7:05 AM", "07:05:", "ab:cd"]) {
    assert.equal(timeFromInput(bad), null, JSON.stringify(bad));
  }
});
