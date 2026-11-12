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

// For a password being chosen (sign-up, reset), not one being typed to log in.
export function newPasswordProblem(password: string): string | null {
  if (password.length < MIN_PASSWORD_LENGTH) {
    return `Use at least ${MIN_PASSWORD_LENGTH} characters.`;
  }
  if (new TextEncoder().encode(password).length > MAX_PASSWORD_BYTES) {
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
    return { field: "password", message: "Choose a longer password." };
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

export function logInProblem(failure: AuthFailure): string {
  if (failure.code === "invalid_credentials") {
    // The same words whether the address is unknown or the password is
    // wrong, as Supabase answers both the same way.
    return "Wrong email or password.";
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
    return "Choose a longer password.";
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
