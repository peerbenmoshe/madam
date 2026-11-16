import { useId } from "react";
import type { Choice } from "./SegmentedControl.tsx";

// The form's choices (docs/design-brief.md §5):
//
//   OptionButtons  one answer: bleeding, secretion, sensation. Radio buttons.
//                  With `clearable`, tapping the chosen answer clears it.
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
  // The legend is heard and not shown: a row of answers under a heading it
  // shares with another row (the cervix's firmness and position).
  legendHidden?: boolean;
  // A line of help between the question and its answers.
  hint?: string;
  options: readonly Choice<T>[];
  // null while nothing is chosen yet.
  value: T | null;
} & (
  | {
      // A question that always has an answer once one is tapped: bleeding.
      clearable?: false;
      onChange: (value: T) => void;
    }
  | {
      // A question that may have no answer: secretion, sensation, cervix.
      // Tapping the chosen answer again clears it, and onChange gets null.
      clearable: true;
      onChange: (value: T | null) => void;
    }
);

export function OptionButtons<T extends string>(props: OptionButtonsProps<T>) {
  const { legend, legendHidden, hint, options, value } = props;
  const name = useId();
  const clearHintId = useId();
  return (
    <fieldset
      className="min-w-0"
      aria-describedby={props.clearable ? clearHintId : undefined}
    >
      <legend
        className={
          legendHidden ? "sr-only" : `font-bold ${hint ? "mb-1" : "mb-2.5"}`
        }
      >
        {legend}
      </legend>
      {hint && <p className="text-muted mb-2.5 text-sm">{hint}</p>}
      {props.clearable && (
        // Sighted people find this by trying; a screen reader says it.
        <p id={clearHintId} className="sr-only">
          Optional. Choose the selected answer again to clear it.
        </p>
      )}
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
              onChange={() => props.onChange(option.value)}
              // A radio button can't be un-chosen: a tap or a click on the
              // chosen one fires click and no change. So clearing listens
              // for that click.
              onClick={() => {
                if (props.clearable && option.value === value) {
                  props.onChange(null);
                }
              }}
              // The space bar on the chosen one does nothing at all in the
              // browser, not even a click (measured in Chrome). So the
              // keyboard gets its own way to clear. On an answer that isn't
              // chosen, space is left to the browser, which chooses it.
              onKeyDown={(event) => {
                if (
                  props.clearable &&
                  event.key === " " &&
                  option.value === value
                ) {
                  event.preventDefault();
                  props.onChange(null);
                }
              }}
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
  // A line of help between the question and its answers.
  hint?: string;
  options: readonly Choice<T>[];
  values: readonly T[];
  onChange: (values: T[]) => void;
};

export function Chips<T extends string>({
  legend,
  hint,
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
      <legend className={`font-bold ${hint ? "mb-1" : "mb-2.5"}`}>
        {legend}
      </legend>
      {hint && <p className="text-muted mb-2.5 text-sm">{hint}</p>}
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
