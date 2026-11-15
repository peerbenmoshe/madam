// The day screen (docs/design-brief.md §6.3): its words, what is wrong with
// the form, the request it becomes, what each of the server's answers
// means for her, and the line that follows her typing. Plain functions with
// no browser and no network in them, so `npm test` covers every line, like
// onboardingText.ts.
//
// The server's own messages are never shown: they are written for a
// developer, and the page reads only the code (docs/tech-stack.md §3).

import { countsAs } from "./adjust.ts";
import { OFFLINE, SESSION_REFUSED, TRY_AGAIN } from "./authText.ts";
import {
  EARLIEST_DATE,
  dateRangeProblem,
  formatDate,
  formatDateLong,
} from "./date.ts";
import {
  formatTemperature,
  isPossibleCelsius,
  parseTemperature,
} from "./temperature.ts";
import { isTimeOfDay, minutesOfDay } from "./time.ts";

// ---- what travels to and from the server ----

export type Bleeding = "none" | "spotting" | "bleeding";
export type SecretionCategory = "clear_stretchy" | "milky" | "sticky";
export type Sensation = "dry" | "creamy" | "wet";
export type CervixFirmness = "firm" | "soft";
export type CervixPosition = "high_open" | "low_closed";
export type Intercourse = "none" | "protected" | "unprotected";
export type Disturbance =
  | "alcohol"
  | "poor_sleep"
  | "illness"
  | "medication"
  | "travel"
  | "vacation"
  | "new_thermometer"
  | "uti"
  | "yeast_infection";
export type Symptom = "breast_tenderness" | "bloating" | "pms";

// A temperature on the wire: whole hundredths, and the unit they are in.
export type Temperature = {
  hundredths: number;
  unit: "celsius" | "fahrenheit";
};

// An entry, as the API sends and reads it (backend/internal/api/day.go).
// null is "not said".
export type Entry = {
  temperature: Temperature | null;
  // "HH:MM". Present with a temperature, null without.
  measured_at: string | null;
  bleeding: Bleeding;
  secretion_category: SecretionCategory | null;
  sensation: Sensation | null;
  secretion_amount: number | null;
  secretion_description: string | null;
  cervix_firmness: CervixFirmness | null;
  cervix_position: CervixPosition | null;
  disturbances: Disturbance[];
  symptoms: Symptom[];
  intercourse: Intercourse | null;
  notes: string | null;
};

// GET /api/days/{date}, and PUT's answer.
export type Day = {
  date: string;
  // Her usual measurement time on that date, "HH:MM".
  fixed_time: string;
  // null on a day with no entry.
  entry: Entry | null;
};

// The questions the server may want answered before it saves. The name is
// the error's code, and what goes into `confirmed` on "yes".
export type QuestionName = "no_temperature" | "unusual_temperature";

// The body of PUT /api/days/{date}.
export type DayBody = { entry: Entry; confirmed: QuestionName[] };

// ---- the words ----

export const TITLE_TODAY = "Today's entry";
// Under the title on a day that has no entry.
export const NO_ENTRY_YET = "No entry yet.";
export const BACK = "Back";
// On the placeholder page, until the graph exists (Slice 6).
export const OPEN_TODAY = "Today's entry";

export const DATE_LABEL = "Date";
export const TEMPERATURE_LABEL = "Temperature";
export const TEMPERATURE_EXAMPLE = "36.45";
export const CELSIUS = "°C";
export const MEASURED_AT_LABEL = "Measured at";
export const CYCLE_START_LABEL = "Cycle starts on this day";
// Under the switch while it is shown but can't be used (until Slice 5).
export const CYCLE_START_NOT_YET = "Cycle starts can't be changed here yet.";
export const BLEEDING_LABEL = "Bleeding";
export const SECRETION_LABEL = "Cervical secretion";
export const DISTURBANCES_LABEL = "Disturbances";
export const DISTURBANCES_HINT =
  "A disturbed reading shows as a ring, and the line skips it.";
export const NOTES_LABEL = "Notes";
export const NOTES_EXAMPLE = "Mood, travel, anything else";
// The server's limit (docs/fam-rules.md §9.5). The box stops there.
export const NOTES_MAX = 2000;

export const SAVE = "Save entry";
export const SAVED = "Entry saved";
export const GO_TO_TODAY = "Go to today's entry";

export const DATE_NEEDED = "Pick a date.";
export const TEMPERATURE_NOT_A_NUMBER =
  "Enter the temperature as a number, like 36.45.";
export const TEMPERATURE_IMPOSSIBLE =
  "That isn't a body temperature. Enter a value between 20.00 and 50.00 °C.";
export const TIME_NEEDED = "Pick a time.";
export const NOTES_TOO_LONG = "Keep the notes under 2,000 characters.";

// The answers, in the order they are shown. The values are the server's.
export const BLEEDING_OPTIONS = [
  { value: "none", label: "None" },
  { value: "spotting", label: "Spotting" },
  { value: "bleeding", label: "Bleeding" },
] as const satisfies readonly { value: Bleeding; label: string }[];

export const SECRETION_OPTIONS = [
  { value: "clear_stretchy", label: "Clear, stretchy" },
  { value: "milky", label: "Milky" },
  { value: "sticky", label: "Sticky" },
] as const satisfies readonly { value: SecretionCategory; label: string }[];

// In docs/fam-rules.md §7's order, which is the order they are stored in.
export const DISTURBANCE_OPTIONS = [
  { value: "alcohol", label: "Alcohol" },
  { value: "poor_sleep", label: "Poor sleep" },
  { value: "illness", label: "Illness" },
  { value: "medication", label: "Medication" },
  { value: "travel", label: "Travel" },
  { value: "vacation", label: "Vacation" },
  { value: "new_thermometer", label: "New thermometer" },
  { value: "uti", label: "UTI" },
  { value: "yeast_infection", label: "Yeast infection" },
] as const satisfies readonly { value: Disturbance; label: string }[];

// dayTitle is the screen's title: "Today's entry", or the weekday and date
// of any other day. today is the device's date.
export function dayTitle(date: string, today: string): string {
  return date === today ? TITLE_TODAY : formatDateLong(date);
}

// Shown when a day that was opened as today is still on screen the next
// day: the screen never changes day by itself, so it says which day it
// holds.
export function otherDayNotice(date: string, today: string): string {
  return `This is the entry for ${formatDateLong(date)}. Today is ${formatDateLong(today)}.`;
}

// ---- the form ----

// What is on the screen. The temperature is text, exactly as typed; the
// rest are the entry's own values.
//
// The fields from sensation down have no control on the screen yet ("Show
// all fields"). They are still here: saving replaces the whole entry, so
// what the screen doesn't show must be sent back as it came.
export type DayForm = {
  temperature: string;
  // "HH:MM", or "" when she cleared it.
  measuredAt: string;
  bleeding: Bleeding;
  secretionCategory: SecretionCategory | null;
  disturbances: Disturbance[];
  notes: string;

  sensation: Sensation | null;
  secretionAmount: number | null;
  secretionDescription: string;
  cervixFirmness: CervixFirmness | null;
  cervixPosition: CervixPosition | null;
  symptoms: Symptom[];
  intercourse: Intercourse | null;
};

// dayFormFrom fills the form from what the server holds for the day. A day
// with no entry is a blank form: bleeding None, and "Measured at" at her
// usual time for that date. An entry without a temperature gets the usual
// time too, ready for when she adds one.
//
// This screen is Celsius only until the unit can be changed (Slice 9). A
// Fahrenheit temperature would be shown with the wrong unit beside it, so
// it throws instead: no account can be set to Fahrenheit yet.
export function dayFormFrom(day: Day): DayForm {
  const entry = day.entry;
  if (entry?.temperature && entry.temperature.unit !== "celsius") {
    throw new Error("dayFormFrom: this screen shows Celsius only");
  }
  return {
    temperature: entry?.temperature
      ? formatTemperature(entry.temperature.hundredths)
      : "",
    measuredAt: entry?.measured_at ?? day.fixed_time,
    bleeding: entry?.bleeding ?? "none",
    secretionCategory: entry?.secretion_category ?? null,
    disturbances: entry?.disturbances ?? [],
    notes: entry?.notes ?? "",
    sensation: entry?.sensation ?? null,
    secretionAmount: entry?.secretion_amount ?? null,
    secretionDescription: entry?.secretion_description ?? "",
    cervixFirmness: entry?.cervix_firmness ?? null,
    cervixPosition: entry?.cervix_position ?? null,
    symptoms: entry?.symptoms ?? [],
    intercourse: entry?.intercourse ?? null,
  };
}

// What is wrong, by the control it is shown under. Empty: nothing.
export type DayProblems = {
  temperature?: string;
  measuredAt?: string;
};

// dayProblems checks the form before anything is sent: only what the page
// can know by itself. No temperature at all is not a problem here: the
// server asks about it. The time counts only with a temperature.
export function dayProblems(form: DayForm): DayProblems {
  const problems: DayProblems = {};
  const typed = parseTemperature(form.temperature);
  if (typed.kind === "invalid") {
    problems.temperature = TEMPERATURE_NOT_A_NUMBER;
  } else if (typed.kind === "value") {
    if (!isPossibleCelsius(typed.hundredths)) {
      problems.temperature = TEMPERATURE_IMPOSSIBLE;
    }
    if (!isTimeOfDay(form.measuredAt)) {
      problems.measuredAt = TIME_NEEDED;
    }
  }
  return problems;
}

// dateProblem says what is wrong with the date in the date box, or null.
// The box is how she moves to another day, so a date it can't open is said
// under it: empty, before the floor, or after today (the device's date).
export function dateProblem(date: string, today: string): string | null {
  if (date === "") {
    return DATE_NEEDED;
  }
  return dateRangeProblem(date, EARLIEST_DATE, today);
}

// dayBody turns a form without problems into the request. confirmed is the
// questions she has answered "yes" to so far.
//
// The time is sent only with a temperature: the server refuses one without
// the other. An empty text is sent as null. A form whose temperature isn't
// a number is a bug in the caller, so it throws rather than send a guess.
export function dayBody(form: DayForm, confirmed: QuestionName[]): DayBody {
  const typed = parseTemperature(form.temperature);
  if (typed.kind === "invalid") {
    throw new Error("dayBody: the temperature isn't a number");
  }
  const hasReading = typed.kind === "value";
  return {
    entry: {
      temperature: hasReading
        ? { hundredths: typed.hundredths, unit: "celsius" }
        : null,
      measured_at: hasReading ? form.measuredAt : null,
      bleeding: form.bleeding,
      secretion_category: form.secretionCategory,
      sensation: form.sensation,
      secretion_amount: form.secretionAmount,
      secretion_description:
        form.secretionDescription === "" ? null : form.secretionDescription,
      cervix_firmness: form.cervixFirmness,
      cervix_position: form.cervixPosition,
      disturbances: form.disturbances,
      symptoms: form.symptoms,
      intercourse: form.intercourse,
      notes: form.notes === "" ? null : form.notes,
    },
    confirmed,
  };
}

// ---- the questions ----

// A question in Save's place (docs/design-brief.md §5 and §7): the words,
// and the two specific answers. "yes" goes ahead.
export type Ask = {
  name: QuestionName;
  question: string;
  yes: string;
  no: string;
};

// questionFor gives the words of a question the server asked about this
// form. The server decides whether to ask; the page only words it.
export function questionFor(name: QuestionName, form: DayForm): Ask {
  switch (name) {
    case "no_temperature":
      return {
        name,
        question: "Save without a temperature?",
        yes: "Save without",
        no: "Add a temperature",
      };
    case "unusual_temperature": {
      const typed = parseTemperature(form.temperature);
      // The server asked about a temperature, so there is one. Without it
      // the question still has to read as a sentence.
      const shown =
        typed.kind === "value"
          ? `${formatTemperature(typed.hundredths)} ${CELSIUS}`
          : "This temperature";
      return {
        name,
        question: `${shown} is unusual. Is it correct?`,
        yes: "Yes, save it",
        no: "No, fix it",
      };
    }
  }
}

// ---- what a failed request means ----

// ask: the server wants a question answered first. notOnboarded: she
// belongs on the onboarding screen, and the page goes there without a
// word. Otherwise a message, under one control when it is about one.
export type DayFailure =
  | { kind: "ask"; name: QuestionName }
  | { kind: "notOnboarded" }
  | {
      kind: "message";
      message: string;
      field?: "date" | "temperature" | "measuredAt" | "notes";
    };

// dayFailure reads a failed GET or PUT of a day.
export function dayFailure(failure: {
  status: number;
  code?: string;
  field?: string;
}): DayFailure {
  if (failure.status === 0) {
    return { kind: "message", message: OFFLINE };
  }
  if (failure.status === 401) {
    return { kind: "message", message: SESSION_REFUSED };
  }
  switch (failure.code) {
    case "no_temperature":
    case "unusual_temperature":
      return { kind: "ask", name: failure.code };
    case "not_onboarded":
      return { kind: "notOnboarded" };
    case "temperature_out_of_range":
      return {
        kind: "message",
        field: "temperature",
        message: TEMPERATURE_IMPOSSIBLE,
      };
  }
  if (failure.field === "date") {
    switch (failure.code) {
      // Her device's date is ahead of every clock on Earth, or the page
      // was open since yesterday. Either way the device is where to look.
      case "date_in_future":
        return {
          kind: "message",
          field: "date",
          message: "That date is in the future. Check the date on your device.",
        };
      case "date_too_early":
        return {
          kind: "message",
          field: "date",
          message: `Pick ${formatDate(EARLIEST_DATE)} or a later date.`,
        };
    }
  }
  if (
    failure.field === "measured_at" &&
    (failure.code === "invalid_time" || failure.code === "missing_field")
  ) {
    return { kind: "message", field: "measuredAt", message: TIME_NEEDED };
  }
  if (failure.field === "notes" && failure.code === "too_long") {
    return { kind: "message", field: "notes", message: NOTES_TOO_LONG };
  }
  // Everything else is a bug of ours or an outage, and there is nothing
  // she can correct.
  return { kind: "message", message: TRY_AGAIN };
}

// ---- the line that follows her typing ----

// What the line under the temperature says. When the reading will count as
// a different value, that value is `value`, shown in bold between `before`
// and `after`.
export type LiveLine = { before: string; value?: string; after?: string };

// liveLine says what the reading will count as (docs/design-brief.md
// §6.3), from the temperature and time as typed and her usual time for
// the day, "HH:MM". It is the mock's five sentences:
//
//   no temperature, at the usual time   "Your usual time is 07:00."
//   no temperature, another time        "Measured 45 min late. The value
//                                        will be adjusted to 07:00."
//   a temperature, at the usual time    "Measured at your usual time, so
//                                        no adjustment."
//   …another time, same after rounding  "Measured 1 min late. Too little
//                                        to change the value."
//   …and a different value              "Measured 45 min late, so it
//                                        counts as 36.30 °C. That's an
//                                        estimate."
//
// A temperature that isn't a number yet, or isn't a possible one, counts
// as no temperature. A cleared time counts as the usual time. The value is
// lib/adjust.ts's, the copy of the server's arithmetic; what is saved and
// drawn is always the server's own.
export function liveLine(
  form: Pick<DayForm, "temperature" | "measuredAt">,
  fixedTime: string,
): LiveLine {
  const fixed = minutesOfDay(fixedTime);
  const measured = isTimeOfDay(form.measuredAt)
    ? minutesOfDay(form.measuredAt)
    : fixed;
  const off = measured - fixed;
  const when = `${Math.abs(off)} min ${off > 0 ? "late" : "early"}`;

  const typed = parseTemperature(form.temperature);
  if (typed.kind !== "value" || !isPossibleCelsius(typed.hundredths)) {
    return off === 0
      ? { before: `Your usual time is ${fixedTime}.` }
      : {
          before: `Measured ${when}. The value will be adjusted to ${fixedTime}.`,
        };
  }
  if (off === 0) {
    return { before: "Measured at your usual time, so no adjustment." };
  }
  const counted = countsAs(typed.hundredths, measured, fixed);
  if (counted === typed.hundredths) {
    return { before: `Measured ${when}. Too little to change the value.` };
  }
  return {
    before: `Measured ${when}, so it counts as `,
    value: `${formatTemperature(counted)} ${CELSIUS}`,
    after: ". That's an estimate.",
  };
}
