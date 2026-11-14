// Run by `npm test`. Plain JavaScript for the reason given in date.test.mjs.
import assert from "node:assert/strict";
import { test } from "node:test";

const { ApiError, apiErrorFrom } = await import("./apiError.ts");

test("an error in the API's shape gives its code and field", () => {
  const error = apiErrorFrom(
    422,
    '{"error":{"code":"date_in_future","field":"cycle_start_date","message":"cycle_start_date is after today"}}',
  );
  assert.ok(error instanceof ApiError);
  assert.equal(error.status, 422);
  assert.equal(error.code, "date_in_future");
  assert.equal(error.field, "cycle_start_date");
});

test("an error without a field has none", () => {
  const error = apiErrorFrom(
    409,
    '{"error":{"code":"already_onboarded","message":"onboarding is already done; nothing was changed"}}',
  );
  assert.equal(error.code, "already_onboarded");
  assert.equal(error.field, undefined);
});

test("the server's message is never kept: it isn't for the person", () => {
  const error = apiErrorFrom(
    500,
    '{"error":{"code":"internal","message":"internal error"}}',
  );
  assert.equal(error.message.includes("internal error"), false);
  assert.equal(JSON.stringify(error).includes("internal error"), false);
});

test("anything else is just a status: a proxy's page, an empty body, a 405", () => {
  for (const body of [
    "",
    "<html>502 Bad Gateway</html>",
    "not found",
    "null",
    "[]",
    '"text"',
    "{}",
    '{"error":null}',
    '{"error":"text"}',
    '{"error":{"code":42,"field":["x"]}}',
  ]) {
    const error = apiErrorFrom(502, body);
    assert.equal(error.status, 502, body);
    assert.equal(error.code, undefined, body);
    assert.equal(error.field, undefined, body);
  }
});
