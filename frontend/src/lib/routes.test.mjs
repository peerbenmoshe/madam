// Run by `npm test`. Plain JavaScript for the reason given in date.test.mjs.
import assert from "node:assert/strict";
import { test } from "node:test";

const { HOME, dayOfPath, dayPath } = await import("./routes.ts");
const { isDate } = await import("./date.ts");

const TODAY = "2026-11-15";

test("a day's address is /day/ and its date", () => {
  assert.equal(dayPath("2026-11-15"), "/day/2026-11-15");
  assert.equal(HOME, "/");
});

test("dayOfPath reads a day's address back", () => {
  assert.equal(dayOfPath("/day/2026-11-15", TODAY), "2026-11-15"); // today
  assert.equal(dayOfPath("/day/2026-11-14", TODAY), "2026-11-14");
  assert.equal(dayOfPath("/day/2020-02-29", TODAY), "2020-02-29"); // a leap day
  assert.equal(dayOfPath("/day/2000-01-01", TODAY), "2000-01-01"); // the floor
  assert.equal(dayOfPath(dayPath("2026-01-01"), TODAY), "2026-01-01");
});

test("dayOfPath answers null for a day the page may not open", () => {
  const closed = [
    "/day/2026-11-16", // tomorrow
    "/day/2027-01-01",
    "/day/1999-12-31", // before the floor
    "/day/0002-11-15",
    "/day/2026-02-30", // no such date
    "/day/2026-13-01",
    "/day/2025-02-29", // not a leap year
  ];
  for (const path of closed) {
    assert.equal(dayOfPath(path, TODAY), null, path);
  }
});

test("dayOfPath answers null for anything that isn't a day's address", () => {
  const others = [
    "/",
    "/day",
    "/day/",
    "/day/today",
    "/day/2026-11-15/",
    "/day/2026-11-15/edit",
    "/day/2026-11-5",
    "/day/15-11-2026",
    "/day/2026-11-15T00:00:00Z",
    "/days/2026-11-15",
    "/api/days/2026-11-15",
    "/day/２０２６-11-15", // full-width digits
    "/day/2026-11-15?x=1",
    "/DAY/2026-11-15",
    "",
  ];
  for (const path of others) {
    assert.equal(dayOfPath(path, TODAY), null, JSON.stringify(path));
  }
});

test("what counts as today moves with the device's date", () => {
  assert.equal(dayOfPath("/day/2026-11-15", "2026-11-14"), null);
  assert.equal(dayOfPath("/day/2026-11-15", "2026-11-15"), "2026-11-15");
});

test("isDate says whether text is a real date, and never throws", () => {
  for (const good of ["2026-11-15", "2024-02-29", "2000-01-01", "0002-11-01"]) {
    assert.equal(isDate(good), true, good);
  }
  for (const bad of [
    "2026-02-30",
    "2026-13-01",
    "2026-2-3",
    "",
    "today",
    "2026-11-15 ",
  ]) {
    assert.equal(isDate(bad), false, JSON.stringify(bad));
  }
});
