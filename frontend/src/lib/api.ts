import { ApiError, apiErrorFrom } from "./apiError.ts";
import { accessToken } from "./auth.ts";
import type { Day, DayBody } from "./dayText.ts";
import type { OnboardingBody } from "./onboardingText.ts";

// Calls to our own API. Every one carries the signed-in user's token; the
// server takes the user's identity from it and from nothing else.

export { ApiError };

// call sends one request and returns the decoded answer, or throws an
// ApiError: status 0 when there was no answer at all, 401 when nobody is
// signed in here, otherwise the server's status with its code and field.
async function call<T>(
  method: "GET" | "POST" | "PUT",
  path: string,
  body?: unknown,
) {
  const token = await accessToken();
  if (token === null) {
    throw new ApiError(401);
  }
  const headers: Record<string, string> = { Authorization: `Bearer ${token}` };
  if (body !== undefined) {
    headers["Content-Type"] = "application/json";
  }
  let res: Response;
  try {
    res = await fetch(path, {
      method,
      cache: "no-store",
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
      signal: AbortSignal.timeout(10_000),
    });
  } catch {
    throw new ApiError(0);
  }
  if (!res.ok) {
    // The body may be anything: read it as text and let apiErrorFrom decide.
    throw apiErrorFrom(res.status, await res.text().catch(() => ""));
  }
  return (await res.json()) as T;
}

// The signed-in user's own account (GET /api/me). The first call creates
// her row in our database.
export type Me = {
  id: string;
  temperature_unit: "celsius" | "fahrenheit";
  // null until onboarding is finished.
  onboarded_at: string | null;
};

export function fetchMe(): Promise<Me> {
  return call<Me>("GET", "/api/me");
}

// Stores her onboarding answers (POST /api/onboarding) and returns her
// account as it now is. A second submit is refused with 409
// already_onboarded and changes nothing.
export function submitOnboarding(body: OnboardingBody): Promise<Me> {
  return call<Me>("POST", "/api/onboarding", body);
}

// What one day holds (GET /api/days/{date}): its entry, or null, and her
// usual measurement time on that date. date is "YYYY-MM-DD".
export function fetchDay(date: string): Promise<Day> {
  return call<Day>("GET", `/api/days/${date}`);
}

// Saves the day's entry, new or existing (PUT /api/days/{date}), and
// returns the day as it now is. When she must be asked something first the
// server answers 409 with the question as the error's code; the same body
// is then sent again with that name in `confirmed`.
export function saveDay(date: string, body: DayBody): Promise<Day> {
  return call<Day>("PUT", `/api/days/${date}`, body);
}
