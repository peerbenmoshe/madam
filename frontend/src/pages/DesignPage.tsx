import { useCallback, useState } from "react";
import type { ReactNode } from "react";
import { AppBar, NavLink } from "../components/AppBar.tsx";
import { Button, LinkButton } from "../components/Button.tsx";
import { Chips, OptionButtons } from "../components/Choices.tsx";
import { DateBox } from "../components/DateBox.tsx";
import { Question } from "../components/Question.tsx";
import { SegmentedControl } from "../components/SegmentedControl.tsx";
import { Switch } from "../components/Switch.tsx";
import { TemperatureBox } from "../components/TemperatureBox.tsx";
import { TextArea } from "../components/TextArea.tsx";
import { TextField } from "../components/TextField.tsx";
import { TimeBox } from "../components/TimeBox.tsx";
import { Toast } from "../components/Toast.tsx";
import { newPasswordProblem } from "../lib/authText.ts";
import { formatDate, formatDateLong } from "../lib/date.ts";

// /design: every shared component of docs/design-brief.md §5 in one place,
// with made-up values, so the design can be checked on a real phone before
// any screen uses it. It reads and stores nothing. Not linked from the app;
// to be removed in the v1 review pass (docs/build-order.md, Slice 10).

// The class names are written out in full: Tailwind only generates the
// classes it finds as whole words in the source.
const COLOURS = [
  { name: "surface", fill: "bg-surface", use: "App background" },
  { name: "sheet", fill: "bg-sheet", use: "Chart-paper band" },
  { name: "ink", fill: "bg-ink", use: "Text, outlines" },
  { name: "muted", fill: "bg-muted", use: "Secondary text" },
  { name: "berry", fill: "bg-berry", use: "Lines, links" },
  { name: "accent", fill: "bg-accent", use: "Fills only" },
  { name: "blush", fill: "bg-blush", use: "Tracks, questions" },
  { name: "grid", fill: "bg-grid", use: "Fine chart lines" },
  { name: "grid-strong", fill: "bg-grid-strong", use: "Every 0.10 °C" },
  { name: "rule", fill: "bg-rule", use: "Hairlines" },
  { name: "danger", fill: "bg-danger", use: "Destructive, errors" },
];

const VIEWS = [
  { value: "dates", label: "Dates" },
  { value: "cycle", label: "Cycle" },
] as const;

const RANGES = [
  { value: "7", label: "7" },
  { value: "14", label: "14" },
  { value: "30", label: "30" },
  { value: "60", label: "60" },
  { value: "90", label: "90" },
  { value: "custom", label: "Custom" },
] as const;

const UNITS = [
  { value: "celsius", label: "°C" },
  { value: "fahrenheit", label: "°F" },
] as const;

const BLEEDING = [
  { value: "none", label: "None" },
  { value: "spotting", label: "Spotting" },
  { value: "light", label: "Light" },
  { value: "medium", label: "Medium" },
  { value: "heavy", label: "Heavy" },
] as const;

const DISTURBANCES = [
  { value: "alcohol", label: "Alcohol" },
  { value: "illness", label: "Illness" },
  { value: "poor-sleep", label: "Poor sleep" },
  { value: "travel", label: "Travel" },
  { value: "stress", label: "Stress" },
] as const;

const SECRETIONS = [
  { value: "clear_stretchy", label: "Clear, stretchy" },
  { value: "milky", label: "Milky" },
  { value: "sticky", label: "Sticky" },
] as const;

type View = (typeof VIEWS)[number]["value"];
type Range = (typeof RANGES)[number]["value"];
type Unit = (typeof UNITS)[number]["value"];
type Bleeding = (typeof BLEEDING)[number]["value"];
type Disturbance = (typeof DISTURBANCES)[number]["value"];
type Secretion = (typeof SECRETIONS)[number]["value"];

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="border-rule grid gap-4 border-t py-6">
      <h2 className="text-xl font-extrabold">{title}</h2>
      {children}
    </section>
  );
}

export default function DesignPage() {
  const [view, setView] = useState<View>("dates");
  const [range, setRange] = useState<Range>("14");
  const [unit, setUnit] = useState<Unit>("celsius");
  const [bleeding, setBleeding] = useState<Bleeding | null>(null);
  const [disturbances, setDisturbances] = useState<Disturbance[]>(["alcohol"]);
  const [cycleStart, setCycleStart] = useState(false);
  const [showRaw, setShowRaw] = useState(true);
  // Real state behind the two disabled switches, so that only `disabled`
  // stands between a tap and a flip.
  const [lockedOff, setLockedOff] = useState(false);
  const [lockedOn, setLockedOn] = useState(true);
  const [date, setDate] = useState("2026-11-08");
  const [emptyDate, setEmptyDate] = useState("");
  const [time, setTime] = useState("07:00");
  const [emptyTime, setEmptyTime] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("short");
  const [asking, setAsking] = useState(false);
  const [answer, setAnswer] = useState("Nothing saved yet.");
  const [secretion, setSecretion] = useState<Secretion | null>("milky");
  const [temperature, setTemperature] = useState("36.45");
  const [badTemperature, setBadTemperature] = useState("12");
  const [notes, setNotes] = useState("");
  const [toast, setToast] = useState<string | null>(null);
  // The same function at every draw, so the toast's timer isn't restarted
  // by a redraw.
  const hideToast = useCallback(() => setToast(null), []);

  return (
    <main className="mx-auto max-w-md px-4 pb-16">
      <AppBar>
        <NavLink>Cycles</NavLink>
        <NavLink>Settings</NavLink>
      </AppBar>

      <div className="pt-2.5 pb-5">
        <h1 className="text-[1.75rem] leading-[1.15] font-extrabold tracking-[-0.015em]">
          Design components
        </h1>
        <p className="text-muted mt-1">
          Every shared piece, with made-up values. Nothing here is saved.
        </p>
      </div>

      <Section title="Colour">
        <ul className="grid grid-cols-2 gap-3">
          {COLOURS.map((colour) => (
            <li key={colour.name} className="flex items-center gap-2.5">
              <span
                className={`${colour.fill} border-rule size-10 flex-none rounded-xl border`}
              />
              <span className="grid text-sm leading-tight">
                <b>{colour.name}</b>
                <span className="text-muted text-xs">{colour.use}</span>
              </span>
            </li>
          ))}
        </ul>
        <p className="bg-accent text-ink rounded-xl px-3 py-2 font-bold">
          Ink on accent: the only way text sits on HotPink.
        </p>
      </Section>

      <Section title="Type">
        <p className="text-[1.75rem] leading-[1.15] font-extrabold tracking-[-0.015em]">
          {formatDateLong("2026-11-08")}
        </p>
        <p className="text-[2.5rem] leading-none font-extrabold tracking-[-0.03em] tabular-nums">
          36.55 <span className="text-muted text-xl font-medium">°C</span>
        </p>
        <p className="text-xl font-extrabold">Section heading</p>
        <p>
          Body text at 16px, weight 400. <b>Field labels are 700.</b> Digits
          line up: <span className="tabular-nums">36.11 / 37.00</span>.
        </p>
        <p className="text-muted text-sm">
          Secondary text at 14px. ‹ 07/11/2026 › … 45 min late − 0.05 °C
        </p>
        <p className="text-muted text-xs font-medium">
          Axis labels and small notes at 12px, weight 500.
        </p>
      </Section>

      <Section title="Buttons">
        <Button>Add today's entry</Button>
        <Button disabled>Save entry (disabled)</Button>
        <div className="flex flex-wrap items-center gap-3">
          <Button variant="secondary">Close</Button>
          <Button variant="secondary" disabled>
            Log out (disabled)
          </Button>
          <LinkButton>Forgot password?</LinkButton>
        </div>
      </Section>

      <Section title="Segmented control">
        <SegmentedControl
          label="Graph view"
          options={VIEWS}
          value={view}
          onChange={setView}
        />
        <SegmentedControl
          label="Range in days"
          options={RANGES}
          value={range}
          onChange={setRange}
        />
        <SegmentedControl
          label="Temperature unit"
          options={UNITS}
          value={unit}
          onChange={setUnit}
        />
        <p className="text-muted text-sm">
          Chosen: {view}, {range}, {unit}.
        </p>
      </Section>

      <Section title="Option buttons">
        <OptionButtons
          legend="Bleeding"
          options={BLEEDING}
          value={bleeding}
          onChange={setBleeding}
        />
        <p className="text-muted text-sm">
          Chosen: {bleeding ?? "nothing yet"}.
        </p>
        <OptionButtons
          clearable
          legend="Cervical secretion"
          options={SECRETIONS}
          value={secretion}
          onChange={setSecretion}
        />
        <p className="text-muted text-sm">
          Secretion: {secretion ?? "none"}. This question may have no answer:
          tap the chosen one again to clear it. Bleeding, above, can't be
          cleared.
        </p>
      </Section>

      <Section title="Chips">
        <Chips
          legend="Disturbances"
          options={DISTURBANCES}
          values={disturbances}
          onChange={setDisturbances}
        />
        <p className="text-muted text-sm">
          Chosen: {disturbances.length > 0 ? disturbances.join(", ") : "none"}.
        </p>
      </Section>

      <Section title="Switch">
        <Switch
          label="Cycle starts on this day"
          checked={cycleStart}
          onChange={setCycleStart}
        />
        <Switch
          label="Show raw readings"
          checked={showRaw}
          onChange={setShowRaw}
        />
        <Switch
          label="Off, and disabled"
          checked={lockedOff}
          onChange={setLockedOff}
          disabled
        />
        <Switch
          label="On, and disabled"
          checked={lockedOn}
          onChange={setLockedOn}
          disabled
        />
      </Section>

      <Section title="Temperature box">
        <div className="bg-sheet grid gap-3.5 rounded-[20px] p-4 shadow-[inset_0_0_0_1px_var(--color-rule)]">
          <TemperatureBox
            label="Temperature"
            unit="°C"
            placeholder="36.45"
            value={temperature}
            onChange={setTemperature}
            describedBy="design-counts-as"
          />
          <p id="design-counts-as" className="text-sm">
            Measured 45 min late, so it counts as{" "}
            <b className="text-berry">36.30 °C</b>. That's an estimate.
          </p>
        </div>
        <TemperatureBox
          label="With an error"
          unit="°C"
          placeholder="36.45"
          value={badTemperature}
          onChange={setBadTemperature}
          error="That isn't a body temperature. Enter a value between 20.00 and 50.00 °C."
        />
        <p className="text-muted text-sm">
          The first holds "{temperature}", exactly as typed. A phone shows its
          number keyboard, with a dot or a comma.
        </p>
      </Section>

      <Section title="Text area">
        <TextArea
          label="Notes"
          placeholder="Mood, travel, anything else"
          maxLength={2000}
          value={notes}
          onChange={(event) => setNotes(event.target.value)}
        />
        <p className="text-muted text-sm">
          {notes.length} characters, {notes.split("\n").length} line
          {notes.split("\n").length === 1 ? "" : "s"}.
        </p>
        <TextArea
          label="With an error"
          defaultValue="Too long, say."
          error="Keep the notes under 2,000 characters."
        />
      </Section>

      <Section title="Toast">
        <div className="flex flex-wrap gap-3">
          <Button variant="secondary" onClick={() => setToast("Entry saved")}>
            Show "Entry saved"
          </Button>
          <Button variant="secondary" onClick={() => setToast("Entry deleted")}>
            Show "Entry deleted"
          </Button>
        </div>
        <p className="text-muted text-sm">
          It rises near the bottom of the screen and leaves after 2.2 seconds.
          Now: {toast ?? "no toast"}.
        </p>
        {toast && <Toast message={toast} onDone={hideToast} />}
      </Section>

      <Section title="Date box">
        <DateBox
          label="Date"
          value={date}
          onChange={setDate}
          min="2026-01-01"
          max="2026-11-12"
        />
        <DateBox label="From" value={emptyDate} onChange={setEmptyDate} />
        <p className="text-muted text-sm">
          The first holds {date === "" ? "no date" : formatDate(date)}, and
          allows 01/01/2026 to 12/11/2026 only. A phone's picker offers later
          dates anyway: picking one is refused, with the reason.
        </p>
      </Section>

      <Section title="Time box">
        <TimeBox label="Measurement time" value={time} onChange={setTime} />
        <TimeBox
          label="Measured at"
          value={emptyTime}
          onChange={setEmptyTime}
          error={emptyTime === "" ? "Pick a time." : undefined}
        />
        <p className="text-muted text-sm">
          The first holds {time === "" ? "no time" : time}. The box always reads
          24-hour. The picker that opens is the phone's own, and may offer AM
          and PM.
        </p>
      </Section>

      <Section title="Text field">
        <TextField
          label="Email"
          type="email"
          autoComplete="off"
          placeholder="you@example.com"
          value={email}
          onChange={(event) => setEmail(event.target.value)}
        />
        <TextField
          label="Invite code"
          autoComplete="off"
          hint="The code you were given with your invitation."
        />
        <TextField
          label="Password"
          type="password"
          autoComplete="off"
          hint="Type 8 characters with an English letter and a digit, and the error goes away."
          value={password}
          onChange={(event) => setPassword(event.target.value)}
          error={newPasswordProblem(password) ?? undefined}
        />
      </Section>

      <Section title="A question in place of Save">
        <p className="text-muted text-sm">{answer}</p>
        {asking ? (
          <Question
            question="Save without a temperature?"
            note="The day will show as a gap in the line."
            no="Add a temperature"
            yes="Save without"
            onNo={() => {
              setAsking(false);
              setAnswer("Answered: add a temperature.");
            }}
            onYes={() => {
              setAsking(false);
              setAnswer("Answered: save without.");
            }}
          />
        ) : (
          <Button onClick={() => setAsking(true)}>Save entry</Button>
        )}
      </Section>
    </main>
  );
}
