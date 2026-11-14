// Browser check 4: onboarding once, for real. A real account logs in through
// the page, answers the two questions, and the real server stores them. It
// is the one check where the page, a real token, our API and the database
// all meet; onboarding.mjs covers every other state of the screen with
// staged answers.
//
// It needs a test account on madam-dev, given in two environment variables
// (README.md says how): MADAM_CHECK_EMAIL and MADAM_CHECK_PASSWORD. The
// password is typed into the page and never printed.
//
// Onboarding happens once per account, so the account's rows must be gone
// before each run. `make browser-check` deletes them from the LOCAL
// database first, which is why this runs against `make docker-run` with the
// default DB=local: the identity is madam-dev's, the rows are local.
//
//   node onboarding-real.mjs <folder for screenshots> [BASE]
import { spawn } from "node:child_process";
import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

const CHROME = "C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe";
const BASE = process.argv[3] ?? "http://localhost:8080";
const OUT = process.argv[2];
const PORT = 9333;
const EMAIL = process.env.MADAM_CHECK_EMAIL;
const PASSWORD = process.env.MADAM_CHECK_PASSWORD;
if (!EMAIL || !PASSWORD) {
  console.log("STOPPED: MADAM_CHECK_EMAIL and MADAM_CHECK_PASSWORD are not set (README.md)");
  console.log("1 CHECK(S) FAILED");
  process.exit(1);
}
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
async function waitFor(expression, what, ms = 15000) {
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
async function type(label, text) {
  await js(`(() => { const l = [...document.querySelectorAll("label")].find((l) => l.textContent.trim() === ${q(label)});
    if (!l) throw new Error("no field labelled " + ${q(label)}); const i = document.getElementById(l.htmlFor); i.focus(); i.select(); })()`);
  await S("Input.insertText", { text });
}
async function click(text) {
  await js(`(() => { const b = [...document.querySelectorAll("button")].find((b) => b.textContent.trim() === ${q(text)});
    if (!b) throw new Error("no button " + ${q(text)}); b.click(); })()`);
  await sleep(150);
}
async function choose(text) {
  await js(`(() => { const l = [...document.querySelectorAll("fieldset label")].find((l) => l.textContent.trim() === ${q(text)});
    if (!l) throw new Error("no answer " + ${q(text)}); l.click(); })()`);
  await sleep(100);
}
async function pick(type, value) {
  await js(`(() => { const i = document.querySelector('input[type="${type}"]');
    if (!i) throw new Error("no ${type} input on the screen");
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value").set.call(i, ${q(value)});
    i.dispatchEvent(new Event("input", { bubbles: true })); i.dispatchEvent(new Event("change", { bubbles: true })); })()`);
  await sleep(100);
}
const bodyText = () => js(`document.body.innerText`);
const h1 = () => js(`document.querySelector("h1")?.textContent ?? ""`);
const has = async (s) => (await bodyText()).includes(s);
async function shot(name) {
  const { data } = await S("Page.captureScreenshot", { format: "png", captureBeyondViewport: true });
  writeFileSync(join(OUT, name + ".png"), Buffer.from(data, "base64"));
}
let failed = 0;
function check(name, ok, detail = "") {
  if (!ok) failed++;
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${ok || !detail ? "" : "\n        got: " + detail}`);
}
// A call to our own API from inside the page, with the token the page
// holds, as the page's own calls are made. Returns the status and the body.
const api = (method, path, body) => js(`(async () => {
  const token = JSON.parse(localStorage.getItem("madam.auth")).access_token;
  const res = await fetch(${q(path)}, { method: ${q(method)}, cache: "no-store",
    headers: { Authorization: "Bearer " + token${body === undefined ? "" : `, "Content-Type": "application/json"`} }${body === undefined ? "" : `, body: ${q(JSON.stringify(body))}`} });
  return { status: res.status, body: await res.json().catch(() => null) };
})()`);
const deviceDay = (offset) => js(`(() => { const d = new Date(); d.setDate(d.getDate() + ${offset});
  return d.getFullYear() + "-" + String(d.getMonth() + 1).padStart(2, "0") + "-" + String(d.getDate()).padStart(2, "0"); })()`);

try {
  // ---- log in as the test account, through the page ----
  await goto("/");
  check("signed out: Log in", (await h1()) === "Log in", await h1());
  await type("Email", EMAIL);
  await type("Password", PASSWORD);
  await click("Log in");
  await waitFor(`["Set up your chart", "You're logged in"].includes(document.querySelector("h1")?.textContent)`, "a signed-in screen");
  const landed = await h1();
  if (landed === "Log in") {
    throw new Error("the test account could not log in: check MADAM_CHECK_EMAIL and MADAM_CHECK_PASSWORD, and that the server uses madam-dev");
  }
  if (landed === "You're logged in") {
    throw new Error("the test account is already onboarded or its account couldn't be read. Run this through `make browser-check`, which deletes its local rows first, against `make docker-run` with DB=local");
  }
  check("a new account's first screen is onboarding", landed === "Set up your chart", landed);
  const me = await api("GET", "/api/me");
  check("the real server knows her, not onboarded", me.status === 200 && me.body?.onboarded_at === null, q(me));

  // ---- answer, and let the real server store it ----
  const today = await deviceDay(0);
  await choose("I know the date");
  await pick("date", today);
  await pick("time", "07:05");
  await click("Start charting");
  check("the real server accepted it: the signed-in page", await waitFor(`document.querySelector("h1")?.textContent === "You're logged in"`, "the signed-in page") && (await has("Your account is ready.")), await bodyText());
  await shot("r-onboarding-done");

  const after = await api("GET", "/api/me");
  check("GET /api/me now says onboarded", after.status === 200 && typeof after.body?.onboarded_at === "string", q(after));

  // ---- the same real token, straight at the endpoint ----
  const again = await api("POST", "/api/onboarding", { cycle_start_unknown: true, measurement_time: "06:00" });
  check("a second submit: 409 already_onboarded", again.status === 409 && again.body?.error?.code === "already_onboarded", q(again));
  const future = await api("POST", "/api/onboarding", { cycle_start_unknown: false, cycle_start_date: await deviceDay(3), measurement_time: "07:00" });
  check("a date three days ahead: 422 date_in_future, about cycle_start_date", future.status === 422 && future.body?.error?.code === "date_in_future" && future.body?.error?.field === "cycle_start_date", q(future));
  const extra = await api("POST", "/api/onboarding", { cycle_start_unknown: true, measurement_time: "07:00", user_id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb" });
  check("a field the endpoint doesn't read: 400 bad_request", extra.status === 400 && extra.body?.error?.code === "bad_request", q(extra));
  const still = await api("GET", "/api/me");
  check("none of that changed her account", still.status === 200 && still.body?.onboarded_at === after.body?.onboarded_at, q(still));

  // ---- and it lasts ----
  await goto("/");
  check("after a reload: still past onboarding", (await h1()) === "You're logged in" && (await has("Your account is ready.")), await bodyText());
  await click("Log out");
  check("Log out returns to Log in", await waitFor(`document.querySelector("h1")?.textContent === "Log in"`, "Log in after log out"), await h1());
} catch (error) {
  failed++;
  console.log("STOPPED: " + error.message);
}

const csp = consoleLines.filter((l) => /content security policy|refused to/i.test(l));
check("no Content-Security-Policy violation on any screen", csp.length === 0, csp.join(" | "));
console.log(failed === 0 ? "ALL CHECKS PASSED" : `${failed} CHECK(S) FAILED`);
ws.close(); chrome.kill();
process.exit(failed === 0 ? 0 : 1);
