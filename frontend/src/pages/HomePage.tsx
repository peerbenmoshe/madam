import type { Session } from "@supabase/auth-js";
import { Button } from "../components/Button.tsx";
import { FormError, Notice, Screen } from "../components/Screen.tsx";
import { todayISO } from "../lib/date.ts";
import { OPEN_TODAY } from "../lib/dayText.ts";
import { navigate } from "../lib/router.ts";
import { dayPath } from "../lib/routes.ts";
import { useLogOut } from "../lib/useLogOut.ts";

// What a signed-in, onboarded person sees until the chart exists
// (Slice 6): proof that the account works end to end, the way to today's
// entry, and a way out.
//
// It is also where a signed-in person lands when her account couldn't be
// read at all (App.tsx): problem then says why, and onRetry, when given,
// tries again.

type Props = {
  session: Session;
  problem?: string;
  onRetry?: () => void;
};

export function HomePage({ session, problem, onRetry }: Props) {
  const logOut = useLogOut();
  return (
    <Screen
      title="You're logged in"
      sub={<span className="wrap-anywhere">{session.user.email}</span>}
    >
      <div className="grid gap-6">
        {problem ? (
          <FormError>{problem}</FormError>
        ) : (
          <>
            <Notice>Your account is ready. The chart comes next.</Notice>
            {/* Today is worked out when she taps, never kept from earlier. */}
            <Button onClick={() => navigate(dayPath(todayISO()))}>
              {OPEN_TODAY}
            </Button>
          </>
        )}
        {logOut.problem && <FormError>{logOut.problem}</FormError>}
        <div className="flex flex-wrap gap-3">
          {onRetry && (
            <Button variant="secondary" onClick={onRetry}>
              Try again
            </Button>
          )}
          <Button
            variant="secondary"
            disabled={logOut.busy}
            onClick={logOut.logOut}
          >
            Log out
          </Button>
        </div>
      </div>
    </Screen>
  );
}
