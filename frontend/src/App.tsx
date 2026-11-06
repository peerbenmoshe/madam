import { useEffect, useState } from "react";

// What the page knows about the server.
type Health =
  { state: "checking" } | { state: "ok" } | { state: "down"; reason: string };

const TIMEOUT_MS = 5000;

// Asks the API whether it can reach the database. Never throws: every
// failure becomes a reason to show.
async function fetchHealth(): Promise<Health> {
  try {
    const res = await fetch("/api/health", {
      cache: "no-store",
      signal: AbortSignal.timeout(TIMEOUT_MS),
    });
    const isText = res.headers.get("Content-Type")?.startsWith("text/plain");
    const body = isText ? (await res.text()).trim() : "";
    if (res.ok && body === "ok") {
      return { state: "ok" };
    }
    // The server's own words ("database unavailable") when it sent some,
    // otherwise just the status: never an HTML error page as text.
    return { state: "down", reason: body || `HTTP ${res.status}` };
  } catch {
    return { state: "down", reason: "can't reach the server" };
  }
}

export default function App() {
  const [health, setHealth] = useState<Health>({ state: "checking" });

  useEffect(() => {
    // Ignore an answer that arrives after the page has gone (or, in
    // development, after React's StrictMode has run the effect twice).
    let current = true;
    void fetchHealth().then((h) => {
      if (current) setHealth(h);
    });
    return () => {
      current = false;
    };
  }, []);

  function checkAgain() {
    setHealth({ state: "checking" });
    void fetchHealth().then(setHealth);
  }

  return (
    <main className="mx-auto flex min-h-dvh max-w-md flex-col justify-center gap-8 px-6 py-10">
      <h1 className="text-4xl font-semibold tracking-tight">Madam</h1>

      <section className="flex flex-col gap-3" aria-live="polite">
        <h2 className="text-muted text-sm font-medium tracking-wide uppercase">
          Server
        </h2>
        <Status health={health} />
      </section>

      <button
        type="button"
        onClick={checkAgain}
        disabled={health.state === "checking"}
        className="border-ink min-h-12 rounded-full border-2 px-6 text-base font-medium disabled:opacity-50"
      >
        Check again
      </button>
    </main>
  );
}

function Status({ health }: { health: Health }) {
  switch (health.state) {
    case "checking":
      return <p className="text-muted text-lg">Checking…</p>;
    case "ok":
      // HotPink only as a fill, with dark text on it (§8.2).
      return (
        <p className="bg-accent text-ink self-start rounded-full px-4 py-2 text-lg font-medium">
          ok
        </p>
      );
    case "down":
      return (
        <p className="border-ink text-ink self-start rounded-full border-2 px-4 py-2 text-lg">
          {health.reason}
        </p>
      );
  }
}
