import { useEffect } from "react";
import { Button } from "./components/Button.tsx";
import { Screen } from "./components/Screen.tsx";
import { navigate, usePath } from "./lib/router.ts";
import { useAuth } from "./lib/useAuth.ts";
import DesignPage from "./pages/DesignPage.tsx";
import { ForgotPasswordPage } from "./pages/ForgotPasswordPage.tsx";
import { HomePage } from "./pages/HomePage.tsx";
import { LogInPage } from "./pages/LogInPage.tsx";
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
      return <HomePage session={state.session} />;

    case "signedOut":
      switch (path) {
        case "/signup":
          return <SignUpPage />;
        case "/forgot-password":
          return <ForgotPasswordPage />;
        case "/reset-password":
          // Here without a session: the link was no good.
          return <ResetLinkExpired />;
        default:
          return <LogInPage linkFailed={state.linkFailed} />;
      }
  }
}
