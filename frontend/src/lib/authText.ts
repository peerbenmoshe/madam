// The words the sign-in screens show: what's wrong with what was typed, and
// what a failure from Supabase Auth means for the person. Plain functions
// with no browser and no network in them, so `npm test` covers every line.
//
// Supabase's own messages are never shown. They are written for developers
// ("Invalid login credentials"), and one of them is ours to word anyway.

// What we read from a failure: the fields of auth-js's AuthError.
export type AuthFailure = {
  name?: string;
  status?: number;
  code?: string;
  message?: string;
};

// The shortest password the page accepts. Supabase's own minimum is lower
// (6, measured on 12 November 2026) and is checked there too.
export const MIN_PASSWORD_LENGTH = 8;
// bcrypt, which Supabase hashes passwords with, reads only the first 72
// bytes. A longer password would be silently cut, so it is refused.
const MAX_PASSWORD_BYTES = 72;

// The exact text our sign-up hook refuses with (backend/internal/api/hook.go).
// Supabase passes it through with status 400 and error_code "unknown"
// (measured on production), so the message is the only way to recognise it.
const INVITE_REFUSAL = "Invalid invite code.";

export const TRY_AGAIN = "Something went wrong. Try again.";
// Supabase refused a password the page had let through: its settings are
// stricter than the page knows. Say the whole rule.
const WEAK_PASSWORD =
  "Choose a stronger password: at least 8 characters, with an English letter and a digit.";
export const OFFLINE =
  "Can't reach the server. Check your connection and try again.";
const TOO_MANY = "Too many attempts. Wait a few minutes and try again.";

export function emailProblem(email: string): string | null {
  const value = email.trim();
  if (value === "") {
    return "Enter your email address.";
  }
  // Only the obvious slips: something, an @, something with a dot. Whether
  // the address really exists is what the confirmation email finds out.
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value)) {
    return "Enter a valid email address.";
  }
  return null;
}

// The rule in one line, shown under the field before anything is typed.
export const PASSWORD_RULE = `At least ${MIN_PASSWORD_LENGTH} characters, with an English letter and a digit.`;

// The same rule on the log-in screen, as a memory aid: knowing what a
// password had to contain helps recall which one it was.
export const PASSWORD_REMINDER = `Reminder: at least ${MIN_PASSWORD_LENGTH} characters, with an English letter and a digit.`;

// English letters, digits, the usual symbols and the space: the characters
// every keyboard layout can type (printable ASCII).
const ENGLISH_KEYBOARD_ONLY = /^[\x20-\x7E]*$/;

// True when a password holds a character an English keyboard doesn't type.
// On a keyboard that switches between two languages, that almost always
// means it was on the other one.
export function typedOnAnotherKeyboard(password: string): boolean {
  return !ENGLISH_KEYBOARD_ONLY.test(password);
}

// For a password being chosen (sign-up, reset), not one being typed to log in.
//
// Measured against Supabase on 13 November 2026, with its "letters and
// digits" requirement on: a letter is a to z in either case, a digit is 0
// to 9, and the minimum is 8. Hebrew letters were allowed but never
// counted, so "Hebrew letters and digits" was refused and adding one English
// letter passed. The page is stricter, on purpose (Pe'er): **only**
// characters an English keyboard types. Choosing a password with the
// keyboard on the wrong language, unnoticed, is how people lock themselves
// out, and this is the one moment it can be caught. The rule is mild
// otherwise: it stops "12345678", not a determined bad password.
export function newPasswordProblem(password: string): string | null {
  // First, because it is the likeliest slip and the other messages would
  // only confuse: "use 8 characters" about a password typed in Hebrew.
  if (typedOnAnotherKeyboard(password)) {
    return "Use English letters, digits and symbols only. Is your keyboard set to another language?";
  }
  if (password.length < MIN_PASSWORD_LENGTH) {
    return `Use at least ${MIN_PASSWORD_LENGTH} characters.`;
  }
  if (!/[a-zA-Z]/.test(password) || !/[0-9]/.test(password)) {
    return "Include at least one English letter and one digit.";
  }
  // Every allowed character is one byte, so characters are bytes here.
  if (password.length > MAX_PASSWORD_BYTES) {
    return "That password is too long. Use a shorter one.";
  }
  return null;
}

export function inviteCodeProblem(code: string): string | null {
  return code.trim() === "" ? "Enter your invite code." : null;
}

// True when the request never got an answer: offline, DNS, a timeout.
function neverAnswered(failure: AuthFailure): boolean {
  return (
    failure.name === "AuthRetryableFetchError" ||
    failure.name === "AuthUnknownError" ||
    failure.status === 0 ||
    failure.status === undefined
  );
}

function isRateLimit(failure: AuthFailure): boolean {
  return (
    failure.status === 429 ||
    failure.code === "over_request_rate_limit" ||
    failure.code === "over_email_send_rate_limit"
  );
}

// Where a sign-up problem is shown: under one of the fields, or for the
// whole form.
export type SignUpProblem = {
  field: "email" | "password" | "invite" | "form";
  message: string;
};

export function signUpProblem(failure: AuthFailure): SignUpProblem {
  if (failure.status === 400 && failure.message === INVITE_REFUSAL) {
    return { field: "invite", message: "That invite code isn't right." };
  }
  if (
    failure.code === "weak_password" ||
    failure.name === "AuthWeakPasswordError"
  ) {
    return { field: "password", message: WEAK_PASSWORD };
  }
  if (
    failure.code === "validation_failed" ||
    failure.code === "email_address_invalid"
  ) {
    return { field: "email", message: "Enter a valid email address." };
  }
  if (failure.code === "email_address_not_authorized") {
    return { field: "email", message: "We can't send email to that address." };
  }
  if (isRateLimit(failure)) {
    return { field: "form", message: TOO_MANY };
  }
  if (neverAnswered(failure)) {
    return { field: "form", message: OFFLINE };
  }
  return { field: "form", message: TRY_AGAIN };
}

// password: what was typed, only to notice a keyboard on another language.
export function logInProblem(failure: AuthFailure, password = ""): string {
  if (failure.code === "invalid_credentials") {
    // The same words whether the address is unknown or the password is
    // wrong, as Supabase answers both the same way.
    const wrong = "Wrong email or password.";
    return typedOnAnotherKeyboard(password)
      ? `${wrong} Check that your keyboard is set to English.`
      : wrong;
  }
  if (failure.code === "email_not_confirmed") {
    return "Confirm your email first: open the link we sent you.";
  }
  if (isRateLimit(failure)) {
    return TOO_MANY;
  }
  if (neverAnswered(failure)) {
    return OFFLINE;
  }
  return TRY_AGAIN;
}

// After this many wrong passwords in a row, the log-in screen offers the
// way out: reset it by email.
export const WRONG_ATTEMPTS_BEFORE_RESET_OFFER = 3;

export function isWrongPassword(failure: AuthFailure): boolean {
  return failure.code === "invalid_credentials";
}

// Asking for a reset link, or for the confirmation email again.
export function sendEmailProblem(failure: AuthFailure): string {
  if (isRateLimit(failure)) {
    return TOO_MANY;
  }
  if (neverAnswered(failure)) {
    return OFFLINE;
  }
  return TRY_AGAIN;
}

export function setPasswordProblem(failure: AuthFailure): string {
  if (failure.code === "same_password") {
    return "That's your current password. Choose a different one.";
  }
  if (
    failure.code === "weak_password" ||
    failure.name === "AuthWeakPasswordError"
  ) {
    return WEAK_PASSWORD;
  }
  if (
    failure.name === "AuthSessionMissingError" ||
    failure.code === "session_not_found"
  ) {
    return "This reset link has expired. Ask for a new one.";
  }
  if (isRateLimit(failure)) {
    return TOO_MANY;
  }
  if (neverAnswered(failure)) {
    return OFFLINE;
  }
  return TRY_AGAIN;
}

// alreadyRegistered recognises Supabase's answer to a sign-up for an address
// that already has a confirmed account. It doesn't refuse: it answers
// "success" with a made-up user that has no identities, creates nothing and
// sends no email, so that the form can't be used to find out who has an
// account. A real new user always has one identity (her email).
//
// Madam shows it anyway: "this email already has an account" (Pe'er, 13
// November 2026), because the silent version left a real person waiting
// for an email that never comes. The cost is accepted and written down in
// docs/tech-stack.md §4: anyone can learn from the sign-up form whether an
// address has an account.
export function alreadyRegistered(
  user: { identities?: unknown[] | null } | null | undefined,
): boolean {
  return (
    user != null &&
    Array.isArray(user.identities) &&
    user.identities.length === 0
  );
}

// The sign-up screen follows it with a "Log in instead" link.
export const ALREADY_REGISTERED = "This email already has an account.";

// How long "Send it again" stays switched off after an email was sent.
// Supabase has its own limit per address; this one saves the round trip and
// tells her how long is left.
export const RESEND_WAIT_SECONDS = 60;

// Whole seconds from now until a moment, never below zero. Both are
// milliseconds, as Date.now() gives them.
export function secondsUntil(nowMs: number, momentMs: number): number {
  return Math.max(0, Math.ceil((momentMs - nowMs) / 1000));
}

export function resendWaitText(seconds: number): string {
  return `You can send it again in ${seconds} ${seconds === 1 ? "second" : "seconds"}.`;
}
