import { useEffect, useRef, useState } from "react";
import type { Session } from "@supabase/auth-js";
import { arrivedByRecoveryLink, authClient } from "./auth.ts";

// What the app knows about who is using it.
export type AuthState =
  // Still asking: the page has just loaded.
  | { status: "loading" }
  // Our server or Supabase couldn't be reached at all.
  | { status: "unavailable" }
  // Nobody is signed in. linkFailed: the page was opened from an emailed
  // link that has expired or was already used.
  | { status: "signedOut"; linkFailed: boolean }
  // Signed in. recovering: she came from a reset link and hasn't chosen her
  // new password yet.
  | { status: "signedIn"; session: Session; recovering: boolean };

// useAuth follows the session for as long as the app is open: logging in or
// out here or in another tab, and the token being refreshed, all arrive
// through the client's own events.
export function useAuth(): {
  state: AuthState;
  // Called once the new password is saved, to leave "recovering".
  finishRecovery: () => void;
} {
  const [state, setState] = useState<AuthState>({ status: "loading" });
  // Not state: changing it alone shouldn't re-render, and the event
  // handler below must read the current value, not the one it started with.
  const recovering = useRef(arrivedByRecoveryLink);

  useEffect(() => {
    let active = true;
    let unsubscribe = () => {};

    void (async () => {
      try {
        const auth = await authClient();
        // Reads a session from the link the page was opened with, or from
        // storage. An error means the link was no good.
        const { error } = await auth.initialize();
        if (!active) {
          return;
        }
        const linkFailed = error !== null;
        if (linkFailed) {
          // An expired reset link proves nothing, even if an older session
          // in this browser is still good: no new-password screen.
          recovering.current = false;
        }

        const { data } = auth.onAuthStateChange((event, session) => {
          if (!active) {
            return;
          }
          if (event === "PASSWORD_RECOVERY") {
            recovering.current = true;
          }
          if (session === null) {
            recovering.current = false;
            setState({ status: "signedOut", linkFailed });
            return;
          }
          setState({
            status: "signedIn",
            session,
            recovering: recovering.current,
          });
        });
        unsubscribe = () => data.subscription.unsubscribe();
      } catch {
        if (active) {
          setState({ status: "unavailable" });
        }
      }
    })();

    return () => {
      active = false;
      unsubscribe();
    };
  }, []);

  // The invite code typed at sign-up stays in the account's metadata, and so
  // in every token. Once she is in, it has done its job: clear it, so the
  // shared code lives only on the server (docs/tech-stack.md §4). Best
  // effort, and done from an effect because the client must not be called
  // from inside its own event handler.
  const signedIn = state.status === "signedIn" ? state.session : null;
  const hasInviteCode = signedIn?.user.user_metadata?.invite_code != null;
  const clearing = useRef(false);
  useEffect(() => {
    if (!hasInviteCode || clearing.current) {
      return;
    }
    clearing.current = true;
    void authClient()
      .then(async (auth) => {
        const { error } = await auth.updateUser({
          data: { invite_code: null },
        });
        // The token she holds was made before the change and still carries
        // the code, for up to an hour. A fresh one doesn't.
        if (!error) {
          await auth.refreshSession();
        }
      })
      .catch(() => {
        // Harmless if it fails: the next page load tries again.
      })
      .finally(() => {
        clearing.current = false;
      });
  }, [hasInviteCode]);

  function finishRecovery() {
    recovering.current = false;
    setState((current) =>
      current.status === "signedIn"
        ? { ...current, recovering: false }
        : current,
    );
  }

  return { state, finishRecovery };
}
