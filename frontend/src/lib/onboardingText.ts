// The words of the onboarding screen (docs/design-brief.md §10), what is
// wrong with the form, the request it becomes, and what each of the
// server's refusals means for her. Plain functions with no browser and no
// network in them, so `npm test` covers every line, like authText.ts.
//
// The server's own messages are never shown: they are written for a
// developer, and the page reads only the code (docs/tech-stack.md §3).

import { OFFLINE, SESSION_REFUSED, TRY_AGAIN } from "./authText.ts";
import { EARLIEST_DATE, dateRangeProblem, formatDate } from "./date.ts";
import { isTimeOfDay } from "./time.ts";

export const TITLE = "Set up your chart";
export const SUB = "Two questions, then you can start.";

export const START_QUESTION = "When did your current cycle start?";
export const START_HINT = "Day 1 is the first day of your period.";
export const START_KNOWN = "I know the date";
export const START_UNKNOWN = "I don't know";
export const START_DATE_LABEL = "Cycle start";
// Shown once she chooses "I don't know", in place of the date box: it is a
// full answer, and this is what it will mean on her chart.
export const START_UNKNOWN_NOTE =
  'That\'s fine. Cycle days will show as "?" until you mark the start of your next cycle.';

export const TIME_QUESTION = "What time do you usually take your temperature?";
export const TIME_HINT =
  "A reading taken earlier or later is adjusted to this time.";
export const TIME_LABEL = "Measurement time";

export const SUBMIT = "Start charting";

export const START_NEEDED = 'Choose a date or "I don\'t know".';
export const START_DATE_NEEDED = "Pick the date your cycle started.";
export const TIME_NEEDED = "Pick a time.";

// What she has answered so far. start is null until she taps one of the
// two answers: neither is chosen for her.
export type OnboardingForm = {
  start: "known" | "unknown" | null;
  // "YYYY-MM-DD", or "". Counts only while start is "known".
  date: string;
  // "HH:MM", or "".
  time: string;
};

// What is wrong, by the control it is shown under. Empty: nothing.
export type OnboardingProblems = {
  start?: string;
  date?: string;
  time?: string;
};

// onboardingProblems checks the form before anything is sent. today is the
// date on her device ("YYYY-MM-DD", from todayISO): the latest start she
// can give. The server checks again, by its own, looser, rule.
export function onboardingProblems(
  form: OnboardingForm,
  today: string,
): OnboardingProblems {
  const problems: OnboardingProblems = {};
  if (form.start === null) {
    problems.start = START_NEEDED;
  } else if (form.start === "known") {
    if (form.date === "") {
      problems.date = START_DATE_NEEDED;
    } else {
      const outside = dateRangeProblem(form.date, EARLIEST_DATE, today);
      if (outside !== null) {
        problems.date = outside;
      }
    }
  }
  if (!isTimeOfDay(form.time)) {
    problems.time = TIME_NEEDED;
  }
  return problems;
}

// The body of POST /api/onboarding (backend/internal/api/onboarding.go).
// With "I don't know" there is no date field at all: the server refuses
// its presence.
export type OnboardingBody =
  | { cycle_start_unknown: true; measurement_time: string }
  | {
      cycle_start_unknown: false;
      cycle_start_date: string;
      measurement_time: string;
    };

// onboardingBody turns a form without problems into the request. A form
// with no answer to the first question is a bug in the caller, so it
// throws rather than guess one.
export function onboardingBody(form: OnboardingForm): OnboardingBody {
  switch (form.start) {
    case "unknown":
      return { cycle_start_unknown: true, measurement_time: form.time };
    case "known":
      return {
        cycle_start_unknown: false,
        cycle_start_date: form.date,
        measurement_time: form.time,
      };
    case null:
      throw new Error("onboardingBody: the first question has no answer");
  }
}

// What a failed submit means. goOn: she is onboarded already (another
// device, or a second tap), which is what she wanted, so the page moves on
// without a word. Otherwise a message, under one control when it is about
// one.
export type OnboardingFailure =
  { goOn: true } | { goOn: false; message: string; field?: "date" | "time" };

// The server's field names, as the form knows them.
const FIELDS: Record<string, "date" | "time"> = {
  cycle_start_date: "date",
  measurement_time: "time",
};

export function onboardingFailure(failure: {
  status: number;
  code?: string;
  field?: string;
}): OnboardingFailure {
  if (failure.status === 0) {
    return { goOn: false, message: OFFLINE };
  }
  if (failure.status === 401) {
    return { goOn: false, message: SESSION_REFUSED };
  }
  if (failure.code === "already_onboarded") {
    return { goOn: true };
  }
  const field = failure.field === undefined ? undefined : FIELDS[failure.field];
  if (field === "date") {
    switch (failure.code) {
      // Her device's date is ahead of every clock on Earth, or the page
      // was open since yesterday. Either way the device is where to look.
      case "date_in_future":
        return {
          goOn: false,
          field,
          message: "That date is in the future. Check the date on your device.",
        };
      case "date_too_early":
        return {
          goOn: false,
          field,
          message: `Pick ${formatDate(EARLIEST_DATE)} or a later date.`,
        };
      case "invalid_date":
      case "missing_field":
        return { goOn: false, field, message: START_DATE_NEEDED };
    }
  }
  if (
    field === "time" &&
    (failure.code === "invalid_time" || failure.code === "missing_field")
  ) {
    return { goOn: false, field, message: TIME_NEEDED };
  }
  // Everything else is a bug of ours or an outage, and there is nothing
  // she can correct.
  return { goOn: false, message: TRY_AGAIN };
}
