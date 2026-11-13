import { useState } from "react";
import type { FormEvent } from "react";
import { Button, LinkButton } from "../components/Button.tsx";
import { FormError, Notice, Screen } from "../components/Screen.tsx";
import { TextField } from "../components/TextField.tsx";
import { authClient } from "../lib/auth.ts";
import {
  OFFLINE,
  PASSWORD_REMINDER,
  WRONG_ATTEMPTS_BEFORE_RESET_OFFER,
  emailProblem,
  isWrongPassword,
  logInProblem,
} from "../lib/authText.ts";
import { navigate } from "../lib/router.ts";

// Log in. On success nothing here changes the screen: the session arrives
// through useAuth, and App shows the signed-in page.

// linkFailed: the page was opened from an emailed link that had expired or
// had already been used.
type Props = {
  // The address being typed, shared by the signed-out screens (App.tsx) so
  // it follows her between them.
  email: string;
  onEmailChange: (email: string) => void;
  linkFailed: boolean;
};

export function LogInPage({ email, onEmailChange, linkFailed }: Props) {
  const [password, setPassword] = useState("");
  const [emailError, setEmailError] = useState<string | null>(null);
  const [passwordError, setPasswordError] = useState<string | null>(null);
  const [problem, setProblem] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  // Wrong passwords in a row, to know when to offer a reset.
  const [wrongAttempts, setWrongAttempts] = useState(0);

  async function logIn(event: FormEvent) {
    event.preventDefault();
    const emailIssue = emailProblem(email);
    const passwordIssue = password === "" ? "Enter your password." : null;
    setEmailError(emailIssue);
    setPasswordError(passwordIssue);
    setProblem(null);
    if (emailIssue || passwordIssue) {
      return;
    }

    setBusy(true);
    try {
      const auth = await authClient();
      const { error } = await auth.signInWithPassword({
        email: email.trim(),
        password,
      });
      if (error) {
        setProblem(logInProblem(error, password));
        if (isWrongPassword(error)) {
          setWrongAttempts((count) => count + 1);
        }
      }
    } catch {
      setProblem(OFFLINE);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Screen title="Log in">
      <form
        noValidate
        onSubmit={(event) => void logIn(event)}
        className="grid gap-6"
      >
        {linkFailed && (
          <Notice>
            That link has expired or was already used. Log in, or ask for a new
            one.
          </Notice>
        )}
        <TextField
          label="Email"
          type="email"
          name="email"
          autoComplete="username"
          autoCapitalize="none"
          spellCheck={false}
          value={email}
          onChange={(event) => onEmailChange(event.target.value)}
          error={emailError ?? undefined}
        />
        <TextField
          label="Password"
          type="password"
          name="password"
          autoComplete="current-password"
          hint={PASSWORD_REMINDER}
          value={password}
          onChange={(event) => setPassword(event.target.value)}
          error={passwordError ?? undefined}
        />
        {problem && <FormError>{problem}</FormError>}
        {wrongAttempts >= WRONG_ATTEMPTS_BEFORE_RESET_OFFER && (
          <Notice>
            Forgot your password?{" "}
            <LinkButton inline onClick={() => navigate("/forgot-password")}>
              Reset it by email
            </LinkButton>
          </Notice>
        )}
        <Button type="submit" disabled={busy}>
          Log in
        </Button>
        <div className="flex flex-wrap justify-between gap-x-6">
          <LinkButton onClick={() => navigate("/signup")}>
            Create account
          </LinkButton>
          <LinkButton onClick={() => navigate("/forgot-password")}>
            Forgot password?
          </LinkButton>
        </div>
      </form>
    </Screen>
  );
}
