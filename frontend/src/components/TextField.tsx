import { useId } from "react";
import type { InputHTMLAttributes, ReactNode } from "react";

// A labelled one-line input: email, password, invite code
// (docs/design-brief.md §5). The text is 16px, because iOS zooms the page
// when a smaller input is focused. Focus shows as a berry border with a
// blush ring, as on the mock's inputs.
//
// Any other input attribute (type, autoComplete, value, onChange…) passes
// straight through.

type Props = Omit<InputHTMLAttributes<HTMLInputElement>, "id"> & {
  label: string;
  // Help shown under the input, until an error takes its place.
  hint?: string;
  // What is wrong with the value. Shown in the danger colour, announced by
  // screen readers, and marks the input invalid. Usually text; it may hold
  // a link to the way out.
  error?: ReactNode;
};

export function TextField({
  label,
  hint,
  error,
  className = "",
  ...rest
}: Props) {
  const id = useId();
  // An error replaces the hint: both at once usually say the same thing
  // twice ("At least 8 characters." / "Use at least 8 characters.").
  const shownHint = error ? undefined : hint;
  const hintId = shownHint ? `${id}-hint` : undefined;
  const errorId = error ? `${id}-error` : undefined;
  return (
    <div className="grid gap-2">
      <label htmlFor={id} className="font-bold">
        {label}
      </label>
      <input
        id={id}
        aria-invalid={error ? true : undefined}
        aria-describedby={
          [hintId, errorId].filter(Boolean).join(" ") || undefined
        }
        className={`border-rule bg-surface focus:border-berry aria-invalid:border-danger min-h-12 w-full rounded-xl border-[1.5px] px-3 py-2.5 text-base focus:shadow-[0_0_0_3px_var(--color-blush)] focus:outline-none ${className}`}
        {...rest}
      />
      {shownHint && (
        <p id={hintId} className="text-muted text-sm">
          {shownHint}
        </p>
      )}
      {error && (
        <p id={errorId} role="alert" className="text-danger text-sm font-bold">
          {error}
        </p>
      )}
    </div>
  );
}
