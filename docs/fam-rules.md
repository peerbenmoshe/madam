# Madam — FAM Rules

_Status: written at step 4. This is the **single source of truth for the domain
logic**: the exact arithmetic and the exact conditions the backend implements.
`spec-v1.md` says what the app does; this file says precisely how each rule
computes. The unit tests in `internal/fam` are written directly from §12._

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

### 3.4 When a reading counts as adjusted

```
raw_hundredths = (raw_milli + 5) / 10        // half-up, positive values only
adjusted       = display_hundredths ≠ raw_hundredths
```

If they are equal — a one-minute difference, for instance — then for all purposes
the reading was taken on time: **no ghost dot, no late/early indicator** (AC-3).

Late/early status is **derived on every read and never stored**. There is no
late/early tag and the user cannot set one.

### 3.5 Fahrenheit

Fahrenheit is **display and input only**. All arithmetic happens in Celsius
thousandths.

- **Input:** convert to Celsius thousandths and store that.
- **Display:** convert from the same exact intermediate value, never from the
  already-rounded Celsius figure, so nothing is rounded twice.

The round trip must be invisible: a user who types `97.91 °F` must be shown
`97.91 °F` back, never `97.92 °F`. Storing thousandths rather than hundredths is
what makes that hold, since one step of 0.01 °F is finer than 0.01 °C.

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
```

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

It needs no storage: it is recognised by `onboarded_at IS NOT NULL` together with
entries that no `cycle_starts` row covers.

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
dismisses it.

### 6.2 Manual start

The day entry screen has a "cycle starts on this day" toggle, usable on any past
date, for corrections and backfilling. It bypasses the conditions in §6.1
entirely — a deliberate override always wins.

Moving a cycle start is a delete plus an insert **in one transaction**
(`data-model.md` §4.3).

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

- Every stat displays **"based on N cycles"**.
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
| No future dates | Rejected | Go |
| Temperature outside **35.00–39.00 °C** | **Question**: "This temperature is unusual, is it correct?" Yes saves, No returns to the field | Go |
| Temperature outside **20.00–50.00 °C** | **Rejected** with a clear message | Go, with a database `CHECK` as backstop |
| Temperature with no measurement time, or vice versa | Impossible | Database `CHECK` |
| Entry with no temperature | Confirmation prompt, then saves; the day is a gap in the line | Go |

The two temperature ranges catch different mistakes and never collide. **35–39**
catches a human typo (34.20 for 36.20) and only ever asks a question. **20–50**
catches a *unit* bug in our own code — a conversion multiplying by 100 instead of
1000 would store `3662` — and rejects. A user should never see the database
error; the Go check exists so the message is readable.

The 35–39 bounds are an assumption and can be tuned.

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
- Time zone handling. Dates and times are taken exactly as entered.
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

T1 is the half-up case: the exact adjusted value is 36.545 and it must display as
**36.55**, not 36.54.

T3 is the cancellation case: the adjusted value differs from raw by less than half
a hundredth, so after rounding it is identical and the reading is **not** adjusted
— no ghost dot, no indicator.

### 12.2 Fahrenheit round trip

| # | Input | Stored (milli) | Displayed back |
|---|---|---|---|
| F1 | 97.91 °F | 36617 | **97.91 °F** |
| F2 | 98.60 °F | 37000 | **98.60 °F** |

F1 is the case that justifies thousandths: stored in hundredths it would come back
as 97.92 °F, silently changing a number the user typed.

### 12.3 Cycles

Cycle starts: 1 Jan, 29 Jan, 26 Feb (all same year).

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

---

## 13. Open items

- **Direction of the correction** (§3.2) — confirm with her that early adds and
  late subtracts, matching her paper chart.
- **The 35–39 °C bounds** (§9) — an assumption, tunable once there is real usage.
- **Legal and privacy check** — required before sign-up opens publicly, not before
  v1: GDPR (health data is a special category) and Israel's Privacy Protection Law
  after Amendment 13. Currently low risk: invite-gated, one user, EU region.
