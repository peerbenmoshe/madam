import type { ReactNode } from "react";
import { AppBar } from "./AppBar.tsx";

// The frame of a simple screen: one phone-width column, the app bar, a
// title, and an optional line under it (docs/design-brief.md §4 and §5).

type Props = {
  title: string;
  sub?: ReactNode;
  children?: ReactNode;
};

export function Screen({ title, sub, children }: Props) {
  return (
    <main className="mx-auto max-w-md px-4 pb-16">
      <AppBar />
      <div className="pt-2.5 pb-6">
        <h1 className="text-[1.75rem] leading-[1.15] font-extrabold tracking-[-0.015em] text-balance">
          {title}
        </h1>
        {sub && <p className="text-muted mt-1">{sub}</p>}
      </div>
      {children}
    </main>
  );
}

// Something the person should know, not an error: "check your email".
export function Notice({ children }: { children: ReactNode }) {
  return (
    <p className="bg-blush rounded-2xl p-3.5 text-sm font-bold">{children}</p>
  );
}

// What went wrong with the whole form. Announced by screen readers.
export function FormError({ children }: { children: ReactNode }) {
  return (
    <p role="alert" className="text-danger text-sm font-bold">
      {children}
    </p>
  );
}
