// Times of day (docs/design-brief.md §7, docs/fam-rules.md §9.2): always
// 24-hour and written "HH:MM", on screen and to the API. No seconds, and
// "24:00" is not a time.

// [0-9], not \d, so the rule can't change with a regex flag: only the ten
// digits an English keyboard types.
const TIME_OF_DAY = /^([01][0-9]|2[0-3]):[0-5][0-9]$/;

// isTimeOfDay says whether value is a time exactly as the API reads one.
// The server checks the same rule (fam.ParseMinutes); the two share their
// test cases.
export function isTimeOfDay(value: string): boolean {
  return TIME_OF_DAY.test(value);
}

// minutesOfDay("07:45") is 465: the minutes after midnight. Anything that
// isn't a time of day is a bug in the caller, so it throws.
export function minutesOfDay(time: string): number {
  if (!isTimeOfDay(time)) {
    throw new RangeError(`not a time of day: "${time}"`);
  }
  return Number(time.slice(0, 2)) * 60 + Number(time.slice(3, 5));
}

// What a native <input type="time"> reports: "HH:MM", with seconds (and
// fractions of one) in some browsers.
const NATIVE_TIME = /^([0-9]{2}:[0-9]{2})(:[0-9]{2}(\.[0-9]+)?)?$/;

// timeFromInput turns a native time input's value into ours.
//
//   ""          ""      she cleared it
//   "07:05"     "07:05"
//   "07:05:00"  "07:05" seconds dropped: a time here is a minute
//   else        null    not a time; the caller keeps what it had
export function timeFromInput(raw: string): string | null {
  if (raw === "") {
    return "";
  }
  const hoursAndMinutes = NATIVE_TIME.exec(raw)?.[1];
  if (hoursAndMinutes === undefined || !isTimeOfDay(hoursAndMinutes)) {
    return null;
  }
  return hoursAndMinutes;
}
