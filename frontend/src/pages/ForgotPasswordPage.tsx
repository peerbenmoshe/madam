import { useState } from "react";
import type { FormEvent } from "react";
import { Button, LinkButton } from "../components/Button.tsx";
import { FormError, Screen } from "../components/Screen.tsx";
import { TextField } from "../components/TextField.tsx";
import { authClient } from "../lib/auth.ts";
import { OFFLINE, emailProblem, sendEmailProblem } from "../lib/authText.ts";
import { navigate } from "../lib/router.ts";

// Ask for a password-reset link. The answer is the same whether or not the
// address has an account (Supabase's is too: measured), so this form can't
// be used to find out who has one.
export function ForgotPasswordPage() {
  const [email, setEmail] = useState("");
  const [emailError, setEmailError] = useState<string | null>(null);
  const [problem, setProblem] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [sentTo, setSentTo] = useState<string | null>(null);

  async function sendLink(event: FormEvent) {
    event.preventDefault();
    const issue = emailProblem(email);
    setEmailError(issue);
    setProblem(null);
    if (issue) {
      return;
    }

    setBusy(true);
    try {
      const auth = await authClient();
      const address = email.trim();
      const { error } = await auth.resetPasswordForEmail(address, {
        // Where the emailed link brings her back to. It must be on the
        // project's redirect allow-list (docs/tech-stack.md §4).
        redirectTo: `${window.location.origin}/reset-password`,
      });
      if (error) {
        setProblem(sendEmailProblem(error));
        return;
      }
      setSentTo(address);
    } catch {
      setProblem(OFFLINE);
    } finally {
      setBusy(false);
    }
  }

  if (sentTo !== null) {
    return (
      <Screen title="Check your email">
        <div className="grid gap-6">
          <p>
            If there's an account for <b className="break-all">{sentTo}</b>, a
            link to choose a new password is on its way. It works on any device.
          </p>
          <p className="text-muted text-sm">
            It can take a minute to arrive. If it isn't in your inbox, look in
            spam, and check the address above.
          </p>
          <div>
            <LinkButton onClick={() => navigate("/")}>
              Back to log in
            </LinkButton>
          </div>
        </div>
      </Screen>
    );
  }

  return (
    <Screen
      title="Forgot password?"
      sub="We'll email you a link to choose a new one."
    >
      <form
        noValidate
        onSubmit={(event) => void sendLink(event)}
        className="grid gap-6"
      >
        <TextField
          label="Email"
          type="email"
          name="email"
          autoComplete="username"
          autoCapitalize="none"
          spellCheck={false}
          value={email}
          onChange={(event) => setEmail(event.target.value)}
          error={emailError ?? undefined}
        />
        {problem && <FormError>{problem}</FormError>}
        <Button type="submit" disabled={busy}>
          Send reset link
        </Button>
        <div>
          <LinkButton onClick={() => navigate("/")}>Back to log in</LinkButton>
        </div>
      </form>
    </Screen>
  );
}
