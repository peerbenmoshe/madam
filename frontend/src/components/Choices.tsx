import { useId } from "react";
import type { Choice } from "./SegmentedControl.tsx";

// The form's choices (docs/design-brief.md §5):
//
//   OptionButtons  one answer: bleeding, secretion, sensation. Radio buttons.
//   Chips          any number: disturbances, symptoms. Checkboxes.
//
// Both are real inputs, hidden, inside the labels that are drawn, in a
// fieldset whose legend names the question. Chosen is a HotPink fill with
// an ink border and bold text, so it never rests on colour alone.

const CHOSEN =
  "has-checked:bg-accent has-checked:border-ink has-checked:font-bold";
const FOCUS =
  "has-focus-visible:outline-berry has-focus-visible:outline-3 has-focus-visible:outline-offset-2";
const BASE = `border-rule bg-surface relative flex cursor-pointer items-center border-[1.5px] font-medium ${CHOSEN} ${FOCUS}`;

type OptionButtonsProps<T extends string> = {
  legend: string;
  options: readonly Choice<T>[];
  // null while nothing is chosen yet.
  value: T | null;
  onChange: (value: T) => void;
};

export function OptionButtons<T extends string>({
  legend,
  options,
  value,
  onChange,
}: OptionButtonsProps<T>) {
  const name = useId();
  return (
    <fieldset className="min-w-0">
      <legend className="mb-2.5 font-bold">{legend}</legend>
      <div className="flex flex-wrap gap-2">
        {options.map((option) => (
          <label
            key={option.value}
            className={`${BASE} min-h-12 rounded-[14px] px-4`}
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
    </fieldset>
  );
}

type ChipsProps<T extends string> = {
  legend: string;
  options: readonly Choice<T>[];
  values: readonly T[];
  onChange: (values: T[]) => void;
};

export function Chips<T extends string>({
  legend,
  options,
  values,
  onChange,
}: ChipsProps<T>) {
  function toggle(value: T, on: boolean) {
    // Kept in the options' order, whatever order they were tapped in.
    onChange(
      options
        .map((option) => option.value)
        .filter((v) => (v === value ? on : values.includes(v))),
    );
  }
  return (
    <fieldset className="min-w-0">
      <legend className="mb-2.5 font-bold">{legend}</legend>
      <div className="flex flex-wrap gap-2">
        {options.map((option) => (
          <label
            key={option.value}
            // 40px tall as designed; `after:` adds 2px of tappable area
            // above and below, for the 44px of §8.
            className={`${BASE} min-h-10 rounded-full px-3.5 text-sm after:absolute after:inset-x-0 after:-inset-y-0.5`}
          >
            <input
              type="checkbox"
              className="sr-only"
              checked={values.includes(option.value)}
              onChange={(event) => toggle(option.value, event.target.checked)}
            />
            {option.label}
          </label>
        ))}
      </div>
    </fieldset>
  );
}
