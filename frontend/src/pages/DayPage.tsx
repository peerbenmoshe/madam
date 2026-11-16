import { useEffect, useId, useRef, useState } from "react";
import { AppBar } from "../components/AppBar.tsx";
import { Button, DangerButton } from "../components/Button.tsx";
import { Chips, OptionButtons } from "../components/Choices.tsx";
import { DateBox } from "../components/DateBox.tsx";
import { Disclosure } from "../components/Disclosure.tsx";
import { Question } from "../components/Question.tsx";
import { FormError, Notice } from "../components/Screen.tsx";
import { Switch } from "../components/Switch.tsx";
import { TemperatureBox } from "../components/TemperatureBox.tsx";
import { TextArea } from "../components/TextArea.tsx";
import { TextField } from "../components/TextField.tsx";
import { TimeBox } from "../components/TimeBox.tsx";
import { ApiError, deleteEntry, fetchDay, saveDay } from "../lib/api.ts";
import { TRY_AGAIN } from "../lib/authText.ts";
import {
  AMOUNT_LABEL,
  AMOUNT_OPTIONS,
  BACK,
  BLEEDING_LABEL,
  BLEEDING_OPTIONS,
  CELSIUS,
  CERVIX_LABEL,
  CYCLE_START_LABEL,
  CYCLE_START_NOT_YET,
  DATE_LABEL,
  DELETE,
  DELETED,
  DESCRIPTION_EXAMPLE,
  DESCRIPTION_LABEL,
  DESCRIPTION_MAX,
  DISTURBANCES_HINT,
  DISTURBANCES_LABEL,
  DISTURBANCE_OPTIONS,
  FIRMNESS_LABEL,
  FIRMNESS_OPTIONS,
  GO_TO_TODAY,
  HIDE_EXTRA,
  INTERCOURSE_LABEL,
  INTERCOURSE_OPTIONS,
  MEASURED_AT_LABEL,
  NOTES_EXAMPLE,
  NOTES_LABEL,
  NOTES_MAX,
  NO_ENTRY_YET,
  POSITION_LABEL,
  POSITION_OPTIONS,
  SAVE,
  SAVED,
  SECRETION_LABEL,
  SECRETION_OPTIONS,
  SENSATION_LABEL,
  SENSATION_OPTIONS,
  SHOW_ALL,
  SYMPTOMS_LABEL,
  SYMPTOM_OPTIONS,
  TEMPERATURE_EXAMPLE,
  TEMPERATURE_LABEL,
  amountFrom,
  amountText,
  dateProblem,
  dayBody,
  dayFailure,
  dayFormFrom,
  dayProblems,
  dayTitle,
  deleteQuestion,
  hasExtraValues,
  liveLine,
  otherDayNotice,
  questionFor,
} from "../lib/dayText.ts";
import type {
  Ask,
  Day,
  DayFailure,
  DayForm,
  QuestionName,
} from "../lib/dayText.ts";
import { navigate } from "../lib/router.ts";
import { HOME, dayPath } from "../lib/routes.ts";
import { useToday } from "../lib/useToday.ts";

// One day of her chart (docs/design-brief.md §6.3): the date, the
// temperature and its time, the cycle-start switch (shown, not usable
// until Slice 5), bleeding, secretion, disturbances, notes, "Show all
// fields", Delete on a day that has an entry, and a Save that stays at the
// bottom of the screen.
//
// Every word and every rule is in lib/dayText.ts. This file lays them out,
// holds what she has typed, and talks to the server.
//
// The date box is how she moves to another day: a pick replaces the
// address, and this screen loads that day in place. It is not rebuilt for
// the new day, so the date box keeps its focus while a date is typed on a
// computer (docs/tech-stack.md §8).

type Props = {
  // "YYYY-MM-DD", from the address. App.tsx has checked it.
  date: string;
  // Called after a save or a delete, with what to tell her. The owner shows the toast
  // and the screen she goes back to.
  onDone: (message: string) => void;
  // The server says the account hasn't onboarded: the owner reads the
  // account again, which shows onboarding.
  onNotOnboarded: () => void;
};

// The day as read from the server.
type Load =
  | { state: "loading" }
  | { state: "failed"; message: string }
  | { state: "ready"; day: Day };

// What the server said to the last save, until she changes something. A
// question is shown in Save's place; anything else is a message.
type Refusal = Extract<DayFailure, { kind: "message" }>;

export function DayPage({ date, onDone, onNotOnboarded }: Props) {
  const today = useToday();

  const [load, setLoad] = useState<Load>({ state: "loading" });
  const [form, setForm] = useState<DayForm | null>(null);
  // What is wrong is said only once she has tapped Save, and then follows
  // the form as she changes it.
  const [tried, setTried] = useState(false);
  const [busy, setBusy] = useState(false);
  const [refusal, setRefusal] = useState<Refusal | null>(null);
  // The question in Save's place, and the ones already answered "yes" for
  // this form. Any change to the form forgets both.
  const [asking, setAsking] = useState<Ask | null>(null);
  const [confirmed, setConfirmed] = useState<QuestionName[]>([]);
  // The delete question is in Save's place.
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  // "Show all fields" is open. Set when the day loads, then hers to change.
  const [moreOpen, setMoreOpen] = useState(false);
  // Asks the server for the day again ("Try again").
  const [attempt, setAttempt] = useState(0);

  // The date box's own value. It runs ahead of the address while a year is
  // being typed on a computer and passes through dates that can't open.
  const [boxDate, setBoxDate] = useState(date);
  // The device's date when this day was opened: if that was today and the
  // date has moved on since, the screen says so.
  const [openedOn, setOpenedOn] = useState(today);

  // A new date from the address: everything about the old day goes. This
  // is React's way of resetting state when a prop changes, during the draw
  // rather than in an effect, so the old day is never drawn for the new one.
  const [shownDate, setShownDate] = useState(date);
  if (shownDate !== date) {
    setShownDate(date);
    setBoxDate(date);
    setOpenedOn(today);
    setLoad({ state: "loading" });
    setForm(null);
    setTried(false);
    setRefusal(null);
    setAsking(null);
    setConfirmed([]);
    setConfirmingDelete(false);
    setMoreOpen(false);
  }

  useEffect(() => {
    // Ignore an answer that arrives after another day was asked for.
    let current = true;
    fetchDay(date).then(
      (day) => {
        if (!current) return;
        try {
          const loaded = dayFormFrom(day);
          setForm(loaded);
          setMoreOpen(hasExtraValues(loaded));
          setLoad({ state: "ready", day });
        } catch {
          setLoad({ state: "failed", message: TRY_AGAIN });
        }
      },
      (error: unknown) => {
        if (!current) return;
        const failure = dayFailure(
          error instanceof ApiError ? error : { status: -1 },
        );
        if (failure.kind === "notOnboarded") {
          onNotOnboarded();
          return;
        }
        setLoad({
          state: "failed",
          message: failure.kind === "message" ? failure.message : TRY_AGAIN,
        });
      },
    );
    return () => {
      current = false;
    };
  }, [date, attempt, onNotOnboarded]);

  const temperatureRef = useRef<HTMLInputElement>(null);
  const deleteRef = useRef<HTMLButtonElement>(null);
  const lineId = useId();

  function change(answer: Partial<DayForm>) {
    setForm((current) =>
      current === null ? current : { ...current, ...answer },
    );
    setRefusal(null);
    setAsking(null);
    setConfirmed([]);
    setConfirmingDelete(false);
  }

  function pickDate(picked: string) {
    // Clearing the box opens nothing: the day stays, and so does its date.
    if (picked === "") return;
    setBoxDate(picked);
    if (picked !== date && dateProblem(picked, today) === null) {
      navigate(dayPath(picked), { replace: true });
    }
  }

  async function save(answered: QuestionName[]) {
    if (form === null) return;
    // Checked again now, with the form as it is at this moment.
    if (Object.keys(dayProblems(form)).length > 0) {
      setTried(true);
      return;
    }
    setBusy(true);
    setRefusal(null);
    setAsking(null);
    setConfirmingDelete(false);
    try {
      await saveDay(date, dayBody(form, answered));
      onDone(SAVED);
    } catch (error) {
      const failure = dayFailure(
        error instanceof ApiError ? error : { status: -1 },
      );
      switch (failure.kind) {
        case "ask":
          // The server wants her answer first: the question takes Save's
          // place, and "yes" sends the same entry again with it.
          setConfirmed(answered);
          setAsking(questionFor(failure.name, form));
          break;
        case "notOnboarded":
          onNotOnboarded();
          break;
        case "message":
          // A refusal about a field under "Show all fields" must be seen.
          if (failure.field === "secretionDescription") setMoreOpen(true);
          setRefusal(failure);
          break;
      }
    } finally {
      setBusy(false);
    }
  }

  // "Delete entry" asks first, in Save's place, and deletes only on "yes".
  function askDelete() {
    setRefusal(null);
    setAsking(null);
    setConfirmed([]);
    setConfirmingDelete(true);
  }

  function keepIt() {
    setConfirmingDelete(false);
    // Back where she was.
    deleteRef.current?.focus();
  }

  async function remove() {
    setConfirmingDelete(false);
    setBusy(true);
    setRefusal(null);
    try {
      await deleteEntry(date);
      onDone(DELETED);
    } catch (error) {
      const failure = dayFailure(
        error instanceof ApiError ? error : { status: -1 },
      );
      switch (failure.kind) {
        case "notOnboarded":
          onNotOnboarded();
          break;
        case "message":
          setRefusal(failure);
          break;
        case "ask":
          // A delete asks nothing: this would be a bug of ours.
          setRefusal({ kind: "message", message: TRY_AGAIN });
          break;
      }
    } finally {
      setBusy(false);
    }
  }

  function answerNo() {
    setAsking(null);
    setConfirmed([]);
    // Both questions are about the temperature: back to it, with what she
    // typed selected so a new number replaces it.
    temperatureRef.current?.focus();
    temperatureRef.current?.select();
  }

  const day = load.state === "ready" ? load.day : null;
  const problems = form !== null && tried ? dayProblems(form) : {};
  // The server's refusal, if it is about this field.
  const fieldRefusal = (field: NonNullable<Refusal["field"]>) =>
    refusal !== null && refusal.field === field ? refusal.message : undefined;
  const dateError =
    boxDate !== date
      ? (dateProblem(boxDate, today) ?? undefined)
      : fieldRefusal("date");
  const line =
    form !== null && day !== null ? liveLine(form, day.fixed_time) : null;

  return (
    <main className="mx-auto max-w-md px-4">
      <AppBar />
      <button
        type="button"
        onClick={() => navigate(HOME, { replace: true })}
        className="text-berry -ml-1 flex min-h-11 cursor-pointer items-center gap-1 font-bold"
      >
        <svg viewBox="0 0 16 16" aria-hidden="true" className="size-4">
          <path
            d="M10 3 5 8l5 5"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
        {BACK}
      </button>

      <div className="pt-1 pb-5">
        <h1 className="text-[1.75rem] leading-[1.15] font-extrabold tracking-[-0.015em] text-balance">
          {dayTitle(date, today)}
        </h1>
        {load.state === "loading" && (
          <p className="text-muted mt-1">Loading…</p>
        )}
        {day !== null && day.entry === null && (
          <p className="text-muted mt-1">{NO_ENTRY_YET}</p>
        )}
      </div>

      <div className="grid gap-[26px]">
        {openedOn === date && today !== date && (
          <div className="grid gap-3">
            <Notice>{otherDayNotice(date, today)}</Notice>
            <Button
              variant="secondary"
              onClick={() => navigate(dayPath(today), { replace: true })}
            >
              {GO_TO_TODAY}
            </Button>
          </div>
        )}

        <DateBox
          label={DATE_LABEL}
          value={boxDate}
          onChange={pickDate}
          max={today}
          error={dateError}
        />

        {load.state === "failed" && (
          <div className="grid gap-3">
            <FormError>{load.message}</FormError>
            <Button
              variant="secondary"
              onClick={() => {
                setLoad({ state: "loading" });
                setAttempt((n) => n + 1);
              }}
            >
              Try again
            </Button>
          </div>
        )}

        {form !== null && day !== null && (
          <>
            <div className="bg-sheet grid gap-3.5 rounded-[20px] p-4 shadow-[inset_0_0_0_1px_var(--color-rule)]">
              <TemperatureBox
                ref={temperatureRef}
                label={TEMPERATURE_LABEL}
                unit={CELSIUS}
                placeholder={TEMPERATURE_EXAMPLE}
                value={form.temperature}
                onChange={(temperature) => change({ temperature })}
                error={problems.temperature ?? fieldRefusal("temperature")}
                describedBy={lineId}
              />
              <TimeBox
                label={MEASURED_AT_LABEL}
                value={form.measuredAt}
                onChange={(measuredAt) => change({ measuredAt })}
                error={problems.measuredAt ?? fieldRefusal("measuredAt")}
              />
              {line && (
                <p
                  id={lineId}
                  aria-live="polite"
                  className="min-h-[1.5em] text-sm"
                >
                  {line.before}
                  {line.value && <b className="text-berry">{line.value}</b>}
                  {line.after}
                </p>
              )}
            </div>

            <div className="grid gap-1">
              <Switch
                label={CYCLE_START_LABEL}
                checked={false}
                onChange={() => {}}
                disabled
              />
              <p className="text-muted text-sm">{CYCLE_START_NOT_YET}</p>
            </div>

            <OptionButtons
              legend={BLEEDING_LABEL}
              options={BLEEDING_OPTIONS}
              value={form.bleeding}
              onChange={(bleeding) => change({ bleeding })}
            />

            <OptionButtons
              clearable
              legend={SECRETION_LABEL}
              options={SECRETION_OPTIONS}
              value={form.secretionCategory}
              onChange={(secretionCategory) => change({ secretionCategory })}
            />

            <Chips
              legend={DISTURBANCES_LABEL}
              hint={DISTURBANCES_HINT}
              options={DISTURBANCE_OPTIONS}
              values={form.disturbances}
              onChange={(disturbances) => change({ disturbances })}
            />

            <TextArea
              label={NOTES_LABEL}
              placeholder={NOTES_EXAMPLE}
              maxLength={NOTES_MAX}
              value={form.notes}
              onChange={(event) => change({ notes: event.target.value })}
              error={fieldRefusal("notes")}
            />

            <Disclosure
              closedLabel={SHOW_ALL}
              openLabel={HIDE_EXTRA}
              open={moreOpen}
              onToggle={setMoreOpen}
            >
              <OptionButtons
                clearable
                legend={SENSATION_LABEL}
                options={SENSATION_OPTIONS}
                value={form.sensation}
                onChange={(sensation) => change({ sensation })}
              />

              <div className="grid gap-4">
                <OptionButtons
                  clearable
                  legend={AMOUNT_LABEL}
                  options={AMOUNT_OPTIONS}
                  value={amountText(form.secretionAmount)}
                  onChange={(amount) =>
                    change({ secretionAmount: amountFrom(amount) })
                  }
                />
                <TextField
                  label={DESCRIPTION_LABEL}
                  placeholder={DESCRIPTION_EXAMPLE}
                  maxLength={DESCRIPTION_MAX}
                  value={form.secretionDescription}
                  onChange={(event) =>
                    change({ secretionDescription: event.target.value })
                  }
                  error={fieldRefusal("secretionDescription")}
                />
              </div>

              {/* One heading over two rows of answers, as in the mock. */}
              <fieldset className="min-w-0">
                <legend className="mb-2.5 font-bold">{CERVIX_LABEL}</legend>
                <div className="grid gap-2.5">
                  <OptionButtons
                    clearable
                    legendHidden
                    legend={FIRMNESS_LABEL}
                    options={FIRMNESS_OPTIONS}
                    value={form.cervixFirmness}
                    onChange={(cervixFirmness) => change({ cervixFirmness })}
                  />
                  <OptionButtons
                    clearable
                    legendHidden
                    legend={POSITION_LABEL}
                    options={POSITION_OPTIONS}
                    value={form.cervixPosition}
                    onChange={(cervixPosition) => change({ cervixPosition })}
                  />
                </div>
              </fieldset>

              <Chips
                legend={SYMPTOMS_LABEL}
                options={SYMPTOM_OPTIONS}
                values={form.symptoms}
                onChange={(symptoms) => change({ symptoms })}
              />

              <OptionButtons
                clearable
                legend={INTERCOURSE_LABEL}
                options={INTERCOURSE_OPTIONS}
                value={form.intercourse}
                onChange={(intercourse) => change({ intercourse })}
              />
            </Disclosure>

            {day.entry !== null && (
              <DangerButton ref={deleteRef} disabled={busy} onClick={askDelete}>
                {DELETE}
              </DangerButton>
            )}
          </>
        )}
      </div>

      {/* Save stays at the bottom of the screen while the form scrolls, so
          it is never more than a thumb away. A question takes its place. */}
      <div className="bg-surface border-rule sticky bottom-0 z-10 mt-6 grid gap-2.5 border-t pt-3 pb-[calc(14px+env(safe-area-inset-bottom,0px))]">
        {refusal !== null && refusal.field === undefined && (
          <FormError>{refusal.message}</FormError>
        )}
        {confirmingDelete ? (
          <Question
            {...deleteQuestion(date)}
            onYes={() => void remove()}
            onNo={keepIt}
          />
        ) : asking !== null ? (
          <Question
            question={asking.question}
            yes={asking.yes}
            no={asking.no}
            onYes={() => void save([...confirmed, asking.name])}
            onNo={answerNo}
          />
        ) : (
          <Button
            disabled={busy || form === null}
            onClick={() => void save(confirmed)}
          >
            {SAVE}
          </Button>
        )}
      </div>
    </main>
  );
}
