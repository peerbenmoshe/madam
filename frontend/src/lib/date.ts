// Dates on screen (docs/design-brief.md §7): dd/mm/yyyy, as in Israel, with
// the weekday in words first where there is room.
//
// A date here is a calendar day with no clock and no time zone, written
// "YYYY-MM-DD", as the API sends it. These functions never build a
// local-time Date from it: in a time zone behind UTC that would show the
// day before.

const ISO_DATE = /^(\d{4})-(\d{2})-(\d{2})$/;

const WEEKDAYS = [
  "Sunday",
  "Monday",
  "Tuesday",
  "Wednesday",
  "Thursday",
  "Friday",
  "Saturday",
];

type DateParts = { year: number; month: number; day: number; weekday: number };

// Splits "YYYY-MM-DD" into numbers and finds its weekday (0 is Sunday).
// Anything else, and any date the calendar doesn't have (30 February), is a
// bug in the caller, so it throws rather than display something wrong.
function parse(iso: string): DateParts {
  const match = ISO_DATE.exec(iso);
  if (!match) {
    throw new RangeError(`not a YYYY-MM-DD date: "${iso}"`);
  }
  const year = Number(match[1]);
  const month = Number(match[2]);
  const day = Number(match[3]);

  // Midnight UTC of that day. The Date is used only as a calendar: it
  // rolls an impossible date over to a real one (30 February becomes
  // 2 March), which is how one is recognised. setUTCFullYear, because
  // Date.UTC reads the years 0 to 99 as 1900 to 1999.
  const utc = new Date(0);
  utc.setUTCFullYear(year, month - 1, day);
  if (
    utc.getUTCFullYear() !== year ||
    utc.getUTCMonth() !== month - 1 ||
    utc.getUTCDate() !== day
  ) {
    throw new RangeError(`no such date: "${iso}"`);
  }
  return { year, month, day, weekday: utc.getUTCDay() };
}

// isDate says whether iso is a real calendar date written "YYYY-MM-DD".
// For text that comes from outside the app, such as the address bar, where
// a wrong value is not a bug and must not throw.
export function isDate(iso: string): boolean {
  try {
    parse(iso);
    return true;
  } catch {
    return false;
  }
}

function two(n: number): string {
  return String(n).padStart(2, "0");
}

// formatDate("2026-11-08") is "08/11/2026".
export function formatDate(iso: string): string {
  const { year, month, day } = parse(iso);
  return `${two(day)}/${two(month)}/${String(year).padStart(4, "0")}`;
}

// formatDateLong("2026-11-08") is "Sunday 08/11/2026".
export function formatDateLong(iso: string): string {
  const { weekday } = parse(iso);
  return `${WEEKDAYS[weekday]} ${formatDate(iso)}`;
}

// dateRangeProblem says what is wrong with picking the date iso when only
// dates from min to max are allowed (either may be left out), in words for
// the person, or returns null when the date is allowed.
//
// It exists because a phone's native date picker doesn't enforce an
// input's min and max (measured on Android and iPhone, 12 November 2026);
// only a computer's does. So the date box checks every pick itself.
export function dateRangeProblem(
  iso: string,
  min?: string,
  max?: string,
): string | null {
  // "YYYY-MM-DD" strings sort like the dates they name, so plain string
  // comparison is date comparison. parse() makes sure they are such strings.
  parse(iso);
  const tooEarly = min !== undefined && iso < min;
  const tooLate = max !== undefined && iso > max;
  if (!tooEarly && !tooLate) {
    return null;
  }
  if (min !== undefined && max !== undefined) {
    return `Pick a date from ${formatDate(min)} to ${formatDate(max)}.`;
  }
  if (max !== undefined) {
    return `Pick a date up to ${formatDate(max)}.`;
  }
  return `Pick ${formatDate(min as string)} or a later date.`;
}

// The earliest date the app accepts (docs/fam-rules.md §9.1): a floor for a
// mangled year, such as 0002 while a year is being typed. The server holds
// the same date (fam.EarliestDate).
export const EARLIEST_DATE = "2000-01-01";

// todayISO is today on this device's own calendar, as "YYYY-MM-DD". "Today"
// is the date she sees on her phone, wherever she is (docs/spec-v1.md §4).
//
// It reads the local year, month and day. It must never go through UTC
// (toISOString): in Israel that is yesterday's date from midnight until
// 02:00, or 03:00 in summer.
export function todayISO(now: Date = new Date()): string {
  const year = String(now.getFullYear()).padStart(4, "0");
  return `${year}-${two(now.getMonth() + 1)}-${two(now.getDate())}`;
}
