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
| `docs/build-log.md` | Retrospective: what was decided and learned in each phase, in order. |

**Never implement a FAM rule from memory or from this file — read
`docs/fam-rules.md`.** If code and that document disagree, the document wins. If
the document is wrong, change it *first*, in the same PR, with the tests.

---

## Stack

Go 1.27 with chi · Supabase PostgreSQL 17 (Frankfurt) · sqlc + pgx · goose
migrations · Supabase Auth (JWT) · React + TypeScript + Tailwind + Recharts,
embedded in the binary with `go:embed` · Google Cloud Run (`europe-west1`,
Belgium) · GitHub Actions.

## Layout

```
backend/
  cmd/server/        main, wiring, config
  internal/
    api/             chi router, handlers, middleware (HTTP only)
    fam/             domain logic. No HTTP, no SQL, no imports from api/ or db/
    db/              sqlc-generated code + hand-written .sql queries
  migrations/        goose migrations, numbered, never edited once merged
frontend/
  src/               React app
docs/                the documents above
```

`internal/fam` is the heart. It takes plain values and returns plain values, so
every acceptance criterion becomes a unit test with no server and no database.
Keep it that way.

## Commands

```
make dev            # API + frontend dev server + local postgres
make test           # go test ./... (add -race)
make lint           # golangci-lint + eslint
make test-integration # also the tests that need a database (local container)
make migrate-up     # goose up against $DATABASE_URL (migrate-status shows state)
make migrate-new NAME=create_entries   # new numbered migration file
make migrate-prod   # production migrations: latest main only, verify-full URL, typed confirmation
make psql           # psql on the selected database (also reads SQL from stdin)
make sqlc           # regenerate db/ from the .sql files
make build          # frontend build + go build with embedded assets
make db-up          # start local postgres; returns once pg_isready passes
make db-down        # stop it (data kept); db-reset also deletes the data
make image          # build the production image locally (docker-run also runs it)
make deploy         # fallback: deploy the latest main from the laptop (CI deploys every merge)
```

This list is the end state. The frontend parts of `dev`, `lint` and `build`
arrive in 0e (`docs/build-order.md`).

**Choosing the database:** every target defaults to the local container
(`DB=local`). Add `DB=dev` to use the `madam-dev` Supabase project, whose URL is
`MADAM_DEV_DATABASE_URL` in `.env` (the full session-pooler URL ending in
`?sslmode=verify-full`). The app only reads `DATABASE_URL`, which the Makefile
sets from `DB`. No `DB` value points at production. The one way to reach it is
`make migrate-prod`, a separate target that asks you to type `madam-prod`.
Names are in `.env.example`.

**Production order:** every merge deploys, so a schema change is two PRs: the
migration alone, then `make migrate-prod` from an up-to-date `main`, then the
code that needs it (`docs/tech-stack.md` §6). The migration must work with the
code already running, because it does.

**Pinned tools:** golangci-lint's version is `GOLANGCI_LINT_VERSION` in the
Makefile, and the same value appears in `.github/workflows/ci.yml`; change both
together. goose and sqlc are `tool` directives in a separate `tools/go.mod`,
run by the Makefile as `go -C tools tool goose|sqlc`.

Regenerate with `make sqlc` after touching any `.sql` file or migration. Never
hand-edit generated code.

---

## Conventions

- **Go:** `gofmt`, `golangci-lint`. Errors wrapped with context, never swallowed.
  No panics in request paths.
- **SQL:** every query lives in a `.sql` file, not in Go strings. **Every query
  that reads or writes a table holding user data filters by `user_id`** — no
  exceptions. The only query that touches no table is the health check's
  `SELECT 1`.
- **Migrations:** numbered goose files. Once merged, a migration is immutable;
  fix forward with a new one. Nobody edits schema in the Supabase dashboard.
- **Tests:** for anything in `internal/fam`, write the test from
  `docs/fam-rules.md` §12 **before** the implementation.
- **Git:** one short-lived branch per chunk, merged by squashed PR even though
  it's a solo project. `main` is protected and always deployable. Commit messages
  say what changed ("Add adjusted-temperature calculation"), not "stuff".
- **Secrets:** never in Git. `.env` locally; in production, **Secret Manager**
  (`madam-database-url` for the app, `madam-migrate-url` for migrations), with
  the app's version pinned in the Makefile. Values are added by Pe'er in his
  own terminal (`docs/tech-stack.md` §7), never through Claude.
  `.claude/settings.json` denies reading `.env` files (that includes any command
  that names `.env`); don't try to work around it. The production database URL
  is never in a local `.env`.
- **New table checklist:** does it cascade on user delete? Does it have an RLS
  policy? Is every query filtered by `user_id`?
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
- **Row Level Security is on, and forced.** Every request must run inside the
  transaction wrapper that sets the user id. **If a query returns zero rows and
  you expected rows, suspect the wrapper before suspecting the query.** The app
  connects with a role that does not own the tables — that is what makes the
  policies apply.
- **Two kinds of hole in the graph line.** A *disturbed* reading is skipped and
  its neighbours connect; a *missing* day is a gap. Recharts can't express both
  with one setting, so the line is drawn as separate segments. Don't "simplify"
  it back to one line.
- **Cycle starts are their own table**, not a flag on an entry, so a cycle can
  start on a day with no entry. Saving a day may therefore be two writes — in one
  transaction.
- **Everything derived is derived on read**: cycle days, cycle lengths, adjusted
  temperatures, late/early status, all statistics. Never store them.
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
  `postgres`, RLS does nothing, even with `FORCE`. Only the app role from 0d
  makes the policies apply.
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
  deploy that doesn't happen. Check `gh run list --branch main`; tell Pe'er
  what's stuck, then "Run workflow" on `main`.
- **`--allow-unauthenticated` is deliberately not in `make deploy`.** gcloud
  re-sets the service's IAM policy on every deploy with it, which CI's
  deployer may not do. Public access is the `allUsers` binding, set once.
- **Cloud Run has a spend cap.** If the deployed app suddenly stops answering and
  nothing changed in the code, check Billing → Budgets for an **Enforced** spend
  cap before debugging anything else. It pauses Cloud Run until lifted by hand.
- **Cloud Run never forwards some paths ending in `z`.** `/healthz` gets Google's
  own 404 and never reaches the app (tested), so the health endpoint is
  `/api/health`. Don't name any route with a trailing `z`, or starting `/_ah/`.
- **Cold start is under a second, database included.** Measured in 0c: a cold
  `/api/health`, which opens the first connection to Supabase, took 0.82–1.11 s
  end to end from the laptop in Israel (median 0.87 s; 0b, without a database,
  was 0.81 s), against about 0.25 s warm. The container itself starts in about
  0.03–0.3 s, because the pool connects on first use. An idle instance usually
  shuts down **15 minutes** after its last request, but once did after 7 s, so
  the 15 minutes isn't guaranteed (`docs/tech-stack.md` §7).
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

- Creating the non-owner application role on Supabase (0d). Known from 0c:
  goose's tables are owned by `postgres`, which has `CREATEROLE`.
- Whether the sign-up invite code reaches Supabase's "before user created" hook.
- The first month's billing report: confirm the spend cap measured gross usage
  and the bill stayed at zero.

Answered in 0c (details in `docs/data-model.md` §9–§10 and `docs/tech-stack.md`
§5): the pgx/sqlc type mappings, `-infinity` round-tripping, table ownership,
how Cloud Run connects (session pooler), and the Belgium–Frankfurt latency.
