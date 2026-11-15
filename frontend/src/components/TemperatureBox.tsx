import { useId } from "react";

// The temperature, typed: the largest thing on the day screen
// (docs/design-brief.md §5 and §6.3). A number at 2.5rem and weight 800
// over an ink underline, with its unit beside it.
//
// It is a text input that asks a phone for its decimal keyboard, not
// <input type="number">: that one drops what it can't read without saying
// so, steps the value on a scroll, and gives "36," back as "". The box
// holds exactly what she typed; lib/temperature.ts reads it.

type Props = {
  label: string;
  // "°C" or "°F", shown beside the number.
  unit: string;
  // What she has typed so far, as typed.
  value: string;
  onChange: (value: string) => void;
  // A greyed example of what to type: "36.45".
  placeholder?: string;
  // What is wrong with the value, shown under the box.
  error?: string;
  // The id of a line elsewhere that describes the reading: the day
  // screen's "counts as" line. A screen reader reads it with the box.
  describedBy?: string;
};

export function TemperatureBox({
  label,
  unit,
  value,
  onChange,
  placeholder,
  error,
  describedBy,
}: Props) {
  const id = useId();
  const errorId = error ? `${id}-error` : undefined;
  return (
    <div className="grid gap-2">
      <label htmlFor={id} className="font-bold">
        {label}
      </label>
      {/* The underline is this row's border. Focus turns it berry over a
          blush band, as the text field's border does. */}
      <div
        className={`flex items-baseline gap-1.5 border-b-[2.5px] focus-within:shadow-[0_4px_0_0_var(--color-blush)] ${error ? "border-danger" : "border-ink focus-within:border-berry"}`}
      >
        <input
          id={id}
          inputMode="decimal"
          autoComplete="off"
          // A phone must not "correct" a number or capitalise it.
          autoCorrect="off"
          spellCheck={false}
          // The longest thing worth typing is "102.25".
          maxLength={8}
          placeholder={placeholder}
          value={value}
          aria-invalid={error ? true : undefined}
          aria-describedby={
            [describedBy, errorId].filter(Boolean).join(" ") || undefined
          }
          onChange={(event) => onChange(event.target.value)}
          className="placeholder:text-grid-strong w-full min-w-0 bg-transparent p-0 text-[2.5rem] leading-[1.1] font-extrabold tracking-[-0.03em] tabular-nums focus:outline-none"
        />
        <span className="text-muted text-xl font-medium">{unit}</span>
      </div>
      {error && (
        <p id={errorId} role="alert" className="text-danger text-sm font-bold">
          {error}
        </p>
      )}
    </div>
  );
}
