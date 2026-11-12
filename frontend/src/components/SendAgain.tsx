import { useEffect, useState } from "react";
import type { ReactNode } from "react";
import { LinkButton } from "./Button.tsx";
import { FormError, Notice } from "./Screen.tsx";
import {
  OFFLINE,
  RESEND_WAIT_SECONDS,
  resendWaitText,
  secondsUntil,
  sendEmailProblem,
} from "../lib/authText.ts";
import type { AuthFailure } from "../lib/authText.ts";

// "Send it again", for the screens that say an email is on its way. It is
// switched off for a minute after each email, the first one included, and
// shows how long is left. Supabase has its own limit per address; this one
// saves the round trip and tells her why she has to wait.

type Props = {
  // Asks Supabase for the email again. Resolves to the failure, or to null
  // when it was accepted.
  send: () => Promise<AuthFailure | null>;
  // What sits beside the link: "Back to log in".
  children?: ReactNode;
};

export function SendAgain({ send, children }: Props) {
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
      const failure = await send();
      if (failure) {
        setProblem(sendEmailProblem(failure));
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
    <>
      {note && <Notice>{note}</Notice>}
      {problem && <FormError>{problem}</FormError>}
      <div className="flex flex-wrap justify-between gap-x-6">
        <LinkButton disabled={busy || waiting} onClick={() => void sendAgain()}>
          Send it again
        </LinkButton>
        {children}
      </div>
      {waiting && (
        <p className="text-muted text-sm tabular-nums">
          {resendWaitText(wait)}
        </p>
      )}
    </>
  );
}
