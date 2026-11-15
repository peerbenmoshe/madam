import { useId } from "react";
import type { ReactNode, TextareaHTMLAttributes } from "react";

// A labelled box for more than one line: the notes (docs/design-brief.md
// §5). The text field's twin: the same label, border, focus and error. The
// text is 16px, because iOS zooms the page when a smaller input is focused.
//
// It starts two lines tall and can be dragged taller. What is typed is
// text and stays text: line breaks are kept, and nothing is read as markup.
//
// Any other textarea attribute (value, onChange, maxLength, placeholder…)
// passes straight through.

type Props = Omit<TextareaHTMLAttributes<HTMLTextAreaElement>, "id"> & {
  label: string;
  // Help shown under the box, until an error takes its place.
  hint?: string;
  // What is wrong with the text. Shown in the danger colour, announced by
  // screen readers, and marks the box invalid.
  error?: ReactNode;
};

export function TextArea({
  label,
  hint,
  error,
  rows = 2,
  className = "",
  ...rest
}: Props) {
  const id = useId();
  const shownHint = error ? undefined : hint;
  const hintId = shownHint ? `${id}-hint` : undefined;
  const errorId = error ? `${id}-error` : undefined;
  return (
    <div className="grid gap-2">
      <label htmlFor={id} className="font-bold">
        {label}
      </label>
      <textarea
        id={id}
        rows={rows}
        aria-invalid={error ? true : undefined}
        aria-describedby={
          [hintId, errorId].filter(Boolean).join(" ") || undefined
        }
        className={`border-rule bg-surface focus:border-berry aria-invalid:border-danger min-h-[72px] w-full resize-y rounded-xl border-[1.5px] px-3 py-2.5 text-base focus:shadow-[0_0_0_3px_var(--color-blush)] focus:outline-none ${className}`}
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
