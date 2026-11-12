// Run by `npm test` (node --test). Plain JavaScript on purpose: importing
// "node:test" from TypeScript would need @types/node, a dependency this
// project doesn't have. Node strips the types from date.ts when loading it.

// A time zone 10 hours behind UTC, set before any Date exists. If the
// helpers ever went through local time, midnight UTC would be the evening
// before here, and every expectation below would be a day off.
process.env.TZ = "Pacific/Honolulu";

import assert from "node:assert/strict";
import { test } from "node:test";

const { formatDate, formatDateLong } = await import("./date.ts");

test("the test really runs behind UTC", () => {
  assert.equal(new Date(Date.UTC(2026, 10, 8)).getDate(), 7);
});

test("formatDate is dd/mm/yyyy", () => {
  // The example in docs/spec-v1.md §3.
  assert.equal(formatDate("2026-11-08"), "08/11/2026");
  // Day first, not month first, and both padded.
  assert.equal(formatDate("2026-01-05"), "05/01/2026");
  assert.equal(formatDate("2026-12-31"), "31/12/2026");
  assert.equal(formatDate("2024-02-29"), "29/02/2024");
  assert.equal(formatDate("0099-03-01"), "01/03/0099");
});

test("formatDateLong puts the weekday first", () => {
  // The example in docs/design-brief.md §7.
  assert.equal(formatDateLong("2026-11-08"), "Sunday 08/11/2026");
  const week = [
    ["2026-11-08", "Sunday"],
    ["2026-11-09", "Monday"],
    ["2026-11-10", "Tuesday"],
    ["2026-11-11", "Wednesday"],
    ["2026-11-12", "Thursday"],
    ["2026-11-13", "Friday"],
    ["2026-11-14", "Saturday"],
  ];
  for (const [iso, weekday] of week) {
    assert.equal(formatDateLong(iso), `${weekday} ${formatDate(iso)}`);
  }
  // Across a year's end and a leap day.
  assert.equal(formatDateLong("2027-01-01"), "Friday 01/01/2027");
  assert.equal(formatDateLong("2024-02-29"), "Thursday 29/02/2024");
});

test("anything that isn't a real YYYY-MM-DD date throws", () => {
  const bad = [
    "",
    "08/11/2026",
    "2026-11-8",
    "2026-1-08",
    "26-11-08",
    "2026-11-08T00:00:00Z",
    " 2026-11-08",
    "2026-13-01",
    "2026-00-10",
    "2026-11-00",
    "2026-11-31",
    "2026-02-29",
    "2026-02-30",
  ];
  for (const value of bad) {
    assert.throws(
      () => formatDate(value),
      RangeError,
      `formatDate("${value}")`,
    );
    assert.throws(
      () => formatDateLong(value),
      RangeError,
      `formatDateLong("${value}")`,
    );
  }
});
