import type { ButtonHTMLAttributes, Ref } from "react";

// The buttons of docs/design-brief.md §5.
//
//   primary    one per screen: a HotPink fill with an ink outline. HotPink
//              is too light to be a border or text, so ink carries both.
//   secondary  "Close", "Log out", the "no" side of a question.
//
// size "large" (54px) is the primary button standing alone; "regular"
// (48px) is every secondary button, and a primary button beside one.

type Props = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "primary" | "secondary";
  size?: "large" | "regular";
  // React 19 passes a ref as a prop; it reaches the <button> with the rest.
  ref?: Ref<HTMLButtonElement>;
};

const VARIANT = {
  primary:
    "bg-accent text-ink w-full font-extrabold shadow-[inset_0_0_0_1.5px_var(--color-ink)] active:translate-y-px",
  secondary:
    "border-ink bg-surface text-ink border-[1.5px] px-[18px] font-bold",
};

const SIZE = {
  large: "min-h-[54px] rounded-2xl text-[1.0625rem]",
  regular: "min-h-12 rounded-[14px] text-base",
};

export function Button({
  variant = "primary",
  size = variant === "primary" ? "large" : "regular",
  type = "button",
  className = "",
  ...rest
}: Props) {
  return (
    <button
      type={type}
      className={`${VARIANT[variant]} ${SIZE[size]} cursor-pointer disabled:cursor-default disabled:opacity-50 ${className}`}
      {...rest}
    />
  );
}

// A button for what can't be undone: "Delete entry", and "Delete account"
// in Settings. Bold text in the danger colour, no fill and no underline, as
// the mock's danger-link. It never acts by itself: a question asks first.
// The padding makes it 44px tall, for §8's tap area.
export function DangerButton({
  type = "button",
  className = "",
  ref,
  ...rest
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  // So the page can put the focus back on it ("Keep it").
  ref?: Ref<HTMLButtonElement>;
}) {
  return (
    <button
      ref={ref}
      type={type}
      className={`text-danger min-h-11 cursor-pointer justify-self-start py-3 font-bold disabled:cursor-default disabled:opacity-50 ${className}`}
      {...rest}
    />
  );
}

// A button that reads as a link: "Forgot password?", "Create account".
//
// Standing alone it has 10px of padding above and below, for a comfortable
// tap. `inline` drops that, for a link inside a sentence; it then takes the
// sentence's own colour and size.
export function LinkButton({
  type = "button",
  inline = false,
  className = "",
  ...rest
}: ButtonHTMLAttributes<HTMLButtonElement> & { inline?: boolean }) {
  return (
    <button
      type={type}
      className={`${inline ? "" : "text-berry py-2.5"} cursor-pointer font-bold underline decoration-[1.5px] underline-offset-[3px] disabled:cursor-default disabled:opacity-50 ${className}`}
      {...rest}
    />
  );
}
