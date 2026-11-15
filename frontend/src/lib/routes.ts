// The addresses of the signed-in screens (docs/tech-stack.md §8). The
// address bar's path is the state (router.ts); this file knows what the
// paths mean.

import { EARLIEST_DATE, isDate } from "./date.ts";

// The graph, from Slice 6. Until then, the placeholder page.
export const HOME = "/";

// A day's own address: /day/2026-11-15.
export function dayPath(date: string): string {
  return `/day/${date}`;
}

// [0-9], not \d: see time.ts.
const DAY_PATH = /^\/day\/([0-9]{4}-[0-9]{2}-[0-9]{2})$/;

// dayOfPath returns the date a path names, or null when the path isn't a
// day the page may open: not a day's address at all, not a real date, a
// date before the floor, or one after today. today is the device's date
// ("YYYY-MM-DD", from todayISO). The server holds the same two limits and
// refuses a day outside them whatever the page does.
//
// The path is whatever someone typed in the address bar, so nothing here
// throws.
export function dayOfPath(path: string, today: string): string | null {
  const date = DAY_PATH.exec(path)?.[1];
  if (date === undefined || !isDate(date)) {
    return null;
  }
  // "YYYY-MM-DD" strings sort like the dates they name.
  return date >= EARLIEST_DATE && date <= today ? date : null;
}
