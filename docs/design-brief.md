# Madam — Design Brief

_Status: approved by Pe'er on 8 November 2026, after four rounds on a
clickable mock. Written in the design step that replaced "design post-v1"
(`tech-stack.md` §11)._

This file is the **source of truth for how Madam looks and how its screens
behave to the touch**. `spec-v1.md` says what the app does, and
`fam-rules.md` says how each rule computes. Where this file and the mock
disagree, this file wins. Where this file and the spec disagree about
behaviour, the spec wins, and one of them is wrong: fix it in the same PR.

**The mock:** `docs/design/mock.html` opens in any browser, phone-sized, with
made-up data. It's a reference for layout, spacing and behaviour, not code to
copy: it draws its own SVG chart, and the app uses Recharts and React.

---

## 1. Who it's for, and when

- **The first user**, who has charted by hand on a printed paper table for
  months. She is a junior UX/UI designer.
- **At about 07:00, on a phone, half awake, often one-handed.** The most
  common job takes seconds: type a temperature, maybe tap Bleeding, save.
- **Later, calmly:** read the graph, step through days, look at cycle
  lengths.
- **Health data.** The tone is calm and plain. No celebration, no
  gamification, no verdicts (interpretation level B).

**The problem this step fixed:** the shell looked "like some admin-control
page" (Pe'er). Design tokens make colours and fonts cheap to change later,
but not layout and interaction. Those are decided here, before Slice 3
builds the first screen.

## 2. Direction: her paper chart, on a phone

The most characteristic thing in her world is the printed chart: a pink
grid, a hand-drawn line, and rows of observations under each day. Madam
keeps that.

- **One bold element: the graph band.** Pink chart paper, a deep berry line,
  and the paper chart's rows (date, cycle day, bleeding, secretion) lined up
  under the days. Everything around it stays quiet.
- **HotPink is the "chosen" colour.** Selected options, dots, the main
  button, the cycle-start marker. Always as a fill, with ink text or an ink
  outline.
- **Avoided on purpose**, as the look of generated design: identical rounded
  cards with soft shadows, a cream background with a serif display face, a
  tile of big numbers for statistics, all-caps labels, gradients as
  decoration.

## 3. Colour

Light only in v1 (Pe'er, 8 November 2026). The values live in the `@theme`
block of `frontend/src/index.css` (`tech-stack.md` §8.2), and nothing else
paints colour.

Contrast was computed with the WCAG formula on 8 November 2026. Text needs
4.5:1, and graphical objects (lines, outlines) need 3:1.

| Token | Value | Use | Contrast |
|---|---|---|---|
| `surface` | `#FFFFFF` | App background | n/a |
| `sheet` | `#FFF8FB` | The chart-paper band, the temperature card | n/a |
| `ink` | `#2B1726` | Text, outlines, the cycle-start line | 16.8:1 on `surface`, 6.3:1 on `accent` |
| `muted` | `#6E5866` | Secondary text, axis labels | 6.5:1 on `surface`, 5.6:1 on `blush` |
| `berry` | `#A0185A` | The temperature line, dot outlines, links, the "late" flag | 7.6:1 on `surface`, 7.2:1 on `sheet` |
| `accent` | `#FF69B4` | HotPink: fills only, with `ink` on top | 2.65:1 on `surface`, so never text or a thin line |
| `blush` | `#FFEAF3` | Today's column, control tracks, highlighted questions | n/a |
| `grid` | `#F7DCE8` | Chart paper, fine lines (every 0.05 °C) | decorative |
| `grid-strong` | `#EBB6CE` | Chart paper, every 0.10 °C; the "sticky" mark | decorative; `ink` on it 9.7:1 |
| `rule` | `#EAD7E0` | Hairlines between groups, unselected borders | decorative |
| `danger` | `#B3261E` | Destructive actions and error text only | 6.5:1 on `surface` |

**Rules:**
- **Neutrals lean toward plum,** not grey, so the page reads as one family
  with the accent.
- **A HotPink dot always has a berry outline,** which carries the 3:1.
- **Status isn't colour alone.** Every state also has a word, a shape or a
  position: a "45 min late" flag, a ring for disturbed, a letter in the
  secretion row.
- **Shadows only on things that float:** the legend, the day popup, the
  toast.

## 4. Type

**Atkinson Hyperlegible Next**, one family. The Braille Institute designed it
for legibility, which suits reading half awake. It is **self-hosted**: the
files are embedded in the binary like the rest of the frontend, so the CSP
stays `'self'` and nothing is fetched from Google. This reverses
`tech-stack.md` §8.2's "no web fonts" (decided by Pe'er, 8 November 2026).
Fallback: the system font stack.

| Role | Size | Weight |
|---|---|---|
| Screen title ("Sunday 08/11/2026") | 1.75rem | 800 |
| Temperature being typed | 2.5rem | 800 |
| Temperature in the popup | 2.75rem | 800 |
| Section heading | 1.25rem | 800 |
| Body, field labels | 1rem | 400 / 700 |
| Secondary text, buttons in controls | 0.875rem | 400–700 |
| Axis labels, small notes | 0.75rem (SVG: 11px; strip row labels 10.5px) | 500 |

- Digits that line up use `tabular-nums`.
- **Inputs are at least 16px,** or iOS zooms the page on focus.
- Sentence case everywhere. No all-caps labels.

## 5. Components

| Component | Look | Used for |
|---|---|---|
| Segmented control | Pill track in `blush`; the chosen segment is a `accent` pill with an ink outline | Dates/Cycle, range presets, °C/°F |
| Option buttons | 48px tall, radius 14, `rule` border; chosen = `accent` fill, ink border, bold | Single choices in the form (bleeding, secretion, sensation…) |
| Chips | 40px, fully rounded, same chosen state | Multiple choices (disturbances, symptoms) |
| Primary button | 54px, radius 16, `accent` with an ink outline, weight 800 | One per screen: "Add today's entry", "Save entry", "Show day" |
| Secondary button | 48px, radius 14, ink outline on `surface` | "Close", "Log out", the "no" side of a question |
| Switch | 50×30, `accent` with ink outline when on | "Cycle starts on this day", "Show raw readings" |
| Date box | Looks like a field and shows the date as dd/mm/yyyy; the native date picker sits invisibly on top of it (§9) | Every date input |
| Text field | 48px, radius 12, `rule` border, with its label above in bold. Focus is a `berry` border with a `blush` ring. An error shows under it in `danger`, bold. *Added in the design foundation (12 November 2026), from the mock's inputs: the table had no text input* | Email, password, invite code |
| Bottom sheet | Radius 26 at the top, a handle, over a dimmed backdrop | The day popup |
| Toast | Ink pill, 2.2 s | "Entry saved", "Entry deleted" |

*As built in the design foundation* (`frontend/src/components/`):
- **Choices are native inputs.** The segmented control and the option
  buttons are radio buttons, the chips are checkboxes and the switch is a
  checkbox with `role="switch"`, each hidden inside the label that is drawn.
  The browser supplies the keyboard behaviour (one tab stop, arrow keys) and
  the screen-reader semantics. The mock's `aria-pressed` buttons would have
  needed script for both.
- **Small controls keep their look and get a 44px tap area** (§8). A segment
  is 36px tall and a chip 40px, as in the mock; an invisible pseudo-element
  extends what a finger can hit. The switch (30px) is extended the same way,
  and its label flips it too.
- **Buttons have two sizes:** 54px for a primary button alone, 48px for a
  secondary button and for a primary button beside one (the question).
- **`/design`** shows every component with made-up values, for checking on a
  phone. It is not linked from the app and goes in the v1 review pass.

Questions that need an answer (save without a temperature, unusual
temperature, "Did bleeding stop?", delete) **replace the Save button in
place** with the question and two buttons, never a browser dialog.

## 6. Screens

### 6.1 Home: the graph

```
● Madam                         Cycles  Settings
Sunday 08/11/2026
Cycle 4, day 8. Nothing logged yet today.
┌──────────── chart paper band (sheet) ─────────────┐
│ [Dates|Cycle]                               (ⓘ)   │
│ [7][14][30][60][90][Custom]                       │
│  ‹   26/10/2026 to 08/11/2026  ›                  │
│      14 days                                      │
│ °C ┌──────────────────────────────────────────┐   │
│36.9│ pink grid, berry line, HotPink dots      │   │
│36.5│ rings for disturbed, gaps for missing    │   │
│    ├──────────────────────────────────────────┤   │
│Date│ 26 27 28 … 8                             │   │
│ Day│ 23 24 … (1) 2 …                          │   │
│Bleeding  ■ ■ ■ ·                              │   │
│Secretion S S       S M                        │   │
│    └──────────────────────────────────────────┘   │
└───────────────────────────────────────────────────┘
[ Add today's entry ]   ← sticky at the bottom
```

**The chart:**
- **Paper:** horizontal lines every 0.05 °C, heavier every 0.10 °C, which is
  where the Y labels are ("36.5"). A vertical line between days. The range
  is at least 36.20–37.00 and grows to fit the visible readings.
- **Rows under the plot**, in the same day columns:
  - **Date:** day of the month.
  - **Day:** cycle day. Day 1 sits in a `accent` circle, and it's "?" inside
    an unknown-start cycle and blank outside any cycle.
  - **Bleeding:** a `berry` square for bleeding, a small dot for spotting.
  - **Secretion:** a cell in `accent` (clear, stretchy, "C"), `accent` at 42%
    (milky, "M") or `grid-strong` (sticky, "S").
- **Marks:**
  - a reading is a `accent` dot with a 1.75px `berry` outline;
  - a disturbed reading is a ring, the line skips it, and its neighbours
    connect;
  - a raw reading (only with "Show raw readings" on) is a small faded
    `berry` dot, joined to its adjusted dot by a dotted line;
  - the line is `berry`, 2px, and a day without a temperature ends it.
- **A cycle start:** a dashed ink line at the left edge of day 1, through
  the plot and the rows, with a `accent` marker on top.
- **Today's column** is shaded `blush`. **The selected day** (popup open) is
  shaded HotPink at 20%, and its dot grows by 2px.
- **Density,** by column width: from 17px every label and every letter
  shows and dots are 5.5px. Narrower, labels thin to every k-th day counting
  back from the last, letters disappear, and dots are 4px (from 9px) or
  2.6px.
- **Legend,** behind ⓘ: every mark above with a drawn sample, then the
  "Show raw readings" switch. The ghost sample shows only while ghosts do.

**Controls:**
- **Dates view:** presets 7, 14, 30, 60, 90 and **Custom**, then **‹ ›**
  around the visible range ("26/10/2026 to 08/11/2026", "14 days").
  - The arrows page by the **whole range**. › is disabled at today. ‹
    stops at the first entry: the window clamps so it never starts before
    it.
  - Choosing a preset keeps the current end date, clamped the same way.
  - **Custom** shows From and To date boxes and "Up to 90 days." The
    pickers' limits block an end before the start, more than 90 days, a
    future date and a start before the first entry. The arrows page a
    custom range by its own length.
- **Cycle view:** ‹ › around "Cycle 4" and its dates ("01/11/2026 to
  today"). The unknown-start cycle is "Cycle ? (start unknown)".

**Taps:** a column is the tap target, full height. **At 17px or wider it
opens exactly that day,** including empty days. Narrower, a column without
an entry opens the nearest day with a reading, within about one column per
14 days visible. Each column is focusable and opens on Enter or Space.

### 6.2 The day popup

- **One fixed height on every day:** `min(88%, max(56%, 440px))` of the
  screen, so the arrows and buttons never move while stepping.
- **Locked top:** the handle; ‹ › around the weekday and date, with the cycle
  text under it; "Cycle 4 starts on this day" with the cycle-start mark, on
  start days; the temperature and, when adjusted, "Estimate. Adjusted from
  36.37 °C to your usual time, 07:00." That temperature area keeps its
  height on empty days too.
- **Scrolling middle:** Measured, At, Timing (a "45 min late" flag only when
  rounding changed the value), Bleeding, Secretion, the disturbance tags
  with "Disturbed, so the line skips this reading.", then **Notes**, with
  line breaks kept. Scroll shadows appear only when there's more to scroll.
- **Locked bottom:** "Show day" (or **"Add entry"** on a day with no entry)
  and "Close".
- **Stepping:** ‹ › move one **calendar day**, empty days included ("Nothing
  logged on this day." or "No temperature on this day."). ‹ stops at the
  first entry and › at today. Swiping sideways does the same on a phone,
  and ← → on a keyboard.
- **Stepping past the edge of the graph pages the graph,** with the same
  paging as the arrows. In Cycle view it moves to the cycle holding that
  day.

### 6.3 Day entry

Order, top to bottom. The things saved most often come first:

1. **Date** box (weekday and dd/mm/yyyy).
2. **Temperature card** (`sheet`):
   - the temperature, 2.5rem, °C beside it;
   - "Measured at" and the time on one row;
   - **a live line** saying what the reading will count as: "Measured 45 min
     late, so it counts as **36.30 °C**. That's an estimate." Or "Too little
     to change the value." when rounding cancels it, or "Measured at your
     usual time, so no adjustment."
3. **Cycle starts on this day** (switch). It sits right above Bleeding
   because the suggestion lives on it (below).
4. **Bleeding:** None, Spotting, Bleeding. One is always chosen; None by
   default.
5. **Cervical secretion:** Clear, stretchy · Milky · Sticky. Tapping the
   chosen one again clears it.
6. **Disturbances** (chips), with "A disturbed reading shows as a ring, and
   the line skips it."
7. **Notes.**
8. **Show all fields** (sensation, secretion amount and description, cervix,
   symptoms, intercourse).
9. **Delete entry** (existing entries only), then the sticky **Save entry**.

**The cycle-start switch and its message.** One control holds the answer.
An earlier mock had a separate pink "Start new cycle" button, and it looked
already chosen (Pe'er). When there's something to say, the switch's box is
framed in `berry` on `blush` with one line above the switch:

| Switch | Day already a cycle start? | Bleeding chosen, previous entry not bleeding? | Message |
|---|---|---|---|
| On | No | Either | "Cycle 5 will start on 08/11/2026 when you save." |
| Off | Yes | Either | "01/11/2026 will no longer start a cycle when you save." |
| Off | No | Yes | "You logged bleeding. If your period started on this day, turn this on." |
| any other case | | | No message, no frame |

The third row is the spec's new-cycle suggestion (`spec-v1.md` §8.2): turning
the switch on confirms it, and leaving it off dismisses it.

### 6.4 Cycles and stats

- Title "Cycle 4, day 8", then "Started Sunday 01/11/2026."
- **Cycle length:** one row of four, Average · Shortest · Longest · Latest,
  with hairlines between them, numbers at 1.625rem/800. Under the row, once:
  "In days, based on 3 completed cycles. The current cycle counts once it
  ends." At 360px or narrower the row becomes 2×2.
- **Bar chart:** one bar per cycle with a known start; gridlines at 7, 14,
  21, 28 and 35 days. The value sits above each bar, and the start date
  below it. The ongoing cycle is a faded, dashed bar labelled "ongoing".
- **List,** newest first: "Cycle 3", "04/10/2026 to 31/10/2026", "28 days".
  The ongoing cycle shows "N days so far". The unknown one shows "Start
  unknown, to 07/08/2026" and "Length unknown". Tapping a row opens that
  cycle in Cycle view.

### 6.5 Settings

Three groups: Units (°C/°F), Measurement time (the current time large, the
history, "Change measurement time"), Account ("Log out", and "Delete
account" in `danger` with "Deleting your account erases every entry and
cycle. It can't be undone.").

## 7. Dates and words

- **Dates show as dd/mm/yyyy,** as in Israel (Pe'er, 8 November 2026). With
  room, the weekday in words comes first: "Sunday 08/11/2026". The graph's
  date row shows the day of the month only.
- **Times are 24-hour,** "07:05".
- **Temperatures always have two decimals and a unit,** "36.55 °C".
- **A control's name stays the same through the flow:** "Save entry" gives
  "Entry saved"; "Delete entry" gives "Entry deleted".
- **Questions offer specific answers:** "Save without" / "Add a
  temperature", "Yes, save it" / "No, fix it", "Yes, it stopped" / "No".
- **Estimates say so:** every adjusted value is labelled an estimate
  (`project-brief.md`, principle 3).

## 8. Motion and access

- **Motion only answers a person's action:** the popup rises, the
  cycle-start frame appears, the "Show all fields" chevron turns. Nothing
  moves on its own. `prefers-reduced-motion` turns all of it off.
- **Focus is visible:** a 3px `berry` outline.
- **Targets are at least 44px,** except the graph's columns, which are full
  height but as wide as a day.
- **Every graph column has a label,** for example "07/11/2026: 36.32 °C".
  The popup is a labelled dialog, and Escape closes it.

## 9. Notes for the slices

- **Date boxes.** A phone's native date picker shows dates in the phone's
  own format, and the page can't change that. So the box shows its own
  dd/mm/yyyy text, with the real `<input type="date">` stretched over it at
  opacity 0. A tap opens the native picker, and `min`/`max` still block bad
  dates. On a computer, call `showPicker()` on click.
- **The rows under the graph with Recharts.** *Unverified:* the mock draws
  its own SVG. In Recharts they could be a custom X-axis tick with four
  lines, or an HTML grid below the chart that uses the plot's exact left
  and right margins. The spike didn't try either; Slice 6 measures it, with
  screenshots at 14 and 90 days.
- **The popup layout** is a flex column with a fixed height. The middle has
  `flex: 1; min-height: 0; overflow-y: auto`; without `min-height: 0` it
  grows instead of scrolling. The scroll shadows are four layered
  backgrounds (`local` covers over `scroll` shadows), with no script.
- **Swipe:** the sheet has `touch-action: pan-y`, so a sideways drag stays
  with the page while vertical scrolling still works.
- **Notes are text,** never markup. React escapes them by default; never
  use `dangerouslySetInnerHTML` for them.
- **The font files:** an npm package, imported in `main.tsx` (§10).

## 10. Open

- ~~**Where the font files come from.**~~ **Decided in the design foundation
  (12 November 2026):** the npm package
  `@fontsource-variable/atkinson-hyperlegible-next` 5.3.0, pinned exactly.
  It is SIL Open Font Licence 1.1, has no dependencies, and holds only CSS
  and font files. One **variable** file covers every weight from 200 to 800
  (the design uses 400, 500, 700 and 800). Measured:
  - the Latin file is **34.0 KB**, and its declared range covers "°", "‹ ›",
    "…" and "−";
  - a Latin-extended file (19.1 KB) ships too, but a browser fetches it only
    for a character in its range;
  - the server sends both as `font/woff2`, immutable and not gzipped, and the
    CSP needed no change.

  Only the upright style is imported; there is no italic. In CSS the family
  is "Atkinson Hyperlegible Next Variable". Rejected: committing the `.woff2`
  (a binary in Git whose origin is only a note; the lockfile's checksum is
  better evidence).
- **The rows under the graph in Recharts** (§9). Slice 6.
- **On a computer,** the spec puts the legend permanently in the top-right
  corner. The mock is phone-only, so the wide layout is still undrawn.
- **Screens not drawn:** sign-up, onboarding, and the measurement-time
  change flow. They use the components above. Slices 2, 3 and 9 sketch them
  in their plans.
- **The first user's readability check** on her phone at 07:00, before
  Slice 6 (`build-order.md` §6). Pe'er approves the design; her check is
  about readability, not taste.
