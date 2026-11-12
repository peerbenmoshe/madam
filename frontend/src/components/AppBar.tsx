import type { ButtonHTMLAttributes, ReactNode } from "react";

// The top of every screen: the wordmark on the left, and on the right
// whatever links the screen has (docs/design-brief.md §5). The mark is the
// graph's own dot: a HotPink fill with a berry outline.

export function AppBar({ children }: { children?: ReactNode }) {
  return (
    <header className="flex items-center justify-between pt-3.5 pb-1.5">
      <div className="flex items-center gap-2 text-lg font-extrabold tracking-[-0.01em]">
        <svg viewBox="0 0 14 14" aria-hidden="true" className="size-3.5">
          <circle
            cx="7"
            cy="7"
            r="5.5"
            strokeWidth="1.75"
            className="fill-accent stroke-berry"
          />
        </svg>
        Madam
      </div>
      {children && (
        <nav aria-label="Main" className="-mr-2.5 flex gap-0.5">
          {children}
        </nav>
      )}
    </header>
  );
}

// One link in the app bar: "Cycles", "Settings".
export function NavLink({
  type = "button",
  className = "",
  ...rest
}: ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button
      type={type}
      className={`hover:bg-blush min-h-11 cursor-pointer rounded-[10px] px-2.5 text-sm font-medium ${className}`}
      {...rest}
    />
  );
}
