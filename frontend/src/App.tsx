import { useEffect, useState } from "react";
import type { Session } from "@supabase/auth-js";
import { Button } from "./components/Button.tsx";
import { Screen } from "./components/Screen.tsx";
import { ApiError, fetchMe } from "./lib/api.ts";
import type { Me } from "./lib/api.ts";
import { accountProblem } from "./lib/authText.ts";
import { navigate, usePath } from "./lib/router.ts";
import { useAuth } from "./lib/useAuth.ts";
import DesignPage from "./pages/DesignPage.tsx";
import { ForgotPasswordPage } from "./pages/ForgotPasswordPage.tsx";
import { HomePage } from "./pages/HomePage.tsx";
import { LogInPage } from "./pages/LogInPage.tsx";
import { OnboardingPage } from "./pages/OnboardingPage.tsx";
import {
  ResetLinkExpired,
  ResetPasswordPage,
} from "./pages/ResetPasswordPage.tsx";
import { SignUpPage } from "./pages/SignUpPage.tsx";

// Which screen to show. Two things decide: the path in the address bar
// (lib/router.ts) and whether anyone is signed in (lib/useAuth.ts).
export default function App() {
  const path = usePath();
  // The component gallery needs no account and no Supabase.
  if (path === "/design") {
    return <DesignPage />;
  }
  return <Account path={path} />;
}

// The paths a signed-out person can be on. "/" is the log-in screen.
const SIGNED_OUT_PATHS = [
  "/",
  "/signup",
  "/forgot-password",
  "/reset-password",
];

function Account({ path }: { path: string }) {
  const { state, finishRecovery } = useAuth();

  // Keep the address bar honest about what is on screen. `replace`, so the
  // Back button doesn't return to a screen that no longer applies.
  const signedIn = state.status === "signedIn";
  const recovering = signedIn && state.recovering;
  useEffect(() => {
    if (recovering && path !== "/reset-password") {
      navigate("/reset-password", { replace: true });
    } else if (signedIn && !recovering && path !== "/") {
      navigate("/", { replace: true });
    } else if (
      state.status === "signedOut" &&
      !SIGNED_OUT_PATHS.includes(path)
    ) {
      navigate("/", { replace: true });
    }
  }, [state.status, signedIn, recovering, path]);

  switch (state.status) {
    case "loading":
      return <Screen title="Madam" sub="Loading…" />;

    case "unavailable":
      return (
        <Screen
          title="Can't reach the server"
          sub="Check your connection, then try again."
        >
          <Button onClick={() => window.location.reload()}>Try again</Button>
        </Screen>
      );

    case "signedIn":
      // She came from a reset email: nothing else until the new password
      // is chosen.
      if (state.recovering) {
        return <ResetPasswordPage onDone={finishRecovery} />;
      }
      // Keyed by the user, so a different account never inherits what was
      // read for the last one.
      return <SignedIn key={state.session.user.id} session={state.session} />;

    case "signedOut":
      return <SignedOut path={path} linkFailed={state.linkFailed} />;
  }
}

// What the app knows about the signed-in person's account in our own
// database, which is separate from her session with Supabase.
type Account =
  | { state: "checking" }
  | { state: "ready"; me: Me }
  // status: the HTTP status of the failed read; 0 for no answer at all.
  | { state: "failed"; status: number };

// The screens for someone who is signed in. Which one depends on her
// account, read from GET /api/me: until she has answered onboarding she
// sees only that. Onboarding has no address of its own, so it can't be
// skipped by typing one (docs/tech-stack.md §8).
function SignedIn({ session }: { session: Session }) {
  const [account, setAccount] = useState<Account>({ state: "checking" });
  // Counts the reads, so "Try again" can ask for another.
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    // Ignore an answer that arrives after the page has gone.
    let current = true;
    fetchMe().then(
      (me) => {
        if (current) setAccount({ state: "ready", me });
      },
      (error: unknown) => {
        if (current) {
          setAccount({
            state: "failed",
            status: error instanceof ApiError ? error.status : -1,
          });
        }
      },
    );
    return () => {
      current = false;
    };
  }, [attempt]);

  switch (account.state) {
    case "checking":
      return <Screen title="Madam" sub="Loading…" />;

    case "failed":
      return (
        <HomePage
          session={session}
          problem={accountProblem(account.status)}
          // A refused session won't get better by asking again: log out.
          onRetry={
            account.status === 401
              ? undefined
              : () => {
                  setAccount({ state: "checking" });
                  setAttempt((n) => n + 1);
                }
          }
        />
      );

    case "ready":
      if (account.me.onboarded_at === null) {
        return (
          <OnboardingPage onDone={(me) => setAccount({ state: "ready", me })} />
        );
      }
      return <HomePage session={session} />;
  }
}

// The screens for someone who isn't signed in. They share the email
// address being typed, so it follows her from one to the next: from "this
// email already has an account" to Log in, from Log in to Forgot password.
// The state lives here and nowhere else, so it is gone the moment she is
// signed in and this component leaves the page.
function SignedOut({
  path,
  linkFailed,
}: {
  path: string;
  linkFailed: boolean;
}) {
  const [email, setEmail] = useState("");
  switch (path) {
    case "/signup":
      return <SignUpPage email={email} onEmailChange={setEmail} />;
    case "/forgot-password":
      return <ForgotPasswordPage email={email} onEmailChange={setEmail} />;
    case "/reset-password":
      // Here without a session: the link was no good.
      return <ResetLinkExpired />;
    default:
      return (
        <LogInPage
          email={email}
          onEmailChange={setEmail}
          linkFailed={linkFailed}
        />
      );
  }
}
