import { useState } from "react";
import { authClient } from "./auth.ts";
import { OFFLINE } from "./authText.ts";

// Logging out, for every signed-in screen that offers it. The screen draws
// the button; this holds what the button needs.
export function useLogOut(): {
  // True while the request is on its way: switch the button off.
  busy: boolean;
  // What went wrong, to show near the button, or null.
  problem: string | null;
  logOut: () => void;
} {
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState<string | null>(null);

  async function run() {
    setBusy(true);
    setProblem(null);
    try {
      const auth = await authClient();
      // "local": this browser only. Her other devices stay logged in.
      // Supabase is told first, so offline it fails and she stays logged in.
      const { error } = await auth.signOut({ scope: "local" });
      if (error) {
        setProblem(OFFLINE);
      }
    } catch {
      setProblem(OFFLINE);
    } finally {
      setBusy(false);
    }
  }

  return { busy, problem, logOut: () => void run() };
}
