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
| Charting | **Recharts** |
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
3. **Transaction wrapper** — open a transaction and set the user id as a session
   variable, so Row Level Security policies can see who is asking
   (`data-model.md` §9). Every data-touching request goes through it.
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
auth"), sub-routers, and a set of ready-made middleware. The transaction wrapper
in §1.4 is itself chi middleware.

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
  *Unverified:* that the user-typed code reaches the hook cleanly (expected via
  sign-up metadata). To confirm during the build.
- **EU data region** available, which should help the later GDPR check.
- Email verification and password reset are built in.
- **Low lock-in:** the auth server is open source and self-hostable later. It is
  written in Go.
- Bundles with the PostgreSQL database (section 5), so one vendor and one region.

Known limits: free-tier email sending is rate-limited, so a real email service will
likely be needed before public use.

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
  PostgreSQL connection are unaffected.
- **"Enable automatic RLS": off.** It installs an event trigger that enables RLS
  on every new table in `public`. Rejected because it is schema behaviour that
  lives outside Git, the local Postgres container wouldn't have it, and every
  migration already enables **and forces** RLS explicitly — the trigger only
  enables it, and writes no policies. Reversible later with SQL.

**Database roles (step 4).** Because RLS is in, the API must connect as a role
that **does not own the tables**, and the tables carry `FORCE ROW LEVEL SECURITY`.
Postgres lets table owners bypass row security unless forced, so connecting as the
owner would mean RLS is enabled and doing nothing. *Unverified:* who ends up
owning tables created by goose migrations on Supabase, and how straightforward
creating a separate application role is there. On the setup checklist.

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
- **Cloud Run's choice is open**, decided in Phase 0c: the session pooler, the
  transaction pooler with prepared statements off, or the direct connection if
  Cloud Run's outbound traffic has IPv6. *Not checked yet.*
- Connection strings and database passwords live in Pe'er's password manager,
  never in the repository. The passwords use letters and digits only, so they can
  sit inside a connection URL without escaping.

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

Inactivity means no API requests, no database connections and no dashboard logins.

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
   number as changeable rather than something to rely on.
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

Pausing is triggered purely by inactivity, and **any request counts**, including a
health check on a schedule. A small **GitHub Actions job pinging the API every few
days** keeps the project awake indefinitely, at no cost. GitHub Actions is already
in the stack (section 12), so this is a few lines. Ready-made keepalive scripts for
Supabase exist.

**Skeleton task:** add the keepalive job as part of the walking skeleton
(section 9), covering **both** the production and the development project.

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

### Considered and rejected: Starter Tier

Google's new Starter Tier runs apps with no billing account at all, under separate
terms. It does not fit: it is offered only through Google AI Studio deployments,
regions, IAM, quotas and enabled APIs are fixed by Google, no other APIs can be
enabled in the project, and its terms limit it to business, trade or professional
use. Our own Dockerfile and deployment pipeline need a standard project.

### Costs and unknowns

- A Google Cloud account with a card on file is required.
- **Cold start** on the first request after idle. *Assumption:* a second or two,
  since Go starts fast. To be measured.
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

### 8.4 Delivery: embedded in the Go binary

TypeScript is a build-time language: the build produces plain HTML, JS and CSS.
Go's `//go:embed` then packs those files into the compiled binary, giving **one
artefact, one URL, and no cross-domain (CORS) configuration**.

During development the frontend dev server runs separately for fast reloads;
embedding happens only in the real build.

Rejected: a separate static host (Cloudflare Pages, Netlify). It would show the
page shell instantly even during a cold start, but costs two deployments and CORS
setup. Reversal later is roughly an afternoon.

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
- **Production (Cloud Run + Supabase):** the real thing, with her real data.

Rules:

- **Never test against production data.** Use a **separate Supabase project for
  development**, clearly named, so a bad migration cannot touch her history. The
  keepalive job should cover this project too.
- **The production database URL never goes into a local `.env`.** Production
  credentials live in Cloud Run configuration and, for the keepalive job, in
  GitHub Actions secrets. How migrations reach production is decided in Phase 0c.
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
  2**, and set environment variables in Cloud Run.
- Supabase: two projects (prod and dev), Frankfurt, Data API off. ✅
  **(Phase 0)** The non-owner application role.
- Secrets (database URL, invite code, Supabase keys) live in Cloud Run
  configuration and a local `.env` — **never in Git**. **(Phase 0)** Variable
  names and `.env.example`.

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
| Go | 1.27.1 | Official tarball, `/usr/local/go` |
| Node / npm | 24 LTS / 11 | `nvm` (shadows a Windows npm that is also on PATH) |
| Docker | 29.4 (Desktop 4.71), Compose 5.1 | Docker Desktop with WSL integration |
| goose | 3.29 | `go install` (unpinned — see `build-order.md` 0a) |
| sqlc | 1.31 | `go install` (unpinned) |
| golangci-lint | 2.15 | Official install script (unpinned) |
| psql | 14 | Ubuntu's `postgresql-client` — older than the server, fine for ad-hoc queries |
| gh | 2.102 | GitHub's apt repository |
| gcloud | SDK 590 | Google's apt repository |
| Claude Code | 2.1.x | Native installer, auto-updating |

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

## 11. The design handoff (optional, post-v1)

The first user is a junior UX/UI designer and may design a nicer UI after v1
ships. **She has no Figma plan**, so at step 4 this moved from "planned workflow"
to "available if she wants it later." Nothing in v1 depends on it.

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

Free and worth doing either way: having her review the spec's screens and flows,
and sanity-check the palette on a phone.

---

## 12. Development practices

### Testing

- Go's standard-library `testing` package; no dependencies needed.
- **Unit tests** for the FAM domain logic — the test vectors in
  `docs/fam-rules.md` §12 are the tests, written before the implementation.
- **Integration tests** for API endpoints against a throwaway PostgreSQL container.
- No frontend tests in v1.
- **Highest-value test: AC-16 isolation** — proven twice, once through the API and
  once directly against the database as the application role with the wrong user
  id set, which is the only thing that proves RLS is active rather than merely
  enabled (`docs/fam-rules.md` §12.7). Runs on every change.

### Code quality

`gofmt` (formatting is not a debate in Go) and `golangci-lint`; Prettier and
ESLint on the frontend.

### CI: GitHub Actions

Runs on every push: build, test, lint. A failing run shows a red X on the pull
request. This is what makes "Claude writes, Pe'er reviews" safe — a second opinion
that never gets tired. Later it also handles automatic deployment on merge to `main`.

Also hosts the Supabase keepalive job (section 5).

### Git workflow

- `main` is always deployable and protected; no direct pushes. Branch protection:
  a pull request is required (no approvals, since there is one person), linear
  history, and no bypass for admins. Required status checks are added once CI
  exists (`build-order.md` 0a).
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
options (§5).

**Still genuinely open:**

- **Supabase restore window** — docs say one year, older sources say 90 days.
  Re-check before relying on it.
- **Palette contrast in practice** — the numbers in §8.2 set the limits; the first
  user should confirm the result is readable on her phone.

**Moved to the build's verification list** (answers require measuring, not
deciding — also listed in `CLAUDE.md`):

- How Cloud Run connects to Supabase (§5, Connecting), and the Belgium–Frankfurt
  query latency.
- Cold-start time on Cloud Run, once the skeleton is deployed.
- The first month's billing report: confirm the spend cap measured gross usage and
  the bill stayed at zero (§7).
- Whether the typed invite code reaches Supabase's "before user created" hook.
- Who owns tables created by goose migrations on Supabase, and how to create a
  non-owner application role.
- pgx/sqlc type mappings and `-infinity` round-tripping (`data-model.md` §10).
