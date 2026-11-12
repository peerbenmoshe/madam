// Run by `npm test`. Plain JavaScript for the reason given in date.test.mjs.
import assert from "node:assert/strict";
import { test } from "node:test";

const {
  ALREADY_REGISTERED,
  MIN_PASSWORD_LENGTH,
  PASSWORD_RULE,
  RESEND_WAIT_SECONDS,
  TRY_AGAIN,
  alreadyRegistered,
  resendWaitText,
  secondsUntil,
  emailProblem,
  inviteCodeProblem,
  logInProblem,
  newPasswordProblem,
  sendEmailProblem,
  setPasswordProblem,
  signUpProblem,
} = await import("./authText.ts");

// Failures as auth-js reports them. The first five were measured against
// Supabase on 12 November 2026; the rest follow auth-js's error classes.
const INVITE = {
  name: "AuthApiError",
  status: 400,
  code: "unknown",
  message: "Invalid invite code.",
};
const WEAK = {
  name: "AuthWeakPasswordError",
  status: 422,
  code: "weak_password",
  message: "Password should be at least 6 characters.",
};
const BAD_EMAIL = {
  name: "AuthApiError",
  status: 400,
  code: "validation_failed",
  message: "Unable to validate email address: invalid format",
};
const BAD_LOGIN = {
  name: "AuthApiError",
  status: 400,
  code: "invalid_credentials",
  message: "Invalid login credentials",
};
const NO_SESSION = {
  name: "AuthSessionMissingError",
  status: 400,
  code: undefined,
  message: "Auth session missing!",
};
const UNCONFIRMED = {
  name: "AuthApiError",
  status: 400,
  code: "email_not_confirmed",
  message: "Email not confirmed",
};
const RATE = {
  name: "AuthApiError",
  status: 429,
  code: "over_email_send_rate_limit",
  message: "email rate limit exceeded",
};
const OFFLINE = {
  name: "AuthRetryableFetchError",
  status: 0,
  code: undefined,
  message: "Failed to fetch",
};
const HOOK_DOWN = {
  name: "AuthApiError",
  status: 500,
  code: "unexpected_failure",
  message: "Unexpected status code returned from hook: 503",
};
const WEAK_TEXT =
  "Choose a stronger password: at least 8 characters, with a letter and a digit.";
const SAME = {
  name: "AuthApiError",
  status: 422,
  code: "same_password",
  message: "New password should be different from the old password.",
};

test("emailProblem catches the obvious slips only", () => {
  for (const ok of [
    "a@b.co",
    "  first.last+tag@example.com  ",
    "x@sub.example.org",
  ]) {
    assert.equal(emailProblem(ok), null, ok);
  }
  assert.equal(emailProblem(""), "Enter your email address.");
  assert.equal(emailProblem("   "), "Enter your email address.");
  for (const bad of [
    "plain",
    "a@b",
    "@b.co",
    "a@.co",
    "a b@c.de",
    "a@b@c.de",
  ]) {
    assert.equal(emailProblem(bad), "Enter a valid email address.", bad);
  }
});

test("newPasswordProblem: 8 characters, a letter and a digit, and nothing bcrypt would cut", () => {
  const COMPOSITION = "Include at least one letter (a–z) and one digit.";
  assert.equal(MIN_PASSWORD_LENGTH, 8);
  assert.equal(
    PASSWORD_RULE,
    "At least 8 characters, with a letter and a digit.",
  );

  // Length is checked first, so a short password hears about length.
  assert.equal(newPasswordProblem(""), "Use at least 8 characters.");
  assert.equal(newPasswordProblem("abc1234"), "Use at least 8 characters.");
  assert.equal(newPasswordProblem("abcd1234"), null);

  // Long enough, but only one kind of character.
  assert.equal(newPasswordProblem("12345678"), COMPOSITION);
  assert.equal(newPasswordProblem("abcdefgh"), COMPOSITION);
  assert.equal(newPasswordProblem("!!!!!!!!"), COMPOSITION);
  // Either case of letter counts, and one of each is enough.
  assert.equal(newPasswordProblem("ABCDEFG1"), null);
  assert.equal(newPasswordProblem("1234567z"), null);
  // A letter means a to z, as Supabase counts it: Hebrew letters and
  // digits alone don't pass, and adding one a-z letter does.
  assert.equal(newPasswordProblem("שלוםעולם12"), COMPOSITION);
  assert.equal(newPasswordProblem("שלוםעולם1a"), null);
  // Spaces count, and nothing is trimmed: a password is exactly what was typed.
  assert.equal(newPasswordProblem("  a1    "), null);

  // 72 is bytes, not characters.
  assert.equal(newPasswordProblem("a1".repeat(36)), null);
  assert.equal(
    newPasswordProblem("a1".repeat(36) + "a"),
    "That password is too long. Use a shorter one.",
  );
  // 24 Hebrew letters are 48 bytes; 24 euro signs are 72. Plus "a1".
  assert.equal(newPasswordProblem("ש".repeat(24) + "a1"), null);
  assert.equal(
    newPasswordProblem("€".repeat(24) + "a1"),
    "That password is too long. Use a shorter one.",
  );
});

test("inviteCodeProblem only requires something", () => {
  assert.equal(inviteCodeProblem(""), "Enter your invite code.");
  assert.equal(inviteCodeProblem("  "), "Enter your invite code.");
  assert.equal(inviteCodeProblem("four random words"), null);
});

test("signUpProblem puts each failure under the right field", () => {
  assert.deepEqual(signUpProblem(INVITE), {
    field: "invite",
    message: "That invite code isn't right.",
  });
  assert.deepEqual(signUpProblem(WEAK), {
    field: "password",
    message: WEAK_TEXT,
  });
  assert.deepEqual(signUpProblem(BAD_EMAIL), {
    field: "email",
    message: "Enter a valid email address.",
  });
  assert.equal(signUpProblem(RATE).field, "form");
  assert.match(signUpProblem(RATE).message, /Too many attempts/);
  assert.match(signUpProblem(OFFLINE).message, /Can't reach the server/);
  // The hook being down is our outage, not her mistake.
  assert.deepEqual(signUpProblem(HOOK_DOWN), {
    field: "form",
    message: TRY_AGAIN,
  });
  // Another 400 with another message is not the invite code.
  assert.equal(
    signUpProblem({ ...INVITE, message: "Signups not allowed" }).field,
    "form",
  );
  // The refusal text with another status isn't ours either.
  assert.equal(signUpProblem({ ...INVITE, status: 500 }).field, "form");
});

test("logInProblem never says which of the two was wrong", () => {
  assert.equal(logInProblem(BAD_LOGIN), "Wrong email or password.");
  assert.match(logInProblem(UNCONFIRMED), /Confirm your email first/);
  assert.match(logInProblem(RATE), /Too many attempts/);
  assert.match(logInProblem(OFFLINE), /Can't reach the server/);
  assert.equal(logInProblem(HOOK_DOWN), TRY_AGAIN);
});

test("a rate limit is recognised by its status or by either of its codes", () => {
  const byStatus = { name: "AuthApiError", status: 429, code: undefined };
  const byRequestCode = {
    name: "AuthApiError",
    status: 400,
    code: "over_request_rate_limit",
  };
  const byEmailCode = {
    name: "AuthApiError",
    status: 400,
    code: "over_email_send_rate_limit",
  };
  for (const failure of [byStatus, byRequestCode, byEmailCode]) {
    assert.match(signUpProblem(failure).message, /Too many attempts/);
    assert.match(logInProblem(failure), /Too many attempts/);
    assert.match(sendEmailProblem(failure), /Too many attempts/);
    assert.match(setPasswordProblem(failure), /Too many attempts/);
  }
});

test("sendEmailProblem and setPasswordProblem", () => {
  assert.match(sendEmailProblem(RATE), /Too many attempts/);
  assert.match(sendEmailProblem(OFFLINE), /Can't reach the server/);
  assert.equal(sendEmailProblem(HOOK_DOWN), TRY_AGAIN);

  assert.match(setPasswordProblem(SAME), /current password/);
  assert.equal(setPasswordProblem(WEAK), WEAK_TEXT);
  assert.match(setPasswordProblem(NO_SESSION), /link has expired/);
  assert.match(setPasswordProblem(OFFLINE), /Can't reach the server/);
  assert.equal(setPasswordProblem(HOOK_DOWN), TRY_AGAIN);
});

test("no message ever repeats Supabase's own words", () => {
  const failures = [
    INVITE,
    WEAK,
    BAD_EMAIL,
    BAD_LOGIN,
    NO_SESSION,
    UNCONFIRMED,
    RATE,
    OFFLINE,
    HOOK_DOWN,
    SAME,
  ];
  for (const failure of failures) {
    const shown = [
      signUpProblem(failure).message,
      logInProblem(failure),
      sendEmailProblem(failure),
      setPasswordProblem(failure),
    ];
    for (const message of shown) {
      assert.notEqual(message, failure.message);
      assert.ok(!message.includes("hook"), message);
    }
  }
});

test("alreadyRegistered recognises Supabase's made-up success", () => {
  // The screen adds a "Log in instead" link after this sentence.
  assert.equal(ALREADY_REGISTERED, "This email already has an account.");
  // What Supabase sends for an address that already has a confirmed account.
  assert.equal(
    alreadyRegistered({ id: "x", email: "a@b.co", identities: [] }),
    true,
  );
  // A real new account has one identity: her email.
  assert.equal(
    alreadyRegistered({ id: "x", identities: [{ provider: "email" }] }),
    false,
  );
  // Anything unclear is not treated as "already registered": the default
  // must be the ordinary "check your email".
  assert.equal(alreadyRegistered({ id: "x" }), false);
  assert.equal(alreadyRegistered({ id: "x", identities: null }), false);
  assert.equal(alreadyRegistered(null), false);
  assert.equal(alreadyRegistered(undefined), false);
});

test("the wait before sending again counts down in whole seconds", () => {
  assert.equal(RESEND_WAIT_SECONDS, 60);
  const sentAt = 1_000_000;
  const availableAt = sentAt + RESEND_WAIT_SECONDS * 1000;
  assert.equal(secondsUntil(sentAt, availableAt), 60);
  assert.equal(secondsUntil(sentAt + 1, availableAt), 60);
  assert.equal(secondsUntil(sentAt + 1000, availableAt), 59);
  assert.equal(secondsUntil(availableAt - 1, availableAt), 1);
  assert.equal(secondsUntil(availableAt, availableAt), 0);
  // Never negative, however long ago it was.
  assert.equal(secondsUntil(availableAt + 5000, availableAt), 0);

  assert.equal(resendWaitText(45), "You can send it again in 45 seconds.");
  assert.equal(resendWaitText(1), "You can send it again in 1 second.");
});
