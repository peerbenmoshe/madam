// Run by `npm test`. Plain JavaScript for the reason given in date.test.mjs.
import assert from "node:assert/strict";
import { test } from "node:test";

const {
  START_DATE_NEEDED,
  START_NEEDED,
  TIME_NEEDED,
  onboardingBody,
  onboardingFailure,
  onboardingProblems,
} = await import("./onboardingText.ts");
const { OFFLINE, SESSION_REFUSED, TRY_AGAIN } = await import("./authText.ts");

const TODAY = "2026-11-14";
const known = { start: "known", date: "2026-11-01", time: "07:00" };
const unknown = { start: "unknown", date: "", time: "07:00" };

test("a complete form has no problems", () => {
  assert.deepEqual(onboardingProblems(known, TODAY), {});
  assert.deepEqual(onboardingProblems(unknown, TODAY), {});
  // Today itself, and the floor itself, are allowed.
  assert.deepEqual(onboardingProblems({ ...known, date: TODAY }, TODAY), {});
  assert.deepEqual(
    onboardingProblems({ ...known, date: "2000-01-01" }, TODAY),
    {},
  );
});

test("an untouched form names all that is missing, each under its own control", () => {
  assert.deepEqual(
    onboardingProblems({ start: null, date: "", time: "" }, TODAY),
    { start: START_NEEDED, time: TIME_NEEDED },
  );
});

test("'I know the date' needs the date", () => {
  assert.deepEqual(onboardingProblems({ ...known, date: "" }, TODAY), {
    date: START_DATE_NEEDED,
  });
});

test("'I don't know' needs no date, and ignores one picked earlier", () => {
  assert.deepEqual(
    onboardingProblems({ ...unknown, date: "2030-01-01" }, TODAY),
    {},
  );
});

test("a date after today or before 2000 is named with the limits, dd/mm/yyyy", () => {
  const limits = "Pick a date from 01/01/2000 to 14/11/2026.";
  assert.deepEqual(
    onboardingProblems({ ...known, date: "2026-11-15" }, TODAY),
    {
      date: limits,
    },
  );
  assert.deepEqual(
    onboardingProblems({ ...known, date: "1999-12-31" }, TODAY),
    {
      date: limits,
    },
  );
  // A year typed halfway on a computer.
  assert.deepEqual(
    onboardingProblems({ ...known, date: "0002-11-01" }, TODAY),
    {
      date: limits,
    },
  );
});

test("a time is needed, and must be HH:MM", () => {
  assert.deepEqual(onboardingProblems({ ...known, time: "" }, TODAY), {
    time: TIME_NEEDED,
  });
  assert.deepEqual(onboardingProblems({ ...known, time: "7:05" }, TODAY), {
    time: TIME_NEEDED,
  });
});

// The exact JSON the server reads (backend/internal/api/onboarding.go).
test("the body for a known start", () => {
  assert.deepEqual(onboardingBody(known), {
    cycle_start_unknown: false,
    cycle_start_date: "2026-11-01",
    measurement_time: "07:00",
  });
});

test("the body for 'I don't know' has no date at all, even one picked earlier", () => {
  const body = onboardingBody({ ...unknown, date: "2026-11-01" });
  assert.deepEqual(body, {
    cycle_start_unknown: true,
    measurement_time: "07:00",
  });
  // The server refuses the field's presence, so it must not be there as
  // null or "" either.
  assert.equal("cycle_start_date" in body, false);
});

test("no body for a form without an answer: the page must check first", () => {
  assert.throws(() => onboardingBody({ start: null, date: "", time: "07:00" }));
});

test("already onboarded means go on, not an error", () => {
  assert.deepEqual(
    onboardingFailure({ status: 409, code: "already_onboarded" }),
    { goOn: true },
  );
});

test("the server's refusals of a date, in her words, under the date", () => {
  assert.deepEqual(
    onboardingFailure({
      status: 422,
      code: "date_in_future",
      field: "cycle_start_date",
    }),
    {
      goOn: false,
      field: "date",
      message: "That date is in the future. Check the date on your device.",
    },
  );
  assert.deepEqual(
    onboardingFailure({
      status: 422,
      code: "date_too_early",
      field: "cycle_start_date",
    }),
    { goOn: false, field: "date", message: "Pick 01/01/2000 or a later date." },
  );
  assert.deepEqual(
    onboardingFailure({
      status: 422,
      code: "invalid_date",
      field: "cycle_start_date",
    }),
    { goOn: false, field: "date", message: START_DATE_NEEDED },
  );
  assert.deepEqual(
    onboardingFailure({
      status: 422,
      code: "missing_field",
      field: "cycle_start_date",
    }),
    { goOn: false, field: "date", message: START_DATE_NEEDED },
  );
});

test("the server's refusals of a time, under the time", () => {
  for (const code of ["invalid_time", "missing_field"]) {
    assert.deepEqual(
      onboardingFailure({ status: 422, code, field: "measurement_time" }),
      { goOn: false, field: "time", message: TIME_NEEDED },
    );
  }
});

test("no connection, a refused session, and everything else", () => {
  assert.deepEqual(onboardingFailure({ status: 0 }), {
    goOn: false,
    message: OFFLINE,
  });
  assert.deepEqual(onboardingFailure({ status: 401, code: "unauthorized" }), {
    goOn: false,
    message: SESSION_REFUSED,
  });
  for (const failure of [
    { status: 500, code: "internal" },
    { status: 503, code: "auth_unavailable" },
    { status: 400, code: "bad_request" },
    { status: 400, code: "bad_request", field: "cycle_start_unknown" },
    { status: 422, code: "unexpected_field", field: "cycle_start_date" },
    { status: 409 },
    { status: 409, code: "something_new" },
    { status: 502 },
  ]) {
    assert.deepEqual(
      onboardingFailure(failure),
      { goOn: false, message: TRY_AGAIN },
      JSON.stringify(failure),
    );
  }
});
