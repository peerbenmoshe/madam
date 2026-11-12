// GoTrueClient is the package's auth client (GoTrue is the old name of
// Supabase Auth's server). Its other export, AuthClient, is the same class
// but can't be used as a type.
import { GoTrueClient } from "@supabase/auth-js";

// The page's one connection to Supabase Auth (docs/tech-stack.md §4). The
// browser talks to Supabase directly to sign up, log in, reset a password
// and keep its token fresh; our own API only ever sees the token.

// Whether this page was opened from a password-reset email. Read here, as
// the script loads, because the client removes the link's tokens from the
// address bar as soon as it has taken them.
//
// A link from Supabase puts its values after the "#": the browser never
// sends that part to a server, so the tokens don't reach ours.
const arrivedBy = new URLSearchParams(window.location.hash.slice(1)).get(
  "type",
);
export const arrivedByRecoveryLink = arrivedBy === "recovery";

type PublicConfig = {
  supabase_url: string;
  supabase_publishable_key: string;
};

let client: Promise<GoTrueClient> | null = null;

// authClient returns the client, creating it on first use. Creating it asks
// our server which Supabase project to use (GET /api/config), so one build
// of the page works locally and in production.
//
// If that fails (offline, server down), the next call tries again.
export function authClient(): Promise<GoTrueClient> {
  client ??= createClient().catch((error: unknown) => {
    client = null;
    throw error;
  });
  return client;
}

async function createClient(): Promise<GoTrueClient> {
  const res = await fetch("/api/config", {
    cache: "no-store",
    signal: AbortSignal.timeout(10_000),
  });
  if (!res.ok) {
    throw new Error(`/api/config answered ${res.status}`);
  }
  const config = (await res.json()) as Partial<PublicConfig>;
  if (!config.supabase_url || !config.supabase_publishable_key) {
    throw new Error("/api/config is missing the Supabase project or key");
  }

  return new GoTrueClient({
    url: `${config.supabase_url}/auth/v1`,
    // The publishable key says which project the request is for. It is
    // public; what a request may do is decided by the user's own token.
    headers: { apikey: config.supabase_publishable_key },
    // Where the session is kept in this browser's localStorage, so she
    // stays logged in between mornings. The strict CSP is what keeps other
    // scripts away from it.
    storageKey: "madam.auth",
    persistSession: true,
    autoRefreshToken: true,
    // Links from our emails carry the session after the "#" ("implicit"
    // flow), so a link opened on another device than the one that asked
    // for it still works. The client picks it up on start.
    flowType: "implicit",
    detectSessionInUrl: true,
  });
}

// accessToken returns the signed-in user's current token for a call to our
// own API, refreshed first if it has expired, or null when signed out.
export async function accessToken(): Promise<string | null> {
  const auth = await authClient();
  const { data } = await auth.getSession();
  return data.session?.access_token ?? null;
}
