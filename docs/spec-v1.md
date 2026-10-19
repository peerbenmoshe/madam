# Madam — v1 Spec

_Status: approved by Pe'er. Amended at step 3 (data model): the rounding mode in
§6 and AC-1 is now decided, §5.2's intercourse field is defined, and §7 gains a
rule for dates before sign-up. See `data-model.md`._

_Amended at step 4 (rules and setup):_
- _§5.3 gains a hard rejection outside 20.00–50.00 °C, distinct from the 35–39 °C question._
- _§8.3 and AC-7 now look at the **most recent earlier entry**, not the previous calendar day. See `docs/fam-rules.md` §6.3 for the failure this closes._
- _The "shortest − 21 (20)" value is **dropped permanently**, not deferred._
- _§15 is trimmed: the method-fidelity research is resolved, the legal check remains._
- _`docs/fam-rules.md` is now the source of truth for the exact domain arithmetic and conditions. This spec says what the app does; that file says precisely how each rule computes, with test vectors._

## 1. Purpose

A mobile-friendly web app that replaces a paper FAM/BBT chart. v1 covers recording daily data, charting temperatures, and showing cycle-length statistics. It gives **no fertility verdicts** (interpretation level B).

The rules implemented are the ones from the paper chart the first user already fills in by hand. They are not a clinical standard, and `docs/fam-rules.md` §1 says so explicitly.

## 2. Users and accounts

- The app is built public-ready (any person can have an account, with strict per-user data isolation), but **sign-up requires an invite code at launch**.
- There is **one shared invite code**. It lives in server configuration so it can be changed if it leaks.
- Auth uses an established library or provider; no hand-rolled password handling.
- Sign-up includes email verification, password reset, and log out.
- **Account deletion permanently wipes all of the user's data.**

## 3. Screens

1. **Sign up / log in:** includes the invite code field at sign-up and a password reset flow.
2. **Onboarding:** current cycle start date (or "I don't know") and fixed measurement time.
3. **Home (graph):** the chart, view toggle, range controls, ghost-dot toggle, legend, and an "add entry" button.
4. **Day popup:** an overlay opened by tapping a dot on the graph.
5. **Day entry:** one screen used for both new and existing entries, with a quick section and "show all fields."
6. **Cycles and stats:** opened only on request. Shows the current cycle day, four stats, a bar chart, and the cycle list.
7. **Settings:** units, measurement time (with change history), and account.

## 4. Onboarding

After sign-up, the user enters two things.

- **Start date of her current cycle** (today or earlier), **or "I don't know."**
  - If she picks "I don't know," the current cycle is created as an **unknown-start cycle** (see section 8.4).
- **Fixed measurement time** (required).

Units default to Celsius and can be changed later in Settings. There are no follow-up prompts in v1 (see Future ideas).

## 5. Daily entry

### 5.1 Quick section (always visible)

| Field | Notes |
|---|---|
| Date | Defaults to today. Editable. No future dates. |
| Measurement time | Defaults to the fixed measurement time in effect for that date. Editable, minute precision. Only relevant when a temperature is entered. |
| Temperature | Optional. Entered to 2 decimals in the user's display unit. Saving an entry without a temperature asks "Save without a temperature?" |
| Bleeding / spotting | None, spotting, or bleeding. |
| Cervical secretion category | Clear/stretchy, milky, or sticky. |
| Disturbance tags | Alcohol, poor sleep, illness, medication, travel, vacation, new thermometer, UTI, yeast infection. |
| Notes | Free text (mood, travel, etc.). |

Late/early measurement is **not** a manual tag. It is derived only from the measurement time (see section 6).

### 5.2 "Show all fields" (expanded)

- **Sensation:** dry, creamy, or wet.
- **Cervical secretion details:** amount (1–3) and a free-text description.
- **Cervix:** firm or soft; high/open or low/closed.
- **Symptoms:** breast tenderness, bloating, PMS.
- **Intercourse:** none, protected, or unprotected. One field, so "no intercourse
  but contraception used" cannot be recorded. The method itself is not a field;
  the notes can hold it.

### 5.3 Rules

- **One entry per date.** Picking a date that already has an entry opens that entry for editing.
- Any past date can be backfilled or edited.
- Deleting an entry asks for confirmation.
- **Unusual temperature.** A temperature outside 35.00–39.00 °C (95.00–102.20 °F) is not rejected. Instead the app asks "This temperature is unusual, is it correct?"
  - **Yes** saves the entry.
  - **No** returns the user to the temperature field.
  - The bounds are an assumption and can be tuned.
- **Impossible temperature.** A temperature outside 20.00–50.00 °C is **rejected** with a clear message, not questioned. This is a wider net catching a different animal: 35–39 catches a human typo and asks, 20–50 catches a unit bug in our own code and refuses. The database carries a matching `CHECK` as a backstop, but the user should never meet it — the readable message comes from the API (`data-model.md` §4.2, `docs/fam-rules.md` §9).

## 6. Time adjustment

- Adjustment applies only to entries with a temperature. Every such entry taken at a time other than the fixed measurement time in effect for that date is adjusted.
- **Formula:** `adjusted = raw + 0.05 °C × (fixed_time − measured_time in minutes) / 30`
  - Early measurement adds; late measurement subtracts.
  - Example: fixed 07:00, measured 07:45, raw 36.62 → 36.62 − 0.075 = 36.545.
- The rate is **fixed in v1** (a user setting is a future idea). It is the rate the first user already uses on paper; it is not traced to a published source, and `docs/fam-rules.md` §1 states that openly.
- Calculations run in the canonical unit (Celsius); Fahrenheit is a display and input conversion only.
- Displayed values are precise to 0.01, rounded **half-up**, so 36.545 displays as **36.55**. The raw value is stored in thousandths of a degree and the adjusted value is rounded once, straight to the displayed precision (`data-model.md` §6.1).
- **Late/early is derived, never stored as a tag.** If the rounded adjusted value differs from the raw value:
  - the reading counts as adjusted and has a ghost dot (shown only when ghosts are toggled on);
  - the day popup shows a late/early indicator and marks the adjusted value as an estimate.
- If rounding makes adjusted equal raw (e.g. 1 minute late), no ghost and no indicator appear.
- The late/early indicator is **informational**: the adjusted dot stays solid and on the line.

## 7. Measurement time changes

- Settings holds a history of measurement times, each valid for a date range.
- When changing the time, the user also sets the **last date the previous time applied**. The new time applies from the following day.
- Guardrails:
  - The "last date" can't be in the future.
  - It can't be earlier than the start of the current period (no overlapping ranges).
- Changing the time never alters adjusted values for dates covered by earlier periods.
- **Dates before sign-up.** The time set during onboarding applies to every date before the first change, including days backfilled from before the account existed. There is never a date without a measurement time in effect.

## 8. Cycles

### 8.1 Basics

- A cycle starts on its day 1. The cycle day of any entry is **computed from the cycle start dates, not stored** on the entry.
- Cycles have no maximum length; day 41 and beyond are normal.
- **Numbering starts at 1.** The first cycle with a known start date is cycle #1.
- Moving or removing a cycle start recomputes all affected cycle days, lengths, and numbers automatically.

### 8.2 Starting a new cycle

- **Suggested start.** When the user logs bleeding (not spotting), the app asks "Start a new cycle on this date?" and she confirms or dismisses. The suggestion appears only when both of these are true:
  - the date isn't already a cycle start;
  - the most recent earlier entry, whatever its date, isn't a bleeding entry, so a single missing day in the middle of a period doesn't trigger a false suggestion.
- **Manual start.** The day entry screen has a "cycle starts on this day" toggle, usable on any past date, for fixes and backfilling. It overrides the conditions above entirely.

### 8.3 "Did bleeding stop?" check

- When the user saves an entry with none or spotting, and **the most recent earlier entry, whatever its date,** is bleeding, the app asks "Did bleeding stop?"
  - **Yes** saves the entry.
  - **No** returns the user to the bleeding field.
- Purpose: stop a forgotten bleeding value from later triggering a false new-cycle suggestion.
- **This check uses the same lookback as 8.2 deliberately.** A narrower "previous calendar day" version leaves a hole: a missing day followed by a temperature-only entry saves bleeding as *none*, which 8.2 then reads as the end of the period and offers a new cycle mid-period. The net has to reach as far as the rule it protects. Worked example in `docs/fam-rules.md` §6.3.
- Accepted cost: returning after a long break may prompt this question about a bleeding entry from weeks ago. One extra tap, chosen over an arbitrary cut-off.

### 8.4 Unknown-start cycle and entries outside cycles

- If onboarding started with "I don't know," the **unknown-start cycle** covers everything from the first entry up to the day before the first known cycle start.
  - It has no number and shows as "Cycle ? (start unknown)."
  - Its cycle days are shown as "?".
  - It is excluded from all stats.
  - If the user later marks a start date inside it (manual toggle), it becomes a normal cycle and numbering updates.
- If onboarding had a known date, entries dated before the first known cycle start belong to no cycle. Their cycle day is shown **blank**, not "?" — blank means "outside any cycle," "?" means "inside a cycle whose start is unknown."

## 9. Home graph

### 9.1 Views

The user toggles between two views. **In both views, the X axis shows the date and the cycle day** (cycle day is "?" in an unknown-start cycle and blank outside any cycle).

- **Date view (default):**
  - opens on the last 14 days;
  - presets: 7, 14, 30, 60, 90 days;
  - custom from–to range capped at 90 days, and a start before the first entry is clamped to the first entry;
  - no future dates, and the date pickers themselves block invalid choices (end before start, more than 90 days, future dates);
  - cycle starts are drawn as vertical lines.
- **Cycle view:**
  - shows one cycle;
  - opens on the current cycle, with previous/next navigation to other cycles.

### 9.2 Dots and lines

- **Solid dot:** the reading (the adjusted value if adjusted).
- **Small faded dot (ghost):** the raw value. It appears only when it differs from the adjusted value **and** the "show raw readings" toggle is on. The toggle is **hidden by default**.
- **Hollow ring:** a reading with any disturbance tag. The line skips it and connects its neighbors.
- **No reading that day:** a gap in the line.

Note that the last two are different holes: a disturbed day is stepped over, a missing day stops the line. A charting library controls that with a single setting per series, so the line is drawn as separate connected segments (`tech-stack.md` §8.3).

### 9.3 Legend and interaction

- A small legend explains the dot types and the cycle-start line.
  - The ghost item appears only while ghosts are shown.
  - On desktop the legend is always visible in the top-right corner. On mobile it sits behind an "ⓘ" icon.
- Tapping a dot or its ghost opens the **day popup**. The popup shows:
  - raw value, adjusted value (labeled as an estimate), and measurement time (the raw value always shows here, whatever the ghost toggle says);
  - the late/early indicator and the disturbance tags;
  - a **"Show day"** button that opens the day entry screen.

## 10. Cycles and stats screen

- The screen is opened on request only; it isn't on the home screen.
- **Current cycle:** "Cycle #N, day X," or "Cycle ? (start unknown)."
- **Stats (all time, completed cycles with a known start only):**
  - latest cycle length;
  - shortest cycle;
  - longest cycle;
  - average cycle length.
- **Cycle length** runs from day 1 up to the day before the next day 1. The current cycle is excluded from stats.
- Each stat shows "based on N cycles." With no completed cycles, it shows "Not enough data yet."
- The average is shown to 1 decimal.
- **Bar chart:** X = cycle start date, Y = length in days. The ongoing cycle appears as a faded bar labeled "ongoing." The unknown-start cycle is not shown.
- **Cycle list:** newest first, with cycle number, start date, end date, and length. The unknown-start cycle shows as "start unknown, length unknown." Tapping a row opens that cycle in Cycle view.

## 11. Settings

- **Units:** °C (default) or °F.
- **Measurement time:** change flow and history (see section 7).
- **Account:** log out and delete account.

## 12. Key acceptance criteria

These are a starting set; more get written per feature during the build. The precise arithmetic behind them, with worked test vectors, is in `docs/fam-rules.md` §12.

- **AC-1 Adjustment (late).** Given fixed time 07:00, when the user saves 36.62 at 07:45, then the stored raw value is 36.62 and the adjusted value is 36.545, displayed as **36.55** (half-up).
- **AC-2 Adjustment (early).** Given fixed time 07:00, when the user saves 36.40 at 06:30, then the adjusted value is 36.45.
- **AC-3 No ghost when rounding cancels.** Given fixed time 07:00, when the user saves 36.50 at 07:01, then no ghost dot and no late/early indicator exist for that day.
- **AC-4 Duplicate date.** Given an entry exists for 3 March, when the user picks 3 March in a new entry, then the existing entry opens for editing.
- **AC-5 Cycle suggestion.** Given the most recent earlier entry is not bleeding, when the user logs bleeding, then the app asks whether to start a new cycle on that date. It does not ask for spotting.
- **AC-6 No false suggestion over a gap.** Given 1 March is bleeding and 2 March has no entry, when the user logs bleeding on 3 March, then no new-cycle suggestion appears.
- **AC-7 "Did bleeding stop?"** Given 3 March is bleeding and 4 March has no entry, when the user saves 5 March with none, then the app asks "Did bleeding stop?", and "No" returns her to the bleeding field. The gap on 4 March does not suppress the question.
- **AC-8 Retroactive cycle start.** Given cycle 3 starts 1 May, when the user marks 20 April as a cycle start, then entries from 20 April onward show recomputed cycle days, and numbers and stats update.
- **AC-9 Measurement time change.** Given 07:00 applied until 31 March and 06:30 from 1 April, when viewing 15 March, then its adjusted value is computed against 07:00.
- **AC-10 Disturbed reading.** Given a reading tagged "alcohol," then it shows as a hollow ring and the line connects its neighbors instead.
- **AC-11 Range cap.** When the user picks a custom range, the date pickers don't allow more than 90 days, an end before the start, or any future date.
- **AC-12 Unknown onboarding.** Given the user picks "I don't know" at onboarding, then her entries show cycle day "?" and the stats screen shows "Not enough data yet" until a cycle with a known start completes.
- **AC-13 No temperature.** When the user saves an entry without a temperature, then the app asks for confirmation. Once saved, the day appears as a gap in the line.
- **AC-14 Unusual temperature.** When the user enters 34.20 °C, then the app asks whether it's correct, and saves only after "Yes."
- **AC-15 Ghost toggle.** Given an adjusted reading, then no ghost dot appears until the user turns on "show raw readings."
- **AC-16 Isolation.** Given two accounts, no API request from user A can read or change user B's data. This is tested twice: once through the API, and once directly against the database as the application role, to prove Row Level Security is active rather than merely enabled (`docs/fam-rules.md` §12.7).
- **AC-17 Impossible temperature.** When a temperature outside 20.00–50.00 °C reaches the API, then it is rejected with a readable message and never reaches the database constraint.

## 13. Non-goals for v1

- Fertile/infertile verdicts and temperature-shift detection.
- The **"shortest − 21 (20)" value**, which is dropped permanently rather than deferred. It appears on the paper chart but was never used in practice.
- Thermometer type, cycle intent, and medical context (pills, PCOS).
- Configurable quick-entry fields and a configurable correction rate.
- Importing past paper data.
- Multiple entries per date.
- Onboarding follow-up prompts.
- Time zone handling. Entries use the date and time as entered; when traveling, the user can add a note.
- Export resembling the paper chart (planned for v2).
- WhatsApp bot (v3). The backend is still an API from day one.
- Reminders or notifications, partner sharing, offline mode, and a native app.

## 14. Future ideas

- Thermometer type, cycle intent, and medical context, with gentle prompts to fill them in later.
- A user-configurable correction rate.
- User-configurable quick-entry fields.
- Option C: rule-based temperature-shift detection (e.g. Sensiplan-style rules). If it is ever built, its rules go in `docs/fam-rules.md` first.
- Time zone handling for travel.
- Paper-chart-style export, and the WhatsApp bot.

## 15. To research

Most of the original list is closed. Madam implements the first user's own paper method rather than a clinical standard, deliberately — the project's primary goals are the process and the learning, and an app that disagreed with her existing history would be worse, not better. `docs/fam-rules.md` §1 records that choice and labels the correction rate as unsourced rather than implying authority it doesn't have.

What remains:

- **Confirm the direction of the correction with the first user** — early adds, late subtracts. Currently an assumption; a sign error would be visibly wrong to someone who has charted by hand for months. Ten seconds to check, so check.
- **Legal and privacy check before opening sign-up publicly.** GDPR (health data is a special category) and Israel's Privacy Protection Law after Amendment 13. This is attached to public sign-up, not to v1: while the app is invite-gated with one user and an EU region, the exposure is minimal.
