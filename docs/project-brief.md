# Madam — Project Brief

_Living document. Update at the end of each planning step._

## The name

**Madam** means "a lady," and in Hebrew it sounds like מד-דם, "blood meter."

## The idea

A mobile-friendly web app that digitizes a paper FAM (Fertility Awareness Method) chart based on BBT (Basal Body Temperature). The first real user currently charts by hand on a printed paper table. The printed form is kept with the planning material, not in this repository.

## Workflow

- Planning (steps 1–4) happens in claude.ai chat, inside this Project, with **one chat per step**.
- Each step ends by updating this brief and/or producing a dedicated doc, which is added to project files.
- Building happens in **Claude Code**, run inside WSL on the Linux filesystem. Claude writes most of the code, and Pe'er reviews every diff and asks "why" about anything unclear.
- **Once the repo exists it is canonical.** The Project keeps a copy of each document, refreshed whenever one changes materially, so planning chats aren't run against a stale spec.
- The build starts with a **walking skeleton**: the thinnest end-to-end slice, deployed to Cloud Run before any features exist. Features ship to the real URL as they are finished.
- Work is done in **small slices**: one feature per branch, per pull request, each mapped to acceptance criteria that already exist. Claude Code proposes a plan before writing code; Pe'er approves the plan, then reviews the diff.
- Two environments only: local (with a separate Supabase dev project) and production. **Never test against production data.**
- CLAUDE.md stays short: stack, commands, conventions, the known traps, and how to work with Pe'er. Detailed FAM logic lives in `docs/fam-rules.md`, which CLAUDE.md references.

## Key documents

- `spec-v1.md`: the full v1 spec (screens, rules, acceptance criteria, non-goals, future ideas). **Source of truth for what the app does**; this brief only summarizes.
- `fam-rules.md`: the exact domain arithmetic and conditions, with the test vectors the unit tests are written from. **Source of truth for how each rule computes.** Where code and this file disagree, the file wins.
- `tech-stack.md`: every stack decision with its reasoning, the rejected options, and the accepted trade-offs. **Source of truth for stack questions.**
- `data-model.md`: the tables, columns, constraints, derived-value formulas, and the reasoning behind each choice. **Source of truth for schema questions.**
- `build-order.md`: what gets built, in what order, and how each slice is known to be done.
- `CLAUDE.md`: the short operating file Claude Code reads every session — where the documents are, the conventions, the non-obvious traps, and the verification list.

## Decisions made

- **Platform:** mobile-friendly web app, no install required.
- **Language:** English UI.
- **Accounts:** built public-ready (multi-user, strict per-user isolation), with sign-up gated behind one shared invite code at launch. Account deletion wipes all user data.
- **Units:** Celsius by default, with Fahrenheit selectable in settings. The database stores one canonical unit (Celsius) and converts on display and input.
- **Interpretation level: B.** Record, chart, and compute cycle-length statistics (latest, shortest, longest, average; all time). No fertile/infertile verdicts. Option C (rule-based temperature-shift detection) may be added later.
- **Principle 1, raw vs derived:**
  - Store the measured temperature and the actual measurement time; compute the adjusted temperature.
  - Cycle days, late/early status, and cycle lengths are also derived, not stored.
  - The correction is fixed in v1 at 0.05 °C per 30 min (early adds, late subtracts), displayed to 0.01 and rounded half-up. A user-configurable rate is a future idea.
  - The fixed measurement time has a dated history, so changing it never rewrites past values.
- **Principle 2, security:** the data is highly sensitive. Real auth using an established provider (never hand-rolled password handling), HTTPS, and strict per-user data isolation — enforced twice, by a `user_id` filter on every query and by forced Row Level Security.
- **Principle 3, honesty in the UI:** anything estimated is clearly labeled as an estimate, with no guarantees.
- **Method fidelity (step 4):** Madam implements the first user's **own paper method**, not a clinical standard. The 0.05 °C / 30 min rate is hers; it is not traced to a published source, and `fam-rules.md` §1 says so openly rather than implying authority the app doesn't have. The project's primary goals are the process and the learning, and an app that disagreed with her existing history would be worse, not better.
- **Data model (step 3, see `data-model.md`):** four tables — `users`, `entries`, `cycle_starts`, `measurement_time_periods` — with natural composite keys `(user_id, date)` and no surrogate IDs. Raw temperatures are integers in thousandths of a degree; the adjusted value is computed with integer arithmetic and rounded half-up, once. Cycle starts stand alone rather than being a flag on an entry, so a cycle can start on a day with no entry. Measurement-time periods store a start date only, so overlap is inexpressible. Closed-value fields are Postgres enums; disturbance tags and symptoms are array columns. Deletion is hard, via `ON DELETE CASCADE`.
- **Tech stack (step 2, see `tech-stack.md`):** Go + chi backend, Supabase Auth, Supabase PostgreSQL in the EU (free plan), sqlc + goose, React + Tailwind frontend embedded in the Go binary, deployed as a container on Google Cloud Run. One repo, **public from day one** (changed during setup, see below). All FAM logic lives in the backend.
- **Step 4 decisions:**
  - **Charting: Recharts.** The graph is built early, right after the skeleton, with fake data. The line is drawn as separate segments, because a single `connectNulls` setting can't express both "skip a disturbed reading" and "break at a missing day."
  - **Row Level Security: in from day one**, with a non-owner database role, `FORCE ROW LEVEL SECURITY`, a per-request transaction wrapper, and a test that proves the policies actually apply. Additive only — every query still filters by `user_id`.
  - **`updated_at` set by a database trigger**, not application code.
  - **Impossible temperatures (outside 20–50 °C) rejected by the API** with a readable message; the database `CHECK` is a backstop the user never meets.
  - **Supabase free plan**, so the keepalive job is load-bearing and a nightly `pg_dump` is the post-v1 backup plan.
  - **Palette: HotPink `#FF69B4` as an accent**, used as a fill with dark text on it. It fails contrast requirements as text or as the chart line on a light background.
  - **"Did bleeding stop?" now uses the same lookback as the new-cycle suggestion** — the most recent earlier entry, whatever its date. The narrower version left a hole that produced a false new-cycle suggestion mid-period.
  - **The "shortest − 21 (20)" value is dropped permanently**, not deferred.
  - **Figma handoff dropped from planning.** The designer has no paid plan and doesn't need one; the redesign is post-v1 touch-ups. Design tokens stay, because they make a later reskin cheap regardless.
- **Setup decisions (step 4, second half):**
  - **The repository is public from day one**, reversing "private until v1." On GitHub's free plan, protected branches exist only on public repositories. The secret risk that motivated "private" is handled by `.gitignore` from the first commit and GitHub's push protection (`tech-stack.md` §12).
  - **The documents refer to "the first user,"** never by name or relationship, because the repository and its full history are public.
  - **Git talks to GitHub over HTTPS**, with `gh` as the credential helper. Commits are signed with an SSH key, so GitHub shows them as verified.

## v1 in one paragraph

Onboarding asks only for the current cycle start (or "I don't know") and the fixed measurement time. Daily entry has a quick section (date, time, temperature, bleeding, cervical secretion category, disturbance tags, notes) with "show all fields" for the rest of the paper chart, one entry per date. The home graph defaults to the last 14 days (with presets, a custom range up to 90 days, and a per-cycle view), shows date and cycle day on the X axis, uses hollow rings for disturbed readings, and hides raw "ghost" dots by default. New cycles are suggested when bleeding is logged and confirmed by the user, or set manually. A separate cycles screen shows stats, a cycle-length bar chart, and the cycle list.

## Dropped permanently

- The **"shortest − 21 (20)"** value from the paper chart. It appears on the printed table but was never used in practice.

## Deferred (not in v1)

- Thermometer type, cycle intent, and medical context.
- Configurable quick-entry fields and correction rate.
- Time zone handling.
- Importing paper history.
- Reminders, partner sharing, and offline mode.

See `spec-v1.md` sections 13–14.

## Phasing

- **v1:** everything in `spec-v1.md`.
- **v2:** export resembling the paper chart, plus a selection of future ideas.
- **v3:** WhatsApp bot. The backend is an API from day one, so the bot is just another client.

## Open items

Almost everything from steps 1–3 is now closed. What's left falls into three kinds.

**Needs the first user, not us:**

- Confirm the **direction of the correction** — early adds, late subtracts. Currently an assumption; a sign error would be visibly wrong to someone who has charted by hand for months.
- Confirm the palette is readable on her phone, early in the morning.

**Answerable only by measuring** (the verification list in `CLAUDE.md`):

- pgx/sqlc type mappings, and `-infinity` round-tripping through pgx.
- Who owns tables created by goose migrations on Supabase, and how to create a non-owner application role.
- Whether the invite code reaches Supabase's "before user created" hook.
- Cloud Run cold-start time.

**Genuinely still open:**

- **Outgoing SSH is reset from the development laptop**, on every network tested and from both WSL and Windows; the cause is not yet found. Git works around it over HTTPS. It matters again if Supabase's direct Postgres connection is affected the same way, which gets tested during Supabase setup.
- The Supabase restore window (docs say a year, older sources say 90 days).
- **Legal and privacy check before opening sign-up publicly** — GDPR (health data is a special category) and Israel's Privacy Protection Law after Amendment 13. Attached to public sign-up, not to v1.

## Planning steps

1. **Spec:** v1 features and screens. ✅ Done (`spec-v1.md`).
2. **Tech stack:** options, then Pe'er chooses. ✅ Done (`tech-stack.md`).
3. **Data model:** tables and relationships. ✅ Done (`data-model.md`).
4. **Rules, CLAUDE.md, and repo setup.** Documents done (`fam-rules.md`, `CLAUDE.md`, `build-order.md`, all four docs amended). ← in progress: machine, repository and cloud setup.
5. **Build in Claude Code.**
