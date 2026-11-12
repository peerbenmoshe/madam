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
