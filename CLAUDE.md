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

**Never implement a FAM rule from memory or from this file — read
`docs/fam-rules.md`.** If code and that document disagree, the document wins. If
the document is wrong, change it *first*, in the same PR, with the tests.

---

## Stack

Go 1.22+ with chi · Supabase PostgreSQL (EU) · sqlc + pgx · goose migrations ·
Supabase Auth (JWT) · React + TypeScript + Tailwind + Recharts, embedded in the
binary with `go:embed` · Google Cloud Run · GitHub Actions.

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
docs/                the four documents above
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
```

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
- **New table checklist:** does it cascade on user delete? Does it have an RLS
  policy? Is every query filtered by `user_id`?

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
- **Never run against production data.** Local development uses its own Supabase
  project and a local Postgres container.

---

## How to work with Pe'er

He reviews every change and is learning Go as he goes. Optimise for him
understanding the diff, not for finishing fast.

- **Explain before the diff.** What you're changing, why, and what you considered
  instead — in plain language, a few sentences, not a wall.
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
- Cloud Run cold-start time once the skeleton is deployed.
