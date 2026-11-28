# Madam — FAM Rules

_Status: written at step 4. This is the **single source of truth for the domain
logic**: the exact arithmetic and the exact conditions the backend implements.
`spec-v1.md` says what the app does; this file says precisely how each rule
computes. The unit tests in `internal/fam` are written directly from §12._

_Amended in Slice 1 (8 November 2026): §3.4 states that "adjusted?" is decided
in Celsius whatever the display unit; §3.5 gains the exact Fahrenheit
arithmetic; §5.2 recognises the unknown-start cycle by the stored onboarding
answer; §12 gains vectors F3, C11–C14 and M1–M5 (§12.8)._

_Amended in Slice 3 (14 November 2026): §9 says what a future date is when
the server doesn't know her time zone, adds a floor (1 January 2000), and
fixes how a date and a time of day are written; §12 gains vectors FD1–FD8
(§12.9), and P1–P10 and H1–H12 (§12.10)._

_Amended in the Slice 5 plan (28 November 2026): §5.1 defines
`start_number`, the number a cycle starting on a date has or would have;
§6.2 says what "moving" a start means on the day screen; §12.3 gains
vectors C15–C17._

---

## 1. Provenance and honesty

**These rules are copied from the paper chart the first user already fills in by
hand**, not from a clinical standard. The printed form itself is not part of this
repository.

That was a deliberate choice. The app's job is to replace *her* paper chart, so
matching her method matters more than matching a textbook. An app that "corrects"
her arithmetic would disagree with her own history and be less useful, not more.

Consequences, stated plainly because Principle 3 (honesty in the UI) demands it:

- **The 0.05 °C / 30 min correction rate is unsourced.** It is the rate she uses.
  We have not traced it to a published method, and we do not claim it is
  clinically validated. It is a convention, implemented faithfully.
- **Madam gives no fertility verdicts.** No fertile window, no ovulation
  detection, no temperature-shift rules (`spec-v1.md` §13). Every computed value
  is a description of what was recorded, not a prediction.
- **Anything derived is labelled as an estimate in the UI**, adjusted
  temperatures included.
- **This is not medical advice**, and the app should never read as though it is.

If a rule here is ever changed to match an established method, it changes **in
this file first**, and the tests change with it.

---

## 2. Vocabulary

| Term | Meaning |
|---|---|
| **Raw temperature** | What the thermometer showed, exactly as she typed it. |
| **Measured time** | The clock time that reading was actually taken. |
| **Fixed time** | The measurement time in effect for that date (§4). |
| **Adjusted temperature** | The raw value corrected for the difference between measured and fixed time (§3). |
| **Adjusted reading** | A reading whose adjusted value differs from its raw value *after rounding* (§3.4). |
| **Disturbed reading** | A reading on a day carrying at least one disturbance tag. |
| **Cycle start** | A date the user declared as day 1 of a cycle. |
| **Completed cycle** | A cycle that has a known start *and* a later cycle start after it. |

---

## 3. Time adjustment

### 3.1 Scope

Adjustment applies **only to entries that have a temperature**. An entry with no
temperature has no adjusted value, no ghost dot, and no late/early indicator.

### 3.2 The formula

```
adjusted = raw + 0.05 °C × (fixed_time − measured_time) / 30 minutes
```

**Direction: measuring early adds, measuring late subtracts.** Body temperature
rises through the morning, so a reading taken later than usual is assumed to be
higher than it would have been at the fixed time, and is corrected downward.

> **Unverified.** The direction is our reading of the method, not something
> confirmed against her paper chart or a source. Confirm with her before the
> graph ships — a sign error here would be visibly wrong to someone who has been
> charting by hand for months.

The rate is **fixed in v1**. A user-configurable rate is a future idea
(`spec-v1.md` §14).

### 3.3 The arithmetic, exactly

Temperatures are stored as integers in **thousandths of a degree Celsius**
(`data-model.md` §8.2): 36.62 °C is `36620`. Times are whole minutes.

```
delta              = fixed_minutes − measured_minutes     // positive = early
numerator          = 3 × raw_milli + 5 × delta            // exact adjusted × 3
display_hundredths = (numerator + 15) / 30                // Go integer division
```

Why it is built this way:

- `5 × delta / 3` is not always a whole number of thousandths (one minute is
  1.667), so the exact value is kept multiplied by 3 and **rounded exactly once**,
  straight to the hundredths that get displayed. Rounding to thousandths first and
  to hundredths second can produce a different answer.
- The `+ 15` before dividing by 30 is what turns truncation into **half-up**
  rounding.

> **Constraint on the implementation.** Go's integer division truncates toward
> zero, which equals floor only when the numerator is positive. Body temperatures
> are always positive, so this is safe — but the function must say so in a comment
> and a test must assert it.

**Rounding mode is half-up**, chosen so a person can check the app's arithmetic
with a pencil and get the same number (`data-model.md` §8.3). Banker's rounding
was rejected.

> **This arithmetic exists twice** (since Slice 4). The server's is
> `internal/fam`. The page holds a copy, `frontend/src/lib/adjust.ts`, only
> for the day screen's live line, which must follow her typing; nothing the
> copy computes is saved or sent (`tech-stack.md` §1.3). **Change the rate or
> the rounding in both.** Two tests hold them together: the vectors of §12.1
> on each side, and one checksum over 17,285,760 readings (four fixed times,
> every accepted temperature, every minute of the day) whose expected value
> is written in `internal/fam/copy_test.go` and in
> `frontend/src/lib/adjust.test.mjs`. A change on one side only fails that
> side's test.

### 3.4 When a reading counts as adjusted

```
raw_hundredths = (raw_milli + 5) / 10        // half-up, positive values only
adjusted       = display_hundredths ≠ raw_hundredths
```

If they are equal — a one-minute difference, for instance — then for all purposes
the reading was taken on time: **no ghost dot, no late/early indicator** (AC-3).

Late/early status is **derived on every read and never stored**. There is no
late/early tag and the user cannot set one.

**Decided in Celsius, whatever the display unit** (decided in Slice 1,
8 November 2026). A user who views °F gets the same adjusted status as one who
views °C with the same data, and the status never changes when she switches
units. Accepted cost: near a rounding edge the two units can disagree, in both
directions:

- raw 35.000 °C measured 3 minutes late is **not** adjusted (35.00 → 35.00 °C),
  although its °F values are 95.00 and 94.99 (vector F3);
- raw 35.003 °C measured 2 minutes early **is** adjusted (35.00 → 35.01 °C),
  although both of its °F values are 95.01.

The alternative, comparing in the unit she views, was rejected: the status would
depend on a setting, and the arithmetic would need the unit as an input.

### 3.5 Fahrenheit

Fahrenheit is **display and input only**. All arithmetic happens in Celsius
thousandths.

- **Input:** convert to Celsius thousandths and store that.
- **Display:** convert from the same exact intermediate value, never from the
  already-rounded Celsius figure, so nothing is rounded twice.

The round trip must be invisible: a user who types `97.91 °F` must be shown
`97.91 °F` back, never `97.92 °F`. Storing thousandths rather than hundredths is
what makes that hold, since one step of 0.01 °F is finer than 0.01 °C.

The arithmetic, exactly (added in Slice 1, 8 November 2026). Input is typed in
hundredths of a degree, like Celsius, and both directions round half-up:

```
// input: accepted from 68.00 to 122.00 °F, which is exactly 20.00–50.00 °C;
// anything outside is rejected, like the 20–50 °C rule in §9
x            = f_hundredths − 3200                // positive across the accepted range
milli        = (100 × x + 9) / 18                 // half-up(x × 50 / 9)

// display: from the exact numerator of §3.3 (for a raw value, 3 × raw_milli)
f_hundredths = (3 × numerator + 25) / 50 + 3200   // half-up(numerator × 3 / 50) + 32.00 °F
```

Both numerators are positive across the accepted range, so §3.3's constraint on
truncating division holds here too. The round trip was checked for every one of
the 5,401 inputs from 68.00 to 122.00 °F, and a unit test checks all of them.

---

## 4. Which fixed time applies to a date

`measurement_time_periods` holds rows of `(valid_from, measurement_time)`. Each
row applies from its `valid_from` until the day before the next row's
`valid_from`; the latest row applies indefinitely (`data-model.md` §4.4).

```
fixed_time(date) = measurement_time of the row with the greatest
                   valid_from ≤ date
```

The first row is written at onboarding with `valid_from = -infinity`, so **every
date has a fixed time in effect**, including days backfilled from before the
account existed (`spec-v1.md` §7). There is no "no time set" case to handle.

Changing the measurement time **never alters past adjusted values**, because
earlier dates still resolve to the earlier row.

Guardrails when changing the time (the user gives "the last date the previous
time applied"; `valid_from` is that date **+ 1 day**):

- the last date cannot be in the future;
- it cannot be earlier than the start of the current period.

---

## 5. Cycles

Nothing in this section is stored. Everything derives from the `cycle_starts`
rows, so inserting or deleting one row recomputes days, lengths and numbers
automatically (AC-8).

### 5.1 Definitions

```
cycle_day(date)    = date − (greatest start_date ≤ date) + 1
cycle_length(s)    = (next start_date after s) − s
cycle_number(s)    = position of s in the user's ordered start dates, from 1
start_number(d)    = (number of start dates before d) + 1
```

`start_number` is the number a cycle starting on `d` has if `d` is a start, or
would have if `d` were made one. The day screen says it before saving:
"Cycle 5 will start on 08/11/2026 when you save" (`spec-v1.md` §8.2). It is
the same as `cycle_number(d)` once `d` is a start, and it is 1 for a date
before every start, inside the unknown-start cycle included (§5.2).

- Cycles have **no maximum length**. Day 41 and beyond are normal and must never
  be treated as an error.
- The **current cycle** (the latest start, with nothing after it) has no length
  and is excluded from statistics.

### 5.2 The unknown-start cycle

Created when onboarding answered *"I don't know"*. It spans from the user's first
entry to the day before her first known cycle start.

- It has no number; it displays as **"Cycle ? (start unknown)"**.
- Its cycle days display as **"?"**.
- It is excluded from every statistic.
- If she later marks a start date inside it, it becomes an ordinary cycle and all
  numbering updates.

With no cycle start at all, it runs from her first entry onward (AC-12). Dates
before her first entry belong to no cycle (§5.3), whatever onboarding answered.

The cycle itself is not stored, but **the onboarding answer is**, as
`users.cycle_start_unknown` (`data-model.md` §4.1; amended in Slice 1,
8 November 2026). An unknown-start cycle exists when that answer is true and she
has entries before her first cycle start, or any entries if there is no start.
The answer has to be stored because the data alone can't tell the two
onboardings apart: "I don't know" followed by entries, and a known start date
followed by entries backfilled before it (§5.3), leave exactly the same rows
behind. `onboarded_at` only says whether onboarding finished, not what she
answered.

### 5.3 Entries outside any cycle

If onboarding gave a known start date, entries dated **before** the first known
cycle start belong to no cycle at all. Their cycle day is **blank**, not `?` —
blank means "outside any cycle", `?` means "inside a cycle whose start we don't
know". They are two different states and must display differently.

---

## 6. Starting a new cycle

### 6.1 Suggested start

When an entry is saved with bleeding level **`bleeding`** (not `spotting`), the
backend suggests starting a new cycle on that date — but **only if both** hold:

1. that date is not already a cycle start;
2. the **most recent earlier entry, whatever its date**, is not a `bleeding`
   entry.

Condition 2 is what stops a missing day mid-period from producing a false
suggestion: 1 March bleeding, 2 March no entry, 3 March bleeding → **no
suggestion**, because the most recent earlier entry (1 March) was bleeding (AC-6).

The suggestion is a **question**, never an automatic action. The user confirms or
dismisses it. It appears on the "cycle starts on this day" switch (§6.2): turning
the switch on confirms, leaving it off dismisses (`design-brief.md` §6.3).

### 6.2 Manual start

The day entry screen has a "cycle starts on this day" toggle, usable on any past
date, for corrections and backfilling. It bypasses the conditions in §6.1
entirely — a deliberate override always wins.

Moving a cycle start is a delete plus an insert **in one transaction**
(`data-model.md` §4.3).

*Added in the Slice 5 plan (28 November 2026):* the day screen has no
"move". Its switch adds or removes the start on the one day it shows, so
moving a start there is two saves, on two days, and each leaves a
consistent set of starts. The one-transaction rule is for a future control
that moves a start in one step. Saving a day, its entry and its start
together, is one transaction whatever it holds.

### 6.3 "Did bleeding stop?"

When an entry is saved with bleeding level `none` or `spotting`, and the **most
recent earlier entry, whatever its date**, has `bleeding`, the backend asks *"Did
bleeding stop?"*

- **Yes** → save.
- **No** → return the user to the bleeding field.

Purpose: a bleeding value left unset saves as `none` (`data-model.md` §4.2), and a
forgotten `none` in the middle of a period would later satisfy §6.1 condition 2
and trigger a false new-cycle suggestion. This check is the net.

> **Both rules use the same lookback, and they must.** §6.1 and §6.3 each look at
> *the most recent earlier entry, whatever its date*. Narrowing §6.3 to the
> previous calendar day would open a hole:
>
> 3 Mar `bleeding` → 4 Mar no entry → 5 Mar saved with only a temperature, so
> bleeding defaults to `none` → 6 Mar `bleeding`.
>
> With a calendar-day lookback, 5 Mar asks nothing (4 Mar has no entry) and the
> wrong `none` is stored. On 6 Mar, §6.1 then sees 5 Mar `none` and suggests a new
> cycle in the middle of her period — precisely the failure §6.3 exists to
> prevent. The net only works if it has the same reach as the rule it protects.
>
> Accepted cost: after a long break from charting, the first entry back may ask
> "Did bleeding stop?" about a bleeding entry from weeks ago. One extra tap, and
> the alternative is an arbitrary cut-off constant.

### 6.4 Where these rules live

Both checks are **backend decisions** (`tech-stack.md` §1.3). The backend decides
*whether* to ask; each client decides *how* to ask. If the frontend owned this
logic, the future WhatsApp bot would silently skip both checks.

---

## 7. Disturbed readings

A reading is disturbed if its `disturbances` array is non-empty. Allowed tags:
`alcohol`, `poor_sleep`, `illness`, `medication`, `travel`, `vacation`,
`new_thermometer`, `uti`, `yeast_infection`.

On the graph:

- the dot is drawn as a **hollow ring**;
- **the line skips it and connects its neighbours** (AC-10).

The reading is still stored, still adjusted normally, and still appears in the day
popup. Disturbance affects **drawing only** — it changes nothing about the
arithmetic and nothing about cycles.

Note the contrast with a day that has no reading at all, which produces a **gap**
in the line. Two different holes: one the line steps over, one it stops at.

**Symptoms** (`breast_tenderness`, `bloating`, `pms`) are recorded only. They
never affect the chart. This is why they are a separate column from disturbances.

---

## 8. Statistics

Computed over **completed cycles with a known start**, all time. The current
cycle and the unknown-start cycle are always excluded.

| Stat | Definition |
|---|---|
| Latest | Length of the most recently completed cycle. |
| Shortest | Minimum completed length. |
| Longest | Maximum completed length. |
| Average | Mean of completed lengths, displayed to **1 decimal**, half-up. |

- The stats display **"based on N cycles"** once, under all four: they count the
  same completed cycles, so N is always the same (amended in the design step,
  8 November 2026; `spec-v1.md` §10).
- With no completed cycles: **"Not enough data yet"**.
- The bar chart shows completed cycles plus the ongoing one as a faded bar
  labelled "ongoing". The unknown-start cycle is not shown at all.

The "shortest − 21 (20)" value from the paper chart is **deliberately not
implemented** and is not a future idea. It was never used on the paper chart in
practice.

---

## 9. Validation

| Rule | Behaviour | Enforced by |
|---|---|---|
| One entry per date | Picking an existing date opens it for editing (AC-4) | Database primary key |
| No future dates (§9.1) | Rejected | Go |
| No date before **1 January 2000** (§9.1) | Rejected | Go |
| A date or a time of day not written exactly as §9.2 says | Rejected | Go |
| Temperature outside **35.00–39.00 °C** | **Question**: "This temperature is unusual, is it correct?" Yes saves, No returns to the field | Go |
| Temperature outside **20.00–50.00 °C** | **Rejected** with a clear message | Go, with a database `CHECK` as backstop |
| Temperature with no measurement time, or vice versa | Impossible | Database `CHECK` |
| Entry with no temperature | Confirmation prompt, then saves; the day is a gap in the line | Go |
| A disturbance or symptom that isn't in §7's lists, or one given twice (§9.4) | Rejected | Go, with a database `CHECK` for the lists |
| Notes over **2,000 characters**, or a secretion description over **200** (§9.5) | Rejected | Go |

The two temperature ranges catch different mistakes and never collide. **35–39**
catches a human typo (34.20 for 36.20) and only ever asks a question. **20–50**
catches a *unit* bug in our own code — a conversion multiplying by 100 instead of
1000 would store `3662` — and rejects. A user should never see the database
error; the Go check exists so the message is readable.

The 35–39 bounds are an assumption and can be tuned.

### 9.1 Which dates are accepted

Added in Slice 3. These two limits apply to **every date the app accepts**:
a cycle start, an entry's date, the last date of a measurement-time period.

**No future dates.** "Future" needs a today, and the server doesn't know
hers: its clock is in UTC, she is usually in Israel, and time zones are not
handled (§11). So the rule has two halves.

- **The page** offers no date after the one her device shows. That is her
  today, wherever she is.
- **The server** refuses a date only when it hasn't begun *anywhere on
  Earth*:

  ```
  latest_today(now) = the calendar date of (now in UTC + 14 hours)
  a date is in the future  ⇔  date > latest_today(now)
  ```

  UTC+14 is the time zone furthest ahead, so a date after `latest_today` is
  nobody's today yet, and a date up to it may be hers.

The server's half never refuses a real today. That is what a stricter rule
would get wrong: by the server's own UTC date, the first two hours of every
day in Israel (three in summer) are "tomorrow". The price is that it is
looser than the page. A request that didn't come from our page can save
tomorrow's date once tomorrow has begun at UTC+14, which in Israel is from
12:00 (13:00 in summer) until midnight. It is her own data, and it takes
going around her own page.

**No date before 1 January 2000.** A floor for a mangled year (0002, 1026),
not a judgement about plausible cycles: cycles still have no maximum length
(§5.1). It is never wrong for a real person and can be tuned.

### 9.2 How dates and times are written

Added in Slice 3, for everything the API reads and writes.

- **A date is `YYYY-MM-DD`,** exactly ten characters: four digits, a
  hyphen, two digits, a hyphen, two digits. It must also exist on the
  calendar.
- **A time of day is `HH:MM`,** 24-hour, exactly five characters: two
  digits, a colon, two digits, from `00:00` to `23:59`. There are no
  seconds, and `24:00` is not a time.

One spelling each, so `7:05` and `2026-2-3` are refused, not tidied up. What
the user sees (dd/mm/yyyy, `design-brief.md` §7) is the page's concern.

### 9.3 A temperature as it arrives

Added in Slice 4. A temperature arrives in whole hundredths of a degree,
with its unit named: 36.62 °C is 3662 hundredths of Celsius
(`tech-stack.md` §3).

```
// Celsius: exact, nothing is rounded
milli = 10 × c_hundredths            // accepted from 20.00 to 50.00 °C

// Fahrenheit: §3.5                  // accepted from 68.00 to 122.00 °F
```

Then, in this order:

1. **Outside the accepted range, it is rejected.** The two ranges are the
   same temperatures, 20.000 to 50.000 °C, and they don't overlap as
   numbers. So a value sent with the wrong unit (97.91 as Celsius, 36.62 as
   Fahrenheit) is always rejected and never stored as a wrong reading.
2. **Otherwise, the stored value decides the question:**

   ```
   unusual  ⇔  milli < 35000  or  milli > 39000
   ```

   35.00 and 39.00 °C themselves are usual. It is decided on the stored
   thousandths, whatever unit she typed, so in Fahrenheit the edges are
   exactly the spec's 95.00 and 102.20 °F (`spec-v1.md` §5.3).

An unusual temperature is saved once she has said it is correct. A rejected
one is never saved.

### 9.4 Disturbances and symptoms

Added in Slice 4. Each is a list of tags from §7.

- A tag that isn't in its own list is rejected. `pms` is a symptom, so it
  is rejected as a disturbance. One spelling each: `Alcohol` is rejected.
- A tag given twice is rejected.
- **Any order is accepted, and the list is stored in §7's order.** A day
  with alcohol and poor sleep is then stored one way, however it was
  tapped.

### 9.5 Free text

Added in Slice 4. The notes hold up to **2,000 characters** and the
secretion description up to **200**. A character is a Unicode code point,
not a byte: 2,000 Hebrew letters are 4,000 bytes and are accepted. Text is
stored as typed, line breaks included; it is never trimmed. An empty text
is the same as none.

Both limits are assumptions and can be tuned. They exist so that an
over-long text is refused about its own field, with a message, and not by
the size limit on a whole request.

---

## 10. Per-user isolation

Every query filters by `user_id`, and Row Level Security enforces the same rule
inside the database as a second layer (`data-model.md` §9). No request from one
account can read or modify another account's data (AC-16).

This is the highest-stakes rule in the project. It gets a test that runs on every
change.

---

## 11. What this file deliberately does not contain

- Fertile/infertile determination of any kind.
- Temperature-shift detection (Sensiplan-style or otherwise). Listed as a future
  idea; if it is ever built, its rules are written here first.
- Thermometer type, cycle intent, or medical context.
- Time zone handling. Dates and times are taken exactly as entered. The one
  place a time zone could matter, "no future dates", is written so that it
  needs none (§9.1).
- "Shortest − 21 (20)" (§8).

---

## 12. Test vectors

These are the unit tests. Every row is a case in `internal/fam`, written before
the implementation.

### 12.1 Adjustment

Fixed time 07:00 (420 minutes) throughout.

| # | Raw (milli) | Measured | delta | numerator | Displayed | Adjusted? | Source |
|---|---|---|---|---|---|---|---|
| T1 | 36620 | 07:45 | −45 | 109635 | **36.55** | yes | AC-1 |
| T2 | 36400 | 06:30 | +30 | 109350 | **36.45** | yes | AC-2 |
| T3 | 36500 | 07:01 | −1 | 109495 | **36.50** | **no** | AC-3 |
| T4 | 36700 | 07:00 | 0 | 110100 | **36.70** | no | on time |
| T5 | 36620 | 08:30 | −90 | 109410 | **36.47** | yes | large late |
| T6 | 36400 | 05:00 | +120 | 109800 | **36.60** | yes | large early |
| T7 | 36628 | 06:56 | +4 | 109904 | **36.63** | **no** | just under a half |

T1 is the half-up case: the exact adjusted value is 36.545 and it must display as
**36.55**, not 36.54.

T3 is the cancellation case: the adjusted value differs from raw by less than half
a hundredth, so after rounding it is identical and the reading is **not** adjusted
— no ghost dot, no indicator.

T7 is the other side of T1 (added in Slice 4). The exact adjusted value is
36.634667, a third of a thousandth under the half, and it must display as
**36.63**, not 36.64, which also makes the reading not adjusted. Its raw
value isn't a whole hundredth of a degree Celsius: 36628 is 97.93 °F as
stored (§3.5). Only a reading typed in Fahrenheit can land this close under
a half; one typed in Celsius can't. It was added because rounding with
`+ 16` in place of `+ 15` passed every vector above it.

### 12.2 Fahrenheit round trip

| # | Input | Stored (milli) | Displayed back |
|---|---|---|---|
| F1 | 97.91 °F | 36617 | **97.91 °F** |
| F2 | 98.60 °F | 37000 | **98.60 °F** |

| # | Raw (milli) | Measured (fixed 07:00) | °C raw → adjusted | °F raw → adjusted | Adjusted? |
|---|---|---|---|---|---|
| F3 | 35000 | 07:03 | 35.00 → 35.00 | 95.00 → **94.99** | **no**: Celsius decides (§3.4) |

F1 is the case that justifies thousandths: stored in hundredths it would come back
as 97.92 °F, silently changing a number the user typed.

### 12.3 Cycles

Cycle starts: 1 Jan, 29 Jan, 26 Feb (all same year). Onboarding gave a known
start date. 20 Dec is in the year **before**.

| # | Case | Expected |
|---|---|---|
| C1 | Cycle day of 1 Jan | 1 |
| C2 | Cycle day of 15 Jan | 15 |
| C3 | Cycle day of 28 Jan | 28 |
| C4 | Cycle day of 29 Jan | 1 |
| C5 | Length of the cycle starting 1 Jan | 28 |
| C6 | Length of the cycle starting 26 Feb | none — current cycle |
| C7 | Number of the cycle starting 26 Feb | 3 |
| C8 | Cycle day of 20 Dec (before the first start) | blank, no cycle |
| C9 | Day 45 of an ongoing cycle | 45, not an error |
| C10 | Insert a start at 20 Dec | everything renumbers; 20 Dec becomes cycle 1 (AC-8) |

Added in Slice 1 (§5.2): the same starts, but onboarding answered "I don't know"
and her first entry is 10 Dec of the year before.

| # | Case | Expected |
|---|---|---|
| C11 | Cycle day of 20 Dec | **?**, the unknown-start cycle |
| C12 | Cycle day of 5 Dec (before her first entry) | blank, no cycle |
| C13 | Number of the cycle starting 1 Jan | 1; the unknown-start cycle has no number |
| C14 | No cycle starts at all; cycle day of 15 Jan | **?** (AC-12) |

Added in the Slice 5 plan (28 November 2026): `start_number` (§5.1), with
the starts at the top of this section (1 Jan, 29 Jan, 26 Feb; known start).

| # | Case | Expected |
|---|---|---|
| C15a | `start_number` of 10 Feb | 3 |
| C15b | `start_number` of 20 Dec (before every start) | 1 |
| C15c | `start_number` of 29 Jan (already a start) | 2, its own number |
| C15d | `start_number` of 1 Mar (after every start) | 4 |

Also added in the Slice 5 plan: a start marked inside the unknown-start cycle,
and one before her first entry (§5.2). Each starts from C11's setup: starts
1 Jan, 29 Jan, 26 Feb, "I don't know", first entry 10 Dec of the year
before.

| # | Case | Expected |
|---|---|---|
| C16a | Mark 20 Dec as a start; cycle day of 15 Dec | **?**: the unknown-start cycle now ends on 19 Dec |
| C16b | The same; cycle day and number of 20 Dec | day 1 of cycle 1 |
| C16c | The same; cycle day of 25 Dec | 6 |
| C16d | The same; number of the cycle starting 1 Jan | 2: everything after renumbers |
| C17a | Mark 1 Dec (before her first entry) as a start; cycle day of 10 Dec | day 10 of cycle 1: no unknown-start cycle is left |
| C17b | The same; cycle day of 30 Nov | blank, no cycle |
| C17c | The same; number of the cycle starting 1 Jan | 2 |

### 12.4 Cycle suggestion

| # | Setup | Action | Expected |
|---|---|---|---|
| S1 | Most recent earlier entry is `none` | log `bleeding` | suggest (AC-5) |
| S2 | Any | log `spotting` | no suggestion (AC-5) |
| S3 | 1 Mar `bleeding`, 2 Mar no entry | log `bleeding` on 3 Mar | **no** suggestion (AC-6) |
| S4 | Date is already a cycle start | log `bleeding` | no suggestion |
| S5 | No earlier entry at all | log `bleeding` | suggest |

### 12.5 "Did bleeding stop?"

| # | Setup | Action | Expected |
|---|---|---|---|
| B1 | 4 Mar `bleeding` | save 5 Mar `none` | ask (AC-7) |
| B2 | 4 Mar `bleeding` | save 5 Mar `spotting` | ask |
| B3 | 4 Mar `none` | save 5 Mar `none` | no question |
| B4 | 3 Mar `bleeding`, 4 Mar no entry | save 5 Mar `none` | **ask** — the most recent earlier entry is bleeding |
| B5 | No earlier entry at all | save 5 Mar `none` | no question |
| B6 | 3 Mar `bleeding`, nothing since | save 1 Apr `none` | ask — no cut-off by design |

### 12.6 Statistics

Completed lengths 28, 30, 27; one ongoing cycle; plus an unknown-start cycle.

| # | Stat | Expected |
|---|---|---|
| A1 | Latest | 27, "based on 3 cycles" |
| A2 | Shortest | 27 |
| A3 | Longest | 30 |
| A4 | Average | **28.3** |
| A5 | Ongoing cycle | excluded from all four |
| A6 | Unknown-start cycle | excluded from all four |
| A7 | No completed cycles | "Not enough data yet" |

### 12.7 Isolation

| # | Case | Expected |
|---|---|---|
| I1 | User A requests user B's entry by date | not found, never B's data |
| I2 | User A updates a date that exists for B only | creates A's own entry; B's is untouched |
| I3 | Query run as the app role with the wrong user id set | zero rows — proves RLS is active, not just the `WHERE` clause |

I3 is the one that proves Row Level Security is really switched on. Without it,
a misconfigured owner role would silently bypass every policy while the app
appeared to work perfectly.

### 12.8 Fixed time

Added in Slice 1 (§4). The onboarding row says 07:00 (`valid_from = -infinity`);
a change says 06:30 from 1 April 2026.

| # | Date | Expected fixed time |
|---|---|---|
| M1 | 15 Mar 2026 | 07:00 (AC-9) |
| M2 | 31 Mar 2026, the last day of the first period | 07:00 |
| M3 | 1 Apr 2026 | 06:30 |
| M4 | 1 Jan 2020, before the account existed | 07:00, from the `-infinity` row |
| M5 | Only the onboarding row; 1 Jan 2030 | 07:00 |

### 12.9 Accepted dates

Added in Slice 3 (§9.1). "Now" is the server's clock, in UTC; the dates are
in 2026. Israel is at UTC+2 in November.

| # | Now (UTC) | Date | Expected |
|---|---|---|---|
| FD1 | 13 Nov, 21:59 (23:59 in Israel) | 13 Nov | accepted |
| FD2 | 13 Nov, 22:30 (00:30 on the 14th in Israel) | 14 Nov | **accepted**: her today, though the server's UTC date is still the 13th |
| FD3 | 13 Nov, 09:59 | 14 Nov | refused: the 14th hasn't begun anywhere (23:59 on the 13th at UTC+14) |
| FD4 | 13 Nov, 10:00 | 14 Nov | accepted: it is 00:00 on the 14th at UTC+14 |
| FD5 | 13 Nov, 22:30 | 15 Nov | refused |
| FD6 | 13 Nov, 22:30 | 1 Jan 2020 | accepted: any past date |
| FD7 | 13 Nov, 22:30 | 31 Dec 1999 | refused: before the floor |
| FD8 | 13 Nov, 22:30 | 1 Jan 2000 | accepted: the floor itself |

FD2 is the case the rule exists for, and FD3 and FD4 are its edge, one
minute apart.

### 12.10 Written dates and times

Added in Slice 3 (§9.2).

| # | Written | Expected |
|---|---|---|
| P1 | `2026-11-01` | 1 November 2026 |
| P2 | `2024-02-29` | 29 February 2024, a leap year |
| P3 | `2026-02-30` | refused: no such date |
| P4 | `2026-13-01` | refused: no such date |
| P5 | `2026-2-3` | refused: not `YYYY-MM-DD` |
| P6 | `01/11/2026` | refused: not `YYYY-MM-DD` |
| P7 | `2026-11-01T00:00:00Z` | refused: not `YYYY-MM-DD` |
| P8 | ` 2026-11-01` (a space first) | refused: not `YYYY-MM-DD` |
| P9 | `２０２６-11-01` (full-width digits) | refused: not `YYYY-MM-DD` |
| P10 | nothing at all | refused: not `YYYY-MM-DD` |

| # | Written | Expected |
|---|---|---|
| H1 | `07:05` | 425 minutes after midnight |
| H2 | `00:00` | 0 |
| H3 | `23:59` | 1439 |
| H4 | `24:00` | refused: outside 00:00–23:59 |
| H5 | `07:60` | refused: outside 00:00–23:59 |
| H6 | `7:05` | refused: not `HH:MM` |
| H7 | `07:05:00` | refused: not `HH:MM` |
| H8 | `07.05` | refused: not `HH:MM` |
| H9 | `0705` | refused: not `HH:MM` |
| H10 | `7:05 AM` | refused: not `HH:MM` |
| H11 | `-7:05` | refused: not `HH:MM` |
| H12 | nothing at all | refused: not `HH:MM` |

Writing a time back out is the reverse: 425 is `07:05`. Every minute of the
day, 0 to 1439, must survive being written and read again.

### 12.11 A temperature as it arrives

Added in Slice 4 (§9.3).

| # | Arrives as | Stored (milli) | Expected |
|---|---|---|---|
| L1 | 34.20 °C | 34200 | accepted; **unusual**, so she is asked (AC-14) |
| L2 | 36.62 °C | 36620 | accepted; usual |
| L3 | 35.00 °C | 35000 | usual: the edge itself |
| L4 | 34.99 °C | 34990 | unusual |
| L5 | 39.00 °C | 39000 | usual: the edge itself |
| L6 | 39.01 °C | 39010 | unusual |
| L7 | 20.00 °C | 20000 | accepted, the lowest; unusual |
| L8 | 19.99 °C | none | **rejected** (AC-17) |
| L9 | 50.00 °C | 50000 | accepted, the highest; unusual |
| L10 | 50.01 °C | none | **rejected** (AC-17) |
| L11 | 95.00 °F | 35000 | usual: the same edge as L3 |
| L12 | 94.99 °F | 34994 | unusual |
| L13 | 102.20 °F | 39000 | usual: the same edge as L5 |
| L14 | 102.21 °F | 39006 | unusual |
| L15 | 68.00 °F | 20000 | accepted, the lowest; unusual |
| L16 | 67.99 °F | none | **rejected** |
| L17 | 122.00 °F | 50000 | accepted, the highest; unusual |
| L18 | 122.01 °F | none | **rejected** |
| L19 | 97.91 °C, a Fahrenheit number sent as Celsius | none | **rejected** |
| L20 | 36.62 °F, a Celsius number sent as Fahrenheit | none | **rejected** |

Every Celsius value from 20.00 to 50.00 must also come back as it was typed:
stored, then displayed, it is the same hundredths.

### 12.12 Disturbances and symptoms

Added in Slice 4 (§9.4).

| # | List | As | Expected |
|---|---|---|---|
| G1 | empty | disturbances | accepted: none |
| G2 | `poor_sleep`, `alcohol` | disturbances | stored as `alcohol`, `poor_sleep` |
| G3 | all nine of §7, last first | disturbances | all nine, in §7's order |
| G4 | `alcohol`, `alcohol` | disturbances | rejected: given twice |
| G5 | `jet_lag` | disturbances | rejected: not in the list |
| G6 | `pms` | disturbances | rejected: a symptom isn't a disturbance |
| G7 | `Alcohol` | disturbances | rejected: one spelling |
| G8 | an empty tag | disturbances | rejected |
| G9 | `pms`, `breast_tenderness` | symptoms | stored as `breast_tenderness`, `pms` |
| G10 | `alcohol` | symptoms | rejected: a disturbance isn't a symptom |

### 12.13 Free text

Added in Slice 4 (§9.5).

| # | Text | Expected |
|---|---|---|
| X1 | notes of 2,000 characters | accepted |
| X2 | notes of 2,001 characters | rejected |
| X3 | notes of 2,000 Hebrew letters (4,000 bytes) | accepted: characters are counted, not bytes |
| X4 | notes of 2,001 Hebrew letters | rejected |
| X5 | a secretion description of 200 characters | accepted |
| X6 | a secretion description of 201 characters | rejected |
| X7 | empty notes | accepted: the same as none |

---

## 13. Open items

- **Direction of the correction** (§3.2) — confirm with her that early adds and
  late subtracts, matching her paper chart.
- **The 35–39 °C bounds** (§9) — an assumption, tunable once there is real usage.
- **Legal and privacy check** — required before sign-up opens publicly, not before
  v1: GDPR (health data is a special category) and Israel's Privacy Protection Law
  after Amendment 13. Currently low risk: invite-gated, one user, EU region.
