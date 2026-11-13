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

---

## Spike — The graph (8 November 2026, branch `spike/graph`, not merged)

**Goal.** Before Slice 1, find out whether Recharts can draw the chart the
rules describe. The known sharp edge was the line: a disturbed day is stepped
over, a missing day breaks the line, and Recharts has one `connectNulls` switch
per line.

**How it was done.** The plan was approved as written. Pe'er chose to test on
phones through a temporary Windows port proxy, rather than merging a `/spike`
page (which would have gone live) or deploying a separate service. The six
criteria for switching to visx were written **before** any chart existed, so
the result couldn't be argued into fitting them.

The spike page used 90 days of fake data, with 18 edge cases placed on known
dates and values from `fam-rules.md` §12. It also reported on itself:
- a CSP violation log, with a button that causes a violation on purpose;
- an audit of every painted colour;
- a tap log.

That let headless Chrome and both phones say what happened, with no console
to read.

**Decisions** (details in `tech-stack.md` §8.3):

| Decision | Choice | Why |
|---|---|---|
| Library | Recharts 3.10.1, plus `react-is` 19.3.1 explicitly | Passed all six criteria. The peer is named in `package.json`, not left in the lockfile |
| The line | One `<Line connectNulls>` per run, from a 20-line `lineSegments` | Inside a run the only blanks are disturbed days, so the one switch does the right thing there |
| Taps | 48 px hit circles under the visible marks | 9–10/10 on both phones. The chart-level alternative reads a stale day on touch |
| Cycle-start line | Dashed ink, with an accent marker on top | A thin line in accent fails 3:1 |
| Where the spike lives | An unmerged, pushed branch | Every merge deploys |

**Surprise: the frozen line.** The first headless screenshot had dots and
no line. Each segment's `stroke-dasharray` was `0px <length>`: Recharts'
draw-on animation, stuck on its first frame. With the animation off, the lines
were exactly right. In Pe'er's browser and on both phones the animation runs.
That's a headless quirk, but one that any screenshot test in Slice 6 would hit.

**Surprise: a tap one step behind.** The "tap a column" mode misfired now
and then on Android. Recharts' source explained it. Pointer moves are handled
once per animation frame (`throttleDelay: 'raf'`). A tap's click runs before
that frame, so it reads the active day the *previous* tap set. A test that
taps the same day ten times hides this, because the stale day is the same day.

**Surprise: a counter that couldn't fail the right way.** In the
alternating-taps test, the counter counted only hits on the target day, so the
perfect score was 5/10, not 10/10. Pe'er's notes ("as intended") are what
confirm every tap landed. The counter alone couldn't have said so.

**Verified rather than assumed:**
- **No CSP loosening.** 0 violations in every state, under the real headers,
  in headless Chrome, Windows Chrome, Android Chrome and iPhone Chrome. The
  negative control added exactly 1 each time, so the zeros mean something.
  Recharts' source sets styles only through the DOM.
- **Every edge case**, from `lineSegments`' output and from screenshots in
  °C and °F, with ghosts on and off, at 14, 30 and 90 days.
- **Tokens only**, by the browser's computed colours. The only black was a clip
  rectangle, which is never painted.
- **Taps:** Android 9/10 on a dot and 10/10 on a ghost; iPhone 10/10 and 10/10;
  neighbours 22 px apart resolved correctly. The popup showed 97.91 °F and the
  adjusted 36.55 "(estimate) · late".
- **Bundle:** +106.7 KB gzipped (69.2 → 175.9), under the 150 KB line.
- **Packages:** 39 added, no new audit advisory.

**Lessons:**
- **Write the exit criteria before looking.** Six lines written in the plan
  turned "it seems fine" into a pass or fail for each one.
- **A zero needs a one beside it.** The negative control is what made the CSP
  result evidence instead of silence.
- **Make the page report on itself.** Phones have no console. A violation log,
  a colour audit and a tap log on the page turned two phones into instruments.
- **A test that repeats one action can hide state bugs.** Varying the target
  is what would show a stale value.

---

## Design step (8 November 2026, PR #25)

**Goal.** Decide how Madam looks and behaves before Slice 3 builds the first
real screen. The plan said "design post-v1": a plain look now, and a
redesign by the first user later. Looking at the graph spike, Pe'er said the
app looked "like some admin-control page" and asked whether that was the
final design.

**How it was done.** Claude offered three options:
- a design step with the first user before Slice 3;
- a design pass by Claude that she reviews;
- keeping design post-v1.

Pe'er chose a design *skill* instead. Claude explained what a skill is (written
instructions Claude loads for a task, not code in the app) and recommended
Anthropic's `frontend-design` plugin, read in full before installing, with
the built-in `dataviz` skill for the charts. Pe'er installed it for his user
only. He answered four questions:
- HotPink stays, and the rest of the palette is open;
- a self-hosted font is allowed;
- light only in v1;
- he approves the design alone.

Claude's first move was to write the brief. Pe'er stopped it: he wanted to
*see* the direction first. So Claude built a clickable phone mock with
made-up data, published as a private page he could open on his phone, and he
reviewed it in four rounds. The brief was written last, from the approved
mock.

**Decisions** (details in `design-brief.md`; the spec changes are listed in
`spec-v1.md`'s header):

| Decision | Choice | Why |
|---|---|---|
| When | Before Slice 3, not post-v1 | Tokens make colours cheap to change, not layout and interaction, and Slices 3–9 build those |
| Direction | Her paper chart: pink chart paper, a berry line, bleeding and secretion rows under the days | The most characteristic thing in her world, and the one bold element on a quiet page |
| Colour | Eleven plum-tinted tokens. HotPink as a fill only; `berry` (7.6:1) for the line | HotPink is 2.65:1 on white, so it can't be a line or text (`tech-stack.md` §8.2) |
| Font | Atkinson Hyperlegible Next, self-hosted | Designed for legibility. Self-hosting keeps the reason for "no web fonts": nothing from another origin |
| Dates | dd/mm/yyyy | As in Israel (Pe'er) |
| Cycle start | The switch sits above Bleeding and carries the suggestion; its message says what Save will change | Round 2: a separate pink "Start new cycle" button looked already chosen |
| The day popup | ‹ › and a swipe step one calendar day; one fixed height; locked top and bottom | Rounds 1–3: stepping is how she reads days, and a popup that changes height moves the arrows under her thumb |
| The mock | Committed as `docs/design/mock.html` | A decision counts once it's in the repo, and the published page is private |

**The rounds:**
1. The first mock was liked as it stood ("It looks great!").
2. Eight requests:
   - ‹ › paging in the days view;
   - ‹ › in the popup;
   - a custom range;
   - a smaller temperature input;
   - the cycle-start button's ambiguity;
   - a cycle-start note in the popup;
   - dd/mm/yyyy;
   - the switch moved above Bleeding.
3. A fixed popup height, with only the details scrolling.
4. The switch's message should also appear without Bleeding, and notes should
   show in the popup.

**Surprises:**
- **The session vanished halfway.** It happened just after Pe'er chose the
  mock over the brief. Nothing had been written to the repo or to memory. The
  next session recovered the decisions from Claude Code's transcript on disk
  (`~/.claude/projects/…/*.jsonl`), and they are now in Claude's memory
  notes as well as here.
- **The CI run on `main` after #24 failed at Lint, with no code to lint
  differently.** A re-run passed and deployed. The cause wasn't investigated.
  It belongs with the GitHub Actions oddities seen in 0a and 0e.
- **Two controls for one fact.** The new-cycle suggestion had its own pink
  button next to the switch that actually decides. Pe'er read the button as
  "already selected" until he saw the switch was off. The fix removed the
  button rather than restyling it.

**Lessons:**
- **Show before writing.** A text brief couldn't have produced the round 2
  list. Every item came from tapping through a working mock on a phone.
- **One fact, one control.** When two elements describe the same state, one
  of them will eventually disagree with it. It mirrors the data model, where
  a cycle start is one row or none.
- **Things you step through must stay still.** Arrows and buttons that move
  as the content changes make stepping hard.
- **Write decisions down as they're made.** The lost session cost only time
  because its transcript happened to survive. The memory note is now written
  as soon as a decision is made.

## Slice 1 — Domain logic (8 November 2026, PR #26)

**Goal.** Build `backend/internal/fam`, the pure-Go heart of the app:
- time adjustment and °F conversion;
- cycle days, lengths and numbers, and the fixed measurement time for a date;
- the two bleeding rules;
- the statistics.

It is test-first from `fam-rules.md` §12, with no dependencies, and imports
nothing from the project.

**How it was done.** Claude read the rules and checked §12's arithmetic by
hand before planning. Then a short script ran §3.4's "adjusted?" test in both
units over every raw value from 35.000 to 39.000 °C and every delta up to
±10 minutes. It found two ambiguities, which Pe'er settled before any code
was written. The documents changed first, in one commit. After that, each
section followed the same rhythm, every red run shown to Pe'er before the
code:
1. tests and stubs that return zero values;
2. a red run;
3. the implementation;
4. a green run.

The sections went T → F → C → M → S → B → A.

**Decisions:**

| Decision | Choice | Why |
|---|---|---|
| "Adjusted?" for a °F user | Decided in °C, as §3.4 says | Pe'er chose it over "in the unit she views". The status never depends on a setting. The cost (the units disagree near a rounding edge) is written into §3.4, and vector F3 pins it |
| Unknown-start vs. outside any cycle | Store the onboarding answer, `users.cycle_start_unknown` (Slice 3's migration) | §5.2's rule can't tell "I don't know" from a known date followed by backfilling. Both leave the same rows |
| °F arithmetic | Hundredths in, accepted 68.00–122.00 °F, half-up both ways, display from the exact numerator | §3.5 named the goal, not the formulas. All 5,401 inputs round-trip |
| Dates | `fam.Date`, a count of days, not `time.Time` | Cycle-day arithmetic becomes subtraction, with no DST and no location, and `==` is safe. No dependency |
| Units | Named integer types: `MilliC`, `HundredthsC`, `HundredthsF`, `Minutes` | Passing °F where °C was meant fails to compile |
| The `-infinity` row | `NewSchedule(initial, changes)`: the onboarding time is a required argument | "No fixed time" and overlapping periods can't be expressed, so they need no error handling |
| The bleeding lookback | One unexported function used by both rules, given dated history | §6.3's "both rules must use the same lookback" holds by construction, and S3, B4 and B6's gaps are tested inside fam |
| Errors | Wrapped sentinel errors (`errors.Is`), never panics | Excluded input (20–50 °C, 00:00–23:59, duplicate dates, 31 Feb) is refused at the constructors, so the lookups can't fail |
| Keeping fam pure | golangci-lint's depguard: `internal/fam` may import only `$gostd` | A planted import of `db` and of pgx failed lint, so the rule works |

**Surprises:**
- **The two units disagree a lot.** Over 35–39 °C and ±10 minutes, the
  check found the following among 80,020 combinations:
  - 2,640 are adjusted in °C but not in °F;
  - 13,043 are adjusted in °F but not in °C.

  Reading §3.4 alone didn't show it. Running the numbers did.
- **A stub turns only half the vectors red.** A predicate stub answers
  "no", so every vector expecting "no" passed before any code existed:
  - S2–S4, B3, B5 and A7;
  - C8 and C12, because "blank" is the zero value.

  The red runs were still honest, because §12 has a "yes" for each rule.
- **`time.Date` doesn't refuse 31 February.** It quietly normalises it to
  3 March. `NewDate` checks that the date comes back unchanged.

**Lessons:**
- **Check a spec by computing it.** The rules were careful and still
  ambiguous in two places. Both showed up only when the arithmetic ran in
  every unit and every onboarding path.
- **Every rule needs a vector for each answer.** If §12 had only "no
  suggestion" cases, a function that always says no would pass them all.
- **Make wrong states impossible before handling them.** The required
  `initial` time, the start-dates-only changes, and `Date`'s unexported field
  removed whole classes of errors that would otherwise need tests.

**Carried forward:** Slice 4's "no future dates" needs a "today". The
server's clock is UTC, so in Israel, between midnight and 02:00 or 03:00,
her own today would look like the future. Raise it when Slice 4 is planned.

---

## Spike — The invite-code hook (11 November 2026, branch `spike/invite-hook`, not merged)

**Goal.** Before Slice 2 is planned, find out whether the invite code typed
at sign-up reaches Supabase Auth's "before user created" hook, so that the
hook can refuse a wrong code (`spec-v1.md` §2). `tech-stack.md` §4 had chosen
Supabase partly on this, and marked it unverified.

**How it was done.** Supabase's own pages didn't settle it:
- the only payload example shows an empty `user_metadata`;
- one page says a 4xx from the hook reaches the client, another that it
  becomes a 500;
- neither says which sign-up paths fire the hook, or what happens when the
  hook is unreachable.

The plan was approved as written. Pe'er chose a Cloudflare quick tunnel over
a throwaway Cloud Run service or a Postgres-only capture, so Supabase's cloud
could reach a Go handler on the laptop. The binary was pinned and its
checksum checked; it needs no account. Everything happened on madam-dev. The
hook's signing secret went from the dashboard into `.env` by Pe'er's hand,
and the Makefile's `export` gave it to the server, so it never passed through
Claude. The spike's invite code was deliberately not secret.

**Decisions** (details in `tech-stack.md` §4):

| Decision | Choice | Why |
|---|---|---|
| Hook type | HTTP, calling the Go API | The code stays in Secret Manager ("server configuration"), checked in Go with tests. A Postgres function would need `supabase_auth_admin` grants the local container can't have, and an `UPDATE` on production to rotate the code |
| Signature check | Standard library HMAC | Standard Webhooks is about 40 lines. The library would be a dependency |
| Test vector | Computed by OpenSSL | The spec has none. A vector from our own code would only check the code against itself |
| How to refuse | HTTP 200 with an error object | Measured: a 400 reaches the user as a 500 with Supabase's message |
| Where the spike lives | An unmerged, pushed branch | Every merge deploys |

**Surprise: the docs' contradiction had a precise answer.** The hook's own
status means "did the hook work". Whether the sign-up is refused is in the
body. Answering 400 for a wrong code told Supabase that *it* had sent a bad
payload, and the user saw `Invalid payload sent to hook`.

**Surprise: the code stays.** The hook can only allow or refuse, so the code
remains in `raw_user_meta_data` and appears in every access token's
`user_metadata`. That field is editable by its own user, so Slice 2 must
never read it as proof of anything.

**Surprise: a test script that erased its own output.** `curl -o /dev/stdout`
opens standard output as a file, which truncates it when output is captured.
The second request wiped the first one's result. curl writes to stdout by
default, so the flag went.

**Verified rather than assumed:**
- **The path:** a real, signed Supabase request carried
  `user.user_metadata.invite_code`, exactly as sent in `data`. supabase-js's
  source sends `options.data` under that name.
- **No residue:** none of the refused attempts left a row in `auth.users`.
- **Every way in:** a magic-link sign-in that creates a user fires the hook
  and was refused. An anonymous sign-up is refused by the setting before the
  hook. The dashboard's "Add user" skips the hook entirely.
- **Fails closed:** with the server down (502) and with the tunnel gone
  (530), the sign-up was a 500 and no user.
- **Forgeries:** an unsigned and a forged request to the public URL got 401
  before the code was looked at.
- **Tokens:** ES256 with a `kid`, so Slice 2 verifies against the JWKS.

**Lessons:**
- **When two docs disagree, measure both readings.** A switch for the refusal
  status turned the contradiction into one row of results.
- **Check a signature against a second implementation.** A test vector made
  by the code under test proves nothing.
- **Keep secrets on a path the assistant can't see.** Dashboard to `.env` to
  `make`'s environment carried the signing secret end to end without it
  appearing in the conversation.

**Carried forward to Slice 2:** two secrets (`madam-invite-code`,
`madam-auth-hook-secret`), JWT verification against the JWKS (a dependency
decision), whether to clear the stored code, and the production dashboard
settings in `tech-stack.md` §4.

---

## Slice 2 — Auth (12–13 November 2026, PRs #28–#43)

**Goal.** Real identities: Supabase Auth in the browser, a JWT verifier in Go
replacing the 0d test stub, the invite code enforced at sign-up, email
confirmation, password reset and log out. Done when AC-16 part one passes:
no API request from user A reaches user B's data.

**How it was done.** One plan, then small PRs, backend first: the verifier,
`GET /api/me` with the AC-16 test, and the hook. Then the design foundation,
moved ahead of the screens so they'd be built once. Then `/api/config` and
the screens. Then two rounds of fixes from Pe'er using the real thing on his
phone. Each PR was measured on the built image before it was opened, and
production was read back after each deploy.

**Decisions** (details in `tech-stack.md` §4, `design-brief.md` §5 and §9–§10):

| Decision | Choice | Why |
|---|---|---|
| Verifying tokens | `golang-jwt/jwt/v5`, plus our own 60-line key cache | Parsing and the algorithm allow-list are where homemade bugs live; fetching keys is not |
| Our `users` row | Created by the first authenticated `GET /api/me` | Idempotent, under the existing policy, no migration |
| The hook | Always HTTP 200; logs carry no personal data; both secrets required at startup | Measured in the spike; a missing secret must fail loudly |
| The browser's client | `@supabase/auth-js` | Session, refresh and tabs are solved there. Costs 26 KB gzipped |
| The page's configuration | `GET /api/config` | One build for every environment |
| Design foundation | Before the screens | So they are built once |
| Font | An npm package, one variable file | The lockfile's checksum is better evidence than a committed binary |
| Production email | Gmail SMTP, a dedicated account | The built-in sender only reaches team members |
| An existing address at sign-up | Named ("already has an account") | Pe'er, after waiting for an email that never comes. Reverses the plan |
| A new password | English-keyboard characters only, 8+, a letter and a digit | A keyboard on the wrong language is how people lock themselves out |

**Surprise: there was a browser all along.** For most of the slice I wrote UI
without seeing it, having looked only for a Linux browser. Chrome on the
Windows side, driven headless from Windows' Node, works from WSL. The first
screenshot showed a segmented control stretched across the screen, shipped
two PRs earlier. A check that only looks at whether a CSS rule exists says
nothing about how it looks.

**Surprise: a tab that copied another tab's state.** The reset link opened
in the mail app's own tab, which shares storage with Chrome. The auth client
announces `PASSWORD_RECOVERY` to every open tab, and the page took the
announcement to mean "this tab is recovering", so the Chrome tab showed the
new-password form and never left it. Found by Pe'er on his phone, reproduced
on production with two headless tabs, fixed by letting only how a tab was
*opened* decide.

**Surprise: Supabase hides more than expected, and checks in its own
order.** It answers a sign-up for an existing address with a made-up
success, creates nothing and never calls our hook. It answers a reset for an
unknown address with 200. It checks the password before the hook. Its
"letters" are a to z only: Hebrew letters are allowed and never count.

**Surprise: a phone's date picker ignores `min` and `max`.** A computer's
greys the dates out. Found on the component gallery, before any screen
depended on it; the date box now enforces its own limits.

**Surprise: Go's base64 decoder skips newlines.** A test expected a signing
secret with a trailing newline to be refused. It decodes to the same key.

**Mistakes of mine worth keeping:**
- I assumed a project URL was production's because I had asked for
  production's. It was dev's. Nothing was written with it, but the rule is:
  confirm which environment before using a value.
- Several numbers in PR descriptions were written from memory and were wrong
  (counts of checks, a bundle size). Count, then write.
- `pkill -f` matched its own command line and stopped itself.

**Verified rather than assumed:**
- **AC-16 part one:** `TestIsolationThroughTheAPI`, through the real router,
  verifier and database as `madam_app`. B's id is put everywhere a request
  has room for one, and the answer stays A's.
- **Every check was seen failing.** About 90 deliberate breakages across the
  slice, each caught by a test. The few that weren't said something: a check
  the library already made, a fixture carrying two signals at once.
- **Production:** a wrong code is refused with our message, 2.35 s on a cold
  instance; the hook's log holds no personal data; `GET /api/me` answered a
  real token with 200; both projects publish one ES256 key with a 10-minute
  cache header.
- **By Pe'er, on two phones and a computer:** the whole path, twice.

**Lessons:**
- **Break the code to test the test.** A test that has only ever passed
  hasn't shown it can fail.
- **Run a new check where the bug still lives.** The two-tab check failed on
  production first, which is what made its pass on the fix mean something.
- **Look at the thing.** Types, lint and a CSS rule's existence are not the
  screen.
- **A real user finds what a plan can't.** The stranded tab, the silent
  "check your email", the wish for the email to follow between screens: none
  were in the plan, and all came from ten minutes on a phone.
- **Hiding information has a cost a person pays.** Supabase's made-up
  success protects against discovery and left a real person waiting. The
  decision to show it is written down with its cost.

**Not done, and known:**
- The hourly token refresh has not been watched over a real hour.
- The first confirmation email landed in spam. A domain of our own with a
  transactional sender is the fix, deferred.
- Sign-up reveals whether an address has an account. To revisit before
  sign-up opens beyond the invite code.
- A hook host that answers nothing at all is still unmeasured.

**Carried forward:** Slice 3 needs `users.cycle_start_unknown` as its own
migration PR first, then `make migrate-prod`, then the code. Slice 4 must
settle whose "today" the no-future-dates rule uses (the server's UTC or the
user's Israel time); the date box already enforces whatever limit it is
given. Slice 9's account deletion needs a Supabase secret key in Secret
Manager (the fifth active version of six free).
