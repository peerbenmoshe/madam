import type { ReactNode } from "react";

// A part of a form that opens and closes: "Show all fields" on the day
// screen (docs/design-brief.md §6.3). A native <details>, so the browser
// supplies the keyboard behaviour (Enter and Space on the summary) and tells
// a screen reader whether it is open. Its label says what a tap will do,
// and the chevron turns (§8: motion only answers a tap, and not at all with
// reduced motion).
//
// The owner holds `open`, so it can decide how the section starts (open when
// something under it holds a value). Its contents stay in the page while it
// is closed: they are hidden, not removed.

type Props = {
  // What the summary says while closed, and while open.
  closedLabel: string;
  openLabel: string;
  open: boolean;
  onToggle: (open: boolean) => void;
  children: ReactNode;
};

export function Disclosure({
  closedLabel,
  openLabel,
  open,
  onToggle,
  children,
}: Props) {
  return (
    <details
      open={open}
      // The browser has already opened or closed it; this reports where it
      // ended up, so a tap and the owner never disagree.
      onToggle={(event) => onToggle(event.currentTarget.open)}
      className="group border-rule border-t pt-1.5"
    >
      <summary className="text-berry flex min-h-12 cursor-pointer list-none items-center justify-between font-bold [&::-webkit-details-marker]:hidden">
        {open ? openLabel : closedLabel}
        <svg
          viewBox="0 0 18 18"
          aria-hidden="true"
          className="size-[18px] transition-transform duration-200 group-open:rotate-180 motion-reduce:transition-none"
        >
          <path
            d="m4 7 5 5 5-5"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
      </summary>
      <div className="grid gap-6 pt-3">{children}</div>
    </details>
  );
}
