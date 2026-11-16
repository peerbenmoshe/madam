// Browser check 5: the day screen, in every state, as a person on a phone
// would use it. See README.md in this folder, and auth-screens.mjs for how
// the browser is driven.
//
// Nothing here reaches our API's data. The script stages:
//   - a signed-in, onboarded person, by putting a made-up session in the
//     page's storage and answering GET /api/me itself;
//   - the server's answers to GET and PUT /api/days/{date} and DELETE
//     /api/days/{date}/entry: an empty day, an entry, each question (409),
//     each refusal, a lost connection;
//   - the device's clock and time zone, for a screen left open past
//     midnight.
// It reads the exact JSON the page sends.
//
// So it proves what the page does, not that the page and the server agree.
// That is day-real.mjs.
//
//   node day.mjs <folder for screenshots> [BASE]
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
const settle = (what) => waitFor(settled, what);
async function goto(path) {
  await S("Page.navigate", { url: BASE + path });
  await sleep(150);
  await settle(`page ${path}`);
}
const q = JSON.stringify;
// A tap on a button moves the focus to it, and a script's click() doesn't:
// so this focuses it first, or a check that the page moved the focus
// somewhere would pass because the focus never left.
async function click(text) {
  await js(`(() => { const b = [...document.querySelectorAll("button")].find((b) => b.textContent.trim() === ${q(text)});
    if (!b) throw new Error("no button " + ${q(text)}); b.focus(); b.click(); })()`);
  await sleep(150);
}
const hasButton = (text) => js(`[...document.querySelectorAll("button")].some((b) => b.textContent.trim() === ${q(text)})`);
// Taps an answer: an option button or a chip is a label over an input.
async function choose(text) {
  await js(`(() => { const l = [...document.querySelectorAll("fieldset label")].find((l) => l.textContent.trim() === ${q(text)});
    if (!l) throw new Error("no answer " + ${q(text)}); l.click(); })()`);
  await sleep(100);
}
const chosen = () => js(`[...document.querySelectorAll("fieldset input:checked")].map((i) => i.closest("label").textContent.trim())`);
// Types into the field with that label, replacing what it held.
async function type(label, text) {
  await js(`(() => { const l = [...document.querySelectorAll("label")].find((l) => l.textContent.trim() === ${q(label)});
    if (!l) throw new Error("no field labelled " + ${q(label)}); const i = document.getElementById(l.htmlFor); i.focus(); i.select(); })()`);
  if (text === "") {
    await S("Input.dispatchKeyEvent", { type: "keyDown", key: "Backspace", code: "Backspace", windowsVirtualKeyCode: 8 });
    await S("Input.dispatchKeyEvent", { type: "keyUp", key: "Backspace", code: "Backspace", windowsVirtualKeyCode: 8 });
  } else {
    await S("Input.insertText", { text });
  }
  await sleep(100);
}
const fieldValue = (label) => js(`(() => { const l = [...document.querySelectorAll("label")].find((l) => l.textContent.trim() === ${q(label)}); return l ? document.getElementById(l.htmlFor).value : null; })()`);
// A pick in a native picker, which can't be opened headless: the input's
// value is set the way the browser sets it, and the page is told.
async function pick(type, value) {
  await js(`(() => { const i = document.querySelector('input[type="${type}"]');
    if (!i) throw new Error("no ${type} input on the screen");
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value").set.call(i, ${q(value)});
    i.dispatchEvent(new Event("input", { bubbles: true })); i.dispatchEvent(new Event("change", { bubbles: true })); })()`);
  await sleep(150);
}
const boxText = (type) => js(`document.querySelector('input[type="${type}"]')?.closest("label").innerText ?? "(no ${type} box)"`);
const temperatureInput = `document.querySelector('input[inputmode="decimal"]')`;
const bodyText = () => js(`document.body.innerText`);
const path = () => js(`location.pathname`);
const h1 = () => js(`document.querySelector("h1")?.textContent ?? ""`);
const has = async (s) => (await bodyText()).includes(s);
const liveLine = () => js(`document.getElementById(${temperatureInput}?.getAttribute("aria-describedby")?.split(" ")[0] ?? "")?.textContent ?? "(no line)"`);
async function shot(name) {
  const { data } = await S("Page.captureScreenshot", { format: "png", captureBeyondViewport: true });
  writeFileSync(join(OUT, name + ".png"), Buffer.from(data, "base64"));
}
// Only what a phone's screen shows at this moment, not the whole page.
async function screen(name) {
  const { data } = await S("Page.captureScreenshot", { format: "png" });
  writeFileSync(join(OUT, name + ".png"), Buffer.from(data, "base64"));
}
let failed = 0;
function check(name, ok, detail = "") {
  if (!ok) failed++;
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${ok || !detail ? "" : "\n        got: " + detail}`);
}

// ---- the staged server ----
const ONBOARDED = { id: "00000000-0000-4000-8000-000000000000", temperature_unit: "celsius", onboarded_at: "2026-11-01T07:00:00Z" };
const NOT_ONBOARDED = { ...ONBOARDED, onboarded_at: null };
const apiError = (code, field) => ({ error: { code, ...(field ? { field } : {}), message: "a message for developers, never shown" } });
const NO_ENTRY = null;
let meReply = { status: 200, body: ONBOARDED };
// GET /api/days/{date}: the entry each date holds (none if not listed), or
// a whole reply in place of the day.
const entries = {};
let getReply = null; // { status, body } or "offline": overrides the day
// PUT /api/days/{date}: answered from this queue, then by saving the entry.
const putReplies = [];
// DELETE /api/days/{date}/entry: from this queue, then by deleting it (204).
const deleteReplies = [];
const sentDays = []; // every request about a day: { method, date, path, body, headers }

const FIXED = "07:00";
const dayOf = (date) => ({ date, fixed_time: FIXED, entry: entries[date] ?? NO_ENTRY });

ws.addEventListener("message", (m) => {
  const msg = JSON.parse(m.data);
  if (msg.method !== "Fetch.requestPaused") return;
  const { requestId, request } = msg.params;
  const url = new URL(request.url);
  let reply;
  if (url.pathname === "/api/me") {
    reply = meReply;
  } else {
    const date = url.pathname.split("/")[3];
    const body = request.postData ? JSON.parse(request.postData) : undefined;
    sentDays.push({ method: request.method, date, path: url.pathname, body, headers: request.headers });
    if (request.method === "GET") {
      reply = getReply ?? { status: 200, body: dayOf(date) };
    } else if (request.method === "DELETE") {
      reply = deleteReplies.length > 0 ? deleteReplies.shift() : { status: 204 };
      if (reply !== "offline" && reply.status === 204) delete entries[date];
    } else if (putReplies.length > 0) {
      reply = putReplies.shift();
    } else {
      entries[date] = body.entry;
      reply = { status: 200, body: dayOf(date) };
    }
  }
  if (reply === "offline") return void S("Fetch.failRequest", { requestId, errorReason: "InternetDisconnected" });
  // A 204 has no body and no Content-Type, as the server sends it.
  const noBody = reply.status === 204;
  setTimeout(() => void S("Fetch.fulfillRequest", { requestId, responseCode: reply.status,
    responseHeaders: [...(noBody ? [] : [{ name: "Content-Type", value: "application/json" }]), { name: "Cache-Control", value: "no-store" }],
    body: noBody ? "" : Buffer.from(JSON.stringify(reply.body)).toString("base64") }), reply.delay ?? 0);
});
await S("Fetch.enable", { patterns: [{ urlPattern: "*/api/me" }, { urlPattern: "*/api/days/*" }] });

const fakeSession = { access_token: "eyJhbGciOiJFUzI1NiIsImtpZCI6IngifQ.eyJzdWIiOiJ4In0.AAAA", refresh_token: "made-up", token_type: "bearer",
  expires_in: 3600, expires_at: Math.floor(Date.now() / 1000) + 3600,
  user: { id: "00000000-0000-4000-8000-000000000000", aud: "authenticated", email: "someone@example.com", app_metadata: {}, user_metadata: {}, created_at: "2026-11-12T00:00:00Z" } };
// Today as the page's own device sees it, and the days either side.
const deviceDay = (offset) => js(`(() => { const d = new Date(); d.setDate(d.getDate() + ${offset});
  return d.getFullYear() + "-" + String(d.getMonth() + 1).padStart(2, "0") + "-" + String(d.getDate()).padStart(2, "0"); })()`);
const ddmmyyyy = (iso) => `${iso.slice(8, 10)}/${iso.slice(5, 7)}/${iso.slice(0, 4)}`;
const puts = () => sentDays.filter((r) => r.method === "PUT");
const lastPut = () => puts()[puts().length - 1];
const same = (a, b) => JSON.stringify(a) === JSON.stringify(b);
// The entry the page sends for a blank day with only these fields changed.
const blankEntry = (changes = {}) => ({
  temperature: null, measured_at: null, bleeding: "none", secretion_category: null, sensation: null,
  secretion_amount: null, secretion_description: null, cervix_firmness: null, cervix_position: null,
  disturbances: [], symptoms: [], intercourse: null, notes: null, ...changes,
});
const toast = () => js(`document.querySelector('[role="status"]')?.textContent ?? null`);
const focusIsTemperature = () => js(`document.activeElement === ${temperatureInput}`);
// "Show all fields": open or closed, and what its summary says.
const details = `document.querySelector("details")`;
const moreState = () => js(`(() => { const d = ${details}; return d ? (d.open ? "open: " : "closed: ") + d.querySelector("summary").textContent : "(no details)"; })()`);
async function toggleMore() {
  await js(`${details}.querySelector("summary").click()`);
  await sleep(250);
}
// Whether an answer can be seen: hidden in a closed section, it can't.
const answerShown = (text) => js(`[...document.querySelectorAll("fieldset label")].find((l) => l.textContent.trim() === ${q(text)})?.checkVisibility() ?? false`);
// Taps an answer of the question in the Save bar. "Delete entry" is both
// the link that asks and the answer that deletes, so this looks only in
// the question.
async function answer(text) {
  await js(`(() => { const b = [...document.querySelectorAll('[role="group"] button')].find((b) => b.textContent.trim() === ${q(text)});
    if (!b) throw new Error("no answer " + ${q(text)}); b.focus(); b.click(); })()`);
  await sleep(150);
}
// The message a field is described by: its error, if it has one.
const describedBy = (label) => js(`(() => { const l = [...document.querySelectorAll("label")].find((l) => l.textContent.trim() === ${q(label)});
  return (document.getElementById(l.htmlFor).getAttribute("aria-describedby") ?? "").split(" ").map((id) => document.getElementById(id)?.textContent ?? "").join(" "); })()`);
const deletes = () => sentDays.filter((r) => r.method === "DELETE");
// What has the focus, and whether it can be seen: nothing covers its middle.
const focused = () => js(`(() => { const e = document.activeElement; const r = e.getBoundingClientRect();
  const top = document.elementFromPoint(r.left + r.width / 2, r.top + r.height / 2);
  return e.textContent.trim() + (top === e || e.contains(top) ? "" : " (covered)"); })()`);
const buttonsOff = (...texts) => js(`${q(texts)}.every((t) => [...document.querySelectorAll("button")].find((b) => b.textContent.trim() === t)?.disabled === true)`);

try {
  await goto("/design");
  await js(`localStorage.setItem("madam.auth", ${q(JSON.stringify(fakeSession))})`);
  const today = await deviceDay(0), yesterday = await deviceDay(-1), tomorrow = await deviceDay(1);

  // ---- from the placeholder page to today's entry ----
  await goto("/");
  check("the placeholder page offers today's entry", (await h1()) === "You're logged in" && (await hasButton("Today's entry")), await bodyText());
  await shot("p-home-today");
  await click("Today's entry");
  await settle("today's entry");
  check("…which opens today's own address", (await path()) === `/day/${today}`, await path());
  check("…titled Today's entry", (await h1()) === "Today's entry", await h1());
  check("…and the page asked the server for that day", sentDays.at(-1)?.method === "GET" && sentDays.at(-1)?.date === today, q(sentDays.at(-1)));

  // ---- an empty day ----
  check("a day with no entry says so", await has("No entry yet."));
  check("the date box shows the day as dd/mm/yyyy", (await boxText("date")).includes(ddmmyyyy(today)), await boxText("date"));
  check("Measured at starts at her usual time", (await boxText("time")).includes(FIXED), await boxText("time"));
  check("…and the line under it says what that time is", (await liveLine()) === "Your usual time is 07:00.", await liveLine());
  check("the temperature box is empty, with a greyed example", (await fieldValue("Temperature")) === "" && (await js(`${temperatureInput}.placeholder`)) === "36.45");
  check("bleeding starts at None, and nothing else is chosen", same(await chosen(), ["None"]), q(await chosen()));
  check("the cycle-start switch is there, off and disabled", await js(`(() => { const s = document.querySelector('input[role="switch"]'); return s && s.disabled && !s.checked; })()`));
  check("…with a line saying it can't be used yet", await has("Cycle starts can't be changed here yet."));
  check("there's no Delete on a day without an entry", !(await hasButton("Delete entry")));
  check("Show all fields starts closed on a new day", (await moreState()) === "closed: Show all fields", await moreState());
  check("nothing on the screen is wider than the phone", await js(`document.documentElement.scrollWidth <= innerWidth`), await js(`document.documentElement.scrollWidth + " in " + innerWidth`));
  // The Save bar stays at the bottom of the screen while the form scrolls.
  await js(`scrollTo(0, 0)`);
  const saveAtTop = await js(`(() => { const b = [...document.querySelectorAll("button")].find((b) => b.textContent.trim() === "Save entry");
    const r = b.getBoundingClientRect(); return [Math.round(r.bottom), innerHeight, document.elementFromPoint(r.left + r.width / 2, r.top + r.height / 2) === b]; })()`);
  check("Save is on screen, and tappable, with the form scrolled to the top", saveAtTop[0] <= saveAtTop[1] && saveAtTop[2] === true, saveAtTop.join(", "));
  await screen("p-day-empty");
  await shot("p-day-empty-whole");

  // ---- the line follows her typing (design-brief §6.3) ----
  await type("Temperature", "36.70");
  check("a temperature at her usual time: no adjustment", (await liveLine()) === "Measured at your usual time, so no adjustment.", await liveLine());
  await pick("time", "07:45");
  check("a different time changes the box", (await boxText("time")).includes("07:45"), await boxText("time"));
  await type("Temperature", "36,62");
  check("AC-1: 36,62 at 07:45 counts as 36.55, labelled an estimate", (await liveLine()) === "Measured 45 min late, so it counts as 36.55 °C. That's an estimate.", await liveLine());
  check("…the value in bold", (await js(`document.getElementById(${temperatureInput}.getAttribute("aria-describedby").split(" ")[0]).querySelector("b")?.textContent`)) === "36.55 °C");
  check("…and read out as it changes", (await js(`document.getElementById(${temperatureInput}.getAttribute("aria-describedby").split(" ")[0]).getAttribute("aria-live")`)) === "polite");
  await screen("p-day-typed");
  await type("Temperature", "36.5");
  await pick("time", "07:01");
  check("AC-3: a minute late is too little to change the value", (await liveLine()) === "Measured 1 min late. Too little to change the value.", await liveLine());
  await pick("time", "06:30");
  await type("Temperature", "36.40");
  check("AC-2: half an hour early adds", (await liveLine()) === "Measured 30 min early, so it counts as 36.45 °C. That's an estimate.", await liveLine());
  await type("Temperature", "36.");
  check("a number still being typed counts as none yet", (await liveLine()) === "Measured 30 min early. The value will be adjusted to 07:00.", await liveLine());

  // ---- what the page refuses by itself: nothing is sent ----
  const before = sentDays.length;
  await click("Save entry");
  check("a temperature that isn't a number: said under it", await has("Enter the temperature as a number, like 36.45."), await bodyText());
  check("…the box is marked invalid", (await js(`${temperatureInput}.getAttribute("aria-invalid")`)) === "true");
  check("…and nothing was sent", sentDays.length === before, `${sentDays.length - before} requests`);
  await type("Temperature", "12");
  check("once tried, the message follows the box: 12 isn't a body temperature", await has("That isn't a body temperature. Enter a value between 20.00 and 50.00 °C."), await bodyText());
  await click("Save entry");
  check("…and still nothing is sent", sentDays.length === before, `${sentDays.length - before} requests`);
  await shot("p-day-impossible");

  // ---- AC-14: an unusual temperature is asked about ----
  putReplies.push({ status: 409, body: apiError("unusual_temperature", "temperature") });
  await type("Temperature", "34,2");
  await pick("time", "07:00");
  await click("Save entry");
  await waitFor(`document.body.innerText.includes("is unusual")`, "the question");
  check("AC-14: the server asks, and the page says it with her number", await has("34.20 °C is unusual. Is it correct?"), await bodyText());
  check("…the question takes Save's place, with two specific answers", !(await hasButton("Save entry")) && (await hasButton("Yes, save it")) && (await hasButton("No, fix it")));
  check("…the first request asked nothing in advance", same(lastPut().body, { entry: blankEntry({ temperature: { hundredths: 3420, unit: "celsius" }, measured_at: "07:00" }), confirmed: [] }), q(lastPut().body));
  check("…a PUT of JSON to the day's own address, with her token", lastPut().date === today && /^application\/json/.test(lastPut().headers["Content-Type"] ?? "") && /^Bearer /.test(lastPut().headers["Authorization"] ?? ""), q(lastPut().headers));
  check("…and the focus is on the answer that changes nothing", (await focused()) === "No, fix it", await focused());
  await screen("p-day-unusual");
  const putsBeforeNo = puts().length;
  await click("No, fix it");
  check("\"No, fix it\": back in the temperature box, nothing saved", (await focusIsTemperature()) && (await hasButton("Save entry")) && puts().length === putsBeforeNo);
  check("…with what she typed selected, so a new number replaces it", (await js(`[${temperatureInput}.selectionStart, ${temperatureInput}.selectionEnd].join()`)) === "0,4", await js(`[${temperatureInput}.selectionStart, ${temperatureInput}.selectionEnd].join()`));
  putReplies.push({ status: 409, body: apiError("unusual_temperature", "temperature") });
  await click("Save entry");
  await waitFor(`document.body.innerText.includes("is unusual")`, "the question again");
  await click("Yes, save it");
  await waitFor(`location.pathname === "/"`, "home after saving");
  check("\"Yes, save it\": the same entry again, with the answer named", same(lastPut().body, { entry: blankEntry({ temperature: { hundredths: 3420, unit: "celsius" }, measured_at: "07:00" }), confirmed: ["unusual_temperature"] }), q(lastPut().body));
  check("saved: back on the home page, saying so", (await path()) === "/" && (await toast()) === "Entry saved", `${await path()} ${await toast()}`);
  await screen("p-day-saved");
  await js(`history.back()`);
  await sleep(400);
  check("the browser's Back doesn't return to the saved day", (await path()) === "/", await path());

  // ---- AC-13: no temperature is asked about ----
  await goto(`/day/${yesterday}`);
  check("another day is titled with its weekday and date", /^[A-Z][a-z]+day \d\d\/\d\d\/\d{4}$/.test(await h1()) && (await h1()).endsWith(ddmmyyyy(yesterday)), await h1());
  await choose("Spotting");
  putReplies.push({ status: 409, body: apiError("no_temperature", "temperature") });
  await click("Save entry");
  await waitFor(`document.body.innerText.includes("Save without a temperature?")`, "the question");
  check("AC-13: saving without a temperature asks first", (await has("Save without a temperature?")) && (await hasButton("Save without")) && (await hasButton("Add a temperature")), await bodyText());
  check("…and the entry sent has no time either", same(lastPut().body, { entry: blankEntry({ bleeding: "spotting" }), confirmed: [] }), q(lastPut().body));
  await screen("p-day-no-temperature");
  await click("Add a temperature");
  check("\"Add a temperature\": back in the box", (await focusIsTemperature()) && (await hasButton("Save entry")));
  // A change forgets an answer given before it.
  putReplies.push({ status: 409, body: apiError("no_temperature", "temperature") });
  await click("Save entry");
  await waitFor(`document.body.innerText.includes("Save without a temperature?")`, "the question again");
  await choose("Bleeding");
  check("changing anything puts Save back and forgets the question", (await hasButton("Save entry")) && !(await has("Save without a temperature?")));
  putReplies.push({ status: 409, body: apiError("no_temperature", "temperature") });
  await click("Save entry");
  await waitFor(`document.body.innerText.includes("Save without a temperature?")`, "the question a third time");
  check("…and the next Save asks again from nothing", same(lastPut().body.confirmed, []), q(lastPut().body));
  await click("Save without");
  await waitFor(`location.pathname === "/"`, "home after saving");
  check("\"Save without\": the same entry, with that answer", same(lastPut().body, { entry: blankEntry({ bleeding: "bleeding" }), confirmed: ["no_temperature"] }), q(lastPut().body));

  // ---- AC-4: a date with an entry opens it ----
  entries[yesterday] = {
    temperature: { hundredths: 3662, unit: "celsius" }, measured_at: "07:45", bleeding: "spotting",
    secretion_category: "milky", sensation: "wet", secretion_amount: 2, secretion_description: "clear, stretches",
    cervix_firmness: "soft", cervix_position: "high_open", disturbances: ["alcohol", "poor_sleep"],
    symptoms: ["bloating"], intercourse: "protected", notes: "Slept badly.\nFlight at 6.",
  };
  await goto(`/day/${today}`);
  await pick("date", yesterday);
  await settle("the picked day");
  check("AC-4: picking a date with an entry opens that entry", (await path()) === `/day/${yesterday}` && (await fieldValue("Temperature")) === "36.62", `${await path()} ${await fieldValue("Temperature")}`);
  check("…its time, not the usual one", (await boxText("time")).includes("07:45"), await boxText("time"));
  check("…its answers chosen, the extra ones too", same(await chosen(), ["Spotting", "Milky", "Alcohol", "Poor sleep", "Wet", "2", "Soft", "High, open", "Bloating", "Protected"]), q(await chosen()));
  check("…with Show all fields open, so nothing saved is hidden", (await moreState()) === "open: Hide extra fields" && (await answerShown("Wet")), await moreState());
  check("…its secretion description", (await fieldValue("Secretion description")) === "clear, stretches", String(await fieldValue("Secretion description")));
  check("…and Delete entry, below Show all fields", await js(`(() => { const b = [...document.querySelectorAll("button")].find((b) => b.textContent.trim() === "Delete entry");
    return !!b && !!(${details}.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING); })()`));
  check("…its notes, line breaks kept", (await fieldValue("Notes")) === "Slept badly.\nFlight at 6.", q(await fieldValue("Notes")));
  check("…no \"No entry yet.\"", !(await has("No entry yet.")));
  check("…and the line says what it counts as", (await liveLine()) === "Measured 45 min late, so it counts as 36.55 °C. That's an estimate.", await liveLine());
  await screen("p-day-existing");
  await shot("p-day-existing-whole");
  await choose("Milky");
  check("tapping the chosen secretion clears it", same(await chosen(), ["Spotting", "Alcohol", "Poor sleep", "Wet", "2", "Soft", "High, open", "Bloating", "Protected"]), q(await chosen()));
  await choose("Illness");
  // A copy: the staged server replaces its own when the page saves.
  const asItCame = { ...entries[yesterday] };
  await click("Save entry");
  await waitFor(`location.pathname === "/"`, "home after saving");
  check("saving sends back exactly what came, with only her two changes", same(lastPut().body, {
    entry: { ...asItCame, secretion_category: null, disturbances: ["alcohol", "poor_sleep", "illness"] }, confirmed: [],
  }), q(lastPut().body));
  check("…and the disturbances in fam-rules §7's order, wherever she tapped", same(lastPut().body.entry.disturbances, ["alcohol", "poor_sleep", "illness"]), q(lastPut().body.entry.disturbances));

  // ---- Show all fields (design-brief §6.3, part 8) ----
  const twoDaysAgo = await deviceDay(-2);
  await goto(`/day/${twoDaysAgo}`);
  check("on a new day the extra answers are out of sight", !(await answerShown("Dry")) && (await answerShown("Spotting")));
  await toggleMore();
  check("a tap opens Show all fields, and it says what the next tap does", (await moreState()) === "open: Hide extra fields", await moreState());
  check("…showing sensation, amount, description, cervix, symptoms and intercourse",
    (await answerShown("Dry")) && (await answerShown("3")) && (await fieldValue("Secretion description")) === "" && (await answerShown("Low, closed")) && (await answerShown("PMS")) && (await answerShown("Unprotected")));
  check("…nothing chosen in them yet", same(await chosen(), ["None"]), q(await chosen()));
  check("the description stops at 200 characters", (await js(`document.getElementById([...document.querySelectorAll("label")].find((l) => l.textContent.trim() === "Secretion description").htmlFor).maxLength`)) === 200);
  await choose("Dry");
  await choose("Dry");
  check("an extra answer clears like secretion: tap it again", same(await chosen(), ["None"]), q(await chosen()));
  await choose("Creamy");
  await choose("3");
  await choose("2");
  await type("Secretion description", "stretchy, a lot");
  await choose("Soft");
  await choose("Low, closed");
  await choose("PMS");
  await choose("Breast tenderness");
  await choose("Unprotected");
  const extras = ["None", "Creamy", "2", "Soft", "Low, closed", "Breast tenderness", "PMS", "Unprotected"];
  check("every extra answer can be chosen", same(await chosen(), extras), q(await chosen()));
  check("nothing on the open screen is wider than the phone", await js(`document.documentElement.scrollWidth <= innerWidth`), await js(`document.documentElement.scrollWidth + " in " + innerWidth`));
  await shot("p-day-more");
  await toggleMore();
  check("closing it keeps every answer", (await moreState()) === "closed: Show all fields" && same(await chosen(), extras) && (await fieldValue("Secretion description")) === "stretchy, a lot", `${await moreState()} ${q(await chosen())}`);
  // A refusal about a field inside a closed section must still be seen.
  putReplies.push({ status: 422, body: apiError("too_long", "secretion_description") });
  await click("Save entry");
  await waitFor(`document.body.innerText.includes("Keep the description under 200 characters.")`, "the refusal");
  check("a refusal about the description opens the section, and is said under it", (await moreState()) === "open: Hide extra fields" && (await describedBy("Secretion description")).includes("Keep the description under 200 characters."), `${await moreState()} / ${await describedBy("Secretion description")}`);
  await type("Secretion description", "stretchy, lots");
  check("…and it goes once she changes it", !(await has("Keep the description under")));
  putReplies.push({ status: 409, body: apiError("no_temperature", "temperature") });
  await click("Save entry");
  await waitFor(`document.body.innerText.includes("Save without a temperature?")`, "the question");
  await answer("Save without");
  await waitFor(`location.pathname === "/"`, "home after saving");
  check("saving sends every extra answer, the symptoms in §7's order", same(lastPut().body, { entry: blankEntry({
    sensation: "creamy", secretion_amount: 2, secretion_description: "stretchy, lots", cervix_firmness: "soft", cervix_position: "low_closed",
    symptoms: ["breast_tenderness", "pms"], intercourse: "unprotected" }), confirmed: ["no_temperature"] }), q(lastPut().body));

  // ---- Delete (design-brief §6.3, part 9) ----
  await goto(`/day/${twoDaysAgo}`);
  check("the saved extras come back, with the section open", (await moreState()) === "open: Hide extra fields" && same(await chosen(), extras), `${await moreState()} ${q(await chosen())}`);
  const deletesBefore = deletes().length;
  await click("Delete entry");
  check("Delete asks first, in Save's place, with the date as dd/mm/yyyy", (await has(`Delete the entry for ${ddmmyyyy(twoDaysAgo)}? This can't be undone.`)) && !(await hasButton("Save entry")) && (await hasButton("Keep it")), await bodyText());
  check("…and asking deletes nothing", deletes().length === deletesBefore, `${deletes().length - deletesBefore} DELETEs`);
  check("…the focus on \"Keep it\", in sight", (await focused()) === "Keep it", await focused());
  await screen("p-day-delete");
  await answer("Keep it");
  check("\"Keep it\": Save is back, nothing is deleted, and the focus is on Delete again", (await hasButton("Save entry")) && deletes().length === deletesBefore && (await focused()) === "Delete entry", await focused());
  await click("Delete entry");
  await choose("Spotting");
  check("changing anything puts Save back in place of the delete question", (await hasButton("Save entry")) && !(await has("This can't be undone.")) && deletes().length === deletesBefore);
  deleteReplies.push({ status: 500, body: apiError("internal") });
  await click("Delete entry");
  await answer("Delete entry");
  await waitFor(`document.body.innerText.includes("Something went wrong")`, "the error");
  check("a delete that fails: try again, in the Save bar, and the day stays", (await has("Something went wrong. Try again.")) && (await path()) === `/day/${twoDaysAgo}` && (await hasButton("Save entry")) && !(await has("a message for developers")), await bodyText());
  deleteReplies.push("offline");
  await click("Delete entry");
  await answer("Delete entry");
  await waitFor(`document.body.innerText.includes("Can't reach the server")`, "the offline message");
  check("a delete with no connection: said so", await has("Can't reach the server. Check your connection and try again."), await bodyText());
  deleteReplies.push({ status: 204, delay: 900 });
  await click("Delete entry");
  await answer("Delete entry");
  check("while a delete is on its way, Save and Delete are off", await buttonsOff("Save entry", "Delete entry"));
  await waitFor(`location.pathname === "/"`, "home after deleting");
  const deleted = deletes().at(-1);
  check("deleted: a DELETE to the entry's own address, with her token and no body", deleted.path === `/api/days/${twoDaysAgo}/entry` && /^Bearer /.test(deleted.headers["Authorization"] ?? "") && deleted.body === undefined, q({ ...deleted, headers: undefined }));
  check("…back on the home page, saying so", (await path()) === "/" && (await toast()) === "Entry deleted", `${await path()} ${await toast()}`);
  await js(`history.back()`);
  await sleep(400);
  check("the browser's Back doesn't return to the deleted day", (await path()) === "/", await path());
  await goto(`/day/${twoDaysAgo}`);
  check("the day has no entry now, and no Delete", (await has("No entry yet.")) && !(await hasButton("Delete entry")) && (await moreState()) === "closed: Show all fields", `${await moreState()} ${await bodyText()}`);

  // ---- moving between days with the date box ----
  await goto(`/day/${today}`);
  await type("Temperature", "36.9");
  await pick("date", tomorrow);
  check("tomorrow can't be opened: the box says why", (await has(`Pick a date up to ${ddmmyyyy(today)}.`)) && (await path()) === `/day/${today}`, `${await path()} ${await bodyText()}`);
  check("…and keeps the day it had", (await boxText("date")).includes(ddmmyyyy(today)), await boxText("date"));
  // On a computer a year is typed digit by digit, through 0002: the box
  // must take it, open nothing, and say the limit.
  await pick("date", "0002-11-14");
  check("a year typed halfway opens nothing, and the box keeps it", (await path()) === `/day/${today}` && (await boxText("date")).includes("14/11/0002"), `${await path()} ${await boxText("date")}`);
  check("…saying the limits", await has(`Pick a date from 01/01/2000 to ${ddmmyyyy(today)}.`), await bodyText());
  check("…and the day below is still the one she was typing in", (await fieldValue("Temperature")) === "36.9");
  await pick("date", yesterday);
  await settle("yesterday");
  check("a pick replaces the address", (await path()) === `/day/${yesterday}`, await path());
  check("…and loads that day in place, leaving what was typed for the other", (await fieldValue("Temperature")) === "36.62", await fieldValue("Temperature"));
  const dateInputStayed = await js(`(() => { const i = document.querySelector('input[type="date"]'); i.dataset.mark = "kept"; return true; })()`);
  await pick("date", today);
  await settle("today again");
  check("the date box itself isn't rebuilt when the day changes", dateInputStayed && (await js(`document.querySelector('input[type="date"]').dataset.mark`)) === "kept");
  await click("Back");
  check("Back goes to the home page", (await path()) === "/" && (await h1()) === "You're logged in", `${await path()} ${await h1()}`);

  // ---- addresses that aren't a day she can open ----
  for (const [what, address] of [["tomorrow", `/day/${tomorrow}`], ["30 February", "/day/2026-02-30"], ["before 2000", "/day/1999-12-31"], ["not a date", "/day/today"], ["below a day", `/day/${today}/edit`]]) {
    await goto(address);
    check(`/day/… for ${what} goes to the home page`, (await path()) === "/" && (await h1()) === "You're logged in", `${await path()} ${await h1()}`);
  }

  // ---- what else the server can answer ----
  await goto(`/day/${today}`);
  await type("Temperature", "36.6");
  putReplies.push({ status: 500, body: apiError("internal") });
  await click("Save entry");
  await waitFor(`document.body.innerText.includes("Something went wrong")`, "the error");
  check("a server error: try again, in the Save bar, never the server's words", (await has("Something went wrong. Try again.")) && !(await has("a message for developers")) && (await hasButton("Save entry")), await bodyText());
  await type("Temperature", "36.7");
  check("…and it goes as soon as she changes something", !(await has("Something went wrong.")));
  putReplies.push("offline");
  await click("Save entry");
  await waitFor(`document.body.innerText.includes("Can't reach the server")`, "the offline message");
  check("no connection: said so", await has("Can't reach the server. Check your connection and try again."), await bodyText());
  putReplies.push({ status: 422, body: apiError("temperature_out_of_range", "temperature") });
  await click("Save entry");
  await waitFor(`document.body.innerText.includes("isn't a body temperature")`, "the refusal");
  check("a refusal about the temperature is said under the temperature", await js(`(() => { const e = document.getElementById(${temperatureInput}.getAttribute("aria-describedby").split(" ")[1] ?? ""); return e?.textContent.includes("isn't a body temperature") === true; })()`), await bodyText());
  putReplies.push({ status: 422, body: apiError("date_in_future", "date") });
  await click("Save entry");
  await waitFor(`document.body.innerText.includes("in the future")`, "the date refusal");
  check("a refusal about the date is said under the date", await has("That date is in the future. Check the date on your device."), await bodyText());
  putReplies.push({ status: 200, body: dayOf(today), delay: 900 });
  await click("Save entry");
  check("while a save is on its way, Save is off", await js(`[...document.querySelectorAll("button")].find((b) => b.textContent.trim() === "Save entry")?.disabled === true`));
  await waitFor(`location.pathname === "/"`, "home after the slow save");
  check("…and a slow save still ends at home, saying so", (await toast()) === "Entry saved", String(await toast()));

  // The day can't be read.
  getReply = { status: 500, body: apiError("internal") };
  await goto(`/day/${yesterday}`);
  check("a day that can't be read: said so, with Try again, and no form", (await has("Something went wrong. Try again.")) && (await hasButton("Try again")) && (await js(`${temperatureInput} === null`)), await bodyText());
  check("…and Save is off", await js(`[...document.querySelectorAll("button")].find((b) => b.textContent.trim() === "Save entry")?.disabled === true`));
  await shot("p-day-unread");
  getReply = null;
  await click("Try again");
  await settle("the day after Try again");
  check("Try again reads it again", (await fieldValue("Temperature")) === "36.62", String(await fieldValue("Temperature")));
  getReply = "offline";
  await goto(`/day/${yesterday}`);
  check("no connection while reading the day: said so", await has("Can't reach the server. Check your connection and try again."), await bodyText());
  getReply = null;

  // Not onboarded after all (another device, a reset): onboarding, at /.
  getReply = { status: 409, body: apiError("not_onboarded") };
  meReply = { status: 200, body: NOT_ONBOARDED };
  await goto(`/day/${today}`);
  await waitFor(`document.querySelector("h1")?.textContent === "Set up your chart"`, "onboarding");
  check("not onboarded: onboarding, at /", (await h1()) === "Set up your chart" && (await path()) === "/", `${await path()} ${await h1()}`);
  getReply = null;
  meReply = { status: 200, body: ONBOARDED };

  // ---- a screen left open past midnight in Israel ----
  // 23:59 on 14 November there (21:59 UTC). The page's clock is moved past
  // midnight while the screen stays open; a phone waking up is a focus.
  await S("Emulation.setTimezoneOverride", { timezoneId: "Asia/Jerusalem" });
  await S("Page.addScriptToEvaluateOnNewDocument", { source: `(() => { const Real = Date; const shift = Real.UTC(2026, 10, 14, 21, 59) - Real.now();
    globalThis.__madamExtra = 0;
    globalThis.Date = class extends Real { constructor(...a) { if (a.length === 0) super(Real.now() + shift + globalThis.__madamExtra); else super(...a); } static now() { return Real.now() + shift + globalThis.__madamExtra; } }; })()` });
  await goto("/");
  await click("Today's entry");
  await settle("today's entry at 23:59");
  check("at 23:59 on the 14th, today's entry is the 14th", (await path()) === "/day/2026-11-14" && (await h1()) === "Today's entry", `${await path()} ${await h1()}`);
  await type("Temperature", "36.55");
  await js(`globalThis.__madamExtra = 2 * 60 * 1000; window.dispatchEvent(new Event("focus"))`);
  await sleep(300);
  check("after midnight the screen keeps its day, and says which it is", (await path()) === "/day/2026-11-14" && (await has("This is the entry for Saturday 14/11/2026. Today is Sunday 15/11/2026.")), await bodyText());
  check("…its title is that day's date, not Today's entry", (await h1()) === "Saturday 14/11/2026", await h1());
  check("…and what she typed is still there", (await fieldValue("Temperature")) === "36.55");
  check("…and the 15th can now be picked", (await js(`document.querySelector('input[type="date"]').max`)) === "2026-11-15");
  await screen("p-day-after-midnight");
  await click("Go to today's entry");
  await settle("the 15th");
  check("\"Go to today's entry\" opens the 15th", (await path()) === "/day/2026-11-15" && (await h1()) === "Today's entry" && !(await has("This is the entry for")), `${await path()} ${await h1()}`);
} catch (error) {
  failed++;
  console.log("STOPPED: " + error.message);
}

const csp = consoleLines.filter((l) => /content security policy|refused to/i.test(l));
check("no Content-Security-Policy violation on any screen", csp.length === 0, csp.join(" | "));
const errors = consoleLines.filter((l) => /^\[(error|console\.error)\]/.test(l) && !/content security policy/i.test(l) && !/Failed to load resource/.test(l));
console.log(`console: ${consoleLines.length} lines, ${errors.length} errors besides failed requests`);
for (const line of [...new Set(errors)].slice(0, 12)) console.log("   " + line.slice(0, 220));
check("no error from the page's own code", errors.length === 0, errors.slice(0, 3).join(" | "));
console.log(failed === 0 ? "ALL CHECKS PASSED" : `${failed} CHECK(S) FAILED`);
ws.close(); chrome.kill();
process.exit(failed === 0 ? 0 : 1);
