import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { Button, LinkButton } from "../components/Button.tsx";
import { FormError, Notice, Screen } from "../components/Screen.tsx";
import { TextField } from "../components/TextField.tsx";
import { authClient } from "../lib/auth.ts";
import {
  ALREADY_REGISTERED,
  MIN_PASSWORD_LENGTH,
  OFFLINE,
  RESEND_WAIT_SECONDS,
  alreadyRegistered,
  emailProblem,
  inviteCodeProblem,
  newPasswordProblem,
  resendWaitText,
  secondsUntil,
  sendEmailProblem,
  signUpProblem,
} from "../lib/authText.ts";
import { navigate } from "../lib/router.ts";

// Create an account: email, password, and the invite code (docs/spec-v1.md
// §2). Supabase asks our server about the code before it creates anything
// (the "before user created" hook); the page only passes the code along.
// Then the address must be confirmed from the email Supabase sends.

type FieldErrors = {
  email?: string;
  password?: string;
  invite?: string;
};

export function SignUpPage() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [inviteCode, setInviteCode] = useState("");
  const [errors, setErrors] = useState<FieldErrors>({});
  const [problem, setProblem] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  // The address the confirmation email went to, once it has.
  const [sentTo, setSentTo] = useState<string | null>(null);

  async function createAccount(event: FormEvent) {
    event.preventDefault();
    const found: FieldErrors = {
      email: emailProblem(email) ?? undefined,
      password: newPasswordProblem(password) ?? undefined,
      invite: inviteCodeProblem(inviteCode) ?? undefined,
    };
    setErrors(found);
    setProblem(null);
    if (found.email || found.password || found.invite) {
      return;
    }

    setBusy(true);
    try {
      const auth = await authClient();
      const address = email.trim();
      const { data, error } = await auth.signUp({
        email: address,
        password,
        // Travels to the hook as user_metadata.invite_code. The page clears
        // it from the account after the first log-in (useAuth).
        options: { data: { invite_code: inviteCode.trim() } },
      });
      if (error) {
        const { field, message } = signUpProblem(error);
        if (field === "form") {
          setProblem(message);
        } else {
          setErrors({ [field]: message });
        }
        return;
      }
      if (alreadyRegistered(data.user)) {
        // Nothing was created and no email was sent (authText.ts).
        setErrors({ email: ALREADY_REGISTERED });
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
    return <CheckYourEmail address={sentTo} />;
  }

  return (
    <Screen title="Create account" sub="You need an invite code to join.">
      <form
        noValidate
        onSubmit={(event) => void createAccount(event)}
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
          error={errors.email}
        />
        <TextField
          label="Password"
          type="password"
          name="new-password"
          autoComplete="new-password"
          hint={`At least ${MIN_PASSWORD_LENGTH} characters.`}
          value={password}
          onChange={(event) => setPassword(event.target.value)}
          error={errors.password}
        />
        <TextField
          label="Invite code"
          name="invite-code"
          autoComplete="off"
          autoCapitalize="none"
          autoCorrect="off"
          spellCheck={false}
          value={inviteCode}
          onChange={(event) => setInviteCode(event.target.value)}
          error={errors.invite}
        />
        {problem && <FormError>{problem}</FormError>}
        <Button type="submit" disabled={busy}>
          Create account
        </Button>
        <div>
          <LinkButton onClick={() => navigate("/")}>
            I already have an account
          </LinkButton>
        </div>
      </form>
    </Screen>
  );
}

// After a sign-up was accepted: the confirmation email is on its way.
// "Send it again" is switched off for a minute after each email, the first
// included, with the time left shown.
function CheckYourEmail({ address }: { address: string }) {
  const [note, setNote] = useState<string | null>(null);
  const [problem, setProblem] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  // When sending again becomes possible, and the clock that counts down to
  // it. Both in milliseconds.
  const [availableAt, setAvailableAt] = useState(
    () => Date.now() + RESEND_WAIT_SECONDS * 1000,
  );
  const [now, setNow] = useState(() => Date.now());
  const wait = secondsUntil(now, availableAt);
  const waiting = wait > 0;

  // Tick once a second, only while there is something to count down.
  useEffect(() => {
    if (!waiting) {
      return;
    }
    const timer = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(timer);
  }, [waiting]);

  async function sendAgain() {
    setBusy(true);
    setNote(null);
    setProblem(null);
    try {
      const auth = await authClient();
      const { error } = await auth.resend({ type: "signup", email: address });
      if (error) {
        setProblem(sendEmailProblem(error));
      } else {
        setNote("Sent again.");
        setAvailableAt(Date.now() + RESEND_WAIT_SECONDS * 1000);
        setNow(Date.now());
      }
    } catch {
      setProblem(OFFLINE);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Screen title="Check your email">
      <div className="grid gap-6">
        <p>
          We sent a link to <b className="break-all">{address}</b>. Open it to
          confirm your address and finish creating your account. It works on any
          device.
        </p>
        <p className="text-muted text-sm">
          It can take a minute to arrive. If it isn't in your inbox, look in
          spam.
        </p>
        {note && <Notice>{note}</Notice>}
        {problem && <FormError>{problem}</FormError>}
        <div className="flex flex-wrap justify-between gap-x-6">
          <LinkButton
            disabled={busy || waiting}
            onClick={() => void sendAgain()}
          >
            Send it again
          </LinkButton>
          <LinkButton onClick={() => navigate("/")}>Back to log in</LinkButton>
        </div>
        {waiting && (
          <p className="text-muted text-sm tabular-nums">
            {resendWaitText(wait)}
          </p>
        )}
      </div>
    </Screen>
  );
}
