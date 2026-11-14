// What a failed call to our own API becomes (docs/tech-stack.md §3). The
// server answers every error as
//
//   {"error": {"code": "date_in_future", "field": "cycle_start_date", "message": "…"}}
//
// The page reads the code and the field. The message is for a developer
// and is dropped here, so it can't reach the screen by accident: the words
// a person sees are in the text modules (authText.ts, onboardingText.ts).

export class ApiError extends Error {
  // The HTTP status; 0 when there was no response at all.
  status: number;
  // The server's stable name for what went wrong, when it gave one.
  code?: string;
  // The request's field the error is about, when it is about one.
  field?: string;

  constructor(status: number, code?: string, field?: string) {
    super(`the API answered ${status}${code ? ` ${code}` : ""}`);
    this.status = status;
    this.code = code;
    this.field = field;
  }
}

// apiErrorFrom reads a failed response's body. Anything that isn't the
// shape above (a proxy's HTML page, an empty body, the router's 405) is
// just its status.
export function apiErrorFrom(status: number, body: string): ApiError {
  let parsed: unknown;
  try {
    parsed = JSON.parse(body);
  } catch {
    return new ApiError(status);
  }
  const error =
    typeof parsed === "object" && parsed !== null && "error" in parsed
      ? parsed.error
      : null;
  if (typeof error !== "object" || error === null) {
    return new ApiError(status);
  }
  const code = "code" in error ? error.code : undefined;
  const field = "field" in error ? error.field : undefined;
  return new ApiError(
    status,
    typeof code === "string" ? code : undefined,
    typeof field === "string" ? field : undefined,
  );
}
