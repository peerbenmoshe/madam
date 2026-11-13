# Browser checks

Two scripts that use the sign-in screens the way a person on a phone does:
they type, tap, read what the screen says and take screenshots, in a real
(headless) Chrome at a 390px width. They were written to verify Slice 2 and
caught two bugs that types, lint and unit tests could not see: a control
stretched across the screen, and a second tab stranded on "Choose a new
password".

| Script | What it covers |
|---|---|
| `auth-screens.mjs` | Log in, Create account, Forgot password, expired links, the signed-in page with a session the server rejects, Log out, the browser's Back button, and the `/design` gallery |
| `auth-first-use.mjs` | The second tab during a password reset, "this email already has an account", "Send it again" and its one-minute wait, on sign-up and on reset |

Both also fail on any Content-Security-Policy violation.

## Running them

They run **by hand, on the development machine**. CI doesn't run them: they
need a browser, and a server with a real Supabase project behind it.

1. In one terminal, start the production image locally (the real CSP, with
   madam-dev's Supabase):

   ```
   make docker-run
   ```

2. In another:

   ```
   sh deploy/browser-check/run.sh
   ```

Each check prints `PASS` or `FAIL`, and the run ends with `ALL CHECKS
PASSED` or a count. Screenshots land in `deploy/browser-check/screenshots/`.

To see a check fail on purpose, point it at a server that has the bug. The
stranded-tab check was first run against production while production still
had it:

```
sh deploy/browser-check/run.sh https://madam-966460417479.europe-west1.run.app
```

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
  never reaches Supabase, so **no account is created and no email is sent**.

Run against production they do the same there; the wrong log-ins count
toward Supabase's rate limit for a few minutes.

## When a screen changes

The checks look for the words on the screen. Changing a message or a label
means changing it here too, and a failing check after such a change is the
reminder.
