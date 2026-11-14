import { useState } from "react";
import { Button, LinkButton } from "../components/Button.tsx";
import { OptionButtons } from "../components/Choices.tsx";
import { DateBox } from "../components/DateBox.tsx";
import { FormError, Notice, Screen } from "../components/Screen.tsx";
import type { Choice } from "../components/SegmentedControl.tsx";
import { TimeBox } from "../components/TimeBox.tsx";
import { ApiError, fetchMe, submitOnboarding } from "../lib/api.ts";
import type { Me } from "../lib/api.ts";
import { TRY_AGAIN } from "../lib/authText.ts";
import { todayISO } from "../lib/date.ts";
import {
  START_DATE_LABEL,
  START_HINT,
  START_KNOWN,
  START_QUESTION,
  START_UNKNOWN,
  START_UNKNOWN_NOTE,
  SUB,
  SUBMIT,
  TIME_HINT,
  TIME_LABEL,
  TIME_QUESTION,
  TITLE,
  onboardingBody,
  onboardingFailure,
  onboardingProblems,
} from "../lib/onboardingText.ts";
import type { OnboardingForm } from "../lib/onboardingText.ts";
import { useLogOut } from "../lib/useLogOut.ts";

// What a signed-in person sees until she has answered it, once
// (docs/spec-v1.md §4; sketched in docs/design-brief.md §10): when her
// current cycle started, or that she doesn't know, and the time she
// usually takes her temperature.
//
// Every word and every rule is in lib/onboardingText.ts. This file only
// lays them out and sends the answer.

const START_ANSWERS: readonly Choice<"known" | "unknown">[] = [
  { value: "known", label: START_KNOWN },
  { value: "unknown", label: START_UNKNOWN },
];

type Props = {
  // Called with her account once it is onboarded.
  onDone: (me: Me) => void;
};

export function OnboardingPage({ onDone }: Props) {
  // Nothing is chosen for her: no answer, no date, no time.
  const [form, setForm] = useState<OnboardingForm>({
    start: null,
    date: "",
    time: "",
  });
  // What is missing is said only once she has tapped the button, and then
  // follows the form as she fills it in.
  const [tried, setTried] = useState(false);
  const [busy, setBusy] = useState(false);
  // What the server said to a submit, until she changes something.
  const [failure, setFailure] = useState<{
    message: string;
    field?: "date" | "time";
  } | null>(null);
  const logOut = useLogOut();

  // Today on her device, read at every draw: the latest date she can give.
  const today = todayISO();
  const problems = tried ? onboardingProblems(form, today) : {};

  function change(answer: Partial<OnboardingForm>) {
    setForm((current) => ({ ...current, ...answer }));
    setFailure(null);
  }

  async function submit() {
    // Checked again now, with today as it is at this moment.
    if (Object.keys(onboardingProblems(form, todayISO())).length > 0) {
      setTried(true);
      return;
    }
    setBusy(true);
    setFailure(null);
    try {
      onDone(await submitOnboarding(onboardingBody(form)));
    } catch (error) {
      const result = onboardingFailure(
        error instanceof ApiError ? error : { status: -1 },
      );
      if (!result.goOn) {
        setFailure(result);
        return;
      }
      // Onboarded already, from another device or a second tap. Ask what
      // the account looks like now, and go on.
      try {
        const me = await fetchMe();
        if (me.onboarded_at === null) {
          setFailure({ message: TRY_AGAIN });
        } else {
          onDone(me);
        }
      } catch {
        setFailure({ message: TRY_AGAIN });
      }
    } finally {
      setBusy(false);
    }
  }

  return (
    <Screen title={TITLE} sub={SUB}>
      <form
        noValidate
        className="grid gap-8"
        onSubmit={(event) => {
          event.preventDefault();
          void submit();
        }}
      >
        <div className="grid gap-3">
          <OptionButtons
            legend={START_QUESTION}
            hint={START_HINT}
            options={START_ANSWERS}
            value={form.start}
            onChange={(start) => change({ start })}
          />
          {problems.start && <FormError>{problems.start}</FormError>}
          {form.start === "known" && (
            <DateBox
              label={START_DATE_LABEL}
              value={form.date}
              onChange={(date) => change({ date })}
              max={today}
              error={
                problems.date ??
                (failure?.field === "date" ? failure.message : undefined)
              }
            />
          )}
          {form.start === "unknown" && <Notice>{START_UNKNOWN_NOTE}</Notice>}
        </div>

        <fieldset className="min-w-0">
          <legend className="mb-1 font-bold">{TIME_QUESTION}</legend>
          <p className="text-muted mb-2.5 text-sm">{TIME_HINT}</p>
          <TimeBox
            label={TIME_LABEL}
            value={form.time}
            onChange={(time) => change({ time })}
            error={
              problems.time ??
              (failure?.field === "time" ? failure.message : undefined)
            }
          />
        </fieldset>

        <div className="grid gap-3">
          {failure && failure.field === undefined && (
            <FormError>{failure.message}</FormError>
          )}
          <Button type="submit" disabled={busy}>
            {SUBMIT}
          </Button>
        </div>
      </form>

      <div className="mt-6 grid justify-items-start gap-1">
        {logOut.problem && <FormError>{logOut.problem}</FormError>}
        <LinkButton disabled={logOut.busy} onClick={logOut.logOut}>
          Log out
        </LinkButton>
      </div>
    </Screen>
  );
}
