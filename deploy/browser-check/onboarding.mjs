// Browser check 3: the onboarding screen, in every state, as a person on a
// phone would use it. See README.md in this folder, and auth-screens.mjs for
// how the browser is driven.
//
// Nothing here reaches our API or Supabase. The script stages:
//   - a signed-in person, by putting a made-up session in the page's storage;
//   - the server's answers to GET /api/me and POST /api/onboarding, by
//     answering those requests itself. That is how it shows the page a 409,
//     a refusal, a lost connection and a slow answer, and how it reads the
//     exact JSON the page sends;
//   - the device's clock and time zone, for the half hour after midnight.
//
// So it proves what the page does, not that the page and the server agree.
// That is onboarding-real.mjs.
//
//   node onboarding.mjs <folder for screenshots> [BASE]
import { spawn } from "node:child_process";
import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

const CHROME = "C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe";
const BASE = process.argv[3] ?? "http://localhost:8080";
const OUT = process.argv[2];
const PORT = 9333;
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

const chrome = spawn(CHROME, ["--headless=new", `--remote-debugging-port=${PORT}`,
  `--user-data-dir=${mkdtempSync(join(tmpdir(), "madam-cdp-"))}`, "--no-first-run", "--disable-gpu", "about:blank"], { stdio: "ignore" });

let version;
for (let i = 0; i < 100 && !version; i++) {
  try { version = await (await fetch(`http://127.0.0.1:${PORT}/json/version`)).json(); } catch { await sleep(100); }
}
const ws = new WebSocket(version.webSocketDebuggerUrl);
await new Promise((r) => ws.addEventListener("open", r));
let nextId = 1;
const pending = new Map();
const consoleLines = [];
ws.addEventListener("message", (m) => {
  const msg = JSON.parse(m.data);
  if (msg.id && pending.has(msg.id)) {
    const { resolve, reject } = pending.get(msg.id);
    pending.delete(msg.id);
    msg.error ? reject(new Error(msg.error.message)) : resolve(msg.result);
  } else if (msg.method === "Log.entryAdded") {
    consoleLines.push(`[${msg.params.entry.level}] ${msg.params.entry.text}`);
  } else if (msg.method === "Runtime.consoleAPICalled") {
    consoleLines.push(`[console.${msg.params.type}] ${msg.params.args.map((a) => a.value ?? a.description).join(" ")}`);
  }
});
const send = (method, params = {}, sessionId) => new Promise((resolve, reject) => {
  const id = nextId++;
  pending.set(id, { resolve, reject });
  ws.send(JSON.stringify({ id, method, params, sessionId }));
});
const { targetId } = await send("Target.createTarget", { url: "about:blank" });
const { sessionId } = await send("Target.attachToTarget", { targetId, flatten: true });
const S = (method, params) => send(method, params, sessionId);
await S("Page.enable"); await S("Runtime.enable"); await S("Log.enable");
// A phone: 390 CSS pixels wide, which plain headless mode can't do.
await S("Emulation.setDeviceMetricsOverride", { width: 390, height: 844, deviceScaleFactor: 2, mobile: true });

async function js(expression) {
  const r = await S("Runtime.evaluate", { expression, awaitPromise: true, returnByValue: true });
  if (r.exceptionDetails) throw new Error(r.exceptionDetails.exception?.description ?? r.exceptionDetails.text);
  return r.result.value;
}
async function waitFor(expression, what, ms = 12000) {
  const end = Date.now() + ms;
  while (Date.now() < end) {
    if (await js(`Boolean(${expression})`)) return true;
    await sleep(100);
  }
  console.log(`   (timed out waiting for: ${what})`);
  return false;
}
const settled = `document.querySelector("h1") && !document.body.innerText.includes("Loading…")`;
async function goto(path) {
  await S("Page.navigate", { url: BASE + path });
  await sleep(150);
  await waitFor(settled, `page ${path}`);
}
const q = JSON.stringify;
async function click(text) {
  await js(`(() => { const b = [...document.querySelectorAll("button")].find((b) => b.textContent.trim() === ${q(text)});
    if (!b) throw new Error("no button " + ${q(text)}); b.click(); })()`);
  await sleep(150);
}
// Taps one of the two answers: an option button is a label over a radio.
async function choose(text) {
  await js(`(() => { const l = [...document.querySelectorAll("fieldset label")].find((l) => l.textContent.trim() === ${q(text)});
    if (!l) throw new Error("no answer " + ${q(text)}); l.click(); })()`);
  await sleep(100);
}
// A pick in a native picker, which can't be opened headless: the input's
// value is set the way the browser sets it, and the page is told.
async function pick(type, value) {
  await js(`(() => { const i = document.querySelector('input[type="${type}"]');
    if (!i) throw new Error("no ${type} input on the screen");
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value").set.call(i, ${q(value)});
    i.dispatchEvent(new Event("input", { bubbles: true })); i.dispatchEvent(new Event("change", { bubbles: true })); })()`);
  await sleep(100);
}
const boxText = (type) => js(`document.querySelector('input[type="${type}"]')?.closest("label").innerText ?? "(no ${type} box)"`);
const bodyText = () => js(`document.body.innerText`);
const path = () => js(`location.pathname`);
const h1 = () => js(`document.querySelector("h1")?.textContent ?? ""`);
const has = async (s) => (await bodyText()).includes(s);
const submitDisabled = () => js(`document.querySelector('button[type="submit"]')?.disabled ?? null`);
const idle = () => waitFor(`!document.querySelector('button[type="submit"]')?.disabled`, "the form to finish");
async function shot(name) {
  const { data } = await S("Page.captureScreenshot", { format: "png", captureBeyondViewport: true });
  writeFileSync(join(OUT, name + ".png"), Buffer.from(data, "base64"));
}
let failed = 0;
function check(name, ok, detail = "") {
  if (!ok) failed++;
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${ok || !detail ? "" : "\n        got: " + detail}`);
}

// ---- the staged server ----
const NOT_ONBOARDED = { id: "00000000-0000-4000-8000-000000000000", temperature_unit: "celsius", onboarded_at: null };
const ONBOARDED = { ...NOT_ONBOARDED, onboarded_at: "2026-11-14T07:00:00Z" };
const apiError = (code, field) => ({ error: { code, ...(field ? { field } : {}), message: "a message for developers, never shown" } });
// What the next request gets: { status, body }, with an optional delay in
// milliseconds, or "offline" for no answer at all.
let meReply = { status: 200, body: NOT_ONBOARDED };
let onboardingReply = { status: 200, body: ONBOARDED };
const sent = []; // every POST /api/onboarding the page made

ws.addEventListener("message", (m) => {
  const msg = JSON.parse(m.data);
  if (msg.method !== "Fetch.requestPaused") return;
  const { requestId, request } = msg.params;
  const isOnboarding = new URL(request.url).pathname === "/api/onboarding";
  if (isOnboarding) sent.push({ method: request.method, headers: request.headers, body: request.postData });
  const reply = isOnboarding ? onboardingReply : meReply;
  if (reply === "offline") return void S("Fetch.failRequest", { requestId, errorReason: "InternetDisconnected" });
  setTimeout(() => void S("Fetch.fulfillRequest", { requestId, responseCode: reply.status,
    responseHeaders: [{ name: "Content-Type", value: "application/json" }, { name: "Cache-Control", value: "no-store" }],
    body: Buffer.from(JSON.stringify(reply.body)).toString("base64") }), reply.delay ?? 0);
});
await S("Fetch.enable", { patterns: [{ urlPattern: "*/api/me" }, { urlPattern: "*/api/onboarding" }] });

const fakeSession = { access_token: "eyJhbGciOiJFUzI1NiIsImtpZCI6IngifQ.eyJzdWIiOiJ4In0.AAAA", refresh_token: "made-up", token_type: "bearer",
  expires_in: 3600, expires_at: Math.floor(Date.now() / 1000) + 3600,
  user: { id: "00000000-0000-4000-8000-000000000000", aud: "authenticated", email: "someone@example.com", app_metadata: {}, user_metadata: {}, created_at: "2026-11-12T00:00:00Z" } };
// Today as the page's own device sees it, and the days either side.
const deviceDay = (offset) => js(`(() => { const d = new Date(); d.setDate(d.getDate() + ${offset});
  return d.getFullYear() + "-" + String(d.getMonth() + 1).padStart(2, "0") + "-" + String(d.getDate()).padStart(2, "0"); })()`);
const ddmmyyyy = (iso) => `${iso.slice(8, 10)}/${iso.slice(5, 7)}/${iso.slice(0, 4)}`;
const lastSent = () => sent[sent.length - 1];
const lastBody = () => JSON.parse(lastSent().body);
const same = (a, b) => JSON.stringify(a) === JSON.stringify(b);

try {
  // ---- a signed-in person who hasn't onboarded sees only onboarding ----
  await goto("/design");
  await js(`localStorage.setItem("madam.auth", ${q(JSON.stringify(fakeSession))})`);
  await goto("/");
  check("not onboarded: the onboarding screen, at /", (await h1()) === "Set up your chart" && (await path()) === "/", `${await path()} ${await h1()}`);
  check("nothing is chosen for her: no answer, no date box, no time",
    (await js(`document.querySelectorAll('input[type="radio"]:checked').length`)) === 0 && (await js(`document.querySelector('input[type="date"]') === null`)) && (await boxText("time")).includes("hh:mm"),
    `${await js(`document.querySelectorAll('input[type="radio"]:checked').length`)} chosen; ${await boxText("time")}`);
  check("the two answers and the one button are there", (await has("I know the date")) && (await has("I don't know")) && (await submitDisabled()) === false);
  await shot("p-onboarding");
  for (const elsewhere of ["/signup", "/onboarding", "/home", "/day/2026-11-14"]) {
    await goto(elsewhere);
    check(`typing ${elsewhere} doesn't get past it`, (await h1()) === "Set up your chart" && (await path()) === "/", `${await path()} ${await h1()}`);
  }

  // ---- what is missing is said under its own control, and nothing is sent ----
  await click("Start charting");
  check("an untouched form: both questions are named", (await has('Choose a date or "I don\'t know".')) && (await has("Pick a time.")), await bodyText());
  check("…and nothing was sent", sent.length === 0, `${sent.length} requests`);
  await shot("p-onboarding-empty");

  await choose("I know the date");
  check("\"I know the date\" shows the date box, empty", (await boxText("date")).includes("dd/mm/yyyy"), await boxText("date"));
  check("…and the first message goes, the date's own takes its place", !(await has('Choose a date or "I don\'t know".')) && (await has("Pick the date your cycle started.")), await bodyText());

  // ---- the date: up to the device's today ----
  const today = await deviceDay(0), tomorrow = await deviceDay(1);
  await pick("date", tomorrow);
  check("tomorrow is refused by the box, with the limit in dd/mm/yyyy", (await has(`Pick a date up to ${ddmmyyyy(today)}.`)) && (await boxText("date")).includes("dd/mm/yyyy"), await bodyText());
  // On a computer, typing the year 2026 passes through 0002: the box must
  // take it (a refusal would put the old value back mid-typing), and the
  // form must refuse it when she submits.
  await pick("date", "0002-11-01");
  check("a year typed halfway isn't interrupted", (await boxText("date")).includes("01/11/0002"), await boxText("date"));
  await pick("time", "07:05");
  await click("Start charting");
  check("…but it can't be sent: the limits are named", (await has(`Pick a date from 01/01/2000 to ${ddmmyyyy(today)}.`)) && sent.length === 0, await bodyText());
  await pick("date", today);
  check("today is accepted, shown dd/mm/yyyy, and the message goes", (await boxText("date")).includes(ddmmyyyy(today)) && !(await has("Pick a date")), await bodyText());
  check("the time box reads 24-hour", (await boxText("time")).includes("07:05"), await boxText("time"));
  await shot("p-onboarding-known");

  // ---- a slow answer, then success ----
  onboardingReply = { status: 200, body: ONBOARDED, delay: 900 };
  await click("Start charting");
  check("while the answer is on its way the button is off", (await submitDisabled()) === true);
  await waitFor(`document.querySelector("h1")?.textContent === "You're logged in"`, "the signed-in page");
  check("a known start: exactly one request", sent.length === 1, `${sent.length} requests`);
  check("…a POST of JSON, with her token", lastSent().method === "POST" && /^application\/json/.test(lastSent().headers["Content-Type"] ?? "") && /^Bearer /.test(lastSent().headers["Authorization"] ?? ""), q(lastSent().headers));
  check("…holding exactly the three fields the server reads", same(lastBody(), { cycle_start_unknown: false, cycle_start_date: today, measurement_time: "07:05" }), lastSent().body);
  check("after onboarding: the signed-in page, at /", (await h1()) === "You're logged in" && (await path()) === "/" && (await has("Your account is ready.")), await bodyText());
  await shot("p-onboarding-done");

  meReply = { status: 200, body: ONBOARDED };
  await goto("/");
  check("once onboarded, a reload never shows onboarding again", (await h1()) === "You're logged in", await h1());

  // ---- "I don't know" ----
  meReply = { status: 200, body: NOT_ONBOARDED };
  onboardingReply = { status: 200, body: ONBOARDED };
  await goto("/");
  await choose("I know the date");
  await pick("date", today);
  await choose("I don't know");
  check("\"I don't know\" replaces the date box with what it will mean", (await js(`document.querySelector('input[type="date"]') === null`)) && (await has('Cycle days will show as "?"')), await bodyText());
  await pick("time", "06:30");
  await shot("p-onboarding-unknown");
  await click("Start charting");
  await waitFor(`document.querySelector("h1")?.textContent === "You're logged in"`, "the signed-in page");
  check("\"I don't know\": no date is sent at all, not even the one picked before", same(lastBody(), { cycle_start_unknown: true, measurement_time: "06:30" }), lastSent().body);

  // ---- what the server can answer ----
  const refill = async () => { await goto("/"); await choose("I know the date"); await pick("date", today); await pick("time", "07:05"); };
  meReply = { status: 200, body: NOT_ONBOARDED };

  await refill();
  onboardingReply = { status: 422, body: apiError("date_in_future", "cycle_start_date") };
  await click("Start charting"); await idle();
  check("the server says the date is in the future: said under the date, in her words", (await h1()) === "Set up your chart" && (await has("That date is in the future. Check the date on your device.")), await bodyText());
  check("the server's own message is never shown", !(await has("a message for developers")));
  await shot("p-onboarding-refused");
  await pick("time", "07:10");
  check("…and it goes as soon as she changes something", !(await has("That date is in the future.")));

  onboardingReply = { status: 500, body: apiError("internal") };
  await click("Start charting"); await idle();
  check("a server error: try again, and the button works again", (await has("Something went wrong. Try again.")) && (await submitDisabled()) === false, await bodyText());

  onboardingReply = "offline";
  await click("Start charting"); await idle();
  check("no connection: said so", await has("Can't reach the server. Check your connection and try again."), await bodyText());

  onboardingReply = { status: 401, body: apiError("unauthorized") };
  await click("Start charting"); await idle();
  check("a refused session: log out and in again", await has("Your session wasn't accepted. Log out and log in again."), await bodyText());

  // Already onboarded, from another device or a second tap: that is what
  // she wanted, so the page asks for her account again and goes on.
  onboardingReply = { status: 409, body: apiError("already_onboarded") };
  meReply = { status: 200, body: ONBOARDED };
  await click("Start charting");
  check("already onboarded: straight on to the signed-in page, with no error", await waitFor(`document.querySelector("h1")?.textContent === "You're logged in"`, "the signed-in page") && !(await has("Something went wrong")), await bodyText());

  // ---- when her account can't be read at all ----
  meReply = { status: 500, body: apiError("internal") };
  await goto("/");
  check("the account can't be read: said so, with a way to try again and a way out",
    (await h1()) === "You're logged in" && (await has("Something went wrong. Try again.")) && (await js(`["Try again", "Log out"].every((t) => [...document.querySelectorAll("button")].some((b) => b.textContent.trim() === t))`)), await bodyText());
  check("…and never \"Your account is ready\"", !(await has("Your account is ready.")));
  await shot("p-account-unread");
  meReply = { status: 200, body: NOT_ONBOARDED };
  await click("Try again");
  check("Try again reads it again, and goes where it should", await waitFor(`document.querySelector("h1")?.textContent === "Set up your chart"`, "onboarding after Try again"), await h1());
  meReply = "offline";
  await goto("/");
  check("no connection while reading the account: said so", await has("Can't reach the server. Check your connection and try again."), await bodyText());

  // ---- half past midnight in Israel ----
  // 22:30 UTC on 13 November is 00:30 on the 14th there. The device's date
  // is the 14th; the UTC date is still the 13th. The page must use hers.
  await S("Emulation.setTimezoneOverride", { timezoneId: "Asia/Jerusalem" });
  await S("Page.addScriptToEvaluateOnNewDocument", { source: `(() => { const Real = Date; const shift = Real.UTC(2026, 10, 13, 22, 30) - Real.now();
    globalThis.Date = class extends Real { constructor(...a) { if (a.length === 0) super(Real.now() + shift); else super(...a); } static now() { return Real.now() + shift; } }; })()` });
  meReply = { status: 200, body: NOT_ONBOARDED };
  onboardingReply = { status: 200, body: ONBOARDED };
  await goto("/");
  check("the staged clock: 00:30 on the 14th on the device, the 13th in UTC", (await deviceDay(0)) === "2026-11-14" && (await js(`new Date().toISOString().slice(0, 10)`)) === "2026-11-13", `${await deviceDay(0)} / ${await js(`new Date().toISOString()`)}`);
  await choose("I know the date");
  await pick("date", "2026-11-15");
  check("at 00:30 the limit is her today, the 14th", await has("Pick a date up to 14/11/2026."), await bodyText());
  await pick("date", "2026-11-14");
  check("…and the 14th itself is accepted", (await boxText("date")).includes("14/11/2026") && !(await has("Pick a date")), await boxText("date"));
  await pick("time", "07:00");
  await click("Start charting");
  await waitFor(`document.querySelector("h1")?.textContent === "You're logged in"`, "the signed-in page");
  check("…and sent as the 14th", same(lastBody(), { cycle_start_unknown: false, cycle_start_date: "2026-11-14", measurement_time: "07:00" }), lastSent().body);

  // ---- the way out ----
  meReply = { status: 200, body: NOT_ONBOARDED };
  await goto("/");
  await click("Log out");
  check("Log out leaves onboarding for Log in", await waitFor(`document.querySelector("h1")?.textContent === "Log in"`, "Log in after log out"), await h1());
  check("…and forgets the session", await js(`localStorage.getItem("madam.auth")`) === null);
} catch (error) {
  failed++;
  console.log("STOPPED: " + error.message);
}

const csp = consoleLines.filter((l) => /content security policy|refused to/i.test(l));
check("no Content-Security-Policy violation on any screen", csp.length === 0, csp.join(" | "));
const errors = consoleLines.filter((l) => /^\[(error|console\.error)\]/.test(l) && !/content security policy/i.test(l));
console.log(`console: ${consoleLines.length} lines, ${errors.length} errors`);
for (const line of [...new Set(errors)].slice(0, 12)) console.log("   " + line.slice(0, 220));
console.log(failed === 0 ? "ALL CHECKS PASSED" : `${failed} CHECK(S) FAILED`);
ws.close(); chrome.kill();
process.exit(failed === 0 ? 0 : 1);
