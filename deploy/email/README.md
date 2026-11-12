# Auth email templates

The emails Supabase Auth sends for Madam. Supabase keeps the live copy in
its dashboard, which isn't in Git, so **these files are the source**: change
a file here first, in a PR, then paste it into the dashboard.

| File | Dashboard template | Subject |
|---|---|---|
| `confirm-signup.html` | Confirm signup | `Welcome to Madam: confirm your email` |
| `reset-password.html` | Reset password | `Choose a new password for Madam` |

## Putting a template into the dashboard

For **madam-prod** (and madam-dev if you want them there too):

1. Dashboard → Authentication → Emails → Templates.
2. Choose the template named in the table.
3. Set the subject from the table.
4. Replace the whole message body with the file's contents, from the first
   `<div` to the end. Leave the comment at the top of the file out.
5. Save, then send yourself one: sign up with a new address, or use "Forgot
   password?".

## Rules for these emails

- **Discreet.** They sit in an inbox and show on a lock screen, so they say
  "Madam" and never what the app is for (decided by Pe'er, 13 November
  2026).
- **Inline styles only, device fonts.** Email clients drop stylesheets and
  web fonts. The colours are the app's tokens (`docs/design-brief.md` §3).
- **The link appears twice:** as the button, and as text for clients that
  don't draw the button.
- **`{{ .ConfirmationURL }}`** is the only variable used. Supabase fills it
  in with the link that confirms the address or starts the reset.
- **The link's lifetime isn't stated,** only "once" and "for a while": it
  is a dashboard setting (Authentication → Sign In / Providers → Email →
  "Email OTP Expiration"), and a number here could drift from it.

The other templates (Magic Link, Invite user, Change Email Address,
Reauthentication) are left as Supabase's defaults: the app doesn't trigger
them.

## Looking at one before pasting

Replace the variable with any address and open the file in a browser:

```
sed 's|{{ .ConfirmationURL }}|https://example.com/link|g' deploy/email/confirm-signup.html > /tmp/preview.html
```
