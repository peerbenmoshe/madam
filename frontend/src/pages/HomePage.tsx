import { useEffect, useState } from "react";
import type { Session } from "@supabase/auth-js";
import { Button } from "../components/Button.tsx";
import { FormError, Notice, Screen } from "../components/Screen.tsx";
import { ApiError, fetchMe } from "../lib/api.ts";
import { authClient } from "../lib/auth.ts";
import { OFFLINE, TRY_AGAIN } from "../lib/authText.ts";

// What a signed-in person sees until onboarding and the chart exist
// (Slices 3 and 6): proof that the account works end to end, and a way out.
//
// It calls GET /api/me with her token. That is the first request our own
// server verifies, and it creates her row in our database.

type Account =
  | { state: "checking" }
  | { state: "ready" }
  | { state: "failed"; message: string };

export function HomePage({ session }: { session: Session }) {
  const [account, setAccount] = useState<Account>({ state: "checking" });
  const [busy, setBusy] = useState(false);
  const [logOutProblem, setLogOutProblem] = useState<string | null>(null);

  useEffect(() => {
    // Ignore an answer that arrives after the page has gone.
    let current = true;
    fetchMe().then(
      () => {
        if (current) setAccount({ state: "ready" });
      },
      (error: unknown) => {
        if (!current) return;
        const status = error instanceof ApiError ? error.status : -1;
        setAccount({
          state: "failed",
          message:
            status === 0
              ? OFFLINE
              : status === 401
                ? "Your session wasn't accepted. Log out and log in again."
                : TRY_AGAIN,
        });
      },
    );
    return () => {
      current = false;
    };
  }, []);

  async function logOut() {
    setBusy(true);
    setLogOutProblem(null);
    try {
      const auth = await authClient();
      // "local": this browser only. Her other devices stay logged in.
      // Supabase is told first, so offline it fails and she stays logged in.
      const { error } = await auth.signOut({ scope: "local" });
      if (error) {
        setLogOutProblem(OFFLINE);
      }
    } catch {
      setLogOutProblem(OFFLINE);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Screen
      title="You're logged in"
      sub={<span className="wrap-anywhere">{session.user.email}</span>}
    >
      <div className="grid gap-6">
        {account.state === "checking" && (
          <p className="text-muted">Checking your account…</p>
        )}
        {account.state === "ready" && (
          <Notice>Your account is ready. The chart comes next.</Notice>
        )}
        {account.state === "failed" && <FormError>{account.message}</FormError>}
        {logOutProblem && <FormError>{logOutProblem}</FormError>}
        <div>
          <Button
            variant="secondary"
            disabled={busy}
            onClick={() => void logOut()}
          >
            Log out
          </Button>
        </div>
      </div>
    </Screen>
  );
}
