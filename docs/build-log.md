# Madam — Build Log

A retrospective of how the project is actually being built: decisions, the
reasons behind them, and what was learned. One entry per phase, slice or
significant event, newest at the bottom.

This is the *story*. The decisions themselves live in the other documents
(`tech-stack.md`, `data-model.md`, `build-order.md`); this file links to them
rather than repeating them. It leaves out routine steps and small errors.

---

## Before the build: planning and setup (up to October 2026)

**What happened.** Six planning documents were written before any code:
`project-brief.md`, `tech-stack.md`, `data-model.md`, `fam-rules.md` (with the
test vectors that will become the first unit tests), `spec-v1.md`, and
`build-order.md`. Then the machine, the repository and the cloud accounts were
set up.

**Decisions worth remembering, with the reason in one line each:**

- **Public repository from day one.** On GitHub's free plan, branch protection
  only exists on public repos. Defences: `.gitignore` for `.env`, secret scanning
  and push protection, and Claude Code denied from reading `.env`.
- **Cloud Run in Belgium (`europe-west1`), not Frankfurt.** Belgium is in the
  cheaper pricing tier.
- **A spend cap, not just a budget alert.** An alert doesn't stop spending; a cap
  does.
- **Supabase in Frankfurt, two free projects** (`madam-dev`, `madam-prod`). The
  free plan pauses inactive projects, so a keepalive job is required, not
  optional.

**Learned from the machine.** The laptop has no IPv6, so Supabase's direct
connection is unreachable and the session pooler is used. Outgoing SSH is reset,
so Git uses HTTPS. Both cost time to discover, which is why they're in
`CLAUDE.md`.

---

## Phase 0a — Local skeleton (21 October 2026, PR #5)

**Goal.** The thinnest possible working backend: a Go module, `GET /healthz`
returning `ok`, a Makefile, and CI that runs build, test and lint on every PR.

**How it was done.** Claude proposed a plan, listing each open decision with
its options and trade-offs. Pe'er chose (all the recommended options in this
case), and only then was code written. That pattern, *plan, choose, then
code*, is the working method for every slice.

**Decisions** (details in `tech-stack.md` §10 and `CLAUDE.md`):

| Decision | Choice | Why |
|---|---|---|
| `go` line in `go.mod` | `go 1.27` | It's a minimum, not an exact version. Any 1.27.x works and nothing auto-downloads. `go mod init` wrote `1.27.1`; changed on purpose. |
| Pinning tools | golangci-lint as a pinned binary. goose and sqlc in a separate `tools/go.mod` (from 0c) | golangci-lint's own docs advise against `go tool`. A separate module keeps big tool dependency trees out of the app. |
| Local Postgres | Docker Compose, waiting on a `pg_isready` healthcheck | A started container isn't a ready database. The check uses TCP because of a socket-check false positive during first start. |
| Choosing a database | `make <target> DB=local\|dev` | The app only knows `DATABASE_URL`. The Makefile decides, and there's no option that points at production. |
| Dependabot | Weekly grouped updates, plus security alerts and fixes | Small dependency surface, so start the habit while it's cheap. |

**After CI existed,** `ci` became a **required status check** on `main`, tied to
the GitHub Actions app so no other tool can post a fake `ci` result.

**Verified rather than assumed:**
- `setup-go` turns `go 1.27` into the newest patch: CI ran **go1.27.3** while the
  laptop had 1.27.1. The laptop was then updated to match.
- The Postgres readiness wait is reliable: three runs from an empty database were
  ready in about 3.5–4.2 seconds.

**Surprise: GitHub Actions dropped events.** Opening the PR triggered no CI run.
Closing and reopening the PR did. Later, the check stayed "in progress" for
minutes after the job had finished, which blocked the merge until it caught up.
The merge to `main` also triggered no run. GitHub's status page said everything
was fine, and the cause is unknown.

On the next PR (#6) the run *did* start, but it was shown as "in progress" for
about 30 minutes. The step timestamps showed the job itself had finished in 45
seconds; only GitHub's reporting of the result was late.

**Lessons:**
- When CI doesn't appear, check GitHub's side (`gh run list`, the check runs)
  before suspecting the workflow file.
- When CI looks slow, compare the job's step timestamps with when the run was
  marked complete. That tells a slow pipeline apart from slow reporting.

**Decision:** add a manual "Run workflow" button (`workflow_dispatch`) only if a
run ever fails to *start* again. It wouldn't help with slow reporting, which is
what PR #6 showed.

**The laptop's Go was updated** from 1.27.1 to 1.27.3 to match CI. Claude
downloaded the official file and verified its checksum. Pe'er ran the two `sudo`
commands in a separate terminal, because replacing `/usr/local/go` needs
administrator rights, which deliberately need a password.

**Process changes that came out of this phase:**
- Pe'er asked that every command be explained (what it does, why, what the flags
  mean), and that this log be kept. Both are now in `CLAUDE.md`.
- **One session per phase.** Each bigger phase starts in a fresh Claude Code
  session with a prompt written at the end of the previous one. Before closing a
  phase, the decisions are checked against the documents that own them, so the
  next session can rely on the documents rather than on memory.

---

## Phase 0b — Deployed (22 October 2026, PR #8)

**Goal.** The 0a server in a container, on Cloud Run in Belgium, at a public
HTTPS URL, inside the cost guardrails.

**Surprise before any code: `/healthz` can't work on Cloud Run.** Pe'er
remembered that Cloud Run reserves some paths ending in `z`. Google's known
issues page confirmed it ("some paths ending with `z`", and advice to avoid them
all), and after deploying, a test confirmed it too: `/healthz` returns Google's
own HTML 404 and never reaches the app. The endpoint became `/api/health`.
Starting the `/api/` prefix now gives 0e a one-line rule for the embedded
frontend: anything not under `/api/` is the React app. The rename went into
the 0b PR as its own first commit, because it exists only to make the deploy
work.

**Decisions** (details in `tech-stack.md` §7, As built):

| Decision | Choice | Why |
|---|---|---|
| Runtime image | distroless static, `nonroot` | No shell or package manager, but it has the CA certificates 0c needs for TLS. 4.2 MB compressed |
| Build context | repository root, `.dockerignore` as an allowlist | `.env` can't enter the build, so it can't end up in a pushed image |
| Where deploys come from | the laptop, `make deploy`, refusing a dirty tree | No keys exist anywhere. Deploy on merge with Workload Identity Federation became its own step, 0c′, because IAM problems are a separate species from deploy problems |
| Runtime identity | a new `madam-run` service account with no roles | The default compute account has `roles/editor` on the project |
| Image cleanup | keep the 5 newest, delete the rest after a day, enforced from the start | Free storage is 0.5 GB. Google documents that a serving revision keeps its own copy of the image, so cleanup can't break the live app. A dry run with one image would have logged nothing, so it was skipped |
| Startup CPU boost | explicitly off | gcloud turns it on by default for new services, and it's billed. Found in `gcloud run deploy --help`, not in our docs |

**Verified rather than assumed:**
- The base image tags and Action versions were looked up in the registries on
  the day: `golang:1.27.3` was the newest 1.27, matching the laptop.
- The spend cap showed **Configured** before the first deploy.
- `gcloud run services describe` showed the deployed settings matching the plan.
- The server's `shut down cleanly` log line, written on SIGTERM, doubled as the
  proof that an instance had scaled to zero before each cold-start sample.
- **Cold start: under a second**, against an assumed "second or two": a median
  of 0.81 s end to end from Israel, with about 0.2 s of that being the container
  start. A background script waited each time for the instance to scale to zero
  (15 minutes, consistently), then timed one cold and three warm requests. The
  laptop's view, the request logs and Cloud Run's startup metric agreed.

**Worth remembering:** the server logs plain text (slog's text handler), so on
Cloud Run its lines are in `textPayload`, not `jsonPayload`. Log queries have
to match that.

**Lesson:** read the platform's known-issues page before naming anything public.
The conventional name (`/healthz`, a Kubernetes habit) was exactly the one the
platform reserves.

---

## Phase 0c — Database connected (4 November 2026, PR #9)

**Goal.** Migration `00001` (`users`), one sqlc query, and `/api/health`
reading a row from Supabase at the public URL. Along the way, answer the
verification items on types, table ownership and latency.

**Decisions** (details in `tech-stack.md` §5–§7 and §12):

| Decision | Choice | Why |
|---|---|---|
| How Cloud Run connects | Session pooler, like the laptop | The transaction pooler solves a scale problem we don't have. IPv6 to the direct connection would need a VPC, a subnet and a new IAM role |
| TLS | `verify-full` against Supabase's own root CA, committed | `require` accepts any certificate. The server refuses to start otherwise |
| Where the URL lives | Secret Manager, version pinned in the Makefile; `madam-run` may read that one secret | Never on the laptop, never readable in revision settings, and 0c′ needs no database credential |
| Migrations to production | `make migrate-prod` from the laptop, own secret, typed confirmation | CI would hold an owner credential in GitHub. Revisit in 0c′ |
| The health query | `SELECT 1`, touching no table | A query on `users` would break under 0d's RLS and leak the user count. `CLAUDE.md`'s `user_id` rule was reworded to "tables holding user data" |
| Startup | Fail on bad config, not on an unreachable database | A bad deploy fails safely; a paused Supabase doesn't crash-loop instances |
| CI | Postgres service, migrations applied, integration tests, generated-code check | Broken SQL fails the PR instead of reaching dev or prod |

**Surprise: Supabase's certificates come from a private CA.** A TLS handshake
with the pooler, before any code, showed a chain ending in "Supabase Root 2021
CA", which no system trust store contains. So "TLS on" had to mean
`verify-full` plus our own copy of the root. That copy came from the dashboard,
and its fingerprint was checked against the one the server sent, because a
root the server supplies itself proves nothing.

**Surprise: pgx's quiet fallbacks.** Two of them, each found by a failure:
- pgx reads the `PGSSLROOTCERT` file **even with `sslmode=disable`**. Exported
  globally, it broke the local database before the file existed.
- The first `.env` line held only `?sslmode=verify-full`, because the
  instruction said "append" to a line that didn't exist yet. pgx read it as
  `key=value` text and fell back to a local Unix socket as the OS user: a 503
  that looked like a network problem. The server now refuses anything that
  isn't a `postgres://` URL.

**The startup check paid for itself on the first deploy.** Secret version 1
lacked `?sslmode=verify-full`. The new revision refused to start, Cloud Run's
startup probe failed, and traffic stayed on the old revision. The fix was
version 2, a one-line pin change and destroying version 1. One cost remained:
`make migrate-prod` has no such check, so migration `00001` ran over encrypted
but unverified TLS.

**Verified rather than assumed:**
- **Type mappings** with a scratch table and a round trip through Postgres. The
  one that would have bitten later: a `nil` Go slice is written as `NULL`, so
  it fails on the `NOT NULL` arrays. **`-infinity` round-trips**, so that spike
  is answered (`data-model.md` §10).
- **Supabase's roles.** `postgres` owns goose's tables and **has `BYPASSRLS`**,
  so 0d's separate role is required, not a precaution. Supabase also grants its
  API roles full access to new tables despite "expose new tables: off"
  (`data-model.md` §9).
- **Latency:** about 11 ms per Belgium → Frankfurt round trip, inside Google.
  pgx's ping on connections idle for more than 1 s showed up as exactly a
  second round trip.
- **Cold start** with a database: median 0.87 s end to end (0b: 0.81 s). The
  container starts just as fast, because the pool connects on first use; the
  first query adds ~0.05–0.1 s for the connection to Frankfurt. One instance
  was shut down 7 s after its last request, so Cloud Run's 15-minute idle
  time is typical, not guaranteed. Every sample was checked as a genuinely new
  instance by its instance ID in the logs.
- CI's integration test really ran. The Test step's logged environment showed
  the database URL, and the test skips only without it.

**Lessons:**
- **Measure the transport before choosing a setting.** One `openssl` handshake
  changed the TLS decision before any code depended on it.
- **When a library falls back quietly, make the app refuse loudly.** Both of
  this phase's confusing failures came from pgx doing something reasonable
  with input it should never have accepted.
- **Instructions about secret files must say the whole thing.** "Append X to
  the line" assumed a line existed. Claude can't see `.env`, so instructions
  for it have to describe the complete, finished line.

---

## Phase 0c′ — Deploy on merge (5 November 2026, PRs #11 and #12)

**Goal.** Every merge to `main` deploys itself, with no key anywhere, so `main`
and the live app can't drift apart.

**Why it mattered, measured on the first morning.** The live revision ran
`93c33e9`, a 0c *branch* commit. `make deploy` had run from the branch, and a
squash merge gives the merged commit a new hash. So the live tag named a
commit that `main` didn't contain.

**Decisions** (details in `tech-stack.md` §6, §7 and §12):

| Decision | Choice | Why |
|---|---|---|
| Authentication | Workload Identity Federation through a service account, `madam-deployer` | No key exists anywhere. One identity to inspect or disable |
| The condition | Repository by numeric ID, `main`, push or "Run workflow", `ci.yml` only, GitHub's runners | Refuses forks, PRs, other branches and other workflows, and `pull_request_target`, which runs with `ref` set to `main` |
| Roles | Four, each bound on one resource, none on the project | The narrowest that a deploy needs |
| Gate | A `deploy` job in `ci.yml` after `ci` | The merged commit itself is tested, because the required check isn't strict |
| Flags | CI runs `make deploy` | One copy, so CI and the laptop can't drift |
| Public access | `--allow-unauthenticated` removed; the `allUsers` binding stays, set once | With the flag, gcloud re-sets the IAM policy on every deploy, which the deployer may not do |
| Missing runs | A "Run workflow" button now | Changed 0a's "only if it happens again": a missing run is now a missing deploy |
| Migrations | Still from the laptop, as two PRs | The owner credential stays out of GitHub's reach, and production only runs migrations that are on `main` |
| Guards | `deploy` and `migrate-prod` accept only a clean, latest `main`; `migrate-prod` also requires `verify-full` | Prevents another `93c33e9`, and closes 0c's unverified-TLS gap |

Pe'er accepted all the recommended options, and asked for documentation solid
and concise enough to learn from. §7 now opens with the flow in six steps, a
glossary and "When something goes wrong". That's the shape for later
infrastructure, too.

**Verified rather than assumed:**
- **The condition refuses a non-main ref.** A temporary job asked for a token
  from a push to the PR's branch, identical to a real deploy except for the
  branch, and from the PR itself. Both were refused: "The given credential is
  rejected by the attribute condition." Google doesn't say which clause failed,
  so only a test that changes one thing shows which rule fired.
- **The roles were enough, and the deployer needs no secret access.**
  `run.developer` bound on the service alone can deploy, and `--set-secrets`
  works without Secret Manager access for the deployer. Only `madam-run`
  needs it.
- **Three deploys in a row** (#11, #10, #12): each live within about 2 minutes,
  with the image tag equal to the merge commit and `madam-deployer` as the
  deployer in the audit log.
- `madam-migrate-url` passes the new `verify-full` check. It was piped in,
  never printed.

**Surprises:**
- Google's docs couldn't be fetched in the session, so gcloud's own source
  and `gcloud iam roles describe` answered instead. The source showed that
  `--allow-unauthenticated` calls `SetIamPolicy` on every deploy, even when
  nothing changes.
- The first audit-log search found deploys with no caller. It had matched
  Cloud Run's `system_event` log; who asked is in **Admin Activity**.
- The GitHub identity in that log uses GitHub's newer ID-based subject
  (`repo:peerbenmoshe@100029602/madam@1425598027:…`), which applies to
  repositories created after July 2026.
- Dependabot's `cel-go` PR waited until the first automatic deploy had been
  verified on its own, so that deploy carried one change. The PR then became the
  second test, and its security alert closed by itself once the fix reached
  `main`.

**Lessons:**
- **A deploy credential is data access.** The deployer can run any image as
  `madam-run`, which can read the database URL. What protects the data is the
  condition and branch protection, not the roles' names.
- **Test a security rule by changing one thing.** A refusal for several
  reasons at once proves less than it seems.
- **When the docs are out of reach, the tool's source and the role
  definitions are authoritative.** Both are on the laptop already.

---

## Phase 0d — Isolation plumbing (5 November 2026, PRs #14, #15 and #16)

**Goal.** The app connects as a role that Row Level Security actually applies
to, and a test proves it: I3, a query as that role with the wrong user id set
returns zero rows.

**Decisions** (details in `data-model.md` §9 and `tech-stack.md` §1.4, §5, §7):

| Decision | Choice | Why |
|---|---|---|
| The role | `madam_app`, created in migration `00002`, `NOBYPASSRLS`, owning nothing | `postgres` has `BYPASSRLS`, so it was required, not a precaution |
| Its password | None in Git: `\password` on Supabase, a throwaway one set by the Makefile locally and in CI | The repository is public. psql hashes the password before sending it |
| Privileges | `SELECT, INSERT, UPDATE, DELETE` on `users`; no default privileges | `TRUNCATE` ignores RLS. A forgotten grant fails loudly instead of granting on a table that forgot its policy |
| Supabase's API-role grants | Revoked in `00002`, in a block that runs only where those roles exist | Nothing uses them (Data API off, Auth uses its own role), and they included `TRUNCATE` |
| The setting | `set_config('madam.user_id', $1, true)`, read with `NULLIF(…, '')` | Takes a parameter, unlike `SET LOCAL`; dies with the transaction, so a pooled connection can't carry it over |
| The wrapper | `Store.InUserTx`, a function the handler calls | It commits before the response, so a failed commit can't hide behind a 200. Chosen by Pe'er over the documented middleware |
| The stub identity | Only in a `_test.go` file | Every merge deploys, and Go never compiles test files into a binary |
| PRs | Four: migration, code, the pin alone, docs | The deploy that changed the app's identity carried nothing else, so a 503 would have had one suspect |

**Surprise: `''` is not `NULL`.** `current_setting(name, true)` returns `NULL`
on a connection that never set the value, but `''` on one where an earlier
transaction set it. `''::uuid` is an error. So without `NULLIF`, a query that
skipped the wrapper would have returned nothing on a fresh connection and
failed on a reused one, depending on how pooled connections happened to be
reused. A test case built for this caught it: `22P02 invalid input syntax for
type uuid: ""`.

**Verified rather than assumed:**
- **I3 failed first, three ways, then passed.** As the owner, the test's guard
  refused to run. As `madam_app` with RLS off, I3, `WITH CHECK` and both
  no-setting cases failed, while "a user sees their own row" passed, which is
  why seeing the app work proves nothing. Without `NULLIF`, the
  reused-connection case errored.
- **madam-dev before madam-prod.** Its grants were inspected first. `anon`,
  `authenticated` and `service_role` held every privilege on our tables,
  `TRUNCATE` included; after `00002` only `postgres` and `madam_app` held any.
  `postgres` got ADMIN on the role it created, so `\password` worked. A
  custom role logs in through the pooler as `madam_app.<project-ref>`, and the
  whole suite passed there.
- **Production.** Version 3 was checked with psql before the pin moved. After
  the deploy: health `ok`, the revision references version 3, and
  `pg_stat_activity` shows `madam_app` as the only client besides the checking
  session. Version 2, the `postgres` URL, was then destroyed.
- **The stub is not in the binary** (`go tool nm`). The one symbol matching
  "stub" was `reflect.embeddedIfaceMethStub`, Go's own.

**Surprises:**
- Supabase has its own `auth.users` next to our `users`, so `00002` writes
  `public.users`. Supabase also already uses `app.settings.*`, which made
  `madam.` the safer prefix.
- The pooler replaces `application_name`: every client shows as `Supavisor`.
  So the app's connections are found by role, one more use for having a role of
  its own.
- A second `postgres` row in `pg_stat_activity` looked like a leftover
  connection. Pe'er stopped before destroying version 2 and asked. Its
  `backend_type` showed it was `pg_net`'s background worker, running since the
  database started.
- #14's deploy took 9 minutes instead of 2. The runner sat silent for 6½ minutes
  in `gcloud auth configure-docker` and was slow in `gcloud run deploy` too.
  The next two deploys were back to normal (about 1½ minutes). The cause is
  unknown and outside our code. The deploy job's 15-minute timeout would have
  stopped it without harm.
- The laptop's psql is version 14, and its `\dp` fails on a version-17 server.
  Querying the catalog with `aclexplode` works with any version.

**Lessons:**
- **Make the test refuse to prove nothing.** I3's guard (not a superuser, not
  `BYPASSRLS`, exactly `madam_app`) means a wrong connection can't make it
  pass, and its "own row is visible" case means zero rows can't be an
  accident.
- **When a credential changes, check it before anything depends on it.** The
  pool connects lazily, so a wrong password would have started fine and failed
  on the first request. psql against version 3, while the app was still
  pinned to version 2, was the check.
- **Before destroying something, account for every row.** "Looks like
  `postgres` twice" was worth a second query: `backend_type` and `client_addr`
  tell a process inside the database apart from a connection to it.

---

## Phase 0e — Frontend shell (6 November 2026, PRs #18 and #19)

**Goal.** The React page and the API from one binary at one URL: Vite +
React + TypeScript + Tailwind, built into the image and embedded with
`go:embed`, showing what `/api/health` says.

**How it was done.** Pe'er accepted every recommended option in the plan.
There were two code PRs, Go first: #18 taught the server to serve embedded
files and briefly put `503 frontend not built` live. #19 added the frontend,
so the deploy that changed the page carried nothing else.

**Decisions** (details in `tech-stack.md` §8.4, with §7, §8.2, §10 and §12):

| Decision | Choice | Why |
|---|---|---|
| Node | 26.13.0, exactly, in `.nvmrc` and the Dockerfile; CI checks they agree | Active LTS for 9 days; 24 had just gone to maintenance, and the laptop's 24.21.0 had missed a security release |
| TypeScript | 6.0.3, not `latest` 7.0.2 | typescript-eslint still requires `<6.1`. Vite's own template had moved to TS 7 with oxlint, which would have lost type-aware lint |
| Packages | npm, exact versions, no install scripts, Dependabot with a 7-day cooldown | Nothing extra to pin; the worm route closed; a week for bad releases to be pulled |
| No build yet | A committed `.gitkeep`; pages answer 503 | A fresh clone compiles, and the image can't ship that state: the smoke test would fail |
| `/api` | Its own 404, `no-store`; everything else is the app, with `index.html` for client-side routes | An unknown API path must never look like success |
| Headers | A strict CSP and the usual protections, now | Easiest to keep strict while nothing needs loosening |
| Recharts | Left for the graph spike | 0e has no chart |

**Surprise: chi's inheritance.** chi hands a mounted subrouter its parent's
NotFound handler when it has none (read in its source, `mux.go`). A
page-as-fallback at the top level, the obvious way to write a single-page app
server, would have answered `/api/typo` with `200` and the page. The `/api`
subrouter now has its own 404. Breaking it on purpose made seven router tests
fail, which is how we know the tests guard it.

**Surprise: "latest" wasn't compatible.** Looking every version up on the
day, instead of from memory, found TypeScript 7 as `latest` and the linter
not ready for it. It also found Node 26 newly LTS and the laptop behind on
security. `npm audit` then reported ten "vulnerabilities", which were one
advisory: `minimatch` inside ESLint, fixed only in a major that ESLint
doesn't accept yet. npm's suggested fix was a downgrade. Pe'er chose to
accept and record it; Dependabot independently reached "update not
possible".

**Verified rather than assumed:**
- **Nothing under `/api/` reaches the page**: unit tests, a smoke test on the
  built image in every PR's CI, and the same script at the public URL
  (14/14).
- **A deep link reloads**, at the public URL and on Pe'er's phone.
- **Distroless needs no MIME table**: the script comes back as
  `text/javascript` from the image (Go's built-in table).
- **`ignore-scripts=true` costs nothing**: install and build work.
- **The type-aware lint does something**: removing a `void` produced
  `no-floating-promises`.
- **`make dev` stops cleanly** on Ctrl+C and when Vite dies alone. The first
  design, `trap 'kill 0'`, would have killed `make` too; the server is now
  run as a binary so `$!` is the server, not `go run`.
- **The build is reproducible**: the script's content hash was the same on
  the laptop and in CI.
- **Size and speed**: the image is 5.41 MB compressed (+124 KB for all of
  0e), still distroless. The `ci` job takes about 3 minutes instead of 1½, and
  merge to live about 4½ instead of 2, mostly the uncached `npm ci` inside the
  image build, which runs in both jobs.
- **Cold start is unchanged by the frontend**: container start 20–249 ms (0c:
  28–303 ms). A cold `/api/health` took 0.87–1.33 s, median 1.03 s (0c: median
  0.87 s, overlapping range); the extra was after the container started,
  probably the first Supabase connection. A cold page, with no database, took
  0.88–1.00 s, and its script 0.28 s more (`docs/tech-stack.md` §7).

**Surprises:**
- The merge's CI run looked missing. It wasn't: the merge changed
  `dependabot.yml`, Dependabot started four runs on `main`, and they filled
  `gh run list --limit 4`. Pe'er spotted the real run on the web page. The
  checks now use `--workflow ci.yml`.
- The tool shell still had Node 24 after installing 26: its environment was
  captured when the session started. `make check-node` caught it with a
  clear message, which is what it's for.

**Lessons:**
- **Read the library before trusting the pattern.** The idiomatic
  single-page-app fallback was exactly the bug. Fifteen lines of chi's
  source settled it before any code depended on it.
- **"Latest" is a claim to check, not a default.** The compatible set was
  one major behind on one package, and only a peer-dependency range said so.
- **A count isn't a finding.** Ten audit entries were one advisory in a dev
  tool; reading each one is what turned alarm into a decision.
- **Filter before concluding something is missing.** A busier branch made a
  healthy run invisible in a short list.

---

## Phase 0f — Keepalive (7 November 2026, PRs #21 and #22)

**Goal.** Neither Supabase project may pause. The free plan pauses after a
week of too little activity, and a paused madam-prod is a broken app on the
morning she tries to log a temperature.

**How it was done.** The plan was approved with two choices put to Pe'er
(below). #21 added the workflow. Its first manual run failed on madam-dev, and
#22 fixed that. The first scheduled run passed the same evening.

**Decisions** (details in `tech-stack.md` §5, Mitigation, and §12):

| Decision | Choice | Why |
|---|---|---|
| Production | `curl` of the public `/api/health` | It already queries madam-prod as `madam_app`, so **no production credential is in GitHub**. §9 had assumed one |
| madam-dev | `psql` as `madam_app`, with the repository secret `MADAM_DEV_DATABASE_URL` | Nothing public reaches it. Pe'er chose this over a new no-privilege role: a leak reaches only fake data, and the role would have cost a migration and an extra PR |
| What counts | A database query | Supabase's docs say "user database activity". So Auth's health endpoint (probably no query) and REST (Data API off) were out |
| Schedule | `41 2,8,14,20 * * *` | Supabase asks for "a few user requests to the database each day". GitHub delays and drops runs at the top of the hour. Pe'er chose four a day over 0f's "every few days" |
| Workflow | A separate file, two jobs, `permissions: {}`, no `id-token`, no `pull_request` trigger | It can't deploy, can't become a required check, and a failure names the project |
| 60 days | No automation | During the build, merges keep the repository active. `main` is protected with no bypass, so a commit-based keepalive couldn't push anyway. Supabase's warning email is the backstop |

**Surprise: my guard was stricter than the server it copied.** The `dev` job
refuses a secret that isn't a `postgres://` URL with `verify-full`, the
server's rule. But the server accepts `postgresql://` too, the scheme
Supabase's dashboard gives, and the guard was written from memory. The first
run failed at the guard, in a fraction of a second, with a readable message
and before any connection. That was the right kind of failure, but for the
wrong reason. #22 matched the server.

**Surprise: `SELECT 1` has no row in `pg_stat_statements`.** After a green
run, madam-dev's statistics showed no `SELECT 1` from any role. A test with
three variants (`SELECT 1`, `SELECT 1::integer AS ok`, `SELECT 1 AS
keepalive`) showed why. Postgres groups statements by a fingerprint that
ignores constants and column aliases, so all of them were counted under the
app's own `-- name: Ping`, the text it saw first. The query was there all
along; the filter was looking for the wrong text.

**Surprise: the first scheduled run came at no scheduled time.**
- Nothing ran at 14:41 UTC, the first slot after the merge. A scheduled run
  started at **19:00 UTC**, which isn't a slot at all.
- GitHub's run record doesn't say which cron line fired. The likeliest
  reading is the 14:41 slot delayed by 4 h 19 min, but that's unverified.
- Either way, this is what four slots a day are for. A pause needs a week of
  silence, about 28 missed slots.

**Verified rather than assumed:**
- **The ping registers on madam-dev.** With counters read before and after,
  one run added exactly one call to `madam_app`'s `Ping` entry and one
  `DISCARD ALL`, the pooler ending the session.
- **Production:** Cloud Run logged a `200` from a GitHub runner, and the
  handler answers `200` only after its query succeeds. Whether Supabase's
  pause counter reads the same numbers stays unverified; its docs don't say.
- **It can't deploy.** The run log's token permissions are `Metadata: read`
  (always granted), plus `Contents: read` for `dev`, with no `IdToken`. The
  deploy condition, re-read with `gcloud`, still names `ci.yml` only.
- **It can't become required.** `main`'s required checks are still only `ci`.
- **Nothing secret in the logs.** The URL appears only as `***`. The only
  `postgres` strings are the script's own text.
- **Manual runs green,** both jobs (after #22), and then the first
  **scheduled** run (run 42404050371). It had the same permissions, a `1`
  from madam-dev, `ok` from production, and nothing secret in its log.

**Lessons:**
- **Copy a rule from its code, not from memory.** A guard meant to match
  another check must be checked against it, with the edge cases that check
  allows.
- **Before concluding something is missing, check how it would be filed.**
  The query was recorded, under another name. A baseline taken before the
  change, plus a test that varies one thing, is what showed it.
- **Read what the provider counts.** "Any request keeps it awake" had become
  this project's belief. Supabase's own words, "user database activity", ruled
  out two options and set the schedule.
