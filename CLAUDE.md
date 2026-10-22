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
make migrate-up     # goose up against $DATABASE_URL
make sqlc           # regenerate db/ from the .sql files
make build          # frontend build + go build with embedded assets
make db-up          # start local postgres; returns once pg_isready passes
make db-down        # stop it (data kept); db-reset also deletes the data
make image          # build the production image locally (docker-run also runs it)
make deploy         # build, push and deploy the committed code to Cloud Run
```

This list is the end state. Targets arrive phase by phase (`docs/build-order.md`):
`migrate-up` and `sqlc` in 0c, and the frontend parts of `dev`, `lint` and `build` in 0e.

**Choosing the database:** every target defaults to the local container
(`DB=local`). Add `DB=dev` to use the `madam-dev` Supabase project, whose URL is
`MADAM_DEV_DATABASE_URL` in `.env`. The app only reads `DATABASE_URL`, which the
Makefile sets from `DB`. No `DB` value points at production. Names are in
`.env.example`.

**Pinned tools:** golangci-lint's version is `GOLANGCI_LINT_VERSION` in the
Makefile, and the same value appears in `.github/workflows/ci.yml`; change both
together. goose and sqlc will be `tool` directives in a separate `tools/go.mod`
from 0c.

Regenerate with `make sqlc` after touching any `.sql` file or migration. Never
hand-edit generated code.

---

## Conventions

- **Go:** `gofmt`, `golangci-lint`. Errors wrapped with context, never swallowed.
  No panics in request paths.
- **SQL:** every query lives in a `.sql` file, not in Go strings. **Every query
  filters by `user_id`** — no exceptions.
- **Migrations:** numbered goose files. Once merged, a migration is immutable;
  fix forward with a new one. Nobody edits schema in the Supabase dashboard.
- **Tests:** for anything in `internal/fam`, write the test from
  `docs/fam-rules.md` §12 **before** the implementation.
- **Git:** one short-lived branch per chunk, merged by squashed PR even though
  it's a solo project. `main` is protected and always deployable. Commit messages
  say what changed ("Add adjusted-temperature calculation"), not "stuff".
- **Secrets:** never in Git. `.env` locally, Cloud Run config in production.
  `.claude/settings.json` denies reading `.env` files; don't try to work around
  it. The production database URL is never in a local `.env`.
- **New table checklist:** does it cascade on user delete? Does it have an RLS
  policy? Is every query filtered by `user_id`?
- **Deploying:** `make deploy`, from the laptop, with a clean working tree. It
  builds the image ourselves, tags it with the commit hash and pushes it; never
  `gcloud run deploy --source` (it uses Cloud Build, billed separately). Every
  Cloud Run setting is a flag in that target, so change settings there, never in
  the Console. Keep min instances at 0 and max instances at 2
  (`docs/tech-stack.md` §7). Deploy on merge comes later (`docs/build-order.md`).

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
  does not support prepared statements, which pgx uses by default.
- **Local Postgres needs a readiness wait.** A started container is not a ready
  database. Poll with `pg_isready`; never `sleep N`. This is the classic cause of
  flaky first runs and CI.
- **Git talks to GitHub over HTTPS only.** SSH connections are reset from this
  laptop; don't switch the remote to `git@github.com:`.
- **Cloud Run has a spend cap.** If the deployed app suddenly stops answering and
  nothing changed in the code, check Billing → Budgets for an **Enforced** spend
  cap before debugging anything else. It pauses Cloud Run until lifted by hand.
- **Cloud Run never forwards some paths ending in `z`.** `/healthz` gets Google's
  own 404 and never reaches the app (tested), so the health endpoint is
  `/api/health`. Don't name any route with a trailing `z`, or starting `/_ah/`.
- **Cold start is under a second.** Measured in 0b: a cold request took 0.67–0.98 s
  end to end from the laptop in Israel (median 0.81 s), against about 0.21 s
  warm. Cloud Run's own metric puts the container start at about 0.2 s. An idle
  instance shuts down **15 minutes** after its last request (measured), so the
  first request after that pays it. Expect this to grow once 0c adds a database
  connection at startup (`docs/tech-stack.md` §7).
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

- pgx/sqlc mappings for `INTEGER`, `DATE`, `TIME`, `TEXT[]`.
- `-infinity` round-tripping through pgx into Go (fallback: a sentinel date).
- Who owns tables created by goose migrations on Supabase, and how to create a
  non-owner application role.
- Whether the sign-up invite code reaches Supabase's "before user created" hook.
- How Cloud Run connects to Supabase: session pooler, transaction pooler with
  prepared statements off, or direct (only if Cloud Run egress has IPv6).
- Query latency between Cloud Run (Belgium) and Supabase (Frankfurt).
- The first month's billing report: confirm the spend cap measured gross usage
  and the bill stayed at zero.
