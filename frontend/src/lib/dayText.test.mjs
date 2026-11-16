// Run by `npm test`. Plain JavaScript for the reason given in date.test.mjs.
import assert from "node:assert/strict";
import { test } from "node:test";

const {
  AMOUNT_OPTIONS,
  DELETE,
  DELETED,
  DESCRIPTION_TOO_LONG,
  DISTURBANCE_OPTIONS,
  FIRMNESS_OPTIONS,
  INTERCOURSE_OPTIONS,
  POSITION_OPTIONS,
  SENSATION_OPTIONS,
  SYMPTOM_OPTIONS,
  amountFrom,
  amountText,
  deleteQuestion,
  hasExtraValues,
  NO_ENTRY_YET,
  NOTES_TOO_LONG,
  TEMPERATURE_IMPOSSIBLE,
  TEMPERATURE_NOT_A_NUMBER,
  TIME_NEEDED,
  TITLE_TODAY,
  dateProblem,
  dayBody,
  dayFailure,
  dayFormFrom,
  dayProblems,
  dayTitle,
  liveLine,
  otherDayNotice,
  questionFor,
} = await import("./dayText.ts");
const { OFFLINE, SESSION_REFUSED, TRY_AGAIN } = await import("./authText.ts");

// Every field filled in, as the server answers it.
const FULL_ENTRY = {
  temperature: { hundredths: 3662, unit: "celsius" },
  measured_at: "07:45",
  bleeding: "spotting",
  secretion_category: "clear_stretchy",
  sensation: "wet",
  secretion_amount: 2,
  secretion_description: "clear, stretches",
  cervix_firmness: "soft",
  cervix_position: "high_open",
  disturbances: ["alcohol", "poor_sleep"],
  symptoms: ["bloating", "pms"],
  intercourse: "protected",
  notes: "line one\nline two",
};
const FULL_DAY = { date: "2026-11-14", fixed_time: "07:00", entry: FULL_ENTRY };
const EMPTY_DAY = { date: "2026-11-14", fixed_time: "06:30", entry: null };

test("a day with no entry is a blank form, measured at her usual time", () => {
  assert.deepEqual(dayFormFrom(EMPTY_DAY), {
    temperature: "",
    measuredAt: "06:30",
    bleeding: "none",
    secretionCategory: null,
    disturbances: [],
    notes: "",
    sensation: null,
    secretionAmount: null,
    secretionDescription: "",
    cervixFirmness: null,
    cervixPosition: null,
    symptoms: [],
    intercourse: null,
  });
});

test("an entry fills the form, its temperature with two decimals", () => {
  const form = dayFormFrom(FULL_DAY);
  assert.equal(form.temperature, "36.62");
  assert.equal(form.measuredAt, "07:45"); // its own time, not the usual one
  assert.equal(form.bleeding, "spotting");
  assert.equal(form.notes, "line one\nline two");
  assert.equal(
    dayFormFrom({
      ...FULL_DAY,
      entry: {
        ...FULL_ENTRY,
        temperature: { hundredths: 3660, unit: "celsius" },
      },
    }).temperature,
    "36.60",
  );
});

test("an entry without a temperature starts its time at the usual one", () => {
  const day = {
    ...FULL_DAY,
    entry: { ...FULL_ENTRY, temperature: null, measured_at: null },
  };
  const form = dayFormFrom(day);
  assert.equal(form.temperature, "");
  assert.equal(form.measuredAt, "07:00");
});

// Saving replaces the whole entry. So every field the screen doesn't show
// yet must go back exactly as it came, or saving would wipe it.
test("an entry read, untouched and saved goes back exactly as it came", () => {
  assert.deepEqual(dayBody(dayFormFrom(FULL_DAY), []), {
    entry: FULL_ENTRY,
    confirmed: [],
  });
  const bare = {
    temperature: null,
    measured_at: null,
    bleeding: "none",
    secretion_category: null,
    sensation: null,
    secretion_amount: null,
    secretion_description: null,
    cervix_firmness: null,
    cervix_position: null,
    disturbances: [],
    symptoms: [],
    intercourse: null,
    notes: null,
  };
  assert.deepEqual(
    dayBody(dayFormFrom({ ...EMPTY_DAY, entry: bare }), []).entry,
    bare,
  );
});

test("a blank form becomes an entry with nothing but bleeding None", () => {
  const body = dayBody(dayFormFrom(EMPTY_DAY), ["no_temperature"]);
  assert.deepEqual(body, {
    entry: {
      temperature: null,
      measured_at: null, // not the usual time: no reading, no time
      bleeding: "none",
      secretion_category: null,
      sensation: null,
      secretion_amount: null,
      secretion_description: null,
      cervix_firmness: null,
      cervix_position: null,
      disturbances: [],
      symptoms: [],
      intercourse: null,
      notes: null,
    },
    confirmed: ["no_temperature"],
  });
});

test("a typed temperature is sent as hundredths of Celsius, with its time", () => {
  const form = {
    ...dayFormFrom(EMPTY_DAY),
    temperature: "36,6",
    measuredAt: "07:10",
  };
  const { entry } = dayBody(form, []);
  assert.deepEqual(entry.temperature, { hundredths: 3660, unit: "celsius" });
  assert.equal(entry.measured_at, "07:10");
  // Spaces around it are fine.
  assert.equal(
    dayBody({ ...form, temperature: " 36.62 " }, []).entry.temperature
      .hundredths,
    3662,
  );
});

test("empty text is sent as nothing, and text is sent as typed", () => {
  const form = { ...dayFormFrom(EMPTY_DAY), notes: "  two\nlines  " };
  assert.equal(dayBody(form, []).entry.notes, "  two\nlines  ");
  assert.equal(dayBody({ ...form, notes: "" }, []).entry.notes, null);
});

test("dayBody refuses a temperature that isn't a number", () => {
  assert.throws(() =>
    dayBody({ ...dayFormFrom(EMPTY_DAY), temperature: "36." }, []),
  );
});

test("a Fahrenheit entry isn't shown with °C beside it", () => {
  assert.throws(() =>
    dayFormFrom({
      ...FULL_DAY,
      entry: {
        ...FULL_ENTRY,
        temperature: { hundredths: 9791, unit: "fahrenheit" },
      },
    }),
  );
});

test("dayProblems says only what the page can know", () => {
  const blank = dayFormFrom(EMPTY_DAY);
  // No temperature is a question for the server, not a problem here.
  assert.deepEqual(dayProblems(blank), {});
  assert.deepEqual(dayProblems({ ...blank, temperature: "36." }), {
    temperature: TEMPERATURE_NOT_A_NUMBER,
  });
  assert.deepEqual(dayProblems({ ...blank, temperature: "36.625" }), {
    temperature: TEMPERATURE_NOT_A_NUMBER,
  });
  assert.deepEqual(dayProblems({ ...blank, temperature: "19.99" }), {
    temperature: TEMPERATURE_IMPOSSIBLE,
  });
  assert.deepEqual(dayProblems({ ...blank, temperature: "50.01" }), {
    temperature: TEMPERATURE_IMPOSSIBLE,
  });
  // 34.20 is unusual, and that is the server's question, not a problem.
  assert.deepEqual(dayProblems({ ...blank, temperature: "34.20" }), {});
  assert.deepEqual(dayProblems({ ...blank, temperature: "20.00" }), {});
  assert.deepEqual(dayProblems({ ...blank, temperature: "50" }), {});
  // A time is needed only with a temperature.
  assert.deepEqual(dayProblems({ ...blank, measuredAt: "" }), {});
  assert.deepEqual(
    dayProblems({ ...blank, temperature: "36.6", measuredAt: "" }),
    {
      measuredAt: TIME_NEEDED,
    },
  );
});

test("the date box can open today and earlier days, back to 01/01/2000", () => {
  const today = "2026-11-15";
  assert.equal(dateProblem("2026-11-15", today), null);
  assert.equal(dateProblem("2000-01-01", today), null);
  assert.equal(
    dateProblem("2026-11-16", today),
    "Pick a date from 01/01/2000 to 15/11/2026.",
  );
  assert.equal(
    dateProblem("1999-12-31", today),
    "Pick a date from 01/01/2000 to 15/11/2026.",
  );
  assert.equal(dateProblem("", today), "Pick a date.");
});

test("the title is Today's entry today, and the date otherwise", () => {
  assert.equal(dayTitle("2026-11-15", "2026-11-15"), TITLE_TODAY);
  assert.equal(dayTitle("2026-11-14", "2026-11-15"), "Saturday 14/11/2026");
  assert.equal(NO_ENTRY_YET, "No entry yet.");
  assert.equal(
    otherDayNotice("2026-11-14", "2026-11-15"),
    "This is the entry for Saturday 14/11/2026. Today is Sunday 15/11/2026.",
  );
});

test("the disturbances are in fam-rules §7's order, all nine", () => {
  assert.deepEqual(
    DISTURBANCE_OPTIONS.map((o) => o.value),
    [
      "alcohol",
      "poor_sleep",
      "illness",
      "medication",
      "travel",
      "vacation",
      "new_thermometer",
      "uti",
      "yeast_infection",
    ],
  );
});

test("the extra fields' answers are the server's values, symptoms in §7's order", () => {
  const values = (options) => options.map((o) => o.value);
  assert.deepEqual(values(SENSATION_OPTIONS), ["dry", "creamy", "wet"]);
  assert.deepEqual(values(FIRMNESS_OPTIONS), ["firm", "soft"]);
  assert.deepEqual(values(POSITION_OPTIONS), ["high_open", "low_closed"]);
  assert.deepEqual(values(SYMPTOM_OPTIONS), [
    "breast_tenderness",
    "bloating",
    "pms",
  ]);
  assert.deepEqual(values(INTERCOURSE_OPTIONS), [
    "none",
    "protected",
    "unprotected",
  ]);
  assert.deepEqual(values(AMOUNT_OPTIONS), ["1", "2", "3"]);
});

test("the amount travels as a number and is chosen as text", () => {
  for (const n of [1, 2, 3]) {
    assert.equal(amountText(n), String(n));
    assert.equal(amountFrom(String(n)), n);
  }
  assert.equal(amountText(null), null);
  assert.equal(amountFrom(null), null);
  // No button for it: none shows chosen.
  assert.equal(amountText(0), null);
  assert.equal(amountText(4), null);
});

test("Show all fields starts open only when something under it is said", () => {
  const blank = dayFormFrom(EMPTY_DAY);
  assert.equal(hasExtraValues(blank), false);
  // The quick section's own fields don't open it.
  assert.equal(
    hasExtraValues({
      ...blank,
      temperature: "36.5",
      bleeding: "bleeding",
      secretionCategory: "milky",
      disturbances: ["alcohol"],
      notes: "x",
    }),
    false,
  );
  // Each extra field alone does.
  for (const change of [
    { sensation: "dry" },
    { secretionAmount: 1 },
    { secretionDescription: "x" },
    { cervixFirmness: "firm" },
    { cervixPosition: "low_closed" },
    { symptoms: ["pms"] },
    { intercourse: "none" },
  ]) {
    assert.equal(
      hasExtraValues({ ...blank, ...change }),
      true,
      JSON.stringify(change),
    );
  }
  assert.equal(hasExtraValues(dayFormFrom(FULL_DAY)), true);
});

test("the delete question, in the words of design-brief §6.3", () => {
  assert.deepEqual(deleteQuestion("2026-11-14"), {
    question: "Delete the entry for 14/11/2026? This can't be undone.",
    yes: "Delete entry",
    no: "Keep it",
  });
  assert.equal(DELETE, "Delete entry");
  assert.equal(DELETED, "Entry deleted");
});

test("the two questions, in the words of design-brief §7", () => {
  const form = { ...dayFormFrom(EMPTY_DAY), temperature: "34,2" };
  assert.deepEqual(questionFor("no_temperature", form), {
    name: "no_temperature",
    question: "Save without a temperature?",
    yes: "Save without",
    no: "Add a temperature",
  });
  // AC-14's own example.
  assert.deepEqual(questionFor("unusual_temperature", form), {
    name: "unusual_temperature",
    question: "34.20 °C is unusual. Is it correct?",
    yes: "Yes, save it",
    no: "No, fix it",
  });
});

test("dayFailure: what each answer of the server means for her", () => {
  const read = (status, code, field) => dayFailure({ status, code, field });
  assert.deepEqual(read(409, "no_temperature", "temperature"), {
    kind: "ask",
    name: "no_temperature",
  });
  assert.deepEqual(read(409, "unusual_temperature", "temperature"), {
    kind: "ask",
    name: "unusual_temperature",
  });
  assert.deepEqual(read(409, "not_onboarded"), { kind: "notOnboarded" });
  assert.deepEqual(read(422, "temperature_out_of_range", "temperature"), {
    kind: "message",
    field: "temperature",
    message: TEMPERATURE_IMPOSSIBLE,
  });
  assert.deepEqual(read(422, "date_in_future", "date"), {
    kind: "message",
    field: "date",
    message: "That date is in the future. Check the date on your device.",
  });
  assert.deepEqual(read(422, "date_too_early", "date"), {
    kind: "message",
    field: "date",
    message: "Pick 01/01/2000 or a later date.",
  });
  assert.deepEqual(read(422, "invalid_time", "measured_at"), {
    kind: "message",
    field: "measuredAt",
    message: TIME_NEEDED,
  });
  assert.deepEqual(read(422, "missing_field", "measured_at"), {
    kind: "message",
    field: "measuredAt",
    message: TIME_NEEDED,
  });
  assert.deepEqual(read(422, "too_long", "notes"), {
    kind: "message",
    field: "notes",
    message: NOTES_TOO_LONG,
  });
  assert.deepEqual(read(422, "too_long", "secretion_description"), {
    kind: "message",
    field: "secretionDescription",
    message: DESCRIPTION_TOO_LONG,
  });
  assert.deepEqual(read(0), { kind: "message", message: OFFLINE });
  assert.deepEqual(read(401, "unauthorized"), {
    kind: "message",
    message: SESSION_REFUSED,
  });
  // Anything she can't correct is "try again", and never the server's words.
  for (const [status, code, field] of [
    [500, "internal"],
    [400, "bad_request", "bleeding"],
    [422, "invalid_value", "disturbances"],
    [422, "invalid_date", "date"],
    [409, "already_onboarded"],
    [413, "too_large"],
    [503, "auth_unavailable"],
    [502],
    [-1],
  ]) {
    assert.deepEqual(
      read(status, code, field),
      { kind: "message", message: TRY_AGAIN },
      `${status} ${code}`,
    );
  }
});

// The live line: design-brief §6.3's five sentences, with fixed 07:00.
test("liveLine: no temperature yet", () => {
  assert.deepEqual(
    liveLine({ temperature: "", measuredAt: "07:00" }, "07:00"),
    {
      before: "Your usual time is 07:00.",
    },
  );
  assert.deepEqual(
    liveLine({ temperature: "", measuredAt: "07:45" }, "07:00"),
    {
      before: "Measured 45 min late. The value will be adjusted to 07:00.",
    },
  );
  assert.deepEqual(
    liveLine({ temperature: "", measuredAt: "06:30" }, "07:00"),
    {
      before: "Measured 30 min early. The value will be adjusted to 07:00.",
    },
  );
  // Half-typed, or impossible: the same as no temperature.
  for (const temperature of ["36.", "abc", "12", "19.99"]) {
    assert.deepEqual(
      liveLine({ temperature, measuredAt: "07:45" }, "07:00"),
      {
        before: "Measured 45 min late. The value will be adjusted to 07:00.",
      },
      temperature,
    );
  }
});

test("liveLine: a temperature at the usual time, or close enough", () => {
  assert.deepEqual(
    liveLine({ temperature: "36.70", measuredAt: "07:00" }, "07:00"),
    {
      before: "Measured at your usual time, so no adjustment.",
    },
  );
  // A cleared time counts as the usual time.
  assert.deepEqual(
    liveLine({ temperature: "36.70", measuredAt: "" }, "07:00"),
    {
      before: "Measured at your usual time, so no adjustment.",
    },
  );
  // AC-3 (T3): one minute late rounds away.
  assert.deepEqual(
    liveLine({ temperature: "36.50", measuredAt: "07:01" }, "07:00"),
    {
      before: "Measured 1 min late. Too little to change the value.",
    },
  );
});

test("liveLine: what the reading counts as, labelled an estimate", () => {
  // AC-1 (T1): 36.62 at 07:45 counts as 36.55.
  assert.deepEqual(
    liveLine({ temperature: "36.62", measuredAt: "07:45" }, "07:00"),
    {
      before: "Measured 45 min late, so it counts as ",
      value: "36.55 °C",
      after: ". That's an estimate.",
    },
  );
  // AC-2 (T2): early adds.
  assert.deepEqual(
    liveLine({ temperature: "36,4", measuredAt: "06:30" }, "07:00"),
    {
      before: "Measured 30 min early, so it counts as ",
      value: "36.45 °C",
      after: ". That's an estimate.",
    },
  );
  // The design brief's own example: 36.37 at 07:45 counts as 36.30.
  assert.equal(
    liveLine({ temperature: "36.37", measuredAt: "07:45" }, "07:00").value,
    "36.30 °C",
  );
  // Against her own usual time, not 07:00.
  assert.deepEqual(
    liveLine({ temperature: "36.62", measuredAt: "06:30" }, "06:30"),
    {
      before: "Measured at your usual time, so no adjustment.",
    },
  );
});
