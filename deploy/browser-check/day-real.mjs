// Browser check 6: the day screen once, for real. A test account logs in
// through the page, saves two days through the screen, and the real server
// asks its questions and stores the answers. It is the one check where the
// day screen, a real token, our API and the database all meet; day.mjs
// covers every other state with staged answers.
//
// It needs the same test account as onboarding-real.mjs (README.md):
// MADAM_CHECK_EMAIL and MADAM_CHECK_PASSWORD. The password is typed into
// the page and never printed. If the account hasn't onboarded, this script
// onboards it through the API, so it runs alone as well as after
// onboarding-real.mjs. It deletes the entries of the two days it uses
// (today and yesterday) before it starts, and touches nothing else of the
// account's.
//
// Against `make docker-run` with the default DB=local only: the identity
// is madam-dev's, the rows are local. Never against production.
//
//   node day-real.mjs <folder for screenshots> [BASE]
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
    if (!b) throw new Error("no button " + ${q(text)}); b.focus(); b.click(); })()`);
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

// ---- the day screen's own pieces ----
const path = () => js(`location.pathname`);
const hasButton = (text) => js(`[...document.querySelectorAll("button")].some((b) => b.textContent.trim() === ${q(text)})`);
const temperatureInput = `document.querySelector('input[inputmode="decimal"]')`;
const fieldValue = (label) => js(`(() => { const l = [...document.querySelectorAll("label")].find((l) => l.textContent.trim() === ${q(label)}); return l ? document.getElementById(l.htmlFor).value : null; })()`);
const boxText = (type) => js(`document.querySelector('input[type="${type}"]')?.closest("label").innerText ?? "(no ${type} box)"`);
const liveLine = () => js(`document.getElementById(${temperatureInput}?.getAttribute("aria-describedby")?.split(" ")[0] ?? "")?.textContent ?? "(no line)"`);
const toast = () => js(`document.querySelector('[role="status"]')?.textContent ?? null`);
const ddmmyyyy = (iso) => `${iso.slice(8, 10)}/${iso.slice(5, 7)}/${iso.slice(0, 4)}`;
// "07:05" plus 45 minutes, within the day.
const later = (hhmm, minutes) => {
  const total = (Number(hhmm.slice(0, 2)) * 60 + Number(hhmm.slice(3, 5)) + minutes) % (24 * 60);
  return String(Math.floor(total / 60)).padStart(2, "0") + ":" + String(total % 60).padStart(2, "0");
};

let today, yesterday;
try {
  // ---- log in as the test account, through the page ----
  await goto("/");
  check("signed out: Log in", (await h1()) === "Log in", await h1());
  await type("Email", EMAIL);
  await type("Password", PASSWORD);
  await click("Log in");
  await waitFor(`["Set up your chart", "You're logged in"].includes(document.querySelector("h1")?.textContent)`, "a signed-in screen");
  if ((await h1()) === "Log in") {
    throw new Error("the test account could not log in: check MADAM_CHECK_EMAIL and MADAM_CHECK_PASSWORD, and that the server uses madam-dev");
  }
  if ((await h1()) === "Set up your chart") {
    const onboarded = await api("POST", "/api/onboarding", { cycle_start_unknown: true, measurement_time: "07:05" });
    check("not onboarded yet: onboarded through the API", onboarded.status === 200, q(onboarded));
    await goto("/");
  }
  check("signed in, past onboarding", (await h1()) === "You're logged in", await h1());

  today = await deviceDay(0);
  yesterday = await deviceDay(-1);
  for (const date of [today, yesterday]) {
    const cleared = await api("DELETE", `/api/days/${date}/entry`);
    check(`${date} starts with no entry (DELETE: 204)`, cleared.status === 204, q(cleared));
  }
  const empty = await api("GET", `/api/days/${today}`);
  check("the real server: today has no entry, and a usual time", empty.status === 200 && empty.body?.entry === null && /^\d\d:\d\d$/.test(empty.body?.fixed_time ?? ""), q(empty));
  const fixed = empty.body.fixed_time;

  // ---- today's entry, through the screen ----
  await click("Today's entry");
  await waitFor(settled, "today's entry");
  check("Today's entry opens today's address", (await path()) === `/day/${today}` && (await h1()) === "Today's entry", `${await path()} ${await h1()}`);
  check("…says it has no entry, and starts at her usual time", (await has("No entry yet.")) && (await boxText("time")).includes(fixed), `${fixed} / ${await boxText("time")}`);

  // AC-14, against the real server: 34,2 is asked about.
  await type("Temperature", "34,2");
  await click("Save entry");
  check("AC-14: the real server asks about 34.20 °C", await waitFor(`document.body.innerText.includes("34.20 °C is unusual. Is it correct?")`, "the question"), await bodyText());
  const asked = await api("GET", `/api/days/${today}`);
  check("…and has stored nothing yet", asked.body?.entry === null, q(asked.body));
  await click("Yes, save it");
  check("\"Yes, save it\": saved, back home, saying so", await waitFor(`location.pathname === "/" && document.querySelector('[role="status"]')?.textContent === "Entry saved"`, "home and the toast"), `${await path()} ${await toast()}`);
  await shot("r-day-saved");
  const saved = await api("GET", `/api/days/${today}`);
  check("the real server holds 34.20 °C at her usual time", saved.body?.entry?.temperature?.hundredths === 3420 && saved.body?.entry?.temperature?.unit === "celsius" && saved.body?.entry?.measured_at === fixed && saved.body?.entry?.bleeding === "none", q(saved.body));

  // ---- AC-13 and AC-4: yesterday, reached with the date box ----
  await click("Today's entry");
  await waitFor(settled, "today's entry again");
  check("today opens with what was saved", (await fieldValue("Temperature")) === "34.20" && !(await has("No entry yet.")), String(await fieldValue("Temperature")));
  await pick("date", yesterday);
  await waitFor(`location.pathname === "/day/${yesterday}" && ${settled}`, "yesterday");
  check("the date box opens yesterday, which has no entry", (await path()) === `/day/${yesterday}` && (await has("No entry yet.")) && (await boxText("date")).includes(ddmmyyyy(yesterday)), `${await path()} ${await bodyText()}`);
  await choose("Spotting");
  await type("Notes", "From the real pass.");
  await click("Save entry");
  check("AC-13: the real server asks before saving without a temperature", await waitFor(`document.body.innerText.includes("Save without a temperature?")`, "the question"), await bodyText());
  await click("Save without");
  check("\"Save without\": saved", await waitFor(`location.pathname === "/"`, "home"), await path());
  const noTemperature = await api("GET", `/api/days/${yesterday}`);
  check("the real server holds yesterday with no temperature and no time", noTemperature.body?.entry?.temperature === null && noTemperature.body?.entry?.measured_at === null && noTemperature.body?.entry?.bleeding === "spotting" && noTemperature.body?.entry?.notes === "From the real pass.", q(noTemperature.body));

  // AC-4: a date that has an entry opens it.
  await click("Today's entry");
  await waitFor(settled, "today's entry");
  await pick("date", yesterday);
  await waitFor(`location.pathname === "/day/${yesterday}" && ${settled}`, "yesterday again");
  check("AC-4: picking a date that has an entry opens that entry", (await fieldValue("Notes")) === "From the real pass." && !(await has("No entry yet.")), String(await fieldValue("Notes")));

  // The line under the temperature, with her real usual time.
  await type("Temperature", "36.62");
  await pick("time", later(fixed, 45));
  check("the line says what 36.62, 45 minutes late, counts as", (await liveLine()) === "Measured 45 min late, so it counts as 36.55 °C. That's an estimate.", await liveLine());
  await shot("r-day-typed");
  await click("Save entry");
  check("a usual temperature is saved without a question", await waitFor(`location.pathname === "/"`, "home"), await bodyText());
  const changed = await api("GET", `/api/days/${yesterday}`);
  check("…replacing yesterday's entry, keeping what wasn't changed", changed.body?.entry?.temperature?.hundredths === 3662 && changed.body?.entry?.measured_at === later(fixed, 45) && changed.body?.entry?.notes === "From the real pass.", q(changed.body));

  // ---- the same real token, straight at the endpoints ----
  const impossible = await api("PUT", `/api/days/${today}`, { entry: { bleeding: "none", disturbances: [], symptoms: [], temperature: { hundredths: 5001, unit: "celsius" }, measured_at: fixed }, confirmed: ["unusual_temperature"] });
  check("AC-17: 50.01 °C is 422 temperature_out_of_range, even confirmed", impossible.status === 422 && impossible.body?.error?.code === "temperature_out_of_range", q(impossible));
  const future = await api("GET", `/api/days/${await deviceDay(3)}`);
  check("a day three days ahead: 422 date_in_future", future.status === 422 && future.body?.error?.code === "date_in_future", q(future));
  const smuggled = await api("PUT", `/api/days/${today}`, { entry: { bleeding: "none", disturbances: [], symptoms: [] }, confirmed: ["no_temperature"], user_id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb" });
  check("a field the endpoint doesn't read: 400 bad_request", smuggled.status === 400 && smuggled.body?.error?.code === "bad_request", q(smuggled));
  const unchanged = await api("GET", `/api/days/${today}`);
  check("none of that changed today's entry", unchanged.body?.entry?.temperature?.hundredths === 3420, q(unchanged.body));

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
