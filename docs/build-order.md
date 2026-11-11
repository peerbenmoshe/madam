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

_Amended in Phase 0b (Oct 2026): the health endpoint is `/api/health`, because
Cloud Run reserves some paths ending in `z`; a new step, 0c′, adds deploy on
merge after 0c._

_Amended in Phase 0c (Nov 2026): 0c is done; 0d gains what 0c measured about
Supabase's roles; the `-infinity` spike is answered._

_Amended in Phase 0c′ (Nov 2026): 0c′ is done, so every merge deploys; a
schema change is now two PRs (`tech-stack.md` §6), and 0d gains how its secret
change reaches production._

_Amended in Phase 0d (Nov 2026): 0d is done. The transaction wrapper is a
function handlers call, not chi middleware, and the stub identity is test-only
(both below)._

_Amended in Phase 0e (Nov 2026): 0e is done. The graph spike gains what 0e
set up for it, and 0f a note on finding its runs (both below)._

_Amended in Phase 0f (Nov 2026): 0f is done, so Phase 0 is complete. The
keepalive runs four times a day, not "every few days", because Supabase asks
for "a few user requests to the database each day" (below)._

_Amended after the graph spike (Nov 2026): the graph spike is done and
Recharts stays; Slice 6 gains what the spike learned (both below)._

_Amended in the design step (8 November 2026): the design is done
(`design-brief.md`), so a small **design foundation** step comes before
Slice 3, and Slices 4–8 gain what the design decided. Slice 6's open
question about how the popup moves between days is answered (all below)._

_Amended in Slice 1 (8 November 2026): Slice 1 is done. Slice 3 gains a
migration for the stored onboarding answer, `users.cycle_start_unknown`
(below)._

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

**The discipline is "thinnest possible."** `/api/health` returning `ok`. One table.
One query. Not a logging framework, not a metrics stack, not an abstraction for a
second database that will never exist. If a step starts growing, it has stopped
being a skeleton.

**Already in place from setup:** the tools in WSL (`tech-stack.md` §10), the
public repository with `main` protected, squash-merge only, secret scanning and
push protection on, `.claude/settings.json` denying reads of `.env`; both
Supabase projects; the Google Cloud project with the Cloud Run and Artifact
Registry APIs enabled, `gcloud` logged in, and both budgets created.

### 0a — Local skeleton

Go module, chi router, `GET /healthz` returning `ok` (renamed `/api/health` in 0b). `Makefile` with `dev`,
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

**✅ Done (21 October 2026, PR #5).** The decisions are recorded in
`tech-stack.md` §9, §10 and §12 and in `CLAUDE.md`, and the story is in
`build-log.md`. `ci` is a required status check on `main`, and Dependabot is on.

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

**✅ Done (22 October 2026, PR #8).** `/api/health` answers `ok` at the public
URL. The health endpoint moved there from `/healthz`, because Cloud Run reserves
some paths ending in `z`. The spend cap was confirmed **Configured** before the
first deploy. The settings are in `tech-stack.md` §7 (As built), the cold start
is in `CLAUDE.md`, and the story is in `build-log.md`. Still open: the first
month's billing report (November 2026).

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

**✅ Done (4 November 2026, PR #9).** `/api/health` at the public URL runs a
query on madam-prod. Cloud Run connects through the session pooler with
`verify-full` against Supabase's own CA. The URL comes from Secret Manager, and
migrations run from the laptop with `make migrate-prod`. The decisions are in
`tech-stack.md` §5–§7 and §12. The measurements are in `data-model.md` §9–§10
(type mappings, ownership, roles) and `tech-stack.md` §5 and §7 (latency, cold
start). The story is in `build-log.md`.

### 0c′ — Deploy on merge

_Added in 0b, which deploys from the laptop with `make deploy`._

A GitHub Actions workflow that runs on every push to `main`: build, push, and
`gcloud run deploy` with the same flags as `make deploy`, so `main` and the live
app can't drift apart.

It authenticates with **Workload Identity Federation**, never a JSON key: a
workload identity pool and provider whose condition accepts only this
repository (by numeric id, which survives renames) on `refs/heads/main`, and a
deployer service account with only Artifact Registry writer on the `madam`
repository, Cloud Run developer on the `madam` service, and permission to act
as `madam-run`.

It comes after 0c because it then also carries the database configuration, and
it stays a separate step because IAM problems are a different species from
deploy problems (§2). Mind the Actions flakiness noted in `build-log.md`: a run
that never starts is a deploy that never happens.

*Done when:* a merged PR is live without anyone running `make deploy`, and the
live revision's image tag is the merge commit.

**✅ Done (5 November 2026, PRs #11 and #12).** Three merges in a row (#11,
#10, #12) each went live in about 2 minutes, with the image tag equal to the
merge commit, deployed by `madam-deployer` according to the audit log. A probe
confirmed that the condition refuses a branch push and a PR. `make deploy`
stays as the fallback, and it and `make migrate-prod` refuse anything but the
latest `main`. `migrate-prod` also checks for `verify-full`. The setup is in
`tech-stack.md` §7 (As built in Phase 0c′), with §6 for migrations and §12 for
CI; the story is in `build-log.md`.

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

*Learned in 0c, to act on here* (`data-model.md` §9):
- Supabase's `postgres` role has **`BYPASSRLS`**, so the app role must be a new
  role created `NOBYPASSRLS`, not `postgres` with `FORCE`.
- Supabase's default privileges grant `anon`, `authenticated` and
  `service_role` full access to every new table in `public`. Revoke them, and
  change the default privileges.
- The app role's URL becomes a new version of `madam-database-url`, and the
  Makefile's pinned version moves with it. `madam-migrate-url` stays the owner.
- CI already has a migrated Postgres for the I3 test (`tech-stack.md` §12).

*Learned in 0c′, to act on here:*
- Every merge deploys. So the order is: the role's migration as its own PR;
  `make migrate-prod`; Pe'er adds the app role's URL as a new version of
  `madam-database-url`; then the PR that moves the Makefile's pin, whose merge
  deploys it. `madam-run`'s access is bound on the secret, so it already covers
  the new version, and the deployer needs nothing new.
- A role's password can't go in a migration: the repository is public.

**✅ Done (5 November 2026, PRs #14, #15 and #16).** Production connects as
`madam_app`, and I3 passes as that role locally, in CI and on madam-dev,
after it was seen failing under three wrong setups. Two decisions differ from
this section's wording, chosen by Pe'er:
- **The transaction wrapper is a function,** `Store.InUserTx`, which the
  handler calls and which commits before the response is written. It's not chi
  middleware, which would commit after the response may already have been sent.
- **The stub identity lives only in a test file.** 0d adds no user-data route,
  and real auth (Slice 2) arrives before the first one (Slice 3), so nothing
  at runtime needs a fake identity, and Go never compiles test files into a
  binary. Slice 2 implements the `Authenticator` interface and passes it in
  from `main.go`.

The decisions are in `data-model.md` §9 (As built in Phase 0d) and
`tech-stack.md` §1.4, §5 (Database roles) and §7; the story is in
`build-log.md`.

### 0e — Frontend shell

Vite + React + TypeScript + Tailwind. A page that fetches `/api/health` and displays
the result. `go:embed` in `backend/internal/web`, and the Makefile step that
copies `frontend/dist/` into `backend/internal/web/assets/` before `go build`.

**`go:embed` cannot reach outside its own directory** — no `../`. The copy step is
not optional, and the copied assets are build output: gitignored, never committed.

*Done when:* the deployed binary serves the React page at the same URL as the API.

**✅ Done (6 November 2026, PRs #18 and #19).** The React page and
`/api/health` are served from one binary at the public URL. A deep link
reloads correctly, on Pe'er's phone too. `deploy/smoke-test.sh` passed 14/14
there, and it runs against the image on every PR, so nothing under `/api/`
can reach the page unnoticed. Node 26.13.0 is pinned exactly in `.nvmrc` and
the Dockerfile. The image is 5.41 MB and still distroless. Two decisions are
worth knowing before later work: TypeScript is 6, because typescript-eslint
doesn't support 7 yet, and the `/api` subrouter has its own 404, because chi
would otherwise give it the page's fallback. The decisions are in
`tech-stack.md` §8.4 (with §7, §8.2, §10 and §12); the story is in
`build-log.md`.

### 0f — Keepalive

A GitHub Actions workflow pinging **both** Supabase projects every few days. The
free plan pauses a project after a week of inactivity, and a paused project is a
broken app on the morning she tries to log a temperature.

*Done when:* the workflow has run green on schedule at least once.

*Learned in 0e, to act on here:* Dependabot's runs share `main` with ours,
so find a workflow's runs with `gh run list --workflow <file>`. The deploy
condition refuses any workflow but `ci.yml` (`tech-stack.md` §7), so a
keepalive workflow can't deploy even by mistake.

**✅ Done (7 November 2026, PRs #21 and #22).**
`.github/workflows/keepalive.yml` runs at :41 past 02, 08, 14 and 20 UTC
and ran green on schedule the same day.
- **Production** is pinged through the public `/api/health`, with no
  credential in GitHub.
- **madam-dev** gets `psql` as `madam_app`, from one repository secret.
- **Measured:** one run adds one recorded query on madam-dev, and the
  workflow has no `id-token`, so it can't deploy.
- Four a day, not "every few days", because Supabase counts "user database
  activity" and asks for "a few user requests to the database each day".
- GitHub disables a public repository's schedules after 60 days without
  activity; during the build, merges prevent that.

The decisions are in `tech-stack.md` §5 (Mitigation), §9 and §12, and the
story is in `build-log.md`.

---

## 3. Spikes

Short, throwaway investigations. Not slices, not merged as features — the output
is an answer written into `CLAUDE.md` or the relevant document.

- **Invite code hook** *(do this early, before Slice 2 is planned)*. Does the
  user-typed code actually reach Supabase's "before user created" hook? An hour of
  poking. If the answer is no, Slice 2 needs a different design, and it is much
  cheaper to learn that now.

  **✅ Done (11 November 2026, branch `spike/invite-hook`, never merged).**
  **Yes:** the code arrives as `user.user_metadata.invite_code`, and a
  refusal stops the sign-up with no row left behind. Measured on madam-dev
  through a Cloudflare quick tunnel to the laptop, with a signed Supabase
  request verified in Go. The answer and the decision (an HTTP hook calling
  the Go API) are in `tech-stack.md` §4 (As built in the invite-code spike).
  The story is in `build-log.md`.
- ~~**`-infinity` round-tripping** through pgx into Go.~~ **Answered in 0c:** it
  round-trips as `pgtype.Date{InfinityModifier: pgtype.NegativeInfinity}`, so
  no sentinel date is needed (`data-model.md` §10).
- **The graph** *(after Phase 0, before Slice 1)*. Recharts with fake data:
  hollow rings, ghost dots, a `ReferenceLine`, a custom X-axis tick, tappable
  dots, and — the actual unknown — the line drawn as **separate segments** so a
  disturbed day is stepped over while a missing day breaks the line. This is the
  only known sharp edge in the project. Meet it while the codebase is small, when
  swapping to visx would still be cheap.
  *Set up by 0e:* Recharts is not installed yet. Add it (React 19 is in its
  peer range) as a dependency to approve. The CSP allows no inline styles,
  so check the browser console: if Recharts writes `style` attributes as
  markup, the CSP needs loosening (`tech-stack.md` §8.4). Colours come from
  the `@theme` tokens (§8.2), never Tailwind's palette, which is switched
  off.

  **✅ Done (8 November 2026, branch `spike/graph`, never merged).**
  **Recharts 3.10.1 stays.** It passed all six visx criteria fixed before
  looking:
  - the line as one `<Line connectNulls>` per run;
  - no CSP loosening, with 0 violations on three browsers;
  - edge cases E1–E18 drawn correctly;
  - taps at least 9/10 on Android and iPhone;
  - +106.7 KB gzipped;
  - tokens only.

  `react-is` 19.3.1 comes with it as a peer. The answer is in `tech-stack.md`
  §8.3 (As built in the graph spike), and the story is in `build-log.md`.
- **The design** *(after the graph spike, before Slice 3)*. Not in the
  original plan: design was "post-v1" (`tech-stack.md` §11). Decide the
  look, layout and interaction of every v1 screen before the first one is
  built.

  **✅ Done (8 November 2026, PR #25).** Pe'er approved a clickable phone
  mock after four rounds. The decisions are in `design-brief.md`, and the
  mock is `docs/design/mock.html`. The spec changes it brought are in
  `spec-v1.md` (its header lists them), and the story is in `build-log.md`.

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

**✅ Done (8 November 2026, PR #26).** `backend/internal/fam` passes every
§12 vector: T1–T6, F1–F3, C1–C14, M1–M5, S1–S5, B1–B6, A1–A7. F3, C11–C14
and M1–M5 were added to `fam-rules.md` first, in the same PR. A depguard rule
keeps the package on the standard library. Reading the rules first found two
ambiguities, which Pe'er settled:
- "adjusted?" is decided in °C, whatever the display unit;
- the "I don't know" onboarding answer is stored.

§3.5 also gained its exact °F arithmetic. The story is in `build-log.md`.

### Slice 2 — Auth

Supabase Auth wiring, JWT verification middleware replacing the 0d stub, the
invite code on sign-up, email verification, password reset, log out.

*Acceptance criteria:* AC-16 part one (no API request from user A reaches user B's
data). Part two already passed in 0d.

*From the invite-code spike* (`tech-stack.md` §4):
- **The hook always answers 200.** It sends `{}` to allow and
  `{"error":{"http_code":400,"message":…}}` to refuse. A 4xx from the hook
  reaches the user as a 500 with Supabase's message, not ours.
- **The signature is checked first:** Standard Webhooks, stdlib only. The
  spike's handler and its OpenSSL-computed test vector are on
  `spike/invite-hook`.
- **Two secrets,** `madam-invite-code` and `madam-auth-hook-secret`, pinned,
  with `madam-run` reading each. The production dashboard settings are listed
  in §4.
- **Tokens are ES256 with a `kid`,** so verification uses the project's JWKS.
  How (a library or not) is a dependency decision.
- **The code persists** in `user_metadata` and in every token. Decide whether
  to clear it. Never trust `user_metadata`: its own user can edit it.

### Design foundation *(before Slice 3)*

A small PR, so Slice 3 is about onboarding and not about restyling. It builds
the shared parts of `design-brief.md` that every screen uses:
- the eleven tokens in the `@theme` block (§3);
- the self-hosted font (§4). Decide first where the files come from: an npm
  package is a dependency to approve, or `.woff2` files are committed with
  their licence. Measure their size;
- the components in §5: segmented control, option buttons, chips, primary
  and secondary buttons, switch, date box (dd/mm/yyyy over a native picker,
  §9), and the in-place question that replaces Save;
- the app bar and the dd/mm/yyyy date formatting helper.

*Done when:* the shell page uses the new tokens and font, the CSP still
reports no violation, and the components render on Pe'er's phone.

### Slice 3 — Onboarding

Current cycle start or "I don't know", fixed measurement time written as the
`-infinity` row in `measurement_time_periods`, `onboarded_at` set.

*From Slice 1* (`fam-rules.md` §5.2): the answer itself is stored, in
`users.cycle_start_unknown` (`data-model.md` §4.1). That column is a
migration, so it is its own PR first, migrated to production before the code
that writes it (`tech-stack.md` §6).

*Acceptance criteria:* the setup half of AC-12.

### Slice 4 — Daily entry

The full day screen: quick section, "show all fields", save, edit, delete. All
validation — no future dates, the 35–39 °C question, the 20–50 °C rejection, the
no-temperature confirmation, one entry per date.

*Acceptance criteria:* AC-4, AC-13, AC-14, AC-17.

*From the design step* (`design-brief.md` §6.3): the field order puts the
"cycle starts on this day" switch directly under the temperature card. The
card shows live what a late or early reading will count as. The questions
replace the Save button in place, never a browser dialog.

### Slice 5 — Cycle rules

The new-cycle suggestion, the manual "cycle starts on this day" toggle, and the
"Did bleeding stop?" check. Both lookbacks use **the most recent earlier entry,
whatever its date** (`fam-rules.md` §6.3 explains why they must match).

Saving a day may now be two writes — `entries` and `cycle_starts` — in one
transaction.

*Acceptance criteria:* AC-5, AC-6, AC-7, AC-8.

*From the design step* (`design-brief.md` §6.3): the suggestion is the
switch itself, highlighted, not a separate button. The switch's message
follows its state: "Cycle N will start on …" when turned on, "… will no
longer start a cycle" when turned off on an existing start, and the question
when bleeding qualifies.

### Slice 6 — The graph, on real data

Take the spike's result and connect it to the API. Date view, presets, custom
range, ghost toggle, legend, day popup, "Show day".

*Acceptance criteria:* AC-10, AC-11, AC-15, plus the visible halves of AC-3 and
AC-13.

*Learned in the graph spike, to act on here* (`tech-stack.md` §8.3):
- Look up Recharts' version again and approve it; the spike's is 3.10.1, with
  `react-is` matching React. The branch `spike/graph` has working code to
  read, not to merge.
- **Taps:** use per-dot hit circles under the visible marks. Never read
  Recharts' active index in a click handler: on touch it's one tap behind.
- **Density:** at 30 and 90 days, dots must shrink, and a tap can only mean
  "nearest day". *Decided in the design step* (`design-brief.md` §6.1–§6.2):
  from 17px per day a tap opens exactly that day, narrower it snaps to the
  nearest reading. The popup has ‹ › and a swipe that step one calendar day
  at a time and page the graph when they leave it.

*From the design step* (`design-brief.md` §6.1–§6.2), also in this slice:
- the chart paper, and the bleeding and secretion rows under the plot. How
  to align them with Recharts' plot is *unverified*; measure it with
  screenshots at 14 and 90 days;
- ‹ › paging by the whole range, and Custom as a sixth preset;
- the popup's fixed height, with locked top and bottom and a scrolling
  middle that includes the notes.
- Decide whether the line enters from the window's edge, and whether to
  animate. Any screenshot test needs the animation off.
- Two **readings** need confirming or writing into `fam-rules.md` §7:
  - normal, disturbed, missing, normal doesn't connect;
  - a tag on a day without a temperature is a gap.

### Slice 7 — Cycle view

The second view on the graph: one cycle at a time, with previous/next navigation.
Cycle day on the X axis, `?` inside an unknown-start cycle, blank outside any
cycle. When the day popup steps out of the cycle shown, the view moves to the
cycle holding that day (`design-brief.md` §6.2).

### Slice 8 — Cycles and stats

The stats screen: current cycle, the four statistics with "based on N cycles"
shown once under them, the bar chart with the ongoing cycle faded, and the
cycle list (`design-brief.md` §6.4).

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
- **The design on her phone**, early in the morning: is it readable? Before
  Slice 6. Pe'er approves the design (`design-brief.md`); this check is about
  readability, not taste.

The rest are in `CLAUDE.md`'s verification list and are attached to the steps
above where they get answered.
