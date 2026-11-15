import { useEffect, useState } from "react";
import { todayISO } from "./date.ts";

// useToday is the device's date, "YYYY-MM-DD", kept current. A screen left
// open overnight must not keep yesterday's today (docs/build-order.md,
// Slice 4): a phone that wakes up shows the page again, and that is when
// it is read again. A check every minute covers a screen that stays on.
export function useToday(): string {
  const [today, setToday] = useState(todayISO);

  useEffect(() => {
    // Setting the same string again changes nothing and redraws nothing.
    const check = () => setToday(todayISO());
    document.addEventListener("visibilitychange", check);
    window.addEventListener("focus", check);
    const timer = window.setInterval(check, 60_000);
    return () => {
      document.removeEventListener("visibilitychange", check);
      window.removeEventListener("focus", check);
      window.clearInterval(timer);
    };
  }, []);

  return today;
}
