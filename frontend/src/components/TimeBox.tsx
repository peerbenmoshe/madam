import { useId } from "react";
import { timeFromInput } from "../lib/time.ts";

// Every time input (docs/design-brief.md §5 and §9): the date box's twin.
// A phone set to a 12-hour clock draws "7:05 AM" in a native time input,
// and times here are 24-hour. So the box shows its own "07:05", with the
// real <input type="time"> stretched over it, invisible. A tap lands on the
// input and opens the phone's own picker. Whatever that picker looks like,
// the value it reports is "HH:MM".

type Props = {
  label: string;
  // "HH:MM", or "" while no time is chosen.
  value: string;
  onChange: (value: string) => void;
  // What is wrong, shown under the box: "Pick a time."
  error?: string;
};

export function TimeBox({ label, value, onChange, error }: Props) {
  const errorId = useId();
  return (
    <div className="grid gap-2">
      <label className="border-rule bg-surface focus-within:border-berry relative grid min-h-12 cursor-pointer gap-px rounded-xl border-[1.5px] px-3 py-1.5 focus-within:shadow-[0_0_0_3px_var(--color-blush)]">
        <span className="text-muted text-xs">{label}</span>
        <span className="font-bold tabular-nums">
          {value === "" ? (
            <span className="text-muted font-normal">hh:mm</span>
          ) : (
            value
          )}
        </span>
        <input
          type="time"
          aria-label={label}
          value={value}
          aria-invalid={error ? true : undefined}
          aria-describedby={error ? errorId : undefined}
          onChange={(event) => {
            // null: not a time. onChange isn't called, so value stays as
            // it was and React puts the input back to it.
            const picked = timeFromInput(event.target.value);
            if (picked !== null) {
              onChange(picked);
            }
          }}
          onClick={(event) => {
            // On a computer a click on a time input only focuses it, so ask
            // for the picker. Browsers without showPicker, or that refuse
            // it here, still let the time be typed.
            try {
              event.currentTarget.showPicker();
            } catch {
              // Nothing to do: the input works without the picker.
            }
          }}
          className="absolute inset-0 size-full cursor-pointer opacity-0"
        />
      </label>
      {error && (
        <p id={errorId} role="alert" className="text-danger text-sm font-bold">
          {error}
        </p>
      )}
    </div>
  );
}
