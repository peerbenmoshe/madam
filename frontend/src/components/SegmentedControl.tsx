import { useId } from "react";

// One choice out of a few short ones, side by side in a pill-shaped track:
// Dates/Cycle, the range presets, °C/°F (docs/design-brief.md §5).
//
// Each segment is a real radio button, hidden, inside the label that is
// drawn. The browser then does the work: one tab stop, arrow keys move the
// choice, and a screen reader announces "radio button, 2 of 3".

export type Choice<T extends string> = { value: T; label: string };

type Props<T extends string> = {
  // What the group is for; read by screen readers, not shown.
  label: string;
  options: readonly Choice<T>[];
  value: T;
  onChange: (value: T) => void;
};

export function SegmentedControl<T extends string>({
  label,
  options,
  value,
  onChange,
}: Props<T>) {
  // Radio buttons are grouped by a shared name, unique to this control.
  const name = useId();
  return (
    <div
      role="radiogroup"
      aria-label={label}
      // w-fit: the track is as wide as its segments, also inside a grid or
      // a column, which would otherwise stretch it across the row.
      className="bg-blush inline-flex w-fit gap-0.5 rounded-full p-[3px]"
    >
      {options.map((option) => (
        <label
          key={option.value}
          // A segment is 36px tall, as designed. `after:` stretches its
          // tappable area 4px above and below, to the 44px of §8, without
          // changing how it looks.
          className="has-checked:bg-accent has-focus-visible:outline-berry relative flex min-h-9 cursor-pointer items-center rounded-full px-[13px] text-sm leading-none font-medium tabular-nums after:absolute after:inset-x-0 after:-inset-y-1 has-checked:font-bold has-checked:shadow-[inset_0_0_0_1.5px_var(--color-ink)] has-focus-visible:outline-3 has-focus-visible:outline-offset-2"
        >
          <input
            type="radio"
            className="sr-only"
            name={name}
            value={option.value}
            checked={option.value === value}
            onChange={() => onChange(option.value)}
          />
          {option.label}
        </label>
      ))}
    </div>
  );
}
