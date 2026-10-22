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
