import { accessToken } from "./auth.ts";

// Calls to our own API. Every one carries the signed-in user's token; the
// server takes the user's identity from it and from nothing else.

// A response that wasn't a success, with its HTTP status (0: no response).
export class ApiError extends Error {
  status: number;
  constructor(status: number) {
    super(`the API answered ${status}`);
    this.status = status;
  }
}

async function get<T>(path: string): Promise<T> {
  const token = await accessToken();
  if (token === null) {
    throw new ApiError(401);
  }
  let res: Response;
  try {
    res = await fetch(path, {
      cache: "no-store",
      headers: { Authorization: `Bearer ${token}` },
      signal: AbortSignal.timeout(10_000),
    });
  } catch {
    throw new ApiError(0);
  }
  if (!res.ok) {
    throw new ApiError(res.status);
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
  return get<Me>("/api/me");
}
