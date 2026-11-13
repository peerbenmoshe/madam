import { useState } from "react";
import type { FormEvent, ReactNode } from "react";
import { Button, LinkButton } from "../components/Button.tsx";
import { FormError, Screen } from "../components/Screen.tsx";
import { SendAgain } from "../components/SendAgain.tsx";
import { TextField } from "../components/TextField.tsx";
import { authClient } from "../lib/auth.ts";
import {
  ALREADY_REGISTERED,
  OFFLINE,
  PASSWORD_RULE,
  alreadyRegistered,
  emailProblem,
  inviteCodeProblem,
  newPasswordProblem,
  signUpProblem,
} from "../lib/authText.ts";
import { navigate } from "../lib/router.ts";

// Create an account: email, password, and the invite code (docs/spec-v1.md
// §2). Supabase asks our server about the code before it creates anything
// (the "before user created" hook); the page only passes the code along.
// Then the address must be confirmed from the email Supabase sends.

type FieldErrors = {
  // A node, not only text: "already has an account" carries a link.
  email?: ReactNode;
  password?: string;
  invite?: string;
};

type Props = {
  // The address being typed, shared by the signed-out screens (App.tsx) so
  // it follows her between them.
  email: string;
  onEmailChange: (email: string) => void;
};

export function SignUpPage({ email, onEmailChange }: Props) {
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
        // Nothing was created and no email was sent (authText.ts). The way
        // out is a link she chooses, not a jump: a mistyped address that
        // happens to be someone else's shouldn't throw her off the form.
        setErrors({
          email: (
            <>
              {ALREADY_REGISTERED}{" "}
              <LinkButton inline onClick={() => navigate("/")}>
                Log in instead
              </LinkButton>
            </>
          ),
        });
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
          onChange={(event) => onEmailChange(event.target.value)}
          error={errors.email}
        />
        <TextField
          label="Password"
          type="password"
          name="new-password"
          autoComplete="new-password"
          hint={PASSWORD_RULE}
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
function CheckYourEmail({ address }: { address: string }) {
  async function sendAgain() {
    const auth = await authClient();
    const { error } = await auth.resend({ type: "signup", email: address });
    return error;
  }

  return (
    <Screen title="Check your email">
      <div className="grid gap-6">
        <p>
          We sent a link to <b className="wrap-anywhere">{address}</b>. Open it
          to confirm your address and finish creating your account. It works on
          any device.
        </p>
        <p className="text-muted text-sm">
          It can take a minute to arrive. If it isn't in your inbox, look in
          spam.
        </p>
        <SendAgain send={sendAgain}>
          <LinkButton onClick={() => navigate("/")}>Back to log in</LinkButton>
        </SendAgain>
      </div>
    </Screen>
  );
}
