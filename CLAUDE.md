# CLAUDE.md

**Madam** — a mobile-friendly web app that replaces a paper FAM/BBT chart
(fertility awareness method, basal body temperature). One real user today, built
public-ready with strict per-user isolation. The data is health data: treat it
accordingly.

This file is short on purpose. It tells you where the real documents are and how
to behave. It is not the spec.

---

## Read before changing behaviour

| File | What it is |
|---|---|
| `docs/fam-rules.md` | **Source of truth for domain logic.** Exact arithmetic, exact conditions, and the test vectors in §12. |
| `docs/spec-v1.md` | What the app does: screens, flows, acceptance criteria. |
| `docs/data-model.md` | Tables, columns, constraints, and the reasoning. |
| `docs/tech-stack.md` | Stack decisions, rejected options, accepted trade-offs. |
| `docs/build-order.md` | What gets built next, in what order, and its acceptance criteria. |
| `docs/design-brief.md` | **Source of truth for the look and touch behaviour** of every screen: tokens, type, components, layouts, dd/mm/yyyy dates. The approved mock is `docs/design/mock.html`. |
| `docs/build-log.md` | Retrospective: what was decided and learned in each phase, in order. |

**Never implement a FAM rule from memory or from this file — read
`docs/fam-rules.md`.** If code and that document disagree, the document wins. If
the document is wrong, change it *first*, in the same PR, with the tests.

---

## Stack

Go 1.27 with chi · Supabase PostgreSQL 17 (Frankfurt) · sqlc + pgx · goose
migrations · Supabase Auth (JWT) · React 19 + TypeScript 6 + Tailwind 4 +
Vite 8 on Node 26 (Recharts 3.10.1, confirmed by the graph spike, is
installed in Slice 6), embedded in the
binary with `go:embed` · Google Cloud Run (`europe-west1`, Belgium) · GitHub
Actions.

## Layout

```
backend/
  cmd/server/        main, wiring, config
  internal/
    api/             chi router, handlers, middleware (HTTP only)
    fam/             domain logic. No HTTP, no SQL, no imports from api/ or db/
    db/              sqlc-generated code + hand-written .sql queries
    web/             serves the embedded frontend; assets/ is build output
  migrations/        goose migrations, numbered, never edited once merged
frontend/
  src/               React app
deploy/              scripts: check-db-url.sh, smoke-test.sh
docs/                the documents above
```

`internal/fam` is the heart. It takes plain values and returns plain values, so
every acceptance criterion becomes a unit test with no server and no database.
Keep it that way: a depguard rule in `backend/.golangci.yml` fails lint if it
imports anything outside the standard library.

## Commands

```
make dev            # API + Vite dev server + local postgres; open :5173 (Vite passes /api/ to :8080)
make test           # go test ./... (add -race)
make lint           # golangci-lint + eslint + prettier --check
make frontend       # type check + vite build + copy into backend/internal/web/assets
make test-integration # also the tests that need a database, as madam_app (local container; DB=dev too)
make migrate-up     # goose up as the owner (migrate-status shows state); locally also sets madam_app's password
make migrate-new NAME=create_entries   # new numbered migration file
make migrate-prod   # production migrations: latest main only, verify-full URL, typed confirmation
make psql           # psql as the owner, sees every row (also reads SQL from stdin)
make psql-app       # psql as madam_app: sees what the app sees
make sqlc           # regenerate db/ from the .sql files
make build          # make frontend + go build with the frontend embedded (bin/server)
make db-up          # start local postgres; returns once pg_isready passes
make db-down        # stop it (data kept); db-reset also deletes the data
make image          # build the production image locally (docker-run also runs it)
make deploy         # fallback: deploy the latest main from the laptop (CI deploys every merge)
sh deploy/smoke-test.sh <url>   # routing, caching and headers of a running server (CI runs it on the image)
gh workflow run keepalive.yml --ref main   # run the Supabase keepalive by hand (it runs 4× a day by itself)
gh run list --workflow keepalive.yml       # its runs; add --event schedule for the scheduled ones only
```

Every frontend target runs `check-node` first and installs `node_modules`
(`npm ci`) when the lockfile is newer.

**Node:** exactly the version in `.nvmrc` (26.13.0). `nvm use` in the repository
switches to it; `make check-node` refuses anything else. The Dockerfile's
`node:` tag must say the same version, and CI fails if it doesn't.

**Choosing the database:** every target defaults to the local container
(`DB=local`). Add `DB=dev` to use the `madam-dev` Supabase project. Each
database has two URLs, one per role: `DATABASE_URL`, the application role
`madam_app` (the app, the tests, `psql-app`), and `MIGRATE_URL`, the owner
(goose, `psql`). For madam-dev they are `MADAM_DEV_DATABASE_URL` and
`MADAM_DEV_MIGRATE_URL` in `.env`: full session-pooler URLs ending in
`?sslmode=verify-full`, with the users `madam_app.<ref>` and `postgres.<ref>`.
The app only reads `DATABASE_URL`, which the Makefile sets from `DB`. The
server also needs `SUPABASE_URL`, the project whose tokens it accepts: locally
always madam-dev's (`MADAM_DEV_SUPABASE_URL` in `.env`, with `DB=local` too,
because the local container has no Auth), and production's is a flag in
`make deploy`. No `DB` value points at production. The one way to reach it is
`make migrate-prod`, a separate target that asks you to type `madam-prod`.
Names are in `.env.example`.

**Production order:** every merge deploys, so a schema change is two PRs: the
migration alone, then `make migrate-prod` from an up-to-date `main`, then the
code that needs it (`docs/tech-stack.md` §6). The migration must work with the
code already running, because it does.

**Pinned tools:** golangci-lint's version is `GOLANGCI_LINT_VERSION` in the
Makefile, and the same value appears in `.github/workflows/ci.yml`; change both
together. goose and sqlc are `tool` directives in a separate `tools/go.mod`,
run by the Makefile as `go -C tools tool goose|sqlc`. npm packages are exact
versions (`frontend/.npmrc`: `save-exact`, and `ignore-scripts`, so no
package runs code on install). TypeScript stays on 6 until typescript-eslint
supports 7 (`docs/tech-stack.md` §8.4).

Regenerate with `make sqlc` after touching any `.sql` file or migration. Never
hand-edit generated code.

---

## Conventions

- **Go:** `gofmt`, `golangci-lint`. Errors wrapped with context, never swallowed.
  No panics in request paths.
- **SQL:** every query lives in a `.sql` file, not in Go strings. **Every query
  that reads or writes a table holding user data filters by `user_id`** — no
  exceptions (on `users`, the user id is `id`). The only queries that touch no
  table are the health check's `SELECT 1`, `SetUserID` and `CurrentRole`
  (`internal/db/queries/session.sql`).
- **Migrations:** numbered goose files. Once merged, a migration is immutable;
  fix forward with a new one. Nobody edits schema in the Supabase dashboard.
- **Tests:** for anything in `internal/fam`, write the test from
  `docs/fam-rules.md` §12 **before** the implementation.
- **Git:** one short-lived branch per chunk, merged by squashed PR even though
  it's a solo project. `main` is protected and always deployable. Commit messages
  say what changed ("Add adjusted-temperature calculation"), not "stuff".
- **Secrets:** never in Git. `.env` locally; in production, **Secret Manager**
  (`madam-database-url`, `madam-invite-code` and `madam-auth-hook-secret` for
  the app, `madam-migrate-url` for migrations), with the app's versions
  pinned in the Makefile. Values are added by Pe'er in his
  own terminal (`docs/tech-stack.md` §7), never through Claude.
  `.claude/settings.json` denies reading `.env` files (that includes any command
  that names `.env`); don't try to work around it. The production database URL
  is never in a local `.env`. GitHub holds one secret,
  `MADAM_DEV_DATABASE_URL`, for the keepalive (`docs/tech-stack.md` §5). It
  must change whenever `madam_app`'s password on madam-dev does. There is
  **no production credential in GitHub**; keep it that way.
- **New table checklist:** does it cascade on user delete? Does its migration
  `ENABLE` and `FORCE` RLS and add a policy `TO madam_app` with `USING` and
  `WITH CHECK`? Does it `GRANT` `madam_app` exactly the statements it needs
  (never `TRUNCATE`, which ignores RLS)? There are no default privileges for
  `madam_app`, so a missing grant fails as "permission denied". Is every query
  filtered by `user_id`?
- **Deploying:** every merge to `main` deploys itself. The `deploy` job in
  `ci.yml` runs `make deploy` once `ci` passes, as `madam-deployer` through
  Workload Identity Federation (`docs/tech-stack.md` §7, 0c′). No keys exist;
  never create one. `make deploy` from the laptop is the fallback, and it
  refuses anything but a clean, latest `main`. It builds the image ourselves,
  tags it with the commit hash and pushes it; never `gcloud run deploy
  --source` (it uses Cloud Build, billed separately). Every Cloud Run setting
  is a flag in that target, so change settings there, never in the Console.
  The exception is IAM (who may call the app, who may deploy): one-time
  `gcloud` commands, recorded in §7. Keep min instances at 0 and max instances
  at 2.

---

## Non-obvious things that will bite you

- **Temperatures are integers in thousandths of a degree Celsius.** 36.62 °C is
  `36620`. Getting the unit wrong is wrong by a factor of 1000. Fahrenheit is
  display and input only.
- **`updated_at` is set by a database trigger**, not by application code. You
  won't find it in any Go or SQL file. That's deliberate.
- **Row Level Security is on, and forced.** Every query on user data must run
  inside `Store.InUserTx` (`internal/db/store.go`), which sets the user id for
  that one transaction. **If a query returns zero rows and you expected rows,
  suspect the wrapper before suspecting the query.** The app connects as
  `madam_app`, which owns nothing and has `NOBYPASSRLS` — that is what makes
  the policies apply. `make psql` connects as the owner and sees every row, so
  use `make psql-app` to see what the app sees.
- **Two kinds of hole in the graph line.** A *disturbed* reading is skipped and
  its neighbours connect; a *missing* day is a gap. Recharts can't express both
  with one setting, so the line is drawn as separate segments: one `<Line
  connectNulls>` per run, where a day without a temperature ends a run and a
  disturbed day doesn't (`docs/tech-stack.md` §8.3; working code on the
  unmerged branch `spike/graph`). Don't "simplify" it back to one line.
- **On touch, Recharts' active index is one tap behind.** It handles pointer
  moves once per animation frame, and a tap's click runs before that frame. So
  never read `activeIndex` in a click handler; tappable dots carry their own
  handlers (measured in the graph spike). Its draw-on line animation also
  stays on frame one in headless Chrome, so screenshot tests turn it off.
- **Cycle starts are their own table**, not a flag on an entry, so a cycle can
  start on a day with no entry. Saving a day may therefore be two writes — in one
  transaction.
- **Everything derived is derived on read**: cycle days, cycle lengths, adjusted
  temperatures, late/early status, all statistics. Never store them.
- **`fam.Date` is not `time.Time`.** It is a count of days with no clock and
  no time zone, so cycle-day arithmetic is subtraction and `==` is safe.
  Convert at the boundary: `fam.DateOf(pgDate.Time)` in, `d.Time()` (midnight
  UTC) out. The `-infinity` row of `measurement_time_periods` never becomes a
  `Date`. Recognise it by `InfinityModifier` and pass its time as
  `NewSchedule`'s `initial` (Slice 1).
- **`?` versus blank depends on a stored answer, not on the dates.** Entries before the first cycle start show `?` only if onboarding
  said "I don't know" (`users.cycle_start_unknown`, from Slice 3). Otherwise
  they show blank. `onboarded_at` can't tell the two apart (`fam-rules.md`
  §5.2).
- **The development machine has no IPv6.** Supabase's direct connection is
  IPv6-only on the free plan, so from the laptop use the **shared pooler in
  session mode** (pooler host, port 5432). The transaction pooler (port 6543)
  does not support prepared statements, which pgx uses by default. Cloud Run
  uses the session pooler too.
- **Supabase signs with its own root CA.** `sslmode=verify-full` works only
  with `backend/certs/supabase-root-2021-ca.crt`, found through `PGSSLROOTCERT`
  (set by the Dockerfile and by the Makefile for Supabase targets). The server
  refuses to start with a remote URL that isn't `verify-full`, or with anything
  that isn't a `postgres://` URL: pgx reads other strings as `key=value` and
  silently falls back to a local socket.
- **Supabase's `postgres` role has `BYPASSRLS`** (measured in 0c). Connected as
  `postgres`, RLS does nothing, even with `FORCE`. Only `madam_app` (0d) makes
  the policies apply. The local owner `madam` is a superuser, so the same holds
  locally.
- **In `pg_stat_activity` on Supabase, every client shows as `Supavisor`.**
  The pooler replaces `application_name`, so find the app by
  `usename = 'madam_app'`. A `postgres` row named `pg_net …` is Supabase's
  background worker (`backend_type` says so), not a connection.
- **`current_setting` reads `''`, not `NULL`, once a connection has used it.**
  After a transaction that set `madam.user_id` ends, the same pooled connection
  reads the setting as an empty string. `''::uuid` is an error, so every policy
  wraps it: `NULLIF(current_setting('madam.user_id', true), '')::uuid`
  (measured in 0d).
- **A `nil` Go slice is written as `NULL`.** Into the `NOT NULL` `TEXT[]` columns
  (`disturbances`, `symptoms`) that fails. Send `[]string{}` (measured in 0c).
- **`/api/health` reads the database.** It returns 503 when Supabase is
  unreachable. Never make it a Cloud Run liveness probe, or a database outage
  would restart working containers.
- **Local Postgres needs a readiness wait.** A started container is not a ready
  database. Poll with `pg_isready`; never `sleep N`. This is the classic cause of
  flaky first runs and CI.
- **Git talks to GitHub over HTTPS only.** SSH connections are reset from this
  laptop; don't switch the remote to `git@github.com:`.
- **A merge can start no CI run at all** (seen in 0a), and since 0c′ that is a
  deploy that doesn't happen. Check `gh run list --workflow ci.yml --branch
  main`; without `--workflow`, Dependabot's runs on `main` can fill the list
  and make a run look missing (0e). Tell Pe'er what's stuck, then "Run
  workflow" on `main`.
- **`--allow-unauthenticated` is deliberately not in `make deploy`.** gcloud
  re-sets the service's IAM policy on every deploy with it, which CI's
  deployer may not do. Public access is the `allUsers` binding, set once.
- **Cloud Run has a spend cap.** If the deployed app suddenly stops answering and
  nothing changed in the code, check Billing → Budgets for an **Enforced** spend
  cap before debugging anything else. It pauses Cloud Run until lifted by hand.
- **Cloud Run never forwards some paths ending in `z`.** `/healthz` gets Google's
  own 404 and never reaches the app (tested), so the health endpoint is
  `/api/health`. Don't name any route with a trailing `z`, or starting `/_ah/`.
- **Cold start is about a second, database included.** Measured in 0c: a cold
  `/api/health`, which opens the first connection to Supabase, took 0.82–1.11 s
  end to end from the laptop in Israel (median 0.87 s; 0b, without a database,
  was 0.81 s), against about 0.25 s warm. Re-measured in 0e with the frontend
  embedded: median 1.03 s (0.87–1.33 s), and a cold page 0.88–1.00 s. The
  container itself starts in about 0.02–0.3 s, unchanged by the embedded
  files, because the pool connects on first use. An idle instance usually
  shuts down **15 minutes** after its last request, but once did after 7 s, so
  the 15 minutes isn't guaranteed (`docs/tech-stack.md` §7).
- **An unknown `/api/` path must be a 404, never the app's page.** The `/api`
  subrouter sets its own NotFound because chi gives a mounted router its
  parent's when it has none: a page fallback at the top would silently answer
  `/api/typo` with `200 text/html`. `router_test.go` and the smoke test guard
  it; keep new API routes inside that subrouter.
- **`backend/internal/web/assets/` is build output.** `make frontend` fills
  it; only the empty `.gitkeep` is committed, so Go compiles on a fresh clone.
  A binary built without `make frontend` serves the API but answers pages
  with `503 frontend not built`. That's expected locally, never in the image.
- **Vite puts every `VITE_*` variable into the public JavaScript.** Never put
  a secret in one, or in a `frontend/.env*` file. `.dockerignore` keeps
  those files out of the image build anyway.
- **The CSP allows only `'self'`, with no inline script or style.** A new
  origin the browser must talk to (Supabase Auth in Slice 2) needs adding to
  `connect-src` in `internal/api/headers.go`. Recharts needs no loosening: it
  sets styles through the DOM, which the CSP allows (graph spike). A blank
  page in production but not in `make dev` (Vite serves its own pages,
  without our headers) usually means a CSP refusal: check the browser
  console.
- **Supabase pauses a free project for too few database queries, not HTTP
  requests.** Its docs say "user database activity" (`docs/tech-stack.md` §5).
  `.github/workflows/keepalive.yml` keeps both projects awake 4× a day.
  Production goes through `/api/health`, so if that endpoint ever stops
  querying the database, the keepalive stops working for production.
- **GitHub disables scheduled workflows after 60 days without repository
  activity** (public repositories). The keepalive then stops silently, and
  Supabase's warning email a week before a pause is the backstop. `gh
  workflow enable keepalive.yml` turns it back on. Scheduled-run failure
  emails go to whoever created the workflow, last edited its cron line or
  re-enabled it.
- **Never name a job in any workflow `ci`.** That name is the required check
  on `main`. The keepalive has no `pull_request` trigger, so it can't become a
  required check by accident.
- **In `pg_stat_statements`, `SELECT 1` has no row of its own.** Statements
  are grouped by a fingerprint that ignores constants and aliases, so every
  `SELECT 1` variant counts under the app's `-- name: Ping` entry. Filter by
  that text, not by `select $1` (measured in 0f).
- **The sign-up hook refuses with HTTP 200, never a 4xx.** Supabase passes
  the body's `error.http_code` and `message` to the user. A 4xx from the hook
  becomes a 500 saying "Invalid payload sent to hook" (measured in the
  invite-code spike). The typed code also stays in `user_metadata` and in
  every access token. Its own user can edit `user_metadata`, so never treat
  it as evidence of anything.
- **The server needs four values to start:** `DATABASE_URL`, `SUPABASE_URL`,
  `INVITE_CODE` and `AUTH_HOOK_SECRET`. Locally the Makefile supplies the
  last two as throwaway values. A new one must also reach `make deploy`,
  `make docker-run` and CI's smoke-test step, or the image won't start there.
- **A token the server can't check is a 503, not a 401.** That happens only
  when Supabase's keys were never fetched (`ErrAuthUnavailable`). If
  everything answers 503 right after a start, look at the `jwks:` log lines.
- **Never read `user_metadata` in Go.** The verifier leaves it out of its
  claims on purpose: its own user can edit it.
- **`docker run --network host` doesn't reach WSL under Docker Desktop.** It
  is the Docker VM's network. CI's Linux runner can use it; locally use the
  Compose network and `-p`, as `make docker-run` does.
- **Never run against production data.** Local development uses its own Supabase
  project and a local Postgres container.

---

## How to work with Pe'er

He reviews every change and is learning Go as he goes. Optimise for him
understanding the diff, not for finishing fast.

- **Explain before the diff.** What you're changing, why, and what you considered
  instead — in plain language, a few sentences, not a wall.
- **Explain every command.** For any command you suggest or run (shell, `git`,
  `gh`, `docker`, `make`, `gcloud`, ...), say what it does, why it's needed, and
  what each non-obvious flag or argument means. Don't assume he already knows the
  tool. This matters most for commands he runs himself, especially with `sudo`.
- **Keep `docs/build-log.md` current.** When a phase or slice finishes, or a
  significant decision or surprise happens, add a short entry covering what was
  decided, why, and what was learned. It's a retrospective of the core process,
  not a diary: leave out routine steps and small errors.
- **Propose options when there's a real choice.** Give the trade-offs and let him
  pick. Don't silently choose and move on.
- **Label assumptions as assumptions.** If you don't know something, say so
  rather than producing confident-sounding filler. Guessing is worse than asking.
- **Stop and ask when a change contradicts a documented decision.** The documents
  encode reasoning that isn't obvious from the code.
- **Small changes.** One concern per PR. If a task grows, split it.
- **Don't add dependencies without asking.**
- He can read Hebrew and English; all code, comments and docs are in **English**.

## Verification list (unanswered until measured)

Carry these forward; answer them in the build, don't assume.

- The first month's billing report: confirm the spend cap measured gross usage
  and the bill stayed at zero.

Answered in the invite-code spike (`docs/tech-stack.md` §4): the sign-up
invite code reaches Supabase's "before user created" hook as
`user.user_metadata.invite_code`. A refusal stops the sign-up, an unreachable
hook fails closed, and OTP sign-ups fire the hook too. Dashboard "Add user"
does not.

Answered in 0c (details in `docs/data-model.md` §9–§10 and `docs/tech-stack.md`
§5): the pgx/sqlc type mappings, `-infinity` round-tripping, table ownership,
how Cloud Run connects (session pooler), and the Belgium–Frankfurt latency.

Answered in 0d (`docs/data-model.md` §9, `docs/tech-stack.md` §5 and §7): the
application role `madam_app`, created by migration `00002` without a password
and given one with `\password`; it logs in through the pooler as
`madam_app.<project-ref>`; I3 passes as it on madam-dev and in CI; production
connects as it (secret version 3); nothing else connects as `postgres`.
