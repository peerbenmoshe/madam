import { useEffect } from "react";

// A short confirmation that something was done: "Entry saved", "Entry
// deleted" (docs/design-brief.md §5 and §7). An ink pill near the bottom of
// the screen, above where the Save button sits, gone by itself after 2.2
// seconds. It never asks for anything and never takes the focus.
//
// It says what it says to a screen reader too (role="status" is a polite
// live region: read when the reader is next idle, not interrupting).

// How long it stays, in milliseconds (design-brief.md §5).
export const TOAST_MS = 2200;

type Props = {
  message: string;
  // Called once the time is up: the owner then stops rendering the toast.
  onDone: () => void;
};

export function Toast({ message, onDone }: Props) {
  useEffect(() => {
    const timer = window.setTimeout(onDone, TOAST_MS);
    // A new message, or the toast leaving early, cancels the old timer.
    return () => window.clearTimeout(timer);
  }, [message, onDone]);

  return (
    <p
      role="status"
      className="bg-ink text-surface animate-rise fixed bottom-[calc(96px+env(safe-area-inset-bottom,0px))] left-1/2 z-40 -translate-x-1/2 rounded-full px-[18px] py-3 text-sm font-bold whitespace-nowrap"
    >
      {message}
    </p>
  );
}
