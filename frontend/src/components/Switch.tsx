import { useId } from "react";

// On or off, taking effect as it is flipped: "Cycle starts on this day",
// "Show raw readings" (docs/design-brief.md §5). A checkbox told to
// announce itself as a switch; the thumb is its ::after. The row is 44px
// tall and the text is its label, so tapping the words flips it too.

type Props = {
  label: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
};

export function Switch({ label, checked, onChange }: Props) {
  const id = useId();
  return (
    <div className="flex min-h-11 items-center justify-between gap-3">
      <label htmlFor={id} className="cursor-pointer font-bold">
        {label}
      </label>
      <input
        id={id}
        type="checkbox"
        role="switch"
        checked={checked}
        onChange={(event) => onChange(event.target.checked)}
        // 50×30 as designed. `before:` stretches the tappable area to 44px
        // tall; `after:` is the thumb, which slides 20px when on.
        className="bg-rule checked:bg-accent after:bg-surface relative h-[30px] w-[50px] flex-none cursor-pointer appearance-none rounded-full transition-colors duration-150 before:absolute before:inset-x-0 before:-inset-y-[7px] after:absolute after:top-[3px] after:left-[3px] after:size-6 after:rounded-full after:shadow-[0_1px_3px_rgb(43_23_38/0.35)] after:transition-transform after:duration-150 checked:shadow-[inset_0_0_0_1.5px_var(--color-ink)] checked:after:translate-x-5"
      />
    </div>
  );
}
