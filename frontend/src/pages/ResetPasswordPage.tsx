import { useState } from "react";
import type { FormEvent } from "react";
import { Button, LinkButton } from "../components/Button.tsx";
import { FormError, Screen } from "../components/Screen.tsx";
import { TextField } from "../components/TextField.tsx";
import { authClient } from "../lib/auth.ts";
import {
  MIN_PASSWORD_LENGTH,
  OFFLINE,
  newPasswordProblem,
  setPasswordProblem,
} from "../lib/authText.ts";
import { navigate } from "../lib/router.ts";

// Choose a new password, after opening the link from a reset email. The
// link has already signed her in (that is how Supabase proves she owns the
// address), so all that's left is the password itself.

// onDone: the password is saved; App then shows the signed-in page.
export function ResetPasswordPage({ onDone }: { onDone: () => void }) {
  const [password, setPassword] = useState("");
  const [passwordError, setPasswordError] = useState<string | null>(null);
  const [problem, setProblem] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function savePassword(event: FormEvent) {
    event.preventDefault();
    const issue = newPasswordProblem(password);
    setPasswordError(issue);
    setProblem(null);
    if (issue) {
      return;
    }

    setBusy(true);
    try {
      const auth = await authClient();
      const { error } = await auth.updateUser({ password });
      if (error) {
        setProblem(setPasswordProblem(error));
        return;
      }
      onDone();
    } catch {
      setProblem(OFFLINE);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Screen title="Choose a new password">
      <form
        noValidate
        onSubmit={(event) => void savePassword(event)}
        className="grid gap-6"
      >
        <TextField
          label="New password"
          type="password"
          name="new-password"
          autoComplete="new-password"
          hint={`At least ${MIN_PASSWORD_LENGTH} characters.`}
          value={password}
          onChange={(event) => setPassword(event.target.value)}
          error={passwordError ?? undefined}
        />
        {problem && <FormError>{problem}</FormError>}
        <Button type="submit" disabled={busy}>
          Save password
        </Button>
      </form>
    </Screen>
  );
}

// /reset-password with nobody signed in: the link was no good, or the page
// was opened without one.
export function ResetLinkExpired() {
  return (
    <Screen
      title="This link has expired"
      sub="A reset link works once, and only for a while."
    >
      <div className="grid gap-4">
        <Button onClick={() => navigate("/forgot-password")}>
          Ask for a new link
        </Button>
        <div>
          <LinkButton onClick={() => navigate("/")}>Back to log in</LinkButton>
        </div>
      </div>
    </Screen>
  );
}
