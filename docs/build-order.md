# Madam — Build Order

_Status: written at step 4. This is the plan for **what gets built, in what
order, and how you know each piece is done.** It is not a schedule — there are no
dates, because there is no deadline._

_Amended during setup: the repository is public from day one and already exists
before Phase 0 starts (§2, 0a), so Slice 10 is a final review pass rather than
the moment the repository goes public._

_Amended during Google Cloud setup (Oct 2026): 0b now deploys to `europe-west1`
(Belgium) and includes the cost guardrails — alert, spend cap, instance limits and
the Artifact Registry cleanup policy (`tech-stack.md` §7)._

_Amended at the end of setup (Oct 2026): the accounts and cloud resources now
exist. Both Supabase projects (Frankfurt) and both budgets were created during
setup, so 0b and 0c start from there. 0a gains four decisions surfaced while
installing the tools._

`docs/spec-v1.md` says what the app does. `docs/fam-rules.md` says how each rule
computes. This file says what to build next.

---

## 1. How work is shaped

**One slice, one branch, one pull request.** Each slice below is a single unit of
work with its own acceptance criteria. If a slice grows while you're in it, split
it rather than letting the diff get too big to review.

**Every slice is vertical where it can be.** Backend and frontend together, so
each merge leaves the deployed app working and slightly more useful than before.
The exceptions are Phase 0 (infrastructure, nothing to show) and Slice 1 (pure
domain logic, nothing to show yet) — both are deliberate.

**Plan before code.** Claude Code proposes an approach; Pe'er approves it; only
then does code get written. A wrong plan costs thirty seconds. Wrong code costs an
hour.

**Tests before implementation** for anything in `internal/fam`. The test vectors
in `docs/fam-rules.md` §12 already exist — they translate almost line for line.

**Definition of done, every slice:** CI green (build, test, lint), acceptance
criteria demonstrably met, merged to `main` by squashed PR, and deployed. `main`
is always deployable.

---

## 2. Phase 0 — the walking skeleton

The goal is **proof that the pipe connects end to end**, deployed to a real URL,
before a single feature exists. Deployment problems are a different species from
coding problems and they arrive in a swarm; meeting them one at a time against an
empty codebase means each one is the only suspect.

**The discipline is "thinnest possible."** `/healthz` returning `ok`. One table.
One query. Not a logging framework, not a metrics stack, not an abstraction for a
second database that will never exist. If a step starts growing, it has stopped
being a skeleton.

**Already in place from setup:** the tools in WSL (`tech-stack.md` §10), the
public repository with `main` protected, squash-merge only, secret scanning and
push protection on, `.claude/settings.json` denying reads of `.env`; both
Supabase projects; the Google Cloud project with the Cloud Run and Artifact
Registry APIs enabled, `gcloud` logged in, and both budgets created.

### 0a — Local skeleton

Go module, chi router, `GET /healthz` returning `ok`. `Makefile` with `dev`,
`test`, `lint`, `build`. GitHub Actions running build + test + lint on push.

Decisions to make in this step's plan, not by default:

- **The `go` line in `go.mod`.** Pin it to **1.27**, deliberately. Since Go 1.21
  the toolchain downloads a newer version automatically when `go.mod` asks for
  one, so this line is a real decision, not boilerplate.
- **Pin the dev tools.** `goose`, `sqlc` and `golangci-lint` were installed
  globally with `@latest` during setup, so the laptop and CI could drift apart.
  Go 1.24+ supports `tool` directives in `go.mod`, run as `go tool <name>`, which
  pins one version for everyone. Decide whether to use them.
- **Local Postgres.** Use the `postgres:17` image, matching Supabase's major
  version. The Makefile must **wait for readiness with `pg_isready`**, never a
  fixed `sleep` — a started container is not yet an accepting database.
- **Environment variables.** Name them here and add `.env.example` (committed,
  no real values). There are two development databases — the local container and
  the `madam-dev` Supabase project — so decide how each is selected.

Also decide whether to turn on **Dependabot** now that there will be dependencies.

Once CI exists, add its check to `main`'s protection as a **required status
check**.

*Done when:* `make test` passes locally and CI is green on a pull request.

### 0b — Deployed

- Dockerfile. The image is **built by us** (locally or in GitHub Actions) and
  pushed to Artifact Registry — not `gcloud run deploy --source`, which goes
  through Cloud Build and is billed separately.
- An Artifact Registry repository in `europe-west1` with a **cleanup policy that
  keeps only the last few images**. Free storage is 0.5 GB, and every deploy adds
  an image.
- `gcloud run deploy` to **`europe-west1` (Belgium)**, request-based billing (the
  default), **min instances 0, max instances 2**.
- **Cost guardrails: already created during setup.** An alerts-only budget of
  ₪15 on the billing account, and a spend cap budget of ₪15 on Cloud Run in the
  project, status **Configured** (`tech-stack.md` §7). Nothing to create here;
  just don't remove them.

*Done when:* a public HTTPS URL returns `ok`, and the spend cap still shows
**Configured**.
*Verification items:* measure the cold-start time and write the number into
`CLAUDE.md`. After the first full month, check the billing report.

### 0c — Database connected

**The two Supabase projects already exist** (`madam-dev` and `madam-prod`,
Frankfurt, Data API off, automatic RLS off — `tech-stack.md` §5). Connection
strings and passwords are in Pe'er's password manager, not in the repository.

goose migration `00001` creating the `users` table only. `sqlc.yaml`, one
generated query, called from the health endpoint.

**Decide how Cloud Run connects** (`tech-stack.md` §5, Connecting): the session
pooler, the transaction pooler with prepared statements turned off in pgx, or the
direct connection if Cloud Run's outbound traffic has IPv6. The laptop has no
IPv6, so it always uses the session pooler.

**Decide how migrations reach production** — from the laptop, or from CI. The
production URL must never sit in a local `.env`.

*Done when:* the deployed URL reads a row from Supabase.
*Verification items:* pgx/sqlc mappings for `INTEGER`, `DATE`, `TIME`, `TEXT[]`;
who owns tables created by goose on Supabase; query latency between Belgium and
Frankfurt.

### 0d — Isolation plumbing

The non-owner application role, `FORCE ROW LEVEL SECURITY`, one policy on `users`
with both `USING` and `WITH CHECK`, and the chi transaction wrapper that sets the
per-request user id.

**This step needs an identity, and auth doesn't exist yet.** Use a **dev-only stub
middleware** returning a hardcoded UUID, behind the same interface the real auth
middleware will implement. Swapping it in Slice 2 is then a contained change,
because everything downstream only asks "who is this request for?"

*Done when:* test case I3 from `fam-rules.md` §12.7 passes — a query run as the
application role with the wrong user id set returns **zero rows**. Until that test
passes, RLS is not proven to be doing anything.

### 0e — Frontend shell

Vite + React + TypeScript + Tailwind. A page that fetches `/healthz` and displays
the result. `go:embed` in `backend/internal/web`, and the Makefile step that
copies `frontend/dist/` into `backend/internal/web/assets/` before `go build`.

**`go:embed` cannot reach outside its own directory** — no `../`. The copy step is
not optional, and the copied assets are build output: gitignored, never committed.

*Done when:* the deployed binary serves the React page at the same URL as the API.

### 0f — Keepalive

A GitHub Actions workflow pinging **both** Supabase projects every few days. The
free plan pauses a project after a week of inactivity, and a paused project is a
broken app on the morning she tries to log a temperature.

*Done when:* the workflow has run green on schedule at least once.

---

## 3. Spikes

Short, throwaway investigations. Not slices, not merged as features — the output
is an answer written into `CLAUDE.md` or the relevant document.

- **Invite code hook** *(do this early, before Slice 2 is planned)*. Does the
  user-typed code actually reach Supabase's "before user created" hook? An hour of
  poking. If the answer is no, Slice 2 needs a different design, and it is much
  cheaper to learn that now.
- **`-infinity` round-tripping** through pgx into Go. Expected to work via
  `pgtype.Date`'s infinity modifier. Fallback: a sentinel date such as
  `0001-01-01`. Needed before `measurement_time_periods` is built in Slice 3.
- **The graph** *(after Phase 0, before Slice 1)*. Recharts with fake data:
  hollow rings, ghost dots, a `ReferenceLine`, a custom X-axis tick, tappable
  dots, and — the actual unknown — the line drawn as **separate segments** so a
  disturbed day is stepped over while a missing day breaks the line. This is the
  only known sharp edge in the project. Meet it while the codebase is small, when
  swapping to visx would still be cheap.

---

## 4. The slices

### Slice 1 — Domain logic (`internal/fam`)

Pure Go. No HTTP, no SQL, no network. Temperature adjustment, the adjusted/raw
comparison, Fahrenheit conversion both ways, cycle day, cycle length, cycle
numbering, the suggestion and bleeding-stop conditions, and the statistics.

Tests come from `fam-rules.md` §12 and are written **first**: T1–T6, F1–F2,
C1–C10, S1–S5, B1–B6, A1–A7.

*Acceptance criteria:* AC-1, AC-2, AC-3, and the arithmetic underneath AC-5 to
AC-8 and AC-12.

*Why first:* zero dependencies means a feedback loop measured in milliseconds, and
it's the one slice in the project that offers that. Every later slice gets easier
because this one is solid. Nothing is user-visible at the end of it, which is the
accepted cost.

### Slice 2 — Auth

Supabase Auth wiring, JWT verification middleware replacing the 0d stub, the
invite code on sign-up, email verification, password reset, log out.

*Acceptance criteria:* AC-16 part one (no API request from user A reaches user B's
data). Part two already passed in 0d.

### Slice 3 — Onboarding

Current cycle start or "I don't know", fixed measurement time written as the
`-infinity` row in `measurement_time_periods`, `onboarded_at` set.

*Acceptance criteria:* the setup half of AC-12.

### Slice 4 — Daily entry

The full day screen: quick section, "show all fields", save, edit, delete. All
validation — no future dates, the 35–39 °C question, the 20–50 °C rejection, the
no-temperature confirmation, one entry per date.

*Acceptance criteria:* AC-4, AC-13, AC-14, AC-17.

### Slice 5 — Cycle rules

The new-cycle suggestion, the manual "cycle starts on this day" toggle, and the
"Did bleeding stop?" check. Both lookbacks use **the most recent earlier entry,
whatever its date** (`fam-rules.md` §6.3 explains why they must match).

Saving a day may now be two writes — `entries` and `cycle_starts` — in one
transaction.

*Acceptance criteria:* AC-5, AC-6, AC-7, AC-8.

### Slice 6 — The graph, on real data

Take the spike's result and connect it to the API. Date view, presets, custom
range, ghost toggle, legend, day popup, "Show day".

*Acceptance criteria:* AC-10, AC-11, AC-15, plus the visible halves of AC-3 and
AC-13.

### Slice 7 — Cycle view

The second view on the graph: one cycle at a time, with previous/next navigation.
Cycle day on the X axis, `?` inside an unknown-start cycle, blank outside any
cycle.

### Slice 8 — Cycles and stats

The stats screen: current cycle, the four statistics with "based on N cycles", the
bar chart with the ongoing cycle faded, and the cycle list.

*Acceptance criteria:* AC-12 in full.

### Slice 9 — Settings and account

Units, the measurement-time change flow with its guardrails and history, log out,
and account deletion (our data cascades, then the Supabase identity is deleted).

*Acceptance criteria:* AC-9.

### Slice 10 — v1 review pass

The repository has been public since day one (`tech-stack.md` §12), so this is a
final sweep, not the moment it goes public.

Scan the Git history for secrets. Confirm every `.sql` query filters by `user_id`.
Confirm every table has an RLS policy and `FORCE` set. Write the portfolio README
with screenshots and the live URL. Add the `LICENSE` (MIT). Then tag `v1.0.0`.

---

## 5. Post-v1, in rough order of value

- **Nightly `pg_dump`** to encrypted storage. Free, and turns "lose the database,
  lose everything" into "lose a day". The single highest-value thing on this list.
- The legal and privacy check, **before** sign-up is opened to anyone beyond the
  invite code.
- Paper-chart-style export (v2).
- The WhatsApp bot (v3) — another client of the same API.
- Everything in `spec-v1.md` §14.

---

## 6. Open items this plan depends on

Two need the first user, not us:

- **The direction of the correction** — early adds, late subtracts. Confirm before
  Slice 6 ships, because a sign error would be visibly wrong to her.
- **The palette on her phone**, early in the morning. Before Slice 6.

The rest are in `CLAUDE.md`'s verification list and are attached to the steps
above where they get answered.
