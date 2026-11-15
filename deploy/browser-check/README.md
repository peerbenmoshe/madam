# Browser checks

Scripts that use the screens the way a person on a phone does: they type,
tap, read what the screen says and take screenshots, in a real (headless)
Chrome at a 390px width. They were written to verify Slice 2 and caught two
bugs that types, lint and unit tests could not see: a control stretched
across the screen, and a second tab stranded on "Choose a new password".

| Script | What it covers |
|---|---|
| `auth-screens.mjs` | Log in, Create account, Forgot password, expired links, the signed-in page with a session the server rejects, Log out, the browser's Back button, and the `/design` gallery |
| `auth-first-use.mjs` | The second tab during a password reset, "this email already has an account", "Send it again" and its one-minute wait, on sign-up and on reset |
| `onboarding.mjs` | The onboarding screen in every state: nothing answered, each answer, each limit on the date, a slow answer, each refusal the server can give, an account that can't be read, and the half hour after midnight in Israel. Staged: see below |
| `onboarding-real.mjs` | Onboarding once, for real: a test account logs in, answers, and the real server stores it. Only with `make browser-check` |
| `day.mjs` | The day screen in every state: an empty day, an entry, the live line, both questions, each refusal, the date box as the way between days, addresses that aren't a day, a lost connection, and a screen left open past midnight. Staged, like `onboarding.mjs` |
| `day-real.mjs` | The day screen for real: the test account saves two days through the screen, and the real server asks its two questions and stores the answers. It onboards the account through the API if it hasn't been, so it also runs alone |

All of them also fail on any Content-Security-Policy violation.

## Running them

They run **by hand, on the development machine**. CI doesn't run them: they
need a browser, and a server with a real Supabase project behind it.

1. In one terminal, start the production image locally (the real CSP, with
   madam-dev's Supabase and the local database):

   ```
   make docker-run
   ```

2. In another:

   ```
   make browser-check
   ```

   or, without the real pass, `sh deploy/browser-check/run.sh`.

Each check prints `PASS` or `FAIL`, and each script ends with `ALL CHECKS
PASSED` or a count. Screenshots land in `deploy/browser-check/screenshots/`.

To see a check fail on purpose, point the staged ones at a server that has
the bug. The stranded-tab check was first run against production while
production still had it:

```
sh deploy/browser-check/run.sh https://madam-966460417479.europe-west1.run.app
```

## Staged, and real

Some things can't be produced with real requests, or shouldn't be: an
account for every run, an email, a server error on demand. So the scripts
**stage** them. Chrome's debugging protocol lets a script pause a request
the page makes and answer it itself, put a session into the page's storage,
and set the page's clock and time zone.

`onboarding.mjs` is staged throughout. Its signed-in person is a made-up
session, and it answers `GET /api/me` and `POST /api/onboarding` itself. So
it can show the page a 409, a 422, a lost connection and a slow answer, and
it reads the exact JSON the page sends. What it can't show is whether the
page and the server agree.

`day.mjs` is staged the same way, for `GET /api/me` and `GET` and `PUT
/api/days/{date}`, and also moves the page's clock past midnight while the
day screen is open.

`onboarding-real.mjs` and `day-real.mjs` are the opposite: nothing staged. It logs in as a real
account, and the page's token, our API and the database all meet. It also
sends that real token straight at the endpoint: a second submit must be a
409, a date three days ahead a 422.

### The test account

One account on madam-dev, made for this and nothing else:

1. In madam-dev's dashboard: Authentication → Users → Add user → Create new
   user. An address of yours, a password of letters and digits (at least 8,
   with a letter and a digit), and "Auto Confirm User" on. No email is sent.
2. Copy the new user's UID from the list.
3. Add three lines to `.env` (names in `.env.example`):
   `MADAM_CHECK_EMAIL`, `MADAM_CHECK_PASSWORD`, `MADAM_CHECK_USER_ID`.

Onboarding happens once per account. So before each run `make browser-check`
deletes that user's rows from the **local** database, as the owner; the
cascade removes everything of hers. The identity stays in madam-dev. This is
why the real pass needs `make docker-run` with the default `DB=local`: with
`DB=dev` her rows would be in madam-dev, where nothing resets them, and the
second run would stop with "already onboarded".

The password reaches the script through the environment (`WSLENV`, which
names the variables WSL hands to a Windows program). It is typed into the
page and never printed, logged or put on a command line.

## What they need

This setup is specific to the development machine: WSL, with **Chrome and
Node 22 or newer installed on the Windows side** (WSL itself has no
browser). `run.sh` copies the scripts to Windows' temp folder and runs them
with Windows' Node, because Chrome's debugging port listens on Windows' own
localhost, which WSL can't reach. On another machine, run the `.mjs` files
with any Node 22+ and change the `CHROME` path at the top of each.

There is no dependency: the scripts speak Chrome's debugging protocol over
Node's built-in `WebSocket`.

## What they do to madam-dev

Almost nothing, and nothing lasting:

- a few log-ins with a wrong password, and one reset request, for an
  address that has no account (`nobody-here@example.com`);
- every sign-up and "send again" is answered by the script itself and
  never reaches Supabase, so **no account is created and no email is sent**;
- `onboarding.mjs` sends one log-out for its made-up session, which
  Supabase refuses;
- the real pass logs the test account in and out, once.

Run against production the staged scripts do the same there; the wrong
log-ins count toward Supabase's rate limit for a few minutes. **The real
pass is never pointed at production:** its account exists only on
madam-dev.

## When a screen changes

The checks look for the words on the screen. Changing a message or a label
means changing it here too, and a failing check after such a change is the
reminder.
