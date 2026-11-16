// Browser check 1 of 2: the sign-in screens and the component gallery, as a
// person on a phone would use them. See README.md in this folder.
//
// It drives headless Chrome through Chrome's debugging protocol (CDP): a
// WebSocket on which the script asks the browser to open a page, type,
// click, read the screen and take a screenshot. Node's built-in WebSocket
// is enough, so there is no dependency.
//
// What it talks to: the app at BASE (default http://localhost:8080, the
// production image from `make docker-run`), and through the app, madam-dev's
// real Supabase. It creates nothing there: it only sends a wrong log-in and
// a reset request for an address that has no account.
//
//   node auth-screens.mjs <folder for screenshots> [BASE]
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
const bodyText = () => js(`document.body.innerText`);
const path = () => js(`location.pathname`);
const h1 = () => js(`document.querySelector("h1")?.textContent ?? ""`);
async function shot(name) {
  const { data } = await S("Page.captureScreenshot", { format: "png", captureBeyondViewport: true });
  writeFileSync(join(OUT, name + ".png"), Buffer.from(data, "base64"));
}
// shotOf saves only the element that expression finds, so one component can
// be looked at closely.
async function shotOf(name, expression) {
  const clip = await js(`(() => { const b = (${expression}).getBoundingClientRect();
    return { x: b.left + scrollX - 8, y: b.top + scrollY - 8, width: b.width + 16, height: b.height + 16, scale: 1 }; })()`);
  const { data } = await S("Page.captureScreenshot", { format: "png", captureBeyondViewport: true, clip });
  writeFileSync(join(OUT, name + ".png"), Buffer.from(data, "base64"));
}
let failed = 0;
function check(name, ok, detail = "") {
  if (!ok) failed++;
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${ok || !detail ? "" : "\n        got: " + detail}`);
}
const has = async (s) => (await bodyText()).includes(s);
const value = (label) => js(`(() => { const l = [...document.querySelectorAll("label")].find((l) => l.textContent.trim() === ${q(label)}); return l ? document.getElementById(l.htmlFor).value : null; })()`);
// Waits for a submit to finish: the button is switched off while it runs.
const idle = () => waitFor(`!document.querySelector('button[type="submit"]')?.disabled`, "the form to finish");

try {
  // ---- log in ----
  await goto("/");
  check("/ shows Log in when nobody is signed in", (await h1()) === "Log in", await h1());
  await shot("p-login");
  check("log in: a greyed reminder of the password rule", await has("Reminder: at least 8 characters, with an English letter and a digit."));
  await click("Log in");
  check("empty log-in: asks for the email", await has("Enter your email address."));
  check("empty log-in: asks for the password", await has("Enter your password."));
  check("the invalid field is marked for screen readers", await js(`document.querySelectorAll('input[aria-invalid="true"]').length`) === 2);
  await type("Email", "nobody-here@example.com");
  await type("Password", "not-a-real-Password-1");
  await click("Log in");
  check("wrong log-in (a real answer from Supabase): our own words", await waitFor(`document.body.innerText.includes("Wrong email or password.")`, "the wrong-login message"), await bodyText());
  check("…and not Supabase's", !(await has("Invalid login credentials")));
  check("still on the log-in screen", (await h1()) === "Log in" && (await path()) === "/");
  check("the reminder stays under the password while the form shows its error", await has("Reminder: at least 8 characters"));
  check("one wrong password: no reset offer yet", !(await has("Forgot your password?")));
  await idle(); await click("Log in"); await sleep(300); await idle();
  check("two wrong passwords: still none", !(await has("Forgot your password?")));
  await click("Log in"); await sleep(300); await idle();
  check("three wrong passwords: the reset offer appears", await waitFor(`document.body.innerText.includes("Forgot your password?")`, "the reset offer"), await bodyText());
  await shot("p-login-wrong");
  await type("Password", "ךשמעוקג1");
  await click("Log in"); await sleep(300); await idle();
  check("a password typed in Hebrew: the error mentions the keyboard", await waitFor(`document.body.innerText.includes("Check that your keyboard is set to English.")`, "the keyboard hint"), await bodyText());
  await shot("p-login-keyboard");
  await click("Reset it by email");
  check("the offer's link goes to the reset form", (await path()) === "/forgot-password" && (await h1()) === "Forgot password?", `${await path()} ${await h1()}`);
  check("…with her email already filled in", (await value("Email")) === "nobody-here@example.com", String(await value("Email")));
  await js(`history.back()`); await sleep(300);
  check("back on Log in, the email is still there", (await h1()) === "Log in" && (await value("Email")) === "nobody-here@example.com", String(await value("Email")));

  // ---- sign up ----
  await click("Create account");
  check("sign-up: the password rule shows before any error", await has("At least 8 characters, with an English letter and a digit."));
  check("Create account goes to /signup", (await path()) === "/signup" && (await h1()) === "Create account", `${await path()} ${await h1()}`);
  check("…and the email came along", (await value("Email")) === "nobody-here@example.com", String(await value("Email")));
  await type("Password", "שלוםעולם12");
  await type("Invite code", "x");
  await click("Create account");
  check("sign-up: a password in Hebrew is refused, asking about the keyboard", await has("Use English letters, digits and symbols only. Is your keyboard set to another language?"));
  check("…and nothing was sent", (await h1()) === "Create account");
  await type("Invite code", "");
  await shot("p-signup");
  await type("Email", "not-an-email");
  await type("Password", "short");
  await click("Create account");
  check("sign-up: bad email is named", await has("Enter a valid email address."));
  check("sign-up: short password is named", await has("Use at least 8 characters."));
  check("sign-up: missing invite code is named", await has("Enter your invite code."));
  check("sign-up: the error replaces the hint", !(await has("At least 8 characters, with an English letter and a digit.")));
  await type("Password", "abcdefgh");
  await click("Create account");
  check("sign-up: letters alone are refused, with the rule", await has("Include at least one English letter and one digit."));
  await type("Password", "12345678");
  await click("Create account");
  check("sign-up: digits alone are refused too", await has("Include at least one English letter and one digit."));
  await type("Password", "short");
  await click("Create account");
  check("sign-up: nothing was sent, still on the form", (await h1()) === "Create account");
  await shot("p-signup-errors");
  await js(`history.back()`); await sleep(300);
  check("the browser's Back returns to Log in", (await path()) === "/" && (await h1()) === "Log in", `${await path()} ${await h1()}`);
  check("…keeping what was typed as the email", (await value("Email")) === "not-an-email", String(await value("Email")));

  // ---- forgot password ----
  await click("Forgot password?");
  check("Forgot password? goes to /forgot-password", (await path()) === "/forgot-password", await path());
  check("…and the email came along again", (await value("Email")) === "not-an-email", String(await value("Email")));
  await click("Send reset link");
  check("forgot: a bad email is named", await has("Enter a valid email address."));
  await type("Email", "nobody-here@example.com");
  await click("Send reset link");
  check("forgot: the same 'check your email' for an unknown address", await waitFor(`document.querySelector("h1")?.textContent === "Check your email"`, "the sent screen"), await bodyText());
  check("…naming the address", await has("nobody-here@example.com"));
  await shot("p-forgot-sent");
  await click("Back to log in");
  check("Back to log in works", (await path()) === "/" && (await h1()) === "Log in");

  // ---- links from emails ----
  await goto("/reset-password");
  check("/reset-password without a link: expired screen", (await h1()) === "This link has expired", await h1());
  await shot("p-reset-expired");
  await click("Ask for a new link");
  check("…and its button leads to the reset form", (await path()) === "/forgot-password");
  await goto("/reset-password#error=access_denied&error_code=otp_expired&error_description=Email+link+is+invalid+or+has+expired");
  check("an expired reset link: expired screen", (await h1()) === "This link has expired", await h1());
  await goto("/#error=access_denied&error_code=otp_expired&error_description=Email+link+is+invalid+or+has+expired");
  check("an expired confirmation link: Log in, with a notice", (await h1()) === "Log in" && (await has("That link has expired or was already used.")), await bodyText());
  await shot("p-login-link-expired");
  await goto("/somewhere-else");
  check("an unknown path goes to Log in", (await path()) === "/" && (await h1()) === "Log in", `${await path()} ${await h1()}`);
  check("signed out: nothing kept in storage", await js(`localStorage.getItem("madam.auth")`) === null);

  // ---- a session the server won't accept ----
  // A made-up session in storage, as if she had logged in earlier and the
  // token has since become worthless. No real account exists to log in with.
  const fake = { access_token: "eyJhbGciOiJFUzI1NiIsImtpZCI6IngifQ.eyJzdWIiOiJ4In0.AAAA", refresh_token: "made-up", token_type: "bearer",
    expires_in: 3600, expires_at: Math.floor(Date.now() / 1000) + 3600,
    user: { id: "00000000-0000-4000-8000-000000000000", aud: "authenticated", email: "someone@example.com", app_metadata: {}, user_metadata: {}, created_at: "2026-11-12T00:00:00Z" } };
  await js(`localStorage.setItem("madam.auth", ${q(JSON.stringify(fake))})`);
  await goto("/signup");
  await waitFor(`document.querySelector("h1")?.textContent === "You're logged in"`, "the signed-in page");
  check("a stored session shows the signed-in page, at /", (await h1()) === "You're logged in" && (await path()) === "/", `${await path()} ${await h1()}`);
  check("…with her address", await has("someone@example.com"));
  check("the server refuses the worthless token, and the page says so", await waitFor(`document.body.innerText.includes("Your session wasn't accepted")`, "the 401 message"), await bodyText());
  await shot("p-home-rejected");
  await click("Log out");
  check("Log out returns to Log in", await waitFor(`document.querySelector("h1")?.textContent === "Log in"`, "Log in after log out"), await h1());
  check("…and forgets the session", await js(`localStorage.getItem("madam.auth")`) === null);

  // ---- the gallery ----
  await goto("/design");
  const seg = await js(`(() => { const g = document.querySelector('[role="radiogroup"]'); return [g.getBoundingClientRect().width, g.parentElement.getBoundingClientRect().width]; })()`);
  check("segmented control hugs its segments", seg[0] < seg[1] * 0.6, `track ${seg[0]}px in a ${seg[1]}px row`);
  check("a tap 3px above a 36px segment still hits it", await js(`(() => { const l = [...document.querySelectorAll('[role="radiogroup"] label')].find((l) => l.textContent.trim() === "Cycle");
    const r = l.getBoundingClientRect(); l.scrollIntoView({ block: "center" }); const r2 = l.getBoundingClientRect();
    return document.elementFromPoint(r2.left + r2.width / 2, r2.top - 3)?.closest("label") === l; })()`));
  await js(`[...document.querySelectorAll("label")].find((l) => l.textContent.trim() === "Cycle").click()`);
  check("choosing a segment updates the choice", await has("Chosen: cycle, 14, celsius."));
  await js(`[...document.querySelectorAll("label")].find((l) => l.textContent.trim() === "Illness").click()`);
  check("chips allow several", await has("Chosen: alcohol, illness."));
  await js(`[...document.querySelectorAll("label")].find((l) => l.textContent.trim() === "Light").click()`);
  await js(`[...document.querySelectorAll("label")].find((l) => l.textContent.trim() === "Heavy").click()`);
  check("option buttons allow one", await has("Chosen: heavy."));
  await js(`[...document.querySelectorAll("label")].find((l) => l.textContent.trim() === "Cycle starts on this day").click()`);
  check("tapping a switch's words flips it", await js(`document.querySelector('input[role="switch"]').checked`) === true);
  check("the gallery's password error starts shown", await has("Use at least 8 characters."));
  await type("Password", "longenough");
  check("…8 letters still need a digit", await has("Include at least one English letter and one digit."));
  await type("Password", "long-en0ugh");
  check("…and it goes away with a letter and a digit", !(await has("Use at least 8 characters.")) && !(await has("Include at least one English letter")));
  await click("Save entry");
  check("Save turns into the question", await has("Save without a temperature?"));
  await click("Save without");
  check("answering brings Save back", (await has("Answered: save without.")) && (await js(`[...document.querySelectorAll("button")].some((b) => b.textContent.trim() === "Save entry")`)));

  // ---- the pieces added for the day screen (Slice 4) ----
  const tap = (text) => js(`[...document.querySelectorAll("label")].find((l) => l.textContent.trim() === ${q(text)}).click()`);
  const radio = (text) => `[...document.querySelectorAll('input[type="radio"]')].find((i) => i.closest("label").textContent.trim() === ${q(text)})`;
  const section = (title) => `[...document.querySelectorAll("h2")].find((h) => h.textContent === ${q(title)}).parentElement`;
  const space = async () => {
    await S("Input.dispatchKeyEvent", { type: "keyDown", key: " ", code: "Space", windowsVirtualKeyCode: 32, text: " " });
    await S("Input.dispatchKeyEvent", { type: "keyUp", key: " ", code: "Space", windowsVirtualKeyCode: 32 });
    await sleep(100);
  };

  // Option buttons that can be cleared: a question that may have no answer.
  check("a clearable question starts with its answer", await has("Secretion: milky."));
  await tap("Sticky");
  check("…choosing another answer replaces it", await has("Secretion: sticky."));
  await tap("Sticky");
  check("…and choosing the chosen answer again clears it", (await has("Secretion: none.")) && (await js(`${radio("Sticky")}.checked`)) === false);
  await tap("Milky");
  await js(`${radio("Milky")}.focus()`);
  await space();
  check("…with the space bar too", await has("Secretion: none."));
  await space();
  check("…and the space bar chooses it again", await has("Secretion: milky."));
  check("a screen reader is told the question can be cleared", await js(`(() => { const f = ${radio("Milky")}.closest("fieldset");
    return document.getElementById(f.getAttribute("aria-describedby"))?.textContent.includes("again to clear it") === true; })()`));
  await tap("Heavy");
  check("bleeding can't be cleared: tapping the chosen answer keeps it", (await has("Chosen: heavy.")) && (await js(`${radio("Heavy")}.checked`)) === true);
  await shotOf("p-option-buttons", section("Option buttons"));

  // A disabled switch: shown, and neither its words nor its track flip it.
  const switches = () => js(`[...document.querySelectorAll('input[role="switch"]')].map((i) => [i.disabled, i.checked])`);
  check("the gallery has two disabled switches, one off and one on", JSON.stringify((await switches()).slice(2)) === "[[true,false],[true,true]]", JSON.stringify(await switches()));
  await tap("Off, and disabled");
  await tap("On, and disabled");
  await js(`document.querySelectorAll('input[role="switch"]')[2].click()`);
  check("…and tapping them changes nothing", JSON.stringify((await switches()).slice(2)) === "[[true,false],[true,true]]", JSON.stringify(await switches()));
  await shotOf("p-switch", section("Switch"));

  // The temperature box: what she types stays exactly as typed.
  const tempInput = `document.querySelector('input[inputmode="decimal"]')`;
  check("the temperature is 40px and asks for the decimal keyboard", await js(`getComputedStyle(${tempInput}).fontSize`) === "40px", await js(`getComputedStyle(${tempInput}).fontSize`));
  check("…and its row is tall enough to tap", (await js(`${tempInput}.parentElement.getBoundingClientRect().height`)) >= 44, String(await js(`${tempInput}.parentElement.getBoundingClientRect().height`)));
  await type("Temperature", "36,62");
  check("a comma is kept as typed", (await value("Temperature")) === "36,62" && (await has('The first holds "36,62"')), String(await value("Temperature")));
  check("focus turns the underline berry", await js(`getComputedStyle(${tempInput}.parentElement).borderBottomColor`) === "rgb(160, 24, 90)", await js(`getComputedStyle(${tempInput}.parentElement).borderBottomColor`));
  check("the box is read together with the line about the reading", await js(`document.getElementById(${tempInput}.getAttribute("aria-describedby"))?.textContent.includes("counts as") === true`));
  check("a temperature's error sits under it, and marks it invalid", (await has("That isn't a body temperature. Enter a value between 20.00 and 50.00 °C.")) && (await js(`document.querySelectorAll('input[inputmode="decimal"]')[1].getAttribute("aria-invalid")`)) === "true");
  await shotOf("p-temperature-box", section("Temperature box"));

  // The text area: more than one line, kept as lines.
  await type("Notes", "Slept badly.");
  await S("Input.insertText", { text: "\nSecond line." });
  check("the notes keep their line break", (await value("Notes")) === "Slept badly.\nSecond line." && (await has("25 characters, 2 lines.")), JSON.stringify(await value("Notes")));
  const notesBox = await js(`(() => { const t = document.querySelector("textarea"); return [t.getBoundingClientRect().height, getComputedStyle(t).fontSize]; })()`);
  check("the notes box is at least 72px tall, with 16px text", notesBox[0] >= 72 && notesBox[1] === "16px", notesBox.join(", "));
  await shotOf("p-text-area", section("Text area"));

  // The danger button: red words, no underline, a tap area of 44px.
  const danger = `[...document.querySelectorAll("button")].find((b) => b.textContent.trim() === "Delete entry")`;
  const dangerLook = await js(`(() => { const b = ${danger}, s = getComputedStyle(b); return [s.color, s.textDecorationLine, s.fontWeight, Math.round(b.getBoundingClientRect().height)]; })()`);
  // The danger colour as the page's own CSS gives it, read from a probe.
  const dangerColour = await js(`(() => { const p = document.createElement("p"); p.className = "text-danger"; document.body.append(p); const c = getComputedStyle(p).color; p.remove(); return c; })()`);
  check("the danger button is bold danger red, with no underline, 44px tall", dangerLook[0] === dangerColour && dangerLook[1] === "none" && Number(dangerLook[2]) >= 700 && dangerLook[3] >= 44, `${dangerLook.join(", ")} (danger is ${dangerColour})`);
  await click("Delete entry");
  check("…and a tap reaches it", await has("Tapped 1 time."));
  await shotOf("p-buttons", section("Buttons"));

  // The cervix: one heading over two rows, each row named for a screen reader.
  const legendOf = (text) => js(`${radio(text)}.closest("fieldset").querySelector("legend").textContent`);
  check("the cervix rows are named Firmness and Position", (await legendOf("Firm")) === "Firmness" && (await legendOf("High, open")) === "Position", `${await legendOf("Firm")} / ${await legendOf("High, open")}`);
  check("…inside one fieldset named Cervix", await js(`${radio("Firm")}.closest("fieldset").parentElement.closest("fieldset").querySelector("legend").textContent`) === "Cervix");
  check("…and only Cervix is shown: the row names are heard, not seen", await js(`(() => { const l = ${radio("Firm")}.closest("fieldset").querySelector("legend"); const r = l.getBoundingClientRect(); return r.width <= 1 && r.height <= 1; })()`));
  await tap("Firm");
  await tap("High, open");
  await tap("Low, closed");
  check("each row has its own answer, and both can be cleared", await has("Cervix: firm, low_closed."));
  await tap("Firm");
  check("…clearing one leaves the other", await has("Cervix: no firmness, low_closed."));

  // The disclosure: closed by default, opened by a tap or the keyboard, and
  // what is inside is kept while it is closed.
  const summary = `${section("Disclosure")}.querySelector("summary")`;
  const extraVisible = () => js(`(() => { const l = [...document.querySelectorAll("label")].find((l) => l.textContent.trim() === "Something extra"); return document.getElementById(l.htmlFor).checkVisibility(); })()`);
  check("a disclosure starts closed, its contents hidden", (await js(`${summary}.textContent`)) === "Show all fields" && !(await extraVisible()) && (await has("Closed.")));
  check("…its summary is at least 48px tall", (await js(`${summary}.getBoundingClientRect().height`)) >= 48, String(await js(`${summary}.getBoundingClientRect().height`)));
  await js(`${summary}.click()`);
  await sleep(350);
  check("a tap opens it and the words say what the next tap does", (await js(`${summary}.textContent`)) === "Hide extra fields" && (await extraVisible()) && (await has("Open.")), await js(`${summary}.textContent`));
  // Tailwind 4's rotate-180 sets CSS's own rotate property, not transform.
  check("…and the chevron has turned half a circle", (await js(`getComputedStyle(${summary}.querySelector("svg")).rotate`)) === "180deg", await js(`getComputedStyle(${summary}.querySelector("svg")).rotate`));
  await shotOf("p-disclosure", section("Disclosure"));
  await type("Something extra", "kept");
  await js(`${summary}.focus()`);
  await S("Input.dispatchKeyEvent", { type: "keyDown", key: "Enter", code: "Enter", windowsVirtualKeyCode: 13, text: "\r" });
  await S("Input.dispatchKeyEvent", { type: "keyUp", key: "Enter", code: "Enter", windowsVirtualKeyCode: 13 });
  await sleep(200);
  check("Enter on the summary closes it", (await js(`${summary}.textContent`)) === "Show all fields" && !(await extraVisible()) && (await has("Closed.")));
  check("…and what was typed inside is still there", (await value("Something extra")) === "kept", String(await value("Something extra")));

  // The toast: appears, says it to a screen reader, and leaves by itself.
  check("no toast until something is done", await js(`document.querySelector('[role="status"]')`) === null);
  await click('Show "Entry saved"');
  const toastShown = Date.now();
  check("a toast says what was done", await js(`document.querySelector('[role="status"]')?.textContent`) === "Entry saved");
  await sleep(300); // past its 0.2 s rise, so the measurements are of it at rest
  const toastBox = await js(`(() => { const r = document.querySelector('[role="status"]').getBoundingClientRect(); return [Math.round(r.left + r.width / 2), Math.round(innerHeight - r.bottom), innerWidth]; })()`);
  check("…centred, 96px above the bottom of the screen", Math.abs(toastBox[0] - toastBox[2] / 2) <= 1 && toastBox[1] === 96, toastBox.join(", "));
  {
    const { data } = await S("Page.captureScreenshot", { format: "png" });
    writeFileSync(join(OUT, "p-toast.png"), Buffer.from(data, "base64"));
  }
  check("…and still there after 1.5 seconds", (await (async () => { await sleep(1500 - (Date.now() - toastShown)); return js(`document.querySelector('[role="status"]')?.textContent`); })()) === "Entry saved");
  await waitFor(`!document.querySelector('[role="status"]')`, "the toast to leave", 3000);
  const toastLasted = Date.now() - toastShown;
  check("…gone by itself after about 2.2 seconds", toastLasted >= 2000 && toastLasted <= 2900 && (await has("Now: no toast.")), `${toastLasted} ms`);
  await click('Show "Entry saved"');
  await sleep(1200);
  await click('Show "Entry deleted"');
  await sleep(1500);
  check("a second toast replaces the first and gets its own full time", await js(`document.querySelector('[role="status"]')?.textContent`) === "Entry deleted");
  await waitFor(`!document.querySelector('[role="status"]')`, "the second toast to leave", 3000);

  check("nothing in the gallery is wider than the phone", await js(`document.documentElement.scrollWidth <= innerWidth`), await js(`document.documentElement.scrollWidth + " in " + innerWidth`));

  // The time box: its own 24-hour text, with the native input invisible on
  // top. A real picker can't be opened headless, so a pick is staged by
  // setting the input's value the way the browser does.
  const setTime = (index, value) => js(`(() => { const i = document.querySelectorAll('input[type="time"]')[${index}];
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value").set.call(i, ${q(value)});
    i.dispatchEvent(new Event("input", { bubbles: true })); i.dispatchEvent(new Event("change", { bubbles: true })); })()`);
  const timeBoxText = (index) => js(`document.querySelectorAll('input[type="time"]')[${index}].closest("label").innerText`);
  check("the time box shows its time as 24-hour text", (await timeBoxText(0)).includes("07:00") && (await has("The first holds 07:00.")), await timeBoxText(0));
  check("a tap on the time box lands on the native input", await js(`(() => { const i = document.querySelector('input[type="time"]'); i.scrollIntoView({ block: "center" });
    const r = i.closest("label").getBoundingClientRect(); return document.elementFromPoint(r.left + r.width / 2, r.top + r.height / 2) === i; })()`));
  await setTime(0, "18:45");
  check("picking a time changes what the box shows", (await timeBoxText(0)).includes("18:45") && (await has("The first holds 18:45.")), await timeBoxText(0));
  check("an empty time box says hh:mm, with its error under it", (await timeBoxText(1)).includes("hh:mm") && (await has("Pick a time.")), await timeBoxText(1));
  await shotOf("p-time-box", `[...document.querySelectorAll("h2")].find((h) => h.textContent === "Time box").parentElement`);
  await setTime(1, "06:30");
  check("…and the error goes once a time is picked", (await timeBoxText(1)).includes("06:30") && !(await has("Pick a time.")), await timeBoxText(1));
  await setTime(0, "");
  check("clearing the time empties the box", (await timeBoxText(0)).includes("hh:mm") && (await has("The first holds no time.")), await timeBoxText(0));
  await setTime(0, "07:00");
  await shot("p-design");
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
