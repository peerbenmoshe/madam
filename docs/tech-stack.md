# Madam — Tech Stack

_Status: decided by Pe'er in step 2. Amended at step 3: two bullets in §5 were
superseded by `data-model.md` — see the note there._

_Amended at step 4 (rules and setup):_
- _**Charting is decided: Recharts.** §8.3 rewritten, including a correction to a claim this document previously made._
- _**Row Level Security is in from day one**, which changes what §5 and §9 require of the database role. See `data-model.md` §9._
- _Free Supabase plan **confirmed** by Pe'er; the keepalive job is therefore load-bearing, not optional (§5)._
- _Palette is **HotPink `#FF69B4`**, with measured contrast limits (§8.2)._
- _Figma handoff moved to optional/post-v1 — the designer has no Figma plan (§11)._
- _§13's open-items list is rewritten: most items are now either decided or moved to the build's verification list._
- _**The repository is public from day one**, reversing "private until v1" (§12). Decided during machine and repository setup._

_Amended during Google Cloud setup (Oct 2026), after checking Google's terms and pricing pages:_
- _**Cloud Run region is `europe-west1` (Belgium)**, a Tier 1 pricing region. Frankfurt, previously implied, is Tier 2 (§7)._
- _**A budget alert does not stop spending.** The previous "set a budget alert" was a false safety net. Cost protection is now an alert **plus a Cloud Run spend cap** (§7)._
- _§7 gains what the free tier does **not** cover: egress to Israel, Artifact Registry beyond 0.5 GB, and Cloud Build._
- _Google's new **Starter Tier** was considered and rejected (§7)._

_Amended at the end of setup (Oct 2026), recording what was actually created and measured:_
- _**Supabase region is Frankfurt (`eu-central-1`)**; PostgreSQL **17**; project options recorded, including why "automatic RLS" is off (§5)._
- _New **"Connecting"** subsection: the development machine has no IPv6, so it uses Supabase's session pooler (§5)._
- _Budgets are **₪15** (the billing account is in shekels), and Google's docs settle that the spend cap counts usage before the free-tier discount (§7)._
- _§10 records the installed tools and the machine's network quirks; §12 records secret scanning and push protection as verified._

_Amended after Phase 0a (Oct 2026):_ _§9 records the local Postgres setup and the variable names; §10 the tool pinning, the `go` line and Go 1.27.3; §12 the CI shape, the required `ci` check, and Dependabot._

_Amended after Phase 0b (Oct 2026):_ _§7 records the image, the registry and the Cloud Run service as built, Cloud Run's reserved paths, and the measured cold start; §9's checklist is complete; §12 adds the image build in CI and Dependabot for Docker._

_Amended after Phase 0c (Nov 2026):_ _§5 decides how Cloud Run connects (session pooler, `verify-full` against Supabase's own CA) and records the pool settings, the measured latency and the database roles; §6 records the tools, sqlc config and how migrations reach production; §7 adds Secret Manager and the re-measured cold start; §9 and §12 follow._

_Amended after Phase 0e (Nov 2026):_ _§8.4 records the frontend as built (toolchain and versions, embedding, routing, caching, security headers, `make dev`); §8.2 the tokens and their measured contrast; §7 the image's frontend stage, its size and the cold start with the page; §10 Node 26.13.0; §12 the frontend steps and smoke test in CI, Dependabot for npm, and ESLint and Prettier as configured._

_Amended after Phase 0f (Nov 2026):_ _§5 quotes what Supabase counts as activity, closes "Open for 0f" with the keepalive as built (production through `/api/health` with no credential; madam-dev with one GitHub secret, and what it can do if it leaks), measures that the ping registers, and records the 60-day rule; §9 follows; §12 records the workflow; §13 the re-checked restore window._

_Amended after the graph spike (Nov 2026):_ _§8.3 records the spike's answer: Recharts 3.10.1 passed every criterion for switching to visx, fixed before looking, and it records how the line, dots, ticks, taps, colours and CSP are done, plus what Slice 6 inherits; §8.4's Recharts row and `style-src` expectation are settled._

_Amended in the design step (8 November 2026):_ _§11 moves design from "optional, post-v1" to a step before Slice 3, now done (`design-brief.md`); §8.2 gains the design's tokens and a **self-hosted web font**, reversing "no web fonts"; §13 follows._

_Amended after the invite-code spike (11 November 2026):_ _§4 replaces "Unverified" with the measured answer (the code reaches the "before user created" hook as `user.user_metadata.invite_code`), records how a refusal must be sent, the signature scheme, which sign-up paths fire the hook, that it fails closed, and that the code persists in every token; decides an HTTP hook calling the Go API over a Postgres function; lists the dashboard settings to repeat for production; §13 follows._

This document records **what** was chosen and **why**, including the options that
were rejected and the honest trade-offs accepted. It is the source of truth for
stack questions; `project-brief.md` only summarises it.

---

## 0. The stack in one table

| Layer | Choice |
|---|---|
| Architecture | REST + JSON API, separate frontend app, one repo |
| Backend language | Go (1.27) |
| HTTP layer | chi |
| Auth | Supabase Auth (managed) |
| Database | Supabase PostgreSQL 17, **Frankfurt (`eu-central-1`)**, free plan |
| DB access | sqlc (+ pgx driver) |
| Migrations | goose |
| Isolation | `user_id` filter in every query **+ Row Level Security** |
| API hosting | Google Cloud Run (container), **`europe-west1` (Belgium)** |
| Cost guardrails | **Budget alert + Cloud Run spend cap** (₪15 each), min instances 0, max instances 2 |
| Frontend framework | React (TypeScript) |
| Styling | Tailwind, design tokens |
| Charting | **Recharts** (3.10.1 confirmed by the graph spike; installed in Slice 6) |
| Frontend delivery | Embedded in the Go binary (`go:embed`) |
| Tests | Go standard library `testing` |
| CI | GitHub Actions |
| Repo | **Public from day one** |
| Dev environment | WSL (Ubuntu), Linux filesystem |

---

## 1. Architecture

### 1.1 Shape

A **REST + JSON API** with a **separate frontend app**. The backend never
renders HTML pages; it only serves JSON. The web app is one client of that API,
and the future WhatsApp bot will be a second client of the same API.

Rejected: server-rendered HTML pages plus a separate JSON API. The graph needs
substantial interactivity anyway (tappable dots, popup, range controls, ghost
toggle), and maintaining two entry points into the same logic is worse than
learning a frontend framework once.

GraphQL and gRPC were considered out of scope for an app this size.

### 1.2 Repository layout

**One repository (monorepo)** with `backend/` and `frontend/` folders. A change
that touches both sides is one branch and one pull request, and Claude Code sees
the whole picture at once. Two repos would only make sense for separate teams on
separate release schedules.

### 1.3 Where the FAM logic lives

**Backend only.** The frontend displays what the API gives it and computes
nothing.

Reasons:

- **Two clients, one truth.** The WhatsApp bot gets identical numbers for free.
  Two copies of the logic would drift and produce different values in the app and
  in WhatsApp.
- **The logic needs data the screen does not have.** Cycle day depends on a cycle
  start that may fall outside the visible range; stats need every completed cycle.
- **Rules must be server-enforced.** One-entry-per-date and per-user isolation are
  not enforceable in a browser.
- **Testability.** Acceptance criteria become plain unit tests with no HTTP or
  database involved.
- **One place to change** the rounding mode or a future configurable correction rate.

Accepted trade-off: no instant on-screen preview of the adjusted temperature while
typing, unless we later add a small frontend copy of that one formula.

**Consequence worth remembering:** rules that look like UI are really domain
logic. The backend decides *whether* to ask "Did bleeding stop?" or "Start a new
cycle?"; each client decides *how* to ask. Otherwise the bot would silently skip
those checks.

**The one exception, added at step 4.** Splitting the graph line into drawn
segments (§8.3) is genuinely presentational and lives in the frontend. It is safe
there precisely because no second client draws a line — the WhatsApp bot never
will.

### 1.4 Backend internal structure

Request path, in order:

1. **Router and handlers** — translate HTTP to and from the rest of the app.
2. **Auth middleware** — verify the token, establish the user identity. Every
   later step is scoped to that user (AC-16).
3. **Transaction wrapper** — open a transaction and set the user id as a
   transaction-local setting, so Row Level Security policies can see who is
   asking (`data-model.md` §9). Every data-touching request goes through it.
   *Decided in 0d:* it's a function the handler calls,
   `Store.InUserTx(ctx, userID, fn)` (`internal/db/store.go`), not middleware.
   It commits before the handler writes its response, so a failed commit
   becomes an error response, never a 200 for data that wasn't saved.
   Rejected: middleware committing after `next.ServeHTTP`, by which time the
   response may already be on its way. Handlers get a `Store`, never the pool,
   so it is the only path to user data.
4. **Validation** — well-formed dates, numbers, no future dates, temperature
   within 20.00–50.00 °C.
5. **Domain logic** — time adjustment, cycle days, suggestion rules, stats. Plain
   Go, no knowledge of HTTP or the database.
6. **Data access** — the only layer that talks to PostgreSQL.

The database stores **raw facts only** (measured temperature, measurement time,
cycle starts). Everything derived is computed on read.

---

## 2. Backend language: Go

### Why

- Building JSON APIs over HTTP is Go's core use case; since Go 1.22 the standard
  library router handles method matching and path wildcards natively.
- Fits Pe'er's target roles (backend, cloud, infrastructure) and his existing
  cloud/QA background.
- **Easy to review.** Go is deliberately small and explicit, which matters when
  Claude writes most of the code and Pe'er reviews the diffs.
- Deploys as a single binary, which also simplifies embedding the frontend.

### Honest accounting

- The commonly hyped strengths (goroutines, raw performance) are **irrelevant** at
  Madam's scale. Go was not chosen for speed.
- Pe'er is mid-way through learning Go (Matt Holiday playlist, video 14). The real
  remaining cost is the "is this right?" radar: for a while he will trust Claude's
  code more than his own judgement. Reviewing and debugging fluency will grow.
- Go's auth story is thin compared with Django's, which pushed the auth decision
  toward a managed provider (section 4).
- Two languages in the project (Go + TypeScript) rather than one.
- **Go has no decimal type**, which is why temperatures are integers
  (`data-model.md` §8.2).

Rejected: **Python** (faster to ship for Pe'er, and Django has the strongest
built-in auth, but Django's magic works against understanding the system);
**TypeScript everywhere** (one language and shared types, but no backend-language
learning and less relevant to his target roles).

---

## 3. HTTP layer: chi

`chi` is a thin router on top of `net/http`. Its handlers and middleware are
ordinary standard-library ones, so the whole Go ecosystem works with it. The core
router is small and depends only on the standard library, and it is used in
production at companies including Cloudflare and Heroku.

It adds what we would otherwise hand-write: route groups ("everything here needs
auth"), sub-routers, and a set of ready-made middleware. Auth is chi
middleware (`RequireUser`, §4). The transaction wrapper in §1.4 was planned as
middleware too; 0d made it a function the handler calls instead (§1.4).

Rejected:

- **Standard library alone** — a fine choice; we would just write a few small
  helpers ourselves.
- **Gin / Echo** — popular and well-documented, but handlers use the framework's
  own context type, so you learn the framework's dialect rather than Go's HTTP model.
- **Fiber** — built on `fasthttp` instead of `net/http`, so it is incompatible with
  much of the ecosystem, for a speed benefit Madam does not need.

This choice affects roughly 10–15% of the backend code. Domain logic and database
code are framework-independent.

---

## 4. Auth: Supabase Auth

### The pattern (identical whichever provider is used)

The identity service handles sign-up, password checks, verification email and
password reset, and issues a **token**. The frontend attaches that token to every
request. A chi middleware verifies the token's signature and reads the user ID.
Every database query filters by that user ID, and RLS policies enforce the same
rule inside the database.

**The auth system holds identity only** (email, password hash). All health data
lives in our own database, never in the provider's.

### Why Supabase

- **The invite code fits.** Supabase has a "before user created" hook that runs
  before the account exists and denies sign-up if it returns an error. The hook can
  be an HTTP endpoint, so it can call our Go API, which checks the shared invite
  code held in server config — exactly `spec-v1.md` §2.
  *Verified in the invite-code spike (11 November 2026):* the typed code
  reaches the hook as `user.user_metadata.invite_code`, sent as sign-up
  metadata, and a refusal stops the sign-up. See "As built in the invite-code
  spike" below.
- **EU data region** available, which should help the later GDPR check.
- Email verification and password reset are built in.
- **Low lock-in:** the auth server is open source and self-hostable later. It is
  written in Go.
- Bundles with the PostgreSQL database (section 5), so one vendor and one region.

Known limits: free-tier email sending is rate-limited, so a real email service will
likely be needed before public use.

### As built in the invite-code spike (11 November 2026)

Branch `spike/invite-hook`, pushed and never merged. The hook was pointed at
a Go handler on the laptop through a Cloudflare quick tunnel (no account; the
URL dies with the process), on **madam-dev only**. Sign-ups were sent with
`curl` exactly as supabase-js sends them. Its source puts `signUp`'s
`options.data` in the body as `data`. `signInWithOtp` sends the same `data`,
plus `create_user` (`shouldCreateUser`, default `true`).

**The answer: yes.** The typed code arrives as
**`user.user_metadata.invite_code`**, and a refusal from the hook stops the
sign-up. The request was a real Supabase request, signed and verified against
the dashboard's secret. No refused attempt left a row in `auth.users`.

**Measured:**

| # | What was tried | What happened |
|---|---|---|
| M1 | Sign-up, wrong code | Refused. The client's status depends on how the hook refuses (next table) |
| M2 | Sign-up, no `data` | The hook saw `"user_metadata": {}`. Refused |
| M3 | Sign-up, right code | Created. The hook saw the code at `user.user_metadata.invite_code` |
| M4 | A request to the hook with no signature, or a forged one | 401 from our handler, before the code is looked at |
| M5 | The created account | The code **persists**: in `auth.users.raw_user_meta_data`, and in **every access token's** `user_metadata` claim. Tokens are signed **`ES256` with a `kid`** (asymmetric keys) |
| M6 | `/otp` with `create_user: true`, no code | **The hook fires** and refuses. Magic-link sign-in is not a way around the code |
| M7 | Anonymous sign-up | Refused by the setting (422 `anonymous_provider_disabled`), before the hook |
| M8 | Dashboard → Users → Add user | **Created without calling the hook.** Admin creation bypasses the code |
| M9 | The hook's address answers 502 (server down) or 530 (tunnel gone) | **Fails closed:** 500 `Unexpected status code returned from hook: 5xx`, no user. Not measured: a host that answers nothing at all (Supabase documents a timeout as an error) |

**How a refusal must be sent.** Supabase's two pages disagreed. Measured:

| The hook answers | The client sees |
|---|---|
| HTTP **200**, `{"error":{"http_code":400,"message":"Invalid invite code."}}` | **400**, `"msg":"Invalid invite code."` |
| HTTP **400**, the same body | **500**, `"Invalid payload sent to hook"`. Our message is lost |

So the hook always answers **200**. It sends `{}` to allow, or the error
object to refuse. `error.http_code` and `error.message` become the client's
status and message. A non-2xx means "the hook failed", never "refused".
`Content-Type: application/json` is required.

**The signature: Standard Webhooks.**
- Supabase sends the headers `webhook-id`, `webhook-timestamp` (Unix
  seconds) and `webhook-signature`. The last is a space-separated list of
  `v1,<base64>`.
- The signature is HMAC-SHA256 over `id.timestamp.body`.
- The dashboard's secret looks like `v1,whsec_<base64>`. The key is the
  decoded base64 after the prefixes.
- The spike checked it with the standard library only (`crypto/hmac`,
  `crypto/subtle`), with a 5-minute timestamp tolerance (the spec names no
  number).
- The spec has no official test vector, so the unit test uses one computed
  by OpenSSL. That way Go's arithmetic is checked against an independent
  implementation, not against itself.

**Other things measured:**
- The payload's `user.id` is already the id the account gets.
- `metadata.ip_address` is the signer-up's IP. Treat it as personal data:
  don't log it.
- The handler took microseconds. A whole sign-up took 0.3–0.6 s through the
  tunnel. A cold Cloud Run instance (about 1 s, §7) fits the hook's 5 s
  budget. Slice 2 measures that for real.

**Decision: an HTTP hook calling our Go API,** as planned. Rejected:

| Option | Why not |
|---|---|
| A Postgres-function hook | No network hop and no signing secret. But the code would live in a table and be rotated by an `UPDATE` on production, outside Git. The function needs grants to `supabase_auth_admin`, a role the local container doesn't have: the same drift `data-model.md` §8.1 avoided. And the check would be PL/pgSQL with no Go tests |
| The `standardwebhooks` Go library | A dependency for about 40 lines of standard library |
| A Supabase Edge Function | A third runtime (Deno) |

**What Slice 2 inherits:**
- **Two new secrets in Secret Manager** (§7), each pinned in the Makefile:
  - `madam-invite-code` (the shared code, `spec-v1.md` §2);
  - `madam-auth-hook-secret` (the hook's signing secret).

  `madam-run` gets `secretAccessor` on each one, bound on the secret as for
  `madam-database-url`. Pe'er adds the values in his own terminal. Rotating
  the code is a new version plus a pin-move PR.
- **The hook endpoint** lives in the `/api` subrouter, public, and signature
  first. It answers 200 for allow and for refuse, and never logs the IP or
  the code.
- **The code stays in every token.** `user_metadata` is editable by its own
  user (`updateUser`), so it is never evidence of anything. Whether to clear
  it after sign-up is Slice 2's decision.
- **JWT verification** works against the project's JWKS (ES256, by `kid`),
  not a shared secret. Which code does that is a dependency decision.
- **Only the email provider is on, and anonymous sign-ins are off.** Admin
  creation skips the hook, so the dashboard's "Add user" and "Invite user"
  are the operator's way around the code. That is by design.

**Dashboard settings** (Authentication is configured in the dashboard, not by
migration). Recorded so production can be set up the same way:

| Where | Setting | madam-dev during the spike | Production (Slice 2) |
|---|---|---|---|
| Authentication → Sign In / Providers | Allow new users to sign up | On | On (the hook is the gate) |
| ″ | Confirm email | On | On (`spec-v1.md` §2) |
| ″ | Allow anonymous sign-ins | Off | Off |
| ″ | Providers | Email only | Email only |
| Authentication → Hooks | Before User Created, HTTPS | The tunnel URL + `/api/hooks/before-user-created`; generated secret into `.env`. **Deleted after the spike** | The Cloud Run URL + the same path; generated secret into `madam-auth-hook-secret` |
| Project Settings → API Keys | Publishable key | Read, never the secret key | Read, for the frontend |
| Authentication → Emails → SMTP Settings | Custom SMTP | Off (the built-in sender, team members only) | **On:** Gmail SMTP, a dedicated account's app password, sender name "Madam" (13 November 2026) |
| Authentication → Emails → Templates | Confirm signup, Reset password | Supabase's defaults | The subjects and bodies in `deploy/email/` (`README.md` there says how) |
| Authentication → URL Configuration | Site URL, redirect allow-list | `http://localhost:5173`, plus `/reset-password` | The Cloud Run URL, plus `/reset-password` |

While the hook points at a dead address, every sign-up fails (M9). Delete or
fix it; never leave it dangling.

### Decided in the Slice 2 plan (12 November 2026)

Decisions only; nothing here is built yet. "As built" follows when it is.
Facts marked *docs* come from Supabase's documentation and are not yet
measured by us.

| Question | Choice | Why | Rejected |
|---|---|---|---|
| Verifying tokens in Go | `github.com/golang-jwt/jwt/v5` for parsing and claims, plus our own JWKS cache (about 60 lines) | Token parsing and the algorithm allow-list are where homemade bugs live. The library has no dependencies of its own (*to verify when added*) | Standard library only (we'd write the parser); `lestrrat-go/jwx` or `MicahParks/keyfunc` (several transitive dependencies for the fetching) |
| The keys | Fetched on the first authenticated request, kept in memory, refreshed after 10 minutes; an unknown `kid` causes one refetch, at most once a minute; a failed refresh keeps the cached keys | The server starts without Supabase, as the database pool does. *Docs:* the JWKS is at `<project-url>/auth/v1/.well-known/jwks.json`, cached 10 minutes at Supabase's edge | Fetching at startup |
| What a token must pass | `alg` exactly `ES256`; a known `kid`; `iss` = `<project-url>/auth/v1`; `aud` = `authenticated`; `exp` present, 30 s leeway; `sub` a UUID; `role` = `authenticated`; not anonymous. `user_metadata` is never read | A fixed algorithm closes `alg: none` and HS256-with-the-public-key | Trusting the token's own `alg` |
| Keys unavailable | 503, not 401 | "We can't check" is not "you're not allowed" | |
| The hook in production | The spike's handler, with: a refusal always HTTP 200; logs carry `allowed`, `webhook-id` and the duration, never the code, email, payload or IP; the route always mounted; both secrets **required at startup**, with throwaway local defaults in the Makefile | One code path. A missing secret stops the revision from starting, and the old one keeps serving | Mounting the route only when the secrets are set (a missing secret would look like a 404) |
| Our `users` row | Created by the first authenticated `GET /api/me`: `INSERT … ON CONFLICT (id) DO NOTHING`, then the read, in one `InUserTx` | Idempotent; the policy's `WITH CHECK` already allows only the caller's id. **No migration in Slice 2** | In the hook (it runs before the account exists and before the email is confirmed); at onboarding (every handler would need a "no row yet" case) |
| The code left in `user_metadata` | The page clears it once: `updateUser({data: {invite_code: null}})`, best effort | The code then lives only in Secret Manager | Accepting it (a leaked token would leak the shared code); clearing it server-side (needs an admin key) |
| The browser's client | `@supabase/auth-js`, the auth part of supabase-js. Exact version and gzipped size recorded when installed | Session storage, refresh, tabs and recovery links are solved there | Hand-written `fetch`; the Go server proxying Auth (cookies and CSRF, and every sign-up from Cloud Run's IP); the full `@supabase/supabase-js` |
| Email-link flow | Implicit (*to verify* that it is auth-js's default) | *Docs:* a PKCE link works only in the browser that started it; she may sign up on one device and open the email on another | PKCE |
| The page's configuration | `GET /api/config` returns the project URL and the publishable key from environment variables | One image for every environment | `VITE_*` at build time: every image would carry production's values |
| CSP | `connect-src` gains the project's origin, built from `SUPABASE_URL` at startup | The browser calls Supabase Auth itself | |
| Sessions | `localStorage`, auth-js's default | She stays logged in between mornings. The CSP (no inline script, `'self'` only) is the XSS guard | Memory only (a log-in every morning) |
| Production email | **Gmail SMTP** as custom SMTP on madam-prod: a dedicated account with an app password, kept only in Supabase's dashboard | *Docs:* the built-in sender delivers **only to the project's team members, 2 emails an hour**; with custom SMTP the limit starts at 30 an hour | A transactional provider (needs a domain we don't have yet); admin-made accounts with no email (breaks `spec-v1.md` §2) |
| Development email | madam-dev keeps the built-in sender, with Pe'er's own address | He is a team member; 2 an hour is enough for testing | |
| Account deletion (Slice 9) | A Supabase **secret key** in Secret Manager, added in Slice 9, used by one handler for `DELETE /auth/v1/admin/users/{id}`. Our row first (it cascades), then the identity | It can be revoked by itself, and the Data API is off. If the second step fails, she has an empty account and can retry; the other order could leave health data with no way to log in | A `SECURITY DEFINER` function on `auth.users` (the local container has no `auth` schema, §8.1 of `data-model.md`); deleting by hand in the dashboard; an Edge Function |

**Redirects.** Both local and `DB=dev` use madam-dev's Auth, because the
local container has none. madam-dev's hook stays deleted, so development
sign-up isn't gated by the code.

| Project | Site URL | Redirect allow-list |
|---|---|---|
| madam-dev | `http://localhost:5173` | `http://localhost:5173/reset-password` |
| madam-prod | `https://madam-966460417479.europe-west1.run.app` | the same, with `/reset-password` |

**Secrets** (§7): `madam-invite-code:1` and `madam-auth-hook-secret:1` bring
the active versions from 2 to **4 of the free 6**; Slice 9's key makes 5.

**Production order:** the dashboard settings above → the hook created in
madam-prod, pointing at Cloud Run (sign-ups fail closed until the handler is
live, M9) → both secrets added by Pe'er → the handler's PR. Before the
frontend's PR: Gmail SMTP, Site URL and the redirect list.

### As built in Slice 2: the backend (12 November 2026, PRs #29–#31)

The Go half of Slice 2 is live. The browser half (the screens, `/api/config`,
the CSP change, email) is not built yet, so nobody can sign in through the
app: everything below was measured with `curl` and tests.

**The verifier** (`internal/api/jwt.go`, `jwks.go`), as decided above.
- `github.com/golang-jwt/jwt/v5` **v5.3.1**. Its own `go.mod` has no
  `require` lines (checked), so it adds one module.
- **Measured on both projects:** `/auth/v1/.well-known/jwks.json` holds one
  `ES256` / `P-256` key with a `kid` and `use: sig`, served with
  `cache-control: public, max-age=600`. The 10 minutes the cache was built
  around is now measured, not only documented.
- With no keys at all (Supabase unreachable since the instance started), a
  request with a token gets **503**, not 401. Measured on the built image
  with an address that doesn't resolve.
- The project is `SUPABASE_URL`: a plain flag in `make deploy` for
  production, and `MADAM_DEV_SUPABASE_URL` in `.env` locally, with either
  database. The server refuses to start without a well-formed one.

**`GET /api/me`** (`internal/api/me.go`) is the first route behind
`RequireUser`. It runs `EnsureUser` (`INSERT … ON CONFLICT (id) DO NOTHING`)
and `GetUser` in one `InUserTx`, and answers `id`, `temperature_unit` and
`onboarded_at`. `ON CONFLICT DO NOTHING` works as `madam_app` under the
forced policy (the test calls it twice). No migration.

**AC-16 part one** is `TestIsolationThroughTheAPI` (`me_test.go`): the real
router, the real verifier (with a test key) and a real database as
`madam_app`. The API has no parameter that names a user, so the test puts
B's id everywhere a request has room for one (query string, headers, a
cookie, the path, a rewritten or self-signed token, `user_metadata`) and
checks that the answer is still A's or a refusal, and that B's row is
untouched.

**The hook** (`internal/api/hook.go`), as decided above, plus:
- The code is compared as two SHA-256 hashes in constant time. A direct
  constant-time compare returns at once when the lengths differ, which would
  show the code's length.
- The server refuses an invite code that starts or ends with a space or a
  newline. The comparison is exact, so such a code could never be typed, and
  every sign-up would be refused with nothing to show why.
- **Go's base64 decoder skips newlines.** A signing secret stored with a
  trailing newline decodes to the same key, so it is harmless there.
- Locally and in CI the two values are throwaway ones from the Makefile and
  `ci.yml`. `.env` can override them (`INVITE_CODE`, `AUTH_HOOK_SECRET`).

**Measured on production** (madam-prod's hook pointing at Cloud Run, sent
with `curl` and the publishable key as supabase-js would):

| Attempt | The client saw | Time, end to end |
|---|---|---|
| Sign-up, wrong code | 400 `Invalid invite code.` | 2.35 s on a **cold** instance (0.32 s of it inside Cloud Run) |
| Sign-up, no code | the same | 0.80 s |
| `/otp` with `create_user: true`, no code | the same | 0.59 s |
| Anonymous sign-up | 422 `anonymous_provider_disabled`, before the hook | 0.65 s |

- The body is `{"code":400,"error_code":"unknown","msg":"Invalid invite
  code."}`. **`error_code` is `unknown`**, so the page must recognise the
  refusal by its status and message.
- A cold start fits the hook's 5 s budget with room to spare. The handler
  took 21 µs to 1.7 ms.
- Each refusal logged `allowed=false`, a webhook id and a duration, and
  nothing else. Our message coming back also proves the signing secret: a
  wrong one would have been a 401 from us and a 500 from Supabase.

**Secrets:** `madam-invite-code:1` and `madam-auth-hook-secret:1`, Belgium
only, `madam-run` the only reader of each. Active versions: **4 of the free
6**. `make deploy` now passes three secrets and one plain variable.

**How the tests were checked.** For each check in the verifier, the route
and the hook, the check was removed and a test had to fail (35 deliberate
breakages over the three PRs). Three went unnoticed at first, and each miss
said something:
- Removing the algorithm allow-list changed no outcome, because the library
  refuses `none` and HS256 for an ECDSA key by itself. The allow-list's own
  effect is refusing *before any key is fetched*, and that is what the test
  now asserts.
- A token with no `kid` was refused anyway (it is never in the key set); the
  same assertion covers it.
- The oversized key set had no keys in it, so it failed for that reason.
  With a valid key inside, removing the explicit size check still changes
  nothing: the capped read cuts the JSON short, so it is refused either way.
  That one stays undetectable; the cap itself holds.

**Not measured yet:** a sign-up with the right code on production; a real
Supabase token sent to a running server; a hook host that answers nothing at
all (M9's open case).

### After the first real use (13 November 2026)

Pe'er signed up, confirmed, logged in, reset his password and logged out on
production, on a phone and a computer. The log agrees: `GET /api/me`
answered 200 seven times, so **the server has now verified real Supabase
tokens**, and the hook allowed exactly one sign-up. What that test changed:

- **Sign-up now says when an address already has an account** ("This email
  already has an account. Log in instead."). This reverses the plan's "the
  same answer either way" for sign-up only. Supabase itself hides it: for an
  address with a confirmed account it answers "success" with a made-up user
  that has **no identities**, creates nothing and sends no email. The page
  recognises that answer (`alreadyRegistered` in
  `frontend/src/lib/authText.ts`). Chosen by Pe'er because the silent
  version left a real person waiting for an email that never comes.
  **Accepted cost:** anyone can learn from the sign-up form whether an
  address has an account, and the log suggests the invite code isn't needed
  for that (the second sign-up never reached our hook; one observation).
  For an app about fertility that is a real disclosure; revisit it before
  sign-up opens beyond the invite code (`spec-v1.md` §15).
- **Password reset still gives one answer for every address.** Supabase
  answers 200 either way (measured), so the page can't know. Telling would
  need a server endpoint with an admin key whose only job is to answer
  "does this address exist": rejected.
- **A reset link opened in another tab stranded the first tab** on "Choose a
  new password". The link opened in the mail app's own tab, which shares
  storage with Chrome. The client announces `PASSWORD_RECOVERY` to every
  open tab, and the page treated the announcement as "this tab is
  recovering". Now only how *this* tab was opened decides
  (`arrivedByRecoveryLink`). Reproduced on production with two tabs, then
  seen fixed.
- **"Send it again" waits 60 seconds** after each email, the first included,
  and shows the time left. Supabase has its own limit per address.
- **The confirmation email landed in spam.** It is sent through Gmail's own
  servers, so the likely causes are a new sender and a short message with
  one link to `supabase.co` (*not measured*). Both "check your email"
  screens now say to look in spam. The real fix is a domain of our own with
  a transactional sender (rejected for now, above).

### Rejected, and when each would win

| Option | Type | Its sweet spot |
|---|---|---|
| Keycloak | Self-hosted (Java) | Enterprise SSO across many internal apps |
| Zitadel | Self-hosted or cloud (Go) | B2B multi-tenant SaaS; strong runner-up |
| Ory Kratos | Self-hosted (Go) | Full control and a fully custom login UX |
| Authentik | Self-hosted (Python) | Homelab / internal SSO |
| Auth0 | Managed | Mature and enterprise-friendly; costly at scale |
| Clerk | Managed | Fastest React setup; invite flow is per-email, not one shared code |
| Firebase / Cognito | Managed | Teams already inside Google or AWS |
| authboss / DIY | Go library | Learning, or teams with unusual needs |

Self-hosting an identity server was rejected deliberately: it is a second service
with its own database, backups, TLS, email and security updates, and **if it is
down, nobody can log in**. A good future learning project, a bad first-project
side quest.

In-app libraries were rejected because assembling auth is where homemade security
bugs live. `authboss`'s own documentation notes it no longer handles CSRF, which
you must add yourself.

**Reversibility:** all serious options speak the same standard protocols (OIDC,
JWT), so our verification middleware looks nearly identical either way. Switching
later means migrating accounts — annoying, not catastrophic, at this scale.

**Account deletion** must wipe our data *and* call the auth provider to delete the
identity.

---

## 5. Database: Supabase PostgreSQL (EU)

### Engine

PostgreSQL. The data is relational, and Postgres features map directly onto the spec:

- **AC-4, one entry per date** → enforced by the database itself even if
  application code has a bug. Step 3 made `(user_id, entry_date)` the primary key,
  so the rule *is* the key rather than a separate unique constraint.
- **Enum types and `CHECK` constraints** for the closed-value fields, so an
  invalid bleeding level or disturbance tag cannot be stored at all.
- **Array columns** (`TEXT[]`) for disturbance tags and symptoms, which most other
  databases cannot express — this is what keeps the 90-day graph query to a single
  join-free read.
- **`ON DELETE CASCADE`** so account deletion cannot leave data behind.
- **Row Level Security** as a second isolation layer, decided at step 4
  (`data-model.md` §9).

*Superseded by step 3:* this section previously planned an **exclusion constraint**
on date ranges for §7's non-overlapping measurement-time periods, and **exact
decimal types** for temperatures. `data-model.md` chose differently in both cases,
with reasons: periods store a start date only, which makes overlap inexpressible
rather than merely forbidden (and drops the `btree_gist` dependency); temperatures
are integers in thousandths of a degree, because Go has no decimal type and
`NUMERIC` would force a third-party library into the domain logic. See
`data-model.md` §8.2 and §4.4.

**Version: PostgreSQL 17** (17.11 at setup). The local development container uses
the `postgres:17` image to match, so a migration that works on the laptop can't
fail in production on syntax the server doesn't support.

Rejected: SQLite (awkward on cloud hosts, weaker constraints), MySQL (no
advantage here), MongoDB (the data is not document-shaped).

### Hosting and region

The PostgreSQL instance that comes with the Supabase project, in an **EU region**
— close enough to Israel for good latency, and expected to simplify the GDPR
check (to be confirmed by the legal research, not assumed).

**Region: Frankfurt (`eu-central-1`)**, for both projects, created during setup.
Cloud Run sits in Belgium (`europe-west1`, §7), so every query crosses from Belgium
to Frankfurt. Supabase may offer Paris, which is marginally closer on the map;
the difference is expected to be a millisecond or two per query, so the projects
were not recreated. *Assumption, to measure in Phase 0c:* the Belgium–Frankfurt
round trip is invisible to a person at this request volume.

**Two projects, both free:** `madam-dev` and `madam-prod`. The free plan's limit
is two free projects **per person**, counted across every organisation where
Pe'er is an owner or administrator; paused projects don't count. Both slots are
now in use, and the keepalive job (below) keeps them from pausing, so a future
project means pausing or deleting one of these, or paying.

**Project options chosen at creation:**

- **Data API: off.** Supabase also offers an auto-generated Data API that can
  expose tables directly to browsers. We do not want that: our Go API is the only
  door. Verified off on both projects after creation.
- **"Automatically expose new tables": off.** The same reasoning. From **30 May
  2026** this is the default for new projects anyway, and apps using a direct
  PostgreSQL connection are unaffected. *Measured in 0c:* it doesn't stop
  Supabase's default privileges from granting `anon`, `authenticated` and
  `service_role` full access to new tables in `public`. Those grants are
  dormant with the Data API off; 0d revokes them (`data-model.md` §9).
- **"Enable automatic RLS": off.** It installs an event trigger that enables RLS
  on every new table in `public`. Rejected because it is schema behaviour that
  lives outside Git, the local Postgres container wouldn't have it, and every
  migration already enables **and forces** RLS explicitly — the trigger only
  enables it, and writes no policies. Reversible later with SQL.

**Database roles (step 4).** Because RLS is in, the API must connect as a role
that **does not own the tables**, and the tables carry `FORCE ROW LEVEL SECURITY`.
Postgres lets table owners bypass row security unless forced, so connecting as the
owner would mean RLS is enabled and doing nothing. *Measured in 0c:* goose's
tables are owned by `postgres`, which is not a superuser but **has
`BYPASSRLS`**, so as `postgres` even forced RLS is skipped. It also has
`CREATEROLE`, so 0d can create the application role in a migration. Details in
`data-model.md` §9.

*As built in 0d:* migration `00002` creates **`madam_app`** (`LOGIN`,
`NOBYPASSRLS`, owns nothing) **with no password**, because the repository is
public. The password is set out of band:
- **On Supabase,** by Pe'er, in psql connected as `postgres`:
  `\password madam_app`. psql hashes it (SCRAM) on the laptop and sends only
  the hash, so the plaintext reaches no server log, file or conversation.
  `postgres` may do this because PG16+ gives a `CREATEROLE` role ADMIN on the
  roles it creates (measured on madam-dev: ADMIN yes, INHERIT and SET no).
  Letters and digits only, like the other passwords, and a different one per
  project.
- **Locally and in CI,** by `make migrate-up` (DB=local): the throwaway
  password `madam_app`, like the owner's `madam:madam`.

**Through the pooler, a custom role's user name is `madam_app.<project-ref>`**
(Supabase's docs, "Connecting to Postgres"), like `postgres.<project-ref>`.
The role's `VALID UNTIL` must not have passed (it's empty: never). After a
password change the pooler can take about 15 s to notice.

So every database has two URLs: the owner's for migrations, the app role's for
everything else. Locally `madam:madam` and `madam_app:madam_app`; on madam-dev
`MADAM_DEV_MIGRATE_URL` and `MADAM_DEV_DATABASE_URL` in `.env`; in production
the secrets `madam-migrate-url` and `madam-database-url` (§7).

### Connecting

Supabase offers several connection methods, and the choice is forced by the
network:

| Method | Port | Network | Notes |
|---|---|---|---|
| Direct connection | 5432 | **IPv6 only** on the free plan (IPv4 is a paid add-on) | Supabase's recommendation for long-running servers |
| Shared pooler, **session mode** | 5432 | IPv4 | Behaves like a direct connection |
| Shared pooler, transaction mode | 6543 | IPv4 | **No prepared statements** — pgx uses them by default, so they must be switched off |

- **The development machine has no IPv6** (WSL's default networking), so from the
  laptop the only working option is the **session pooler**. A connection through
  it was tested from WSL during setup and works.
- **Cloud Run uses the session pooler too** (decided in Phase 0c). Rejected:
  - *The transaction pooler.* It packs many clients into few server
    connections, which we don't need (at most 8). It needs pgx switched off
    prepared statements, and the laptop and production would then behave
    differently.
  - *The direct connection.* Google's docs say Cloud Run reaches IPv6 only
    through Direct VPC egress on a dual-stack subnet with external IPv6. That
    means a VPC, a subnet, a new IAM role (`compute.publicIpAdmin`) and three
    more deploy flags, all to save one hop. Because we wouldn't take it anyway,
    Cloud Run's IPv6 was not tested directly.
- Connection strings and database passwords live in Pe'er's password manager,
  never in the repository. The passwords use letters and digits only, so they can
  sit inside a connection URL without escaping.

**TLS: `sslmode=verify-full`, against Supabase's own root CA** (Phase 0c).
- The poolers (`aws-0-` and `aws-1-eu-central-1.pooler.supabase.com`) present a
  `*.pooler.supabase.com` certificate chained to **Supabase Root 2021 CA**, a
  private CA (expires April 2031). It is not in any system trust store, so
  verification needs that root.
- The root is committed as `backend/certs/supabase-root-2021-ca.crt`. It was
  downloaded from the dashboard, and its SHA-256 fingerprint
  (`80:70:25:AD:…:CA:FA`) matched the root the pooler served. Verifying it
  against an independent copy is what makes it a trust anchor.
- `PGSSLROOTCERT` points pgx at it: set in the Dockerfile, and by the Makefile
  for `DB=dev` and `migrate-prod`. It's not set globally, because pgx reads the
  file **even with `sslmode=disable`**, and the local container shouldn't need it.
- Rejected: `sslmode=require`. It encrypts but accepts any certificate. pgx's
  default, `prefer`, can even fall back to plain text.
- **The server enforces it** (`cmd/server/db.go`). It refuses to start when
  `DATABASE_URL` isn't a `postgres://` URL. It also refuses when a host that
  isn't local (loopback, Unix socket, or a name without a dot) isn't using
  `verify-full`. The first check exists because pgx reads any other string as
  `key=value` pairs and ignores what it doesn't know, which turned a broken
  `.env` line into a silent attempt on a local socket.
- The check paid off on the first production deploy: secret version 1 lacked
  `?sslmode=verify-full`, the revision refused to start, and Cloud Run kept
  serving the old one. `make migrate-prod` has no such check yet, so the one
  migration run with that value (`00001`) connected encrypted but unverified.

**The pool** (`cmd/server/db.go`):
- `MaxConns` **4**. With at most 2 instances that's 8 connections. In session
  mode each one occupies one of the pooler's server slots for its lifetime, and
  the dashboard shows a pool size of **15** for both projects. That leaves room
  for migrations and the dashboard.
- `MinConns` **0**. The pool connects on first use, so a database outage can't
  stop the server from starting, and an idle instance holds nothing open.
- `application_name=madam`, which makes the app visible in `pg_stat_activity`
  **locally only**. *Measured in 0d:* through Supabase's pooler every client
  connection shows as `Supavisor`, so on Supabase the app is told apart by its
  role, `usename = madam_app`.
- Everything else is pgx's default. On acquire, pgx pings any connection idle
  for more than 1 s before using it. That matters here because with
  request-based billing Cloud Run throttles the CPU between requests, so pgx's
  background health checks barely run.

**Latency, measured in 0c** (Cloud Run's request log, `httpRequest.latency`,
which is time inside Google):

| `/api/health` | Median |
|---|---|
| 0b, no database (baseline) | ~2 ms |
| One database round trip (requests under 1 s apart) | **13.3 ms** |
| Ping plus query (requests 2 s apart) | 25.0 ms |

So a Belgium → Frankfurt round trip through the pooler costs about **11 ms**.
This confirms the step-2 assumption that it's invisible to a person. From the
laptop in Israel the same request took ~70 ms to madam-dev.

### Plan and backups

**Free plan, confirmed by Pe'er at step 4.** Pro (~$25/month, daily backups) stays
available any month it's wanted; nothing about the free choice is one-way. His
reasoning: the project's primary goals are learning and a portfolio piece, and the
first user uses the chart mainly to track the present.

Two facts that come with the free plan:

- Free projects **pause after a week of inactivity** (see below).
- **No backups on the free plan.** A database loss loses *everything*, including
  the history behind the all-time stats. Nothing is snapshotted while a project
  sits paused either — pausing and backups are separate problems.

### What pausing actually means

*Checked against Supabase's docs in 0f* (7 November 2026, "Project
pausing" and its FAQ): "A Free plan project is considered inactive if it does
not receive sufficient **user database activity** over the past week", and
"Projects with too few user queries during that window are the clearest
candidates for pausing". "Typically a few user requests to the database each
day over the previous week is enough", and "Opening the project in the
dashboard or sending real requests from your app both count as activity",
"though that isn't a guarantee". The owner gets "A warning email roughly one
week before the pause takes effect", and another once it happens.
*Unverified:* how Supabase counts user queries (which roles, what threshold),
and whether requests that never reach the database, such as Auth's, count.
The docs don't mention keepalive pings either way. (This section used to say
any API request counts; the docs say database activity.)

1. **Paused.** The database volume is frozen in place: no reads, no writes, no
   connections accepted. **The data is still on disk — a pause is not a
   deletion.** Storage files are preserved too. From the app's side it just looks
   broken until restored.
2. **Restore.** A button in the Supabase dashboard. Takes a few minutes; the
   project returns with the **same URL and keys**, so no config changes are needed.
   Supabase's current docs state a **one-year window** to restore from within
   Supabase Studio, because backups are retained only for a limited period and
   platform changes may not be backward compatible with older backups.
   *Caution:* a 2024 changelog announced a **90-day** window and many blog posts
   still repeat that figure. The docs are the better source, but treat the exact
   number as changeable rather than something to rely on. *Re-checked in 0f*
   (7 November 2026): "there is a 1-year window to restore", and 90 days no
   longer appears on the pausing pages.
3. **Past the window.** The automatic path closes but the data is not gone: the
   database backup file and Storage objects can be **downloaded from Project
   Overview** before deletion, then loaded into a fresh project. The URL and keys
   change, so the app's config must be updated.
4. **Deleted.** Once a project is genuinely deleted, all associated data including
   backups is permanently removed and cannot be recovered. This is the only true
   point of no return, and it does not happen by surprise inside the restore window.

*Unverified:* scattered user reports describe a table returning empty after a
restore. The thread responses suggest user error is the likelier explanation, but
"restore worked" is not an assumption to make without checking.

### Mitigation: a keepalive ping

**Now load-bearing, not optional**, because the free plan is confirmed: a paused
project is a broken app on the morning she tries to log a temperature.

What counts is a **query on the database** (above), not an HTTP request as
such. A scheduled GitHub Actions workflow makes each project answer one,
at no cost.

*As built in 0f* (`.github/workflows/keepalive.yml`; the workflow itself is
in §12):

| Project | How | Credential |
|---|---|---|
| madam-prod | `curl` of the public `/api/health`, which runs `SELECT 1` as `madam_app` and answers `200` only if it succeeded | **None.** No production credential is in GitHub |
| madam-dev | `psql "$URL" -c 'SELECT 1'` as `madam_app`, `verify-full` against the committed root CA | The repository secret **`MADAM_DEV_DATABASE_URL`**, the same URL as in `.env` |

**If the dev secret leaks:** `madam_app` on madam-dev, nothing in production.
RLS does **not** contain it: whoever holds the role's password can call
`set_config('madam.user_id', …)` themselves, so they can read and write any
row they can name, and hold the pooler's 15 slots. RLS guards against the
app's own bugs, not against a stolen app credential. madam-dev holds fake data
only (§9), which is why this was accepted. **To rotate:** `\password
madam_app` on madam-dev (§5, Database roles), then the `.env` line, then `gh
secret set MADAM_DEV_DATABASE_URL --repo peerbenmoshe/madam` (hidden prompt).
Until the secret changes, the keepalive's `dev` job fails. Fork PRs and
Dependabot runs never receive repository secrets.

Rejected for madam-dev:
- **A new role with no privileges,** for a smaller leak. It costs a migration
  and an extra PR, to protect fake data.
- **Supabase Auth's health endpoint with the publishable key.** It probably
  makes no database query, so it likely doesn't count.
- **The REST endpoint.** The Data API is off.
- **Letting madam-dev pause.** §9 keeps both projects awake.

**Measured in 0f.** In `pg_stat_statements` (schema `extensions`, readable by
`postgres`), one keepalive run added exactly one call to `madam_app`'s `Ping`
entry and one `DISCARD ALL` (the pooler ending the session). The keepalive's
`SELECT 1` has no row of its own: Postgres files statements by a fingerprint
that ignores constants and column aliases, so it's counted under the app's
`-- name: Ping … SELECT $1::integer AS ok`, the text seen first. Whether
Supabase's pause counter reads these numbers is unverified.

**The 60-day rule.** GitHub's docs: "In a public repository, scheduled
workflows are automatically disabled when no repository activity has occurred
in 60 days." They don't define "repository activity" (assumed: pushes and
merges) and document no email when it happens. Nothing automates against
it:
- During the build, merges, including Dependabot's weekly PRs, keep the
  repository active.
- Once the first user logs daily, production stays awake through real use.
- The backstops are the banner on the Actions tab and Supabase's own warning
  email, a week before a pause.
- **Revisit after v1** if the repository may go quiet for two months: then
  re-enable with `gh workflow enable keepalive.yml`, or add something.

Rejected:
- **A commit-based keepalive.** `main` is protected with no bypass, so it
  couldn't push, and it would clutter the history.
- **The workflow re-enabling itself.** It needs `actions: write`, and whether
  that resets the 60 days is undocumented.
- **A third-party action.** A new dependency.

**Post-v1, free:** an automated nightly `pg_dump` to encrypted storage. Costs
nothing and turns "database loss = everything" into "database loss = one day". A
good interview talking point too.

---

## 6. Database access: sqlc + goose

**sqlc** reads our SQL queries plus the schema and generates type-safe Go
functions. We write `SELECT … WHERE user_id = $1 AND date BETWEEN $2 AND $3` in a
`.sql` file and call `ListEntriesInRange(ctx, userID, from, to)` from Go. Mismatches
with the schema surface at generation time, not in production.

**goose** manages migrations: numbered SQL files describing how the schema was
built and changed. Applying them in order brings any database (laptop, production)
to the same state. **Nobody edits the schema by hand in a dashboard** — that change
would be invisible to Git. RLS policies and the `updated_at` trigger are migrations
like any other.

sqlc reads the goose migration files as its schema source, so the two cooperate.

Why this over the alternatives:

- **All database code stays in SQL, which Pe'er already knows well**, while Go is
  still new. The most security-sensitive review question — "does every query filter
  by `user_id`?" — can be answered by reading `.sql` files, no Go needed.
- **No magic.** An ORM (GORM) writes the SQL for you and hides exactly the part
  worth seeing; debugging surprises means learning the ORM's internals.
- Raw `pgx` was rejected as repetitive, with column-name typos only failing at runtime.

Known weakness: highly dynamic queries (many optional filters in any combination)
are awkward in sqlc. Madam's queries are fixed shapes, so this barely applies.

Driver: `pgx`.

### As built in Phase 0c

**Versions:** pgx **v5.12.0**, goose **v3.29.0**, sqlc **v1.31.1**, the newest
on 4 November 2026. pgx 5.12.0 was four days old; the fallback, if it misbehaves,
is v5.11.0.

**Tools** live in `tools/go.mod`, as `tool` directives. The Makefile runs them
with `go -C tools tool goose` and `go -C tools tool sqlc`, using absolute paths
(`-C` makes go act from inside `tools/`, and must be its first flag). Their
dependency trees (ClickHouse, Azure, SQLite drivers, and many more; 346 lines of
`go.sum`) stay out of the app's module. sqlc needs cgo, because its SQL parser
is C, so building it needs a C compiler; the laptop and CI's runner both have
one. The first build of each tool takes about 80 s, after which Go's build
cache keeps it.

**sqlc** (`backend/sqlc.yaml`):
- Reads `migrations/` as the schema, skipping goose's `Down` sections, and
  `internal/db/queries/*.sql` as the queries.
- Generates package `db` into `internal/db` with `sql_package: pgx/v5`.
- No type overrides yet (`data-model.md` §10).

**The first query** is `Ping` (`SELECT 1::integer`), called by `/api/health`.
It deliberately reads no table:
- A query on `users` would return nothing without the 0d transaction wrapper,
  once RLS is forced.
- `count(*)` would publish the number of users.
- `goose_db_version` isn't part of sqlc's schema.

That is why `CLAUDE.md`'s rule reads "every query that reads or writes a table
holding user data filters by `user_id`".

**Migration files** are numbered sequentially (`make migrate-new NAME=…` passes
goose's `-s`), not by timestamp. Each has a `Down` section for local use;
nothing runs down migrations against production.

**How migrations reach production: from the laptop, `make migrate-prod`.**
1. It refuses uncommitted or untracked files and anything but the latest
   `main` (the same guard as `make deploy`; since 0c′).
2. It reads the owner URL from the Secret Manager secret `madam-migrate-url`
   with the developer's own `gcloud` login, into one shell's memory only.
3. It refuses the URL unless it's `postgres://…` with `sslmode=verify-full`,
   exactly once (`deploy/check-db-url.sh`; since 0c′). It's the server's rule,
   which `migrate-prod` lacked in 0c, so migration `00001` ran over unverified
   TLS. The script reads the URL on stdin and never prints it.
4. It prints `goose status`.
5. It applies the migrations only after `madam-prod` is typed at the prompt.

Its secret is separate from the app's (§7) because from 0d the two hold
different roles: migrations run as the owner, the app as the non-owner role.
`madam-run` can't read it.

Rejected: migrating from CI. In 0c it would have meant a long-lived owner
credential in GitHub. *Revisited in 0c′, still rejected:* with Workload
Identity Federation (§7) it would need no stored key, but every step of that
job, third-party Actions included, would hold the owner URL (`BYPASSRLS`,
schema changes) with no typed confirmation, exposed to the Actions flakiness
mid-migration.

**Order, now that every merge deploys (since 0c′): a schema change is two
PRs.**
1. The migration alone. Its merge deploys unchanged code, which is fine:
   a migration must keep the running code working anyway.
2. `make migrate-prod`, from an up-to-date `main`.
3. The code that needs it. Its merge deploys it.

So production only ever runs migrations that are on `main`. Rejected: one PR,
migrating from its branch just before merging. If the PR then changed or was
abandoned, production would hold a migration `main` doesn't have.
*Later, maybe:* the app refusing to start when the schema is older than it
needs, which would turn a forgotten step 2 into a safely failed deploy.

**`goose_db_version`** is goose's bookkeeping table, created on first use and
owned by `postgres`. It lives in `public` alongside our tables.

---

## 7. Hosting: Google Cloud Run

The API is packaged as a **container** and run by Google. It scales to zero when
idle and wakes on the next request.

**Region: `europe-west1` (Belgium)**, decided during setup. Cloud Run's free tier
is a dollar amount computed at **Tier 1** prices; Frankfurt (`europe-west3`) is a
**Tier 2** region, so the same free allowance buys fewer seconds there. Belgium is
Tier 1 and in the EU. At Madam's volume either region would stay free — Belgium
was chosen as the margin of safety, at the cost of slightly more distance to the
database (§5).

### Why Cloud Run

- The always-free monthly allowance, with **request-based billing** (the default):
  **180,000 vCPU-seconds, 360,000 GiB-seconds and 2 million requests**. It resets
  monthly rather than expiring, is aggregated per billing account, and is applied
  as a spending-based discount. Madam's expected use is a few hundred vCPU-seconds a
  month (*estimate*, not measured).
- Automatic HTTPS, no server administration, no certificate management.
- Containers are the industry-standard packaging format and a genuinely portable
  skill.

### What the free tier does *not* cover

Checked against Google's terms and pricing pages in October 2026.

- **Outbound data to Israel.** Cloud Run's free egress is 1 GiB per month **within
  North America** only. Traffic from Belgium to her phone is billed at Premium tier
  internet rates — about $0.12/GiB according to a secondary source; the exact rate
  for Middle East destinations is unverified. Even a pessimistic 50 MB a month is
  well under one cent.
- **Artifact Registry**, where the container images live: free up to **0.5 GB per
  billing account**, then $0.10 per GB per month. Every deploy adds an image and
  nothing deletes old ones automatically. **A cleanup policy keeping the last few
  images is part of the skeleton.**
- **Cloud Build** is billed separately. It is what `gcloud run deploy --source`
  uses behind the scenes. We build the image ourselves (locally or in GitHub
  Actions) and push it, so Cloud Build is not involved, and its API is not
  enabled. *Its free tier was not checked.*
- **Secret Manager** (enabled in 0c): $0.06 per active version per location per
  month, and $0.03 per 10,000 accesses. The free tier covers **6 active
  versions and 10,000 accesses** a month per billing account. **Disabled
  versions count as active; destroyed ones don't.** We use 4 versions since
  Slice 2 (§4; 2 before it), and
  Cloud Run reads the secret once per instance start, so this stays $0. Covered
  by the alert, not by the Cloud Run spend cap.
- **The contract.** Under the Google Cloud Terms of Service (§2.6), Google may
  change Cloud Run's fees at any time; the 30-day notice in that clause applies to
  Workspace-type products only. "Free" is a published price, not a contractual
  promise. The card on file is liable for whatever the bill says.
- **Inactivity termination** (ToS §8.4) needs 60 days with no network activity *and*
  no fees. Daily use and the keepalive job make it irrelevant.

### Cost guardrails (decided and created during setup)

**Budget alerts do not stop spending.** Google's own documentation says an
alerts-only budget does not cap usage — it sends an email. An earlier version of
this document treated "set a budget alert" as protection; it is only a smoke
detector, not a sprinkler. So we use both:

1. **Alerts-only budget, ₪15, on the whole billing account** (`global-alert`).
   Emails at 50%, 90% and 100% of actual spend, plus 100% of forecasted spend.
   It counts savings such as the free tier, so it watches the **real bill**.
   Covers every service, including the ones a cap cannot stop.
2. **Spend cap budget, ₪15, on Cloud Run in the project** (`cloud-run-spend-cap`),
   status **Configured**. When the estimated spend passes the amount, Cloud Run is
   **paused for the rest of the month** until the cap is lifted by hand. It
   resets automatically at the start of the next month.
3. **Minimum instances: 0.** A warm minimum instance is billed while idle — the
   most common way to accidentally pay for a quiet app.
4. **Maximum instances: 2.** The URL and repository are public, and bots will probe
   both. Capping instances caps how fast a flood of requests can spend money.
5. **Request-based billing** (the default). Instance-based billing charges for the
   whole lifetime of every instance and has different free-tier numbers.

The amounts are in shekels because the billing account's currency is ILS; ₪15 is
roughly $4 (exchange rate not checked at the time of writing), in line with the
"about $5" originally planned.

Accepted cost of the cap, chosen by Pe'er: **if it ever triggers, the app goes dark
for the first user** until it is lifted. A dark app is a far better failure than a
surprise bill, and the guardrails above make triggering it unlikely.

Known limits of the spend cap:

- It covers **one service in one project**, so Artifact Registry storage and egress
  are covered by the alert (1) only.
- It does not pause persistent resources such as stored images.
- Google's documentation says it uses **gross, estimated costs, excluding savings
  and credits** — so it counts usage *before* the free-tier discount. A ₪15 cap
  therefore means ₪15 of full-price Cloud Run usage, against an expected gross
  cost of a few cents a month: still a large margin. **Check the first month's
  billing report** to confirm in practice.
- Enforcement isn't instant, because cost reporting lags; a small overshoot past
  the amount can still be billed.
- To lift it: Budgets & alerts → **View spend cap details** → **Lift spend cap**.
  Cloud Run may take up to an hour to return. A budget can't be edited or deleted
  while its cap is enforced.

### As built in Phase 0b

The service is live at `https://madam-966460417479.europe-west1.run.app`. Every
setting below is a flag in `make deploy` (`Makefile`), so Git holds the
configuration and the Console is never used to change it. The exception, since
0c′, is IAM (who may call the app, who may deploy): one-time `gcloud` commands,
each recorded in this section.

**The image** (`Dockerfile`, at the repository root so 0e can add a frontend
stage). Two stages:

- Build: `golang:1.27.3-trixie`, an exact patch tag like the rest of the
  toolchain. The binary is static (`CGO_ENABLED=0`), with `-trimpath` and
  `-ldflags="-s -w"`.
- Runtime: `gcr.io/distroless/static-debian13:nonroot`. It has no shell and no
  package manager, but it does have CA certificates (needed for TLS to Supabase),
  timezone data, and the non-root user 65532 the container runs as. Rejected:
  `scratch` (we'd have to add the certificates and the user ourselves) and Alpine
  (a shell and a package manager are attack surface and CVEs to rebuild for).
- The compressed image is **4.2 MB**.
  *Since 0c* it is 5.28 MB (pgx); *since 0e*, **5.41 MB** in the registry
  (`94d9daf` added the `web` package and chi's compressor, +52 KB; `990685a`
  the embedded frontend, +72 KB). Still distroless, user 65532.
- *Added in 0e:* a third stage comes first, `frontend`, on
  `node:26.13.0-trixie-slim` (the version in `.nvmrc`; Debian, because the
  lockfile's native packages are the glibc ones). It runs `npm ci`, then the
  type check and `vite build`, and the Go stage copies its `dist/` into
  `internal/web/assets/`. Nothing else from it reaches the runtime image.
  `npm ci` there has no cache: 117 s on the laptop, and most of the extra
  CI time (§8.4).
- `.dockerignore` is an **allowlist** (`*`, then `!backend/`). `.env` can't
  enter the build context, so it can't end up in a pushed image. *Since 0e*
  it re-includes the frontend's sources file by file (so `node_modules`,
  `dist` and any `frontend/.env*` stay out; Vite inlines `VITE_*` variables
  into the public bundle), and excludes the laptop's own build in
  `backend/internal/web/assets/`, which would otherwise mix stale chunks in.
  A new frontend config file must be added there; forgetting it fails CI's
  image build.
- Base images are pinned by tag, not digest (the same readability choice as the
  Actions), and Dependabot proposes updates.

**Artifact Registry:** the repository `madam`, Docker format, in `europe-west1`
(the same region as Cloud Run, so pulls don't cross regions). Images are
`europe-west1-docker.pkg.dev/madam-512309/madam/server:<commit>`, never
`:latest`, because a moving tag hides which code is running. The cleanup policy
(`deploy/cleanup-policy.json`): **keep the 5 most recent versions, delete the
rest once older than a day.** A keep rule wins over a delete rule. Five images
is about 20 MB of the free 0.5 GB. Cleanup runs "within approximately one day",
per Google's docs. Deleting an image can't break the live app: Cloud Run
*"keeps this copy of the container image as long as it is used by a serving
revision"* and doesn't pull from the registry when it starts an instance. The
Container Scanning API, which is billed per image, stays off.

The policy is **enforced**, not in dry run. Dry run was skipped on purpose: with
fewer than six images the keep rule protects everything, so a dry run's log
would be empty and prove nothing. To change the policy, run
`gcloud artifacts repositories set-cleanup-policies madam --location=europe-west1 --policy=deploy/cleanup-policy.json --no-dry-run`.
**Pass `--no-dry-run` explicitly.** The help doesn't list it, but leaving out
`--dry-run` *keeps* the previous mode rather than turning dry run off.

**The Cloud Run service `madam`:**

| Setting | Value | Why |
|---|---|---|
| Service account | `madam-run`, **with no roles** | The default compute account has `roles/editor` on the project; code running as it could change almost anything. Each later phase grants only what it needs |
| Access | `allUsers` holds `roles/run.invoker`, set once by 0b's `--allow-unauthenticated`. No longer a deploy flag (0c′, below) | A phone browser must reach it. Authentication is the app's job (Slice 2) |
| Instances | min 0, max 2 | Cost guardrails, above |
| CPU / memory | 1 vCPU / 512 MiB (the defaults) | At 512 MiB the free tier's vCPU-seconds run out before its GiB-seconds, so less memory would save nothing |
| Concurrency | 80 (default) | With a maximum of 2 instances, a lower value only means 429s sooner |
| Request timeout | 30 s (default 300 s) | The server's own write timeout is 10 s; this caps what a stuck request can bill |
| Startup CPU boost | **off** | gcloud turns it on by default for new services, and it's billed. Measured without it |

**The database URL: Secret Manager** (Phase 0c).
- Two secrets, each replicated only in `europe-west1`:
  `madam-database-url` (the app's) and `madam-migrate-url` (for `make
  migrate-prod`, §6). Both are madam-prod's session pooler, ending in
  `?sslmode=verify-full`. **Since 0d they hold different roles:**
  `madam-migrate-url` the owner `postgres`, `madam-database-url` version 3
  the application role `madam_app` (user `madam_app.<project-ref>`). Versions
  1 and 2, both `postgres` URLs, are destroyed.
- `madam-run` holds `roles/secretmanager.secretAccessor` **on
  `madam-database-url` only**, bound on the secret rather than the project. It
  is still `madam-run`'s only permission anywhere, and nothing but the owner
  can read `madam-migrate-url`.
- `make deploy` passes `--set-secrets=DATABASE_URL=madam-database-url:3`. The
  version is **pinned** in the Makefile, so changing the value is a visible
  commit. With `:latest`, any new instance would pick up whatever was last
  added.
- Values are added by Pe'er in his own terminal, never through Claude, with no
  trailing newline and nothing in the shell history:

  ```
  read -rs DBURL && printf %s "$DBURL" | gcloud secrets versions add madam-database-url --data-file=- --project=madam-512309; unset DBURL
  ```

- After a change, **destroy** the old version, don't just disable it: disabled
  versions still count as active for billing. There's a second reason, found
  in 0d: `madam-run`'s access is bound on the *secret*, so it covers every
  version, and an old version stays readable by the app's identity until it's
  destroyed. Destroy it only once the new one is verified live, because
  until then, reverting the pin is the rollback.
- *Switching the role in 0d*, step by step: the migration PR; `make
  migrate-prod`; `\password madam_app` on madam-prod; version 3 added and
  checked with psql (`current_user` = `madam_app`, no `BYPASSRLS`) while the
  app was still pinned to `:2`; the code PR; then a PR moving only the pin, so
  that deploy changed nothing but the credential. A wrong password would not
  have stopped the revision from starting (the pool connects on first use), so
  the psql check before the pin move was the safety net. After the deploy,
  `pg_stat_activity` on madam-prod showed `madam_app` as the only client
  connection besides the checking session. The other `postgres` row was
  `pg_net`'s background worker (`backend_type` `pg_net 0.20.4 worker`, no
  `client_addr`, running since the database started), part of Supabase, not a
  connection.
- Rejected: a plain environment variable. The value would pass through a shell
  or `.env` at every deploy, and anyone with viewer access could read it in the
  revision's settings. 0c′ benefits too: CI deploys with the same, non-secret
  flag.

**`/api/health` runs a database query** and answers `503 database unavailable`
when it fails or takes longer than 3 s. The cause goes to the log, never to the
response. **It must never become a Cloud Run liveness probe**: a Supabase outage
would then restart containers that are working fine. The startup probe stays
Cloud Run's default TCP check. The server listens even when the database is
down; it refuses to start only on a missing or invalid `DATABASE_URL`.

**Where deploys come from:** in 0b and 0c, the laptop, with `make deploy`. It
refuses uncommitted or untracked files, then builds, pushes and deploys, with
the image tagged by commit hash. Docker authenticates with the developer's own
`gcloud` login (`gcloud auth configure-docker`), so no service-account keys
exist. **Since 0c′, every merge to `main` deploys itself** (below), and `make
deploy` is the fallback. A long-lived key in the secrets of a public repository
was rejected outright.

**Reserved paths.** Cloud Run never forwards paths starting with `/_ah/` or
"some paths ending with `z`" to the container, and Google recommends avoiding
all paths ending in `z` ([known issues](https://docs.cloud.google.com/run/docs/known-issues)).
Tested: `/healthz` returns Google's own HTML 404, not ours. The health endpoint
is therefore `/api/health`. The `/api/` prefix also separates the API from the
frontend routes added in 0e.

### As built in Phase 0c′: deploy on merge

Every push to `main` that passes CI deploys itself from GitHub Actions. No key
exists anywhere: GitHub vouches for the job, and Google checks what it says.

**How a merge becomes a deploy**

1. A PR is merged. `main` gets a new commit, and `ci.yml` runs on it.
2. The `ci` job tests that exact commit. Only if it passes does the `deploy`
   job start (`needs: ci`).
3. The `deploy` job asks GitHub for a signed **OIDC token**: a note saying which
   repository, branch, event and workflow file this run is. Only this job may
   ask (`id-token: write`).
4. `google-github-actions/auth` hands the note to Google. The **provider**
   checks GitHub's signature and the **attribute condition**. If both pass,
   Google returns a short-lived token.
5. With it, the job **impersonates** the service account `madam-deployer`
   (up to an hour).
6. The job runs `make deploy`, the laptop's own target: build, push, `gcloud
   run deploy`. The flags exist only there, so CI and the laptop can't drift.

**The words**

| Term | Here |
|---|---|
| OIDC token | GitHub's signed note about the running job. Lives for minutes; can't be forged |
| Workload identity pool | `github`: Google's container for outside identities. Accepts nothing by itself |
| Provider | `madam-deploy`, in the pool: "trust notes signed by GitHub Actions", plus the condition |
| Attribute condition | The rule a note must pass to be exchanged at all (below) |
| Service account | A Google identity for software. `madam-deployer` deploys; `madam-run` is what the app runs as |
| Impersonation | Acting as a service account through a short-lived token, instead of holding its key |

**The condition**, on the provider:

```
assertion.repository_id == '1425598027'
&& assertion.ref == 'refs/heads/main'
&& (assertion.event_name == 'push' || assertion.event_name == 'workflow_dispatch')
&& assertion.workflow_ref == 'peerbenmoshe/madam/.github/workflows/ci.yml@refs/heads/main'
&& assertion.runner_environment == 'github-hosted'
```

| Who tries | Refused by |
|---|---|
| A fork | `repository_id`: GitHub's number for this repository, never reused or changed by a rename |
| Any pull request, from a fork or not | `ref` (`refs/pull/N/merge`) and `event_name` |
| `pull_request_target`, which runs with `ref` = `main` | `event_name` |
| A push to another branch | `ref` and `workflow_ref` |
| Another workflow on `main` (0f's keepalive, say) | `workflow_ref` |
| A self-hosted runner (none exist) | `runner_environment` |

`workflow_ref` names the repository and the file, so renaming either refuses
deploys until the condition is updated: a safe failure.

*Tested before the first deploy (PR #11):* a temporary job asked for a token
from a push to the PR's branch, the same repository, file and event as a
deploy with only the branch different, and from the PR itself. Google refused
both: `unauthorized_client`, "The given credential is rejected by the
attribute condition." It doesn't say which clause failed.

**What `madam-deployer` may do.** Each role is bound on one resource; none is
at project level.

| Role | On | Means |
|---|---|---|
| `iam.workloadIdentityUser`, held by `principalSet://…/github/attribute.repository_id/1425598027` | `madam-deployer` | Jobs from this repository may become it. A second check on the repository, independent of the condition |
| `artifactregistry.writer` | repository `madam` | Push images and tags. Not delete |
| `run.developer` | service `madam` | Read and update (deploy) the service; also delete it. **Not** change who may call it: no `setIamPolicy` |
| `iam.serviceAccountUser` | `madam-run` | Start revisions that run as `madam-run` (`actAs`) |

It has **no secret permission**, and needs none: gcloud doesn't read secrets
when deploying (checked in its source), and every deploy since 0c′ has passed
`--set-secrets` without one. Only `madam-run`, which reads the secret at
instance start, needs access. `run.developer` bound on the service alone is
enough for a deploy (also measured). It has **no keys**:
`gcloud iam service-accounts keys list --iam-account=madam-deployer@madam-512309.iam.gserviceaccount.com --managed-by=user`
lists none. Never create one.

**What that means:** the deployer can run *any* image as `madam-run`, and
`madam-run` can read `madam-database-url`. Whoever can deploy can, in effect,
read the database. That's true of any deploy on merge. What guards it is the
condition plus `main`'s protection (a PR is required; no bypass).

**Public access is a binding, not a flag.** `allUsers` holds `run.invoker` on
the service, set once in 0b; it survives deploys. `gcloud run deploy
--allow-unauthenticated` re-sets that policy on *every* deploy, even when
nothing changes (gcloud's source), and `run.developer` may not, so CI would
warn every time. The flag was removed. To change who may call the app, use
`gcloud run services add-iam-policy-binding` or `remove-iam-policy-binding`
by hand, and record it here.

**Created in 0c′** (project `madam-512309`, number `966460417479`): the APIs
`iam`, `iamcredentials` (issues the impersonation token) and `sts` (exchanges
GitHub's note); the pool, the provider, `madam-deployer`, and the four bindings
above.

**Measured** (the first three automatic deploys, 5 November 2026, PRs #11, #10
and #12): each went live with the image tag equal to the merge commit, and the
audit log names `madam-deployer` as the deployer. Merge to live took about **2
minutes**: `ci` about 1 minute, then `deploy` about 1 minute, of which `gcloud
run deploy` is about 50 s. A change to `tools/go.sum` empties CI's Go cache,
so that run's `ci` took 4 minutes. GitHub identifies a run of this repository
with its newer ID-based subject,
`repo:peerbenmoshe@100029602/madam@1425598027:ref:refs/heads/main`.

| Decision | Choice | Why | Rejected |
|---|---|---|---|
| Identity | Through the service account `madam-deployer` | One place to see and revoke what deploys may do; readable audit logs | Direct federation, roles on a `principalSet` (the auth Action's README now prefers it, to avoid service accounts and their keys; ours has none) |
| Gate | A `deploy` job in `ci.yml`, `needs: ci` | Tests the merged commit itself: the `ci` check isn't strict, so a PR may be behind `main` | A separate workflow on `workflow_run`: it runs the newest `main`, not the tested commit, and is one more event that can go missing |
| One set of flags | CI runs `make deploy` | Drift is impossible | Copying the flags into the YAML |
| gcloud in CI | The runner's preinstalled one (590.0.1 when built) | One dependency fewer; unpinned like the laptop's | `setup-gcloud` with a pinned version, bumped by hand |
| Concurrency | Runs on `main` queue, never cancelled | A deploy isn't cut off midway; the newest waiting run wins | Cancelling everywhere |
| A run that never starts | "Run workflow" (`workflow_dispatch`) on `main` | A merge-to-main run did go missing in 0a; now that would be a missing deploy | Waiting for it to happen again |
| Fallback | `make deploy`, refusing anything but a clean, latest `main` | Actions can be down. The guard stops branch deploys: 0c's live image was `93c33e9`, a branch commit, because squash merging makes a new hash | No guard; no fallback |
| Migrations | Still from the laptop (§6) | The owner credential stays out of GitHub's reach | Running them in the workflow |

**When something goes wrong**

- *A merge started no run.* `gh run list --workflow ci.yml --branch main
  --limit 3` (with `--workflow`: Dependabot's runs share the branch and can
  fill an unfiltered list). If the
  merge commit has no run: Actions → CI → **Run workflow** on `main` (or
  `gh workflow run ci.yml --ref main`). If Actions itself is down: `make
  deploy` from an up-to-date `main`.
- *The deploy job failed.* The old revision keeps serving. If the auth step
  says the token was rejected by the attribute condition, the run wasn't
  `ci.yml` on `main` by push or dispatch. A "permission denied" during `make
  deploy` points at a role above. After fixing something outside Git (IAM),
  use **Re-run failed jobs**.
- *"deploy: only the latest commit on main".* A newer merge landed while this
  run waited; that merge's run deploys it. Nothing to fix.
- *Stop all deploys now:* `gcloud iam service-accounts disable
  madam-deployer@madam-512309.iam.gserviceaccount.com --project=madam-512309`
  (`enable` undoes it).
- *Roll back:* revert the PR on GitHub. Merging the revert deploys the previous
  code. Sending traffic to an old revision by hand would leave the live app
  different from `main`.
- *What's live?* `gcloud run services describe madam --region=europe-west1
  --format='value(spec.template.spec.containers[0].image)'`. The tag should be
  `git rev-parse --short origin/main`.
- *Who deployed?* The **Admin Activity** audit log, which records who asked
  for each change:
  `gcloud logging read 'logName="projects/madam-512309/logs/cloudaudit.googleapis.com%2Factivity" AND protoPayload.methodName:"ReplaceService"' --limit=1 --format='value(timestamp,protoPayload.authenticationInfo.principalEmail)'`.
  Cloud Run's `system_event` log also mentions `ReplaceService`, but it has
  no caller.

### Considered and rejected: Starter Tier

Google's new Starter Tier runs apps with no billing account at all, under separate
terms. It does not fit: it is offered only through Google AI Studio deployments,
regions, IAM, quotas and enabled APIs are fixed by Google, no other APIs can be
enabled in the project, and its terms limit it to business, trade or professional
use. Our own Dockerfile and deployment pipeline need a standard project.

### Costs and unknowns

- A Google Cloud account with a card on file is required.
- **Cold start** on the first request after idle. *Measured in 0b* (4 samples,
  22 October 2026, before any database code), replacing the assumption of "a
  second or two":

  | Measure | Cold | Warm |
  |---|---|---|
  | End to end from the laptop in Israel (`curl`) | 0.67–0.98 s, median 0.81 s | ~0.21 s |
  | Inside Google (request log `httpRequest.latency`) | 0.15–0.37 s | ~2 ms |
  | Container start (`container/startup_latencies` metric) | 181–230 ms (3 of 4 samples appeared in the metric) | n/a |

  An instance shut down 15 minutes after its last request, every time; the
  proof was our own `shut down cleanly` log line. Startup CPU boost was off.

  *Re-measured in 0c* (4 samples, 4 November 2026, same method), with
  `/api/health` now running a query on madam-prod:

  | Measure | Cold | Warm |
  |---|---|---|
  | End to end from the laptop (`curl`) | 0.82–1.11 s, median 0.87 s | ~0.25 s (median of 12) |
  | Inside Google (`httpRequest.latency`) | 0.26–0.48 s, median 0.26 s | 14–16 ms |
  | Container start (`container/startup_latencies`) | 28–303 ms (4 of 4) | n/a |

  The container start didn't grow, because the pool connects on first use. The
  cold *request* gained about 0.05–0.1 s: opening the first connection to
  Frankfurt (TCP, TLS, authentication) takes a few round trips. A warm request
  gained about 12 ms, one round trip.

  **Idle shutdown isn't always 15 minutes.** Three instances shut down after 15
  minutes, but one shut down **7 seconds** after its last request. Fifteen
  minutes is typical, not guaranteed, so a cold start can come sooner than
  expected.

  *Re-measured in 0e* (6 samples, 6 November 2026, same method), with the
  frontend embedded in the binary. Four samples were a cold `/api/health`,
  comparable with 0c; two were a cold page (`/`), which doesn't touch the
  database:

  | Measure | Cold `/api/health` | Cold `/` | Warm |
  |---|---|---|---|
  | End to end from the laptop (`curl`) | 0.87–1.33 s, median 1.03 s | 0.88 s, 1.00 s | ~0.22 s |
  | Inside Google (`httpRequest.latency`) | 0.26–0.70 s, median 0.35 s | 0.33 s, 0.33 s | n/a |
  | Container start (`container/startup_latencies`) | 20–249 ms (6 of 6, both kinds) | | n/a |

  After the cold page, the script (69 KB gzipped) took 0.28–0.29 s more.
  **The embedded frontend doesn't slow the start**: container start is in
  0c's range, and the files are only read when requested. The `/api/health`
  median rose 0.16 s, but the ranges overlap, and the two slow samples
  (0.44 s and 0.70 s inside Google) had normal container starts. So the extra
  time came after the start; our guess, not measured, is the first connection
  to Supabase. Four samples can't tell that from noise.
- Custom domains may need extra setup depending on region.

### Rejected

| Option | Why not |
|---|---|
| AWS App Runner | Since Jul 2025 new accounts get $100–$200 in credits that expire after 6 months or when spent — not free-forever. AWS also has well-known billing traps (data transfer, NAT gateways, idle IPs). Legitimate if the AWS line on a CV matters more than the bill. |
| Azure Container Apps | Works; $200 trial credit for 30 days plus partial free grants. No advantage over Cloud Run here. |
| Fly.io | Simple, no cold starts, ~$2/month. No free tier for new accounts since Oct 2024. Good runner-up. |
| Render (free) | Free web services sleep after 15 minutes and take 30–60 s to wake — she would hit that nearly every morning. Paid is $7/month. |
| Railway | No free tier; Hobby from $5/month. |
| VPS (Hetzner, ~€4.59/mo) | Most learning, most ops work: OS updates, TLS, restarts. Too much surface for a first project. |

AWS Lambda was considered and rejected: it needs an adapter layer and changes how
the app is structured.

**Build a container image from day one** — that is the portability insurance.
Moving hosts later means pointing a new platform at the same image and copying the
environment variables (hours, mostly DNS and config).

---

## 8. Frontend

### 8.1 Framework: React (TypeScript)

Chosen for the largest ecosystem (charting libraries, component libraries), the
strongest CV value, and because the Figma-to-code tooling is built around it.
Pe'er explicitly does not intend to master the frontend right now, so the steeper
learning curve is not a factor.

Rejected: **Svelte** (genuinely pleasanter to learn and write, smaller ecosystem)
and **Vue** (a middle ground, no decisive advantage here).

### 8.2 Styling: Tailwind

Style classes written directly in the markup; currently dominant, and it pairs
naturally with **design tokens** — named variables for every colour, font and
spacing value instead of hardcoded ones.

**Tokens are a deliberate investment**: if a redesign ever happens, named tokens
make it a reskin rather than a rewrite. Changing the palette is then editing a
handful of variables.

**Palette: HotPink `#FF69B4`** (chosen by Pe'er at step 4), on a light background.

Measured contrast, so the limits are known rather than guessed:

- HotPink on white is about **2.6:1**, and lower on cream.
- WCAG asks for **4.5:1** for normal text and **3:1** for graphical objects such as
  chart lines.
- Dark text **on** a HotPink fill is about **7.9:1**, comfortably above both.

So HotPink works as a **fill**: buttons, dot fills, cycle-start markers, accents.
It does **not** work for thin elements on a light background — body text and the
temperature line need a much darker tone to stay readable on a phone at 07:00.

This is an arithmetic limit, not a taste judgement; the taste judgement is the
first user's, and tokens make it cheap to revise.

*As built in 0e:* Tailwind 4's tokens live in one `@theme` block in
`frontend/src/index.css`, and Tailwind's own colour palette is switched off
(`--color-*: initial`), so only the names below exist and `bg-pink-400` is not
a class. Recomputed with the WCAG formula on 6 November 2026; the "7.9:1"
above is pure black on HotPink.

| Token | Value | Use | Contrast |
|---|---|---|---|
| `accent` | `#FF69B4` | fills only | 2.65:1 on `surface` |
| `ink` | `#1A1A1A` | text, borders | 17.4:1 on `surface`, 6.6:1 on `accent` |
| `muted` | `#595959` | secondary text | 7.0:1 on `surface` |
| `surface` | `#FFFFFF` | background | n/a |

Cream would lower every pair, so the background is white. No web fonts: the
system font stack, so nothing is fetched from another origin. Seen working on
Pe'er's phone; the first user's check remains before Slice 6.

*Revised in the design step (8 November 2026).* Pe'er approved a design on a
clickable mock (`design-brief.md`; §11 below). Three things above change:

- **Eleven tokens instead of four,** with the neutrals tinted toward plum.
  The names `accent`, `ink`, `muted` and `surface` stay. `ink` becomes
  `#2B1726` (16.8:1 on `surface`, 6.3:1 on `accent`) and `muted` `#6E5866`
  (6.5:1). A new `berry`, `#A0185A` (7.6:1), carries the temperature line
  and links, which HotPink can't. HotPink stays, as a fill only. The full
  table with contrast is `design-brief.md` §3. The `@theme` block changes in
  the design foundation step (`build-order.md` §4).
- **A web font, self-hosted:** Atkinson Hyperlegible Next, chosen for
  legibility at 07:00. This reverses "no web fonts". That rule existed so
  that nothing is fetched from another origin, and self-hosting keeps it:
  the files are embedded in the binary like the rest of the frontend, so the
  CSP stays `'self'`. The cost is the files' size, not yet measured. Where
  the files come from (an npm package, or committed `.woff2` files) is open
  until that step (`design-brief.md` §10).
- **Light only in v1.** There is no dark theme.

### 8.3 Charting: Recharts (decided at step 4)

**Correction to this document.** An earlier version of this section said
"everything unusual in our spec comes down to custom dot rendering." That was
wrong. The dots are the easy part; the line is where the real constraint is.

What Recharts handles straightforwardly:

- **Hollow rings** for disturbed readings — the `dot` prop accepts a custom
  component, so each point decides whether it draws filled or hollow.
- **Ghost dots** — a second series carrying the raw value, faded, toggled by a
  boolean.
- **Cycle-start lines** — `<ReferenceLine x={date} />`, positioned by data value
  and resize-safe.
- **Tappable dots** — the custom dot component carries its own click handler,
  which is more reliable than the library's dot-event history suggests.
- **Date + cycle day on the X axis** — a custom tick renderer.

**The actual constraint.** `spec-v1.md` §9.2 needs two different behaviours for a
day with no plotted value: a **disturbed** reading is stepped over and its
neighbours connect, while a **missing** day is a true gap. Recharts controls this
with `connectNulls`, a single boolean per line — all holes bridged, or none. There
is no per-point version.

**Chosen solution:** compute the connected runs in the frontend and render the
line as **separate segments**, one per run. Explicit, no tricks, nothing hidden.

Rejected alternatives:

- Giving disturbed days an interpolated value so the path passes through where it
  would have gone anyway. Fewer moving parts, but it only works with straight
  segments rather than curves, the edges (disturbed day beside a gap, or at the
  range boundary) need special care, and it is a trick in a project whose point is
  understanding.
- **visx**, where you build the path yourself and the question never arises — at
  the cost of writing most of the chart by hand. The fallback if Recharts fights
  back.

**Build the graph early**, immediately after the walking skeleton, with fake data.
It is the centrepiece and the only part with a known sharp edge; meeting that edge
while the codebase is small keeps a swap to visx cheap.

### As built in the graph spike (8 November 2026)

**Verdict: Recharts stays.** It passed all six criteria for switching to visx,
which were fixed before looking. The code is on the branch `spike/graph`
(commit `b20b2a1`), pushed but never merged. `main` has no Recharts until Slice
6, which looks the version up again on the day.

**Package.** `recharts` **3.10.1**, released 25 July 2026; nothing stable since
(3.11 is canary). Its peers are `react` and `react-dom` (React 19 is in range)
and **`react-is`**, so `react-is` **19.3.1**, matching React, is added
explicitly. Left to npm, the peer would sit in the lockfile only.

Together they add **39 installed packages**:
- 11 `@types/*` (types only, never in the browser);
- 11 d3 pieces through `victory-vendor`;
- the Redux Toolkit stack (Recharts 3 keeps each chart's state in a small
  store);
- small helpers.

`npm audit` found nothing new, and 0 in runtime packages.

| # | Criterion for visx, fixed in advance | Result |
|---|---|---|
| 1 | The line as segments, from documented components and props only | **Pass**: one `<Line connectNulls>` per run |
| 2 | No loosening of the CSP | **Pass**: 0 violations in every state, under the real headers, in Windows Chrome, Android Chrome and iPhone Chrome (WebKit). A negative control, a deliberate `style` attribute, showed exactly 1 each time |
| 3 | Edge cases E1–E18 render as `fam-rules.md` §7 and `spec-v1.md` §9.2 say | **Pass** (below) |
| 4 | Taps: ≥ 9 of 10 at the 14-day default, on both phones | **Pass**, per-dot: Android 9/10 on a dot and 10/10 on a ghost; iPhone 10/10 and 10/10. Alternating taps on neighbours 22 px apart all landed as aimed |
| 5 | Recharts adds ≤ 150 KB gzipped | **Pass**: script 69.2 → 175.9 KB gzipped (**+106.7**, spike page included); 221 → 588 KB raw |
| 6 | Every colour a token | **Pass**: the computed fill and stroke of every painted shape is one of the four tokens |

**How each piece is done**

- **The line.** `lineSegments(days)` walks the visible days in date order:
  - a day with **no temperature** (no entry, or an entry without one) closes
    the current run;
  - a **disturbed** day is skipped, and the run stays open;
  - any other day joins the run.

  Run *k* puts its value in a field `sK` on its own rows only. Each run is drawn
  by `<Line dataKey="sK" connectNulls dot={false}>`. `connectNulls` is safe
  inside a run, because the only blanks there are disturbed days.

  There is **one row per calendar day**, missing days included, so the
  category axis is a true calendar and a `ReferenceLine` can sit on a gap. A
  one-day run draws no line; the dot layer still draws its dot.
- **Dots.** A dots-only `<Line stroke="none" dot={fn}>` draws them:
  - a **solid dot**: accent fill, ink outline;
  - a **hollow ring**: surface fill, 2 px ink outline;
  - a **ghost**: another dots-only line underneath, muted at 50 % opacity,
    smaller.

  Recharts calls the dot function for every row, with `cy` undefined where
  there is no value, so the function returns nothing there. Dots are clipped to
  the plot area, so the X axis has 10 px of padding to keep edge dots whole.
- **Cycle starts.** A `ReferenceLine` per start, dashed **ink**, with an
  accent-filled triangle on top as its `label`. A 1 px line is a thin
  element, and accent is 2.65:1, below the 3:1 that graphics need (§8.2), so
  accent is used only on the fill.
- **X tick.** A custom tick function draws the date as `d/m` (an assumption;
  the spec names no format) over the cycle day, `?`, or blank. Labels are
  thinned to about 32 px each. On a phone that's every day at 14 days, every
  3rd at 30, and every 7th at 90.
- **Taps.** Each dot and ghost has an invisible **48 px hit circle**, in a layer
  *under* the visible marks, and the marks themselves are clickable too. So a
  tap on a mark gets that mark, even where a neighbour's circle overlaps it.

  Rejected: a chart-level click that reads Recharts' active index. Recharts
  processes moves once per animation frame (`throttleDelay: 'raf'`). On a
  touch screen the click therefore runs before the frame that updates the
  index, and gets the **previous** tap's day. It misfired on Android. If column
  taps are ever wanted, compute the day from the tap's x position.
- **Colours.** Elements Recharts draws get the token as an attribute,
  `stroke="var(--color-ink)"`, which browsers resolve. Our own SVG uses
  Tailwind classes (`fill-accent`, `stroke-ink`).
- **CSP.** Recharts writes styles only through the DOM: its text-measuring
  element uses `Object.assign(el.style, …)` (`util/DOMUtils.js`). Its line
  animation writes `stroke-dasharray` as an SVG attribute. Neither is
  inline style in the CSP's sense, so `style-src 'self'` stays as it is.

**Edge cases checked** (fake data, 90 days):
- **Line:**
  - E1: the line bridges one disturbed day.
  - E2: it bridges two in a row.
  - E3, E4: a gap at one missing day, and at three.
  - E5: an entry without a temperature is a gap.
  - E6: normal, disturbed, missing, normal does **not** connect.
  - E7: a lone reading is a dot with no line.
- **Window edges:**
  - E8: a disturbed first day is a ring at the left edge, and the line starts
    on day 2.
  - E9: the last day's dot is whole at the right edge.
- **Ghosts:**
  - E10: T1 shows a dot at 36.55 and a ghost at 36.62, only with ghosts on.
  - E11: T3 has no ghost.
  - E12: a disturbed early reading shows a ring at 36.45 and a ghost at 36.40.
- **Cycles:**
  - E13: a cycle start on a day with no entry draws its line over a gap.
  - E14: cycle days 41 and 42 appear in the ticks.
  - E15: days before the first start show blank, or `?` with an unknown start.
- **Values and units:**
  - E16: a 38.90 fever is a ring, and the axis stretches to fit it.
  - E17: the axis, dots and popup switch to °F, and 36617 shows as 97.91 °F.
  - E18: a disturbance tag on a day without a temperature is a gap, not a
    ring.

E6 and E18 are **readings**, not rules; `fam-rules.md` doesn't name these
cases. In E6, the bridge would have to cross a missing day. In E18, there is
no reading to step over.

**For Slice 6**
- **Density.** At 90 days a phone has about 4.7 px per day against 9 px dots, so
  the dots merge into a band. Dot size must shrink with the range. Beyond about
  14 days, no tap target can be 48 px wide, whatever the library. A tap then
  means "nearest day", which is a spec question (for example, ‹ › in the
  popup).
- **Animation.** Recharts' draw-on animation runs in real browsers, but stayed
  on its first frame in headless Chrome, which leaves the lines invisible.
  Automated screenshots need `isAnimationActive={false}`. Whether to animate
  at all is a design choice.
- **The window's edge.** Segments are computed inside the visible window. So
  when the day before the window has a reading, the line doesn't enter from the
  left edge.
- **Bundle.** Vite now warns about a chunk over 500 KB. Code-splitting the
  graph is an option, not a need.
- **Testing on a phone.** WSL here is NAT-networked, so phones reached the
  laptop through a temporary Windows port proxy and a firewall rule limited to
  the local subnet. Both were removed afterwards. Headless Chrome on Windows
  won't go narrower than 504 px, so phone widths need a real phone.

### 8.4 Delivery: embedded in the Go binary

TypeScript is a build-time language: the build produces plain HTML, JS and CSS.
Go's `//go:embed` then packs those files into the compiled binary, giving **one
artefact, one URL, and no cross-domain (CORS) configuration**.

During development the frontend dev server runs separately for fast reloads;
embedding happens only in the real build.

Rejected: a separate static host (Cloudflare Pages, Netlify). It would show the
page shell instantly even during a cold start, but costs two deployments and CORS
setup. Reversal later is roughly an afternoon.

### As built in Phase 0e

**How a page reaches the phone**

1. `make frontend` (or the Dockerfile's `frontend` stage) runs `tsc -b`, then
   `vite build`, which writes `frontend/dist/`: `index.html` and
   content-hashed files in `assets/`.
2. The build is copied into `backend/internal/web/assets/`. `go:embed` can't
   reach `../frontend`, because a pattern may not contain `..`. That
   directory is gitignored except an empty, committed `.gitkeep`.
3. `//go:embed all:assets` (`internal/web/web.go`) packs it into the binary.
   `all:` also takes names starting with `_` or `.`, which Go otherwise
   skips; Rollup can emit `_`-prefixed chunks.
4. The router sends `/api/*` to the API and every other `GET` or `HEAD` to
   `web.Handler`, which serves the files.

**Toolchain**, every version exact, looked up on 6 November 2026: Node
**26.13.0** (Active LTS since 28 October 2026; 24 went to maintenance on 20
October) with npm 11.20.0, Vite 8.3.6, React 19.3.1, TypeScript **6.0.3**,
Tailwind 4.3.4, ESLint 10.14.0 with typescript-eslint 8.72.1 and
eslint-plugin-react-hooks 7.1.1, and Prettier 3.9.10. That's 14 direct
packages and 175 installed (`frontend/package.json`).

| Decision | Choice | Why | Rejected |
|---|---|---|---|
| Pinning Node | `.nvmrc` (read by nvm and by CI's `setup-node`) and the Dockerfile's exact `node:26.13.0-trixie-slim`. CI fails if they differ; `make check-node` refuses any other local Node | The laptop, CI and the image run the same Node, which is stricter than Go, where CI takes the newest 1.27.x. Dependabot updates the Dockerfile but not `.nvmrc`, so the failing check is the reminder to update both | A Docker `ARG` from `.nvmrc` (Dependabot can't see it); `engines` + `engine-strict` (a third copy); corepack (no longer shipped since Node 25) |
| Packages | npm with `package-lock.json` and `npm ci`; `frontend/.npmrc` sets `save-exact` and `ignore-scripts` | Nothing extra to install or pin. No package runs code on install, the usual route of npm worms. Verified: install and build need no scripts | pnpm (one more tool to pin), Yarn |
| TypeScript | 6.0.3, not 7.0.2 (`latest`) | typescript-eslint, canary included, requires `typescript <6.1.0`, and its TS 7 issue is open. Vite's template has moved to TS 7 with oxlint; that would lose type-aware lint. Dependabot holds TS majors back | TS 7 + oxlint, which would also contradict §12 |
| Lint and format | ESLint flat config: `@eslint/js` recommended, typescript-eslint `recommendedTypeChecked`, react-hooks recommended; Prettier with defaults | Type-aware rules catch bugs the compiler doesn't. A missing `void` on a promise was flagged as `no-floating-promises` (tried on purpose) | `eslint-plugin-react-refresh`, `globals`, `eslint-config-prettier`, `prettier-plugin-tailwindcss`: each optional, none needed |
| Tailwind | 4, through `@tailwindcss/vite`; tokens in CSS (§8.2) | No JS config, no PostCSS | Keeping the default palette beside ours |
| Recharts | Not yet. The graph spike confirmed 3.10.1 with `react-is` 19.3.1 (§8.3); Slice 6 installs it | 0e has no chart, and the spike is throwaway | Adding an unused dependency now |
| No build yet | A committed `.gitkeep`. `go build`, `go test` and golangci-lint work on a fresh clone; pages then answer `503 frontend not built`, `/api/` works, and startup logs a warning | Production can't ship that state: the image always builds the frontend, and CI's smoke test checks for the real page | A build tag (two code paths); nothing committed (a fresh clone doesn't compile); Vite writing into `assets/` (deletes `.gitkeep`) |
| Routing | `/api` is a subrouter with its **own NotFound** (plain 404). `GET`/`HEAD /*` goes to the frontend: a file is served as itself; a missing name with an extension, or anything under `/assets/`, is 404; any other path gets `index.html` (client-side routes). Hidden names are 404, paths are cleaned first, and directories are never listed | An unknown API path must never be `200 text/html`: clients and monitoring would read it as success. **chi gives a mounted router its parent's NotFound when it has none of its own** (`mux.go`), so a top-level fallback would silently cover `/api/typo`. Breaking it on purpose made 7 router tests fail. A missing chunk gets 404 so an old tab fails clearly instead of receiving HTML as script | The page as the top-level NotFound |
| Caching | `/assets/*`: `public, max-age=31536000, immutable`. Everything else on the page side: `no-cache`. `/api/*`: `no-store` | Hashed names never change content. `index.html` must be checked on every load to pick up a deploy. API responses will carry health data, and setting `no-store` on the subrouter means no route can forget it | ETags (embedded files have no modification time; `index.html` is 0.4 KB) |
| Compression | chi's `middleware.Compress` on the frontend only; byte ranges ignored | The script is 221 KB, 69 KB gzipped, and egress to Israel is billed. No new dependency. A range of a compressed body would be wrong | Compressing `/api` (BREACH-style risk once responses carry secrets) |
| Security headers | On every response: a CSP of `'self'` only (no inline script or style; `object-src`/`base-uri` `'none'`, `frame-ancestors 'none'`), `nosniff`, `Referrer-Policy: no-referrer`, a `Permissions-Policy`, `COOP same-origin` | A strict CSP is easiest to keep when nothing yet needs loosening; the built page has no inline code. No HSTS: `.app` is HSTS-preloaded as a whole TLD | Adding headers in Slice 2 |
| `make dev` | Builds the server, runs it in the background, runs Vite in the foreground; a `trap` stops the server when Vite exits. Vite (`:5173`) passes `/api/` to `:8080`, so the browser sees one origin, as in production | One command, no dependency. Tested: Ctrl+C stops both, and so does Vite dying alone | `trap 'kill 0'` (also kills make itself); `go run` (`$!` would be the go tool, not the server); `concurrently`; two terminals |
| Smoke test | `deploy/smoke-test.sh BASE_URL`: the page, a deep link, the script (type, `immutable`, gzip), `/api/health` (`no-store`), 404 for `/api`, `/api/`, `/api/nope`, `/api/health/extra`, a missing chunk, `/favicon.ico`, `/.gitkeep`, and the CSP. CI runs it on the image; it also checks production by hand | Tests the real image as deployed, not only the handlers | Unit tests only |

The page itself (`frontend/src/App.tsx`) asks `/api/health` with `no-store`
and a 5 s timeout and shows `ok`, the server's own reason (`database
unavailable`), or "can't reach the server", with a "Check again" button. It
is mobile first: a single column, `max-w-md`, and a 48 px tap target. There is
no router yet.

**Measured** (6 November 2026):
- Bundle: 221 KB of script and 6.8 KB of CSS; **69 KB and 2.2 KB gzipped**.
  The build is reproducible: the script's hashed name was the same on the
  laptop and in CI.
- Image: see §7 (+72 KB compressed).
- CI: the `ci` job went from about 1½ to about 3 minutes (npm install 45 s on
  an empty cache; the image build 71 s, up from about 30 s, because `npm ci`
  inside Docker has no cache). The deploy job builds the image too, so
  **merge to live took about 4½ minutes**, up from about 2.
- Production: the smoke test passed 14/14 at the public URL; the page and a
  deep link work, reload included, on Pe'er's phone.

**Known and accepted:** `npm audit` reports one moderate advisory,
GHSA-g3hv-75wg-m249 (`minimatch` <11.0.1, a denial-of-service through glob
patterns). It is pulled in only by ESLint and typescript-eslint, a dev tool
matching patterns we write, and never reaches the browser. The fix exists only
in `minimatch` 11, and both packages require `^10`. Dependabot's security
update concluded "update not possible" for the same reason. Its alert stays
open and should close by itself once the fix reaches `main`, as 0c′'s `cel-go`
alert did. npm's suggested "fix" is a downgrade to ESLint 9.

**Expected to change, not yet verified:**
- `connect-src` gains the Supabase URL in Slice 2, if the browser calls
  Supabase Auth itself.
- ~~`style-src` may need loosening if the spike finds Recharts writing `style`
  attributes as markup.~~ **Settled by the graph spike:** it doesn't. Recharts
  writes styles through the DOM, as React's `style` prop does, which CSP
  allows; 0 violations on three browsers (§8.3).
- HSTS arrives with a custom domain.
- TypeScript 7 arrives once typescript-eslint supports it.

---

## 9. Environments and deployment timing

**Deploy early.** Before any features, build a **walking skeleton**: a Go API with
one "ok" endpoint, a "hello" page embedded in the binary, in a container, on Cloud
Run, connected to Supabase with one table. No features — just proof the pipe
connects end to end.

Deployment problems (container config, environment variables, permissions,
connection strings) are a different class of problem from coding problems. Meeting
them one at a time on an empty codebase is far cheaper than meeting all of them at
once on launch day.

Also part of the skeleton:

- the **Supabase keepalive job** (section 5), for both projects;
- the **non-owner application role** and the transaction wrapper, so RLS has
  somewhere to stand from the first migration onward;
- the **cost guardrails** (section 7): instance limits and the Artifact Registry
  cleanup policy. The two budgets were already created during setup.

Two environments:

- **Local (laptop):** Go API, frontend dev server, PostgreSQL in a container, fake
  seed data. Fast, and disposable.
  *Decided in Phase 0a:* the container is defined in `compose.yaml` and started
  with `make db-up`, which runs `docker compose up --wait`. That returns only once
  a `pg_isready` healthcheck passes. The check uses TCP (`-h 127.0.0.1`), not the
  Unix socket: during first-time initialisation the image runs a temporary
  socket-only server, so a socket check can report "ready" too early. The port is
  bound to `127.0.0.1` only. Any target can use the `madam-dev` Supabase project
  instead with `DB=dev` (`CLAUDE.md`, Commands).
- **Production (Cloud Run + Supabase):** the real thing, with her real data.

Rules:

- **Never test against production data.** Use a **separate Supabase project for
  development**, clearly named, so a bad migration cannot touch her history. The
  keepalive job should cover this project too.
- **The production database URL never goes into a local `.env`.** Production
  credentials live in Secret Manager (§7), read by Cloud Run at instance start
  and by `make migrate-prod` with the developer's own login. The keepalive
  needs none: it keeps production awake through the public `/api/health` (§5,
  decided in 0f), so no production credential is in GitHub. Migrations reach
  production from the laptop (§6, decided in Phase 0c).
- No staging environment for v1; local plus production is enough for one user.

### Setup checklist

_Status: the setup items below were completed in October 2026 and verified by a
pre-flight check. Items marked **(Phase 0)** belong to the build, not to setup._

- Go (from the official tarball on go.dev — **not `apt`**, which ships an old
  version), Node (via `nvm`), Docker, `gcloud`, `goose`, `sqlc`. ✅
- Google Cloud: create a project, enable the Cloud Run and Artifact Registry APIs,
  install and log in to `gcloud`, create **an alerts-only budget on the billing
  account and a spend cap budget on Cloud Run** in the project. ✅
  **(Phase 0)** Add a Dockerfile, create an Artifact Registry repository in
  `europe-west1` **with a cleanup policy**, push the image, then
  `gcloud run deploy` to `europe-west1` with **min instances 0 and max instances
  2**. ✅ Done in 0b (§7, As built). Environment variables in Cloud Run: the
  first, `DATABASE_URL`, arrived in 0c, from Secret Manager (§7).
- Supabase: two projects (prod and dev), Frankfurt, Data API off. ✅
  **(Phase 0)** The non-owner application role.
- Secrets (database URL, invite code, Supabase keys) live in Cloud Run
  configuration and a local `.env` — **never in Git**. Variable names and
  `.env.example`: done in Phase 0a (`PORT`, `DATABASE_URL`,
  `MADAM_DEV_DATABASE_URL`; see `CLAUDE.md`, Commands). ✅ Later variables are
  named in the slice that first needs them.

---

## 10. Development environment: WSL

**Build inside WSL, in the Linux filesystem** (e.g. `~/projects/madam`), and run
Claude Code from there. Install Go, Node, git, goose and sqlc **inside WSL**, not
on Windows.

**Do not keep the project under `/mnt/c/...`.** Files there cross a translation
layer between two filesystems and are markedly slower for tools that touch many
small files — exactly what Go builds and `node_modules` do. It also causes
line-ending churn in Git and breaks executable permissions on scripts.

Notes:

- Ubuntu 22.04 is fine, with the Go and Node installation caveats above.
- Enable Docker Desktop's **WSL integration** for the distribution, so `docker`
  works from the Ubuntu shell. **Docker Desktop must be running** on Windows;
  otherwise `docker` in Ubuntu only prints a "could not be found" message.
- VS Code with the WSL extension edits WSL files natively (`code .` from inside WSL).
- **Git credentials, SSH keys and the `gcloud` login live inside WSL** and do not
  carry over from the Windows side. One-time setup.

### What's installed (October 2026)

| Tool | Version | Installed via |
|---|---|---|
| Go | 1.27.3 (updated from 1.27.1 in Phase 0a to match CI) | Official tarball, `/usr/local/go` |
| Node / npm | 24 LTS / 11 at setup; **26.13.0 / 11.20.0** since 0e, pinned in `.nvmrc` | `nvm` (shadows a Windows npm that is also on PATH). nvm's default is `lts/*`, the newest LTS installed, so new shells pick up 26 |
| Docker | 29.4 (Desktop 4.71), Compose 5.1 | Docker Desktop with WSL integration |
| goose | 3.29 | `go install` (global copy). The project pins **v3.29.0** in `tools/go.mod` (0c) |
| sqlc | 1.31 | `go install` (global copy). The project pins **v1.31.1** in `tools/go.mod` (0c) |
| golangci-lint | 2.15 | Official install script (global copy). The project pins **v2.15.0**: `make lint` installs that exact binary into `bin/`, and CI uses `golangci-lint-action` with the same version |
| psql | 14 | Ubuntu's `postgresql-client` — older than the server, fine for ad-hoc queries |
| gh | 2.102 | GitHub's apt repository |
| gcloud | SDK 590 | Google's apt repository |
| Claude Code | 2.1.x | Native installer, auto-updating |

**Pinning the dev tools (decided in Phase 0a).** A global `@latest` install lets
the laptop and CI drift apart, so the project pins its own versions:

- **golangci-lint** is pinned as a **binary**. Its docs say `go tool` and `tool`
  directive installs "aren't guaranteed to work" and recommend the binary.
- **goose and sqlc** are `tool` directives in a **separate `tools/go.mod`** at
  the repository root, run through the Makefile, so their large dependency trees
  stay out of the app's `go.sum` (added in 0c; §6, As built).

**Node** (decided in 0e, §8.4) is pinned exactly in `.nvmrc` and in the
Dockerfile's `node:` tag, and CI checks that they agree. `make check-node`
refuses any other Node, with a hint: `nvm use` reads `.nvmrc`. nvm doesn't
switch versions by itself when you `cd` into the repository.

**The `go` line is `go 1.27`**, not the `go 1.27.1` that `go mod init` writes. It
is the minimum language version, so any 1.27.x toolchain satisfies it and nothing
is downloaded automatically. CI reads it from `go.mod`.

### Network quirks of this machine

Found during setup; each one cost real time, so they are written down.

- **DNS:** WSL uses Windows' DNS through DNS tunneling (`nameserver
  10.255.255.254`). A hardcoded `8.8.8.8` broke resolution on networks that block
  outside DNS servers, and was removed.
- **apt over HTTPS:** plain-HTTP connections to Ubuntu's package mirrors failed
  from this laptop, so `/etc/apt/sources.list` uses `https://` mirrors.
- **No IPv6** inside WSL (default NAT networking). Most tools fall back to IPv4
  silently; Supabase's direct connection cannot (§5, Connecting).
- **Outgoing SSH is reset**, on every network tested, from both WSL and Windows,
  including GitHub's SSH-over-443 endpoint. The cause was never found. Git uses
  HTTPS (§12); Postgres connections are unaffected.

---

## 11. Design (done before Slice 3)

*Until 8 November 2026 this section was "The design handoff (optional,
post-v1)":* v1 would get a plain look from Claude, and the first user, a
junior UX/UI designer, might redesign it after v1. **That was reversed in the
design step.** Tokens make colours and fonts cheap to change later, but not
layout and interaction: how the graph handles 90 days on a phone, how the
popup moves between days, how the entry form unfolds. Slices 3–9 build all
of those, and redesigning them after v1 would mean rebuilding screens. Pe'er
also found the shell looked "like some admin-control page".

**How it was done.** Two Claude skills: Anthropic's `frontend-design` plugin
(installed for Pe'er's user only, so nothing in the repository) and the
built-in `dataviz` skill for the charts. Claude wrote a design plan and built
a clickable phone mock with made-up data. Pe'er reviewed it on his phone in
four rounds; he approves the design alone. The result is `design-brief.md`,
and the mock is `docs/design/mock.html`.

**The Figma route stays optional,** unchanged. **She has no Figma plan**, and
nothing in v1 depends on it.

If it does happen: Figma's official MCP server gives Claude Code structured access
to Figma files — components, variables, layout — and can generate code from
selected frames. It also works in reverse, sending live web interfaces into Figma
as editable layers, so she could pull our v1 screens in as a starting point rather
than redrawing them.

Caveats:

- It is not pixel-perfect magic. It works best when her file uses components and
  variables and our code uses matching names — hence the design-token decision in
  section 8.2, which is worth keeping regardless.
- *Unverified:* what a Figma plan would need to allow for MCP usage; some guides
  suggest the official route needs a paid plan. Only worth checking if she decides
  to do it.

Free and worth doing either way: her check that the design reads well on her
phone early in the morning (`build-order.md` §6).

---

## 12. Development practices

### Testing

- Go's standard-library `testing` package; no dependencies needed.
- **Unit tests** for the FAM domain logic — the test vectors in
  `docs/fam-rules.md` §12 are the tests, written before the implementation.
- **Integration tests** for API endpoints against a throwaway PostgreSQL container.
- No frontend tests in v1. *Since 0e* the Go side of serving the frontend
  is tested (`internal/web`, `internal/api/router_test.go`) on an in-memory
  file tree, so no build is needed, and CI's smoke test checks the built
  image (below).
- **Highest-value test: AC-16 isolation** — proven twice, once through the API and
  once directly against the database as the application role with the wrong user
  id set, which is the only thing that proves RLS is active rather than merely
  enabled (`docs/fam-rules.md` §12.7). Runs on every change.

### Code quality

`gofmt` (formatting is not a debate in Go) and `golangci-lint`; Prettier and
ESLint on the frontend. *As built in 0e:* ESLint is type-aware
(typescript-eslint `recommendedTypeChecked`, plus the Rules of Hooks) and
Prettier keeps its defaults; `make lint` and CI run both (`npm run lint`).
`npm run format` rewrites files to Prettier's style.

### CI: GitHub Actions

Build, test, lint. A failing run shows a red X on the pull request. This is what
makes "Claude writes, Pe'er reviews" safe — a second opinion that never gets
tired. Since 0c′ it also deploys every merge to `main` (§7).

*As built in Phase 0a* (`.github/workflows/ci.yml`):

- **Triggers:** every pull request, and pushes to `main`. Not every push to every
  branch, because a PR already runs CI on its branch.
- **One job, named `ci`.** That name is the required status check on `main` (see
  Git workflow below), so **renaming the job blocks every PR** until branch
  protection is updated to match.
- Go comes from the `go` line in `go.mod` (CI installs the newest 1.27.x). The
  golangci-lint version matches the Makefile.
- *Added in 0b:* a last step, **Build image**, runs `docker build` (no push, no
  credentials), so `main` can't receive a Dockerfile that doesn't build.
- *Added in 0c:*
  - A **`postgres:17` service container**, with the same `pg_isready` check
    as `compose.yaml`. Steps wait for it to be healthy, never for a fixed time.
  - **Generated code is current:** runs `make sqlc` and fails if anything
    changed, so hand-edits and a forgotten regeneration can't merge.
  - **Migrate:** runs `make migrate-up`, so a migration that doesn't apply to
    an empty database fails the PR before it can reach production.
  - **Test** with `MADAM_TEST_DATABASE_URL` set, which switches on the tests
    that need a database. They skip when it's unset, and fail when it's set
    but unreachable, so CI can't pass by losing its database. It runs with
    **`-count=1`**: `setup-go` restores Go's test-result cache, and Go can't
    see the database, so a docs-only change produced `(cached)` passes for
    the database tests too. In 0d that could hide a broken RLS migration.
  - The Go cache now also keys on `tools/go.sum`. On a cold cache the first
    run took about 4 minutes, mostly compiling sqlc and goose.
- *Changed in 0d:* the tests connect as **`madam_app`**
  (`MADAM_TEST_DATABASE_URL`), the role the server uses, so the isolation test
  I3 sees Row Level Security. **Migrate** still runs as the owner `madam`,
  then sets `madam_app`'s throwaway password with the runner's `psql`.
- *Added in 0c′* (how it works: §7, As built in Phase 0c′):
  - A second job, **`deploy`**, after `ci`, only on `main`. It's skipped on
    PRs and isn't a required check.
  - A **`workflow_dispatch`** trigger: the "Run workflow" button.
  - Runs on `main` are **never cancelled** by a newer one; they queue.
- *Added in 0e* (why: §8.4), in the same `ci` job so the required check is
  unchanged, before the Go steps:
  - `setup-node` with `node-version-file: .nvmrc` and npm's download cache
    keyed on `frontend/package-lock.json`.
  - **Node versions agree:** `.nvmrc` must equal the Dockerfile's `node:`
    tag.
  - **Frontend install** (`npm ci`), **Frontend lint** (ESLint, then
    Prettier's check), and **Frontend type check and build** (`make
    frontend`, which also copies the build into `internal/web/assets`). So
    `go build`, the tests and golangci-lint all see the real frontend.
  - After **Build image**: **Smoke test image** runs the image on the
    runner's network against the service Postgres, waits until it answers,
    and runs `deploy/smoke-test.sh`. "Nothing under `/api/` reaches the
    page" is therefore checked on every PR against the image that would
    deploy.

**Dependabot** (Phase 0a): weekly version updates, one grouped PR per ecosystem
(`gomod` in `/backend`, `github-actions`, since 0b `docker` for the
Dockerfile's base images, and since 0c `gomod` in `/tools`), configured in
`.github/dependabot.yml`.
*Since 0e:* **`npm` in `/frontend`**, with a **7-day cooldown** (a release
is proposed only once it is a week old, because compromised npm versions are
usually found and pulled within days) and TypeScript majors held back until
typescript-eslint supports 7. The `docker` entry holds back Node majors (odd
majors are never LTS; a major move is a decision). A Dependabot PR that moves
the `node:` tag fails "Node versions agree" until `.nvmrc` is updated on the
same PR. Dependabot's own runs also appear on `main`
(`gh run list --branch main`), so look for CI's run with `--workflow ci.yml`:
in 0e, four Dependabot runs crowded an unfiltered list, and the merge's CI
run looked missing when it wasn't.
Dependabot alerts and security updates are also switched on in the repository
settings. Actions are pinned to major tags (`@v7`), not commit SHAs, so the diffs
stay readable.

**Known flakiness, unexplained:** on this repository GitHub has sometimes failed to
start a run, or reported a finished run as still in progress for up to half an
hour (`build-log.md`, Phase 0a). Check `gh run list` and the step timestamps
before suspecting the workflow. Since 0c′ a run that never starts is a deploy
that never happens: start it with "Run workflow" on `main` (§7).

*Added in 0f:* the **Supabase keepalive**, `.github/workflows/keepalive.yml`
(what it keeps awake and with which credential: §5, Mitigation). It's a
separate file because a schedule in `ci.yml` would run the whole CI four times
a day.

- **Schedule:** `41 2,8,14,20 * * *` (UTC), plus `workflow_dispatch` for
  testing by hand. GitHub's docs: the schedule "can be delayed during periods
  of high loads… High load times include the start of every hour… some queued
  jobs may be dropped." Hence :41, and four runs a day: Supabase asks for "a
  few user requests to the database each day", and a dropped run then doesn't
  matter. Scheduled workflows run only on the default branch. *Observed on
  the first day:* nothing ran at 14:41 UTC, and the first scheduled run
  started at 19:00 UTC, which isn't a slot. Expect hours of delay, and judge
  the schedule by days, not by slots.
- **Two jobs,** `production` and `dev`, so a failure names the project.
  **Never name a job `ci`**, the required check. There's no `pull_request`
  trigger, so the workflow never posts a check on a PR and can't become
  required by accident.
- **Least privilege:**
  - `permissions: {}`. The run log still shows `Metadata: read`, which GitHub
    always grants. `dev` adds `contents: read` for a sparse checkout of
    `backend/certs`, with `persist-credentials: false`.
  - There is **no `id-token`** permission, so the workflow can't ask for the
    token deploys use. The deploy condition (§7) also names `ci.yml` only,
    re-read in 0f.
- **Actions:** only `actions/checkout@v7`, by major tag like the rest.
  Dependabot's `github-actions` entry already covers every workflow file. The
  jobs use the runner's `curl` and `psql`.
- **Failing safely:**
  - `timeout-minutes: 5` per job. curl and psql retry a couple of times, 30 s
    apart, to ride out a blip, so a paused project still fails within about
    two minutes. The next run, six hours later, is the real retry.
  - A guard refuses the secret unless it's a `postgres://` or `postgresql://`
    URL with `verify-full`, like the server (`cmd/server/db.go`), so a bad
    value can't quietly try a local socket.
- **Who hears about a failure:** "Notifications for scheduled workflows are
  sent to the user who initially created the workflow". The recipient changes
  to whoever later edits the cron line or re-enables the workflow. Email
  needs Settings → Notifications → System → Actions → Email (opt-in), ideally
  with "Only notify for failed workflows".
- **Finding its runs:** `gh run list --workflow keepalive.yml` (`--event
  schedule` for scheduled ones only). Run it by hand with `gh workflow run
  keepalive.yml --ref main`.

### Git workflow

- `main` is always deployable and protected; no direct pushes. Branch protection:
  a pull request is required (no approvals, since there is one person), linear
  history, and no bypass for admins. **Required status check: `ci`**, added in
  Phase 0a. It's tied to the GitHub Actions app, so no other tool can post a
  `ci` result that satisfies it. It's *not* strict: a PR doesn't have to be
  rebased onto the latest `main` before merging, which saves Dependabot PRs from
  forcing each other to rebase.
- One **short-lived branch per chunk of work**: `feature/daily-entry`,
  `fix/ghost-toggle`. Branch when a change takes more than one commit, is
  experimental, or might be thrown away. Branches are deleted automatically after
  merging — long-lived branches are how solo projects rot. The PR page keeps the
  full history, and "Restore branch" is there if ever needed.
- **Merge via pull request, even solo.** The PR page is where diffs get reviewed
  and "why" gets asked, and where CI runs.
- **Squash merge only** (the other merge methods are disabled in the repository
  settings), so each feature lands as one clean commit. Reverting a feature means
  reverting that one commit.
- Small commits, clear messages ("Add adjusted-temperature calculation", not "stuff").
- `.gitignore` excludes `.env`, binaries and `node_modules`. **No secrets in Git, ever.**
- Git talks to GitHub **over HTTPS**, with `gh` as the credential helper, and
  commits are **signed with an SSH key** so GitHub marks them verified. Commits use
  GitHub's noreply email address, so no personal email is published in the
  history. (SSH transport to GitHub is reset from the development laptop; §10.)
- Tag `v1.0.0` when she starts relying on it.

Rejected: **git-flow** (permanent `develop` branch, release branches) as overkill
for one person.

### Repository visibility

**Public from day one.** _Changed during setup. This section previously said
"private during the build; public at v1 after a review pass."_

Why it changed: on GitHub's free plan, **protected branches exist only on public
repositories**, so "private" and "`main` is protected" could not both be true
without paying. Starting public is also common practice, and the repository never
holds user data — that lives in Supabase, never in Git.

What the original reasoning protected against still stands: **secrets committed
to a public repo are in the history forever**, and public repos are scanned by
bots within minutes. The defences, all in place before the first commit:

- `.gitignore` excludes `.env` and every `.env.*` except `.env.example`, from the
  first commit onward. Tested: a dummy `.env` did not appear in `git status`.
- GitHub **secret scanning and push protection** reject pushes that contain
  recognised secret formats. Verified **enabled** on the repository.
- **Claude Code is denied reading `.env` files** by `.claude/settings.json`
  (committed). Tested: a read attempt was blocked. It is a guardrail, not a vault:
  it covers Claude's file tools and the shell commands Claude Code recognises, not
  a script that opens the file by itself.
- Secrets live only in a local `.env` and in Cloud Run configuration (§9) — never
  in the documents, `CLAUDE.md`, or test fixtures.
- **If a secret is ever pushed: rotate it first**, then clean the history.
  Rotation is the actual fix; once bots have seen a secret, rewriting history is
  only tidying.

The documents refer to **"the first user"**, never by name or relationship, for
the same reason: everything committed is public, permanently.

The review pass before v1 (`build-order.md` Slice 10) stays, as a final sweep
rather than a gate: scan history for secrets, confirm every query filters by user,
confirm RLS policies exist on every table, check the README.

---

## 13. Open items after setup

Decided at step 4 and no longer open: the charting library (§8.3), the free plan
(§5), the palette (§8.2), the Figma handoff (§11), Row Level Security
(`data-model.md` §9), and repository visibility (§12). Decided during setup: the
Cloud Run region and the cost guardrails (§7), the Supabase region and project
options (§5). Decided in the design step: the visual design and a self-hosted
font (§8.2, §11, `design-brief.md`).

**Still genuinely open:**

- **Supabase restore window** — docs say one year, older sources say 90 days.
  Re-check before relying on it. (Re-checked in 0f, 7 November 2026: still one
  year in the docs.)
- **Palette contrast in practice** — the numbers in §8.2 and `design-brief.md`
  §3 set the limits; the first user should confirm the result is readable on
  her phone.

**Moved to the build's verification list** (answers require measuring, not
deciding — also listed in `CLAUDE.md`):

- The first month's billing report: confirm the spend cap measured gross usage and
  the bill stayed at zero (§7).

*Answered in the invite-code spike:* the typed invite code reaches Supabase's
"before user created" hook as `user.user_metadata.invite_code` (§4, As built
in the invite-code spike).

*Answered in Phase 0d:* creating the non-owner application role (§5, Database
roles; `data-model.md` §9).

*Answered in Phase 0c:* how Cloud Run connects and the Belgium–Frankfurt latency
(§5, Connecting), table ownership (§5, Database roles), and the pgx/sqlc type
mappings and `-infinity` round-tripping (`data-model.md` §10).
