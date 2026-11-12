import { formatDate } from "../lib/date.ts";

// Every date input (docs/design-brief.md §5 and §9). A phone's native date
// picker shows dates in the phone's own format, and a page can't change
// that. So the box shows its own dd/mm/yyyy text, with the real
// <input type="date"> stretched over it, invisible. A tap lands on the
// input and opens the native picker, and min and max still block bad dates.

type Props = {
  label: string;
  // "YYYY-MM-DD", or "" while no date is chosen.
  value: string;
  onChange: (value: string) => void;
  min?: string;
  max?: string;
};

export function DateBox({ label, value, onChange, min, max }: Props) {
  return (
    <label className="border-rule bg-surface focus-within:border-berry relative grid min-h-12 cursor-pointer gap-px rounded-xl border-[1.5px] px-3 py-1.5 focus-within:shadow-[0_0_0_3px_var(--color-blush)]">
      <span className="text-muted text-xs">{label}</span>
      <span className="font-bold tabular-nums">
        {value === "" ? (
          <span className="text-muted font-normal">dd/mm/yyyy</span>
        ) : (
          formatDate(value)
        )}
      </span>
      <input
        type="date"
        aria-label={label}
        value={value}
        min={min}
        max={max}
        onChange={(event) => onChange(event.target.value)}
        onClick={(event) => {
          // On a computer a click on a date input only focuses it, so ask
          // for the picker. Browsers without showPicker, or that refuse it
          // here, still let the date be typed.
          try {
            event.currentTarget.showPicker();
          } catch {
            // Nothing to do: the input works without the picker.
          }
        }}
        className="absolute inset-0 size-full cursor-pointer opacity-0"
      />
    </label>
  );
}
