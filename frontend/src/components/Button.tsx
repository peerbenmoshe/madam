import type { ButtonHTMLAttributes } from "react";

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

// A button that reads as a link: "Forgot password?", "Create account".
export function LinkButton({
  type = "button",
  className = "",
  ...rest
}: ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button
      type={type}
      className={`text-berry cursor-pointer py-2.5 font-bold underline decoration-[1.5px] underline-offset-[3px] disabled:cursor-default disabled:opacity-50 ${className}`}
      {...rest}
    />
  );
}
