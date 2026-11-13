// Browser check 2 of 2: the behaviours found in the first real use of the
// sign-in screens. See README.md in this folder, and auth-screens.mjs for
// how the browser is driven.
//
// Three things here can't be produced with real requests, so the script
// stages them:
//   - a second tab announcing a password recovery, as the auth client does
//     to every open tab when a reset link is opened in one of them;
//   - Supabase's two answers to a sign-up (a new account, and its made-up
//     "success" for an address that already has one), by answering the
//     page's request itself, so no account is created and no email is sent;
//   - a minute passing, by moving the page's clock.
//
//   node auth-first-use.mjs <folder for screenshots> [BASE] [--only-tabs]
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
let failed = 0;
function check(name, ok, detail = "") {
  if (!ok) failed++;
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${ok || !detail ? "" : "\n        got: " + detail}`);
}
const has = async (s) => (await bodyText()).includes(s);
const value = (label) => js(`(() => { const l = [...document.querySelectorAll("label")].find((l) => l.textContent.trim() === ${q(label)}); return l ? document.getElementById(l.htmlFor).value : null; })()`);
// Waits for a submit to finish: the button is switched off while it runs.
const idle = () => waitFor(`!document.querySelector('button[type="submit"]')?.disabled`, "the form to finish");


// Answer some of Supabase's endpoints ourselves, so no account is created
// and no email is sent. Everything else goes to the network untouched.
const CORS = [
  { name: "Access-Control-Allow-Origin", value: "*" },
  { name: "Access-Control-Allow-Headers", value: "*" },
  { name: "Access-Control-Allow-Methods", value: "GET,POST,PUT,OPTIONS" },
  { name: "Content-Type", value: "application/json" },
];
let signupReply = null; // the JSON to answer /signup with
let resends = 0;
let recovers = 0;
const user = (email, identities) => ({ id: "11111111-1111-4111-8111-111111111111", aud: "authenticated", role: "authenticated", email, phone: "",
  app_metadata: { provider: "email", providers: ["email"] }, user_metadata: {}, identities, created_at: "2026-11-13T00:00:00Z", updated_at: "2026-11-13T00:00:00Z", is_anonymous: false });
ws.addEventListener("message", (m) => {
  const msg = JSON.parse(m.data);
  if (msg.method !== "Fetch.requestPaused") return;
  const { requestId, request } = msg.params;
  const reply = (status, body) => S("Fetch.fulfillRequest", { requestId, responseCode: status, responseHeaders: CORS, body: Buffer.from(JSON.stringify(body)).toString("base64") });
  if (request.method === "OPTIONS") return void reply(204, {});
  if (request.url.includes("/auth/v1/signup")) return void reply(200, signupReply);
  if (request.url.includes("/auth/v1/resend")) { resends++; return void reply(200, {}); }
  if (request.url.includes("/auth/v1/recover")) { recovers++; return void reply(200, {}); }
  void S("Fetch.continueRequest", { requestId });
});
await S("Fetch.enable", { patterns: [{ urlPattern: "*/auth/v1/signup*" }, { urlPattern: "*/auth/v1/resend*" }, { urlPattern: "*/auth/v1/recover*" }] });

const fakeSession = { access_token: "eyJhbGciOiJFUzI1NiIsImtpZCI6IngifQ.eyJzdWIiOiJ4In0.AAAA", refresh_token: "made-up", token_type: "bearer",
  expires_in: 3600, expires_at: Math.floor(Date.now() / 1000) + 3600,
  user: { id: "00000000-0000-4000-8000-000000000000", aud: "authenticated", email: "someone@example.com", app_metadata: {}, user_metadata: {}, created_at: "2026-11-12T00:00:00Z" } };

try {
  // ---- the tab left behind while a reset link opens elsewhere ----
  await goto("/forgot-password");
  check("tab A waits on the reset form's screen", (await h1()) === "Forgot password?", await h1());
  await type("Email", "typed-before@example.com");
  // Tab B: another tab of the same browser, where the reset link "opened".
  const b = await send("Target.createTarget", { url: BASE + "/design" });
  const bs = (await send("Target.attachToTarget", { targetId: b.targetId, flatten: true })).sessionId;
  await sleep(1500);
  await send("Runtime.evaluate", { expression: `localStorage.setItem("madam.auth", ${q(JSON.stringify(fakeSession))});
    new BroadcastChannel("madam.auth").postMessage({ event: "PASSWORD_RECOVERY", session: ${JSON.stringify(fakeSession)} });` }, bs);
  await waitFor(`document.querySelector("h1")?.textContent !== "Forgot password?"`, "tab A reacting to the other tab");
  await sleep(500);
  check("tab A does NOT show the new-password screen", (await h1()) !== "Choose a new password", await h1());
  check("tab A shows the signed-in page instead, at /", (await h1()) === "You're logged in" && (await path()) === "/", `${await path()} ${await h1()}`);
  await shot("x-tab-a");
  await send("Target.closeTarget", { targetId: b.targetId });
  await click("Log out");
  await waitFor(`document.querySelector("h1")?.textContent === "Log in"`, "Log in after log out");
  check("after being signed in, the typed email is forgotten", (await value("Email")) === "", String(await value("Email")));
  await js(`localStorage.removeItem("madam.auth")`);

  if (!process.argv.includes("--only-tabs")) {
    // ---- sign-up for an address that already has an account ----
    await goto("/signup");
    signupReply = user("taken@example.com", []);
    await type("Email", "taken@example.com");
    await type("Password", "long-enough-passw0rd");
    await type("Invite code", "any-code");
    await click("Create account");
    check("an existing address is named as such", await waitFor(`document.body.innerText.includes("This email already has an account.")`, "the existing-account message"), await bodyText());
    check("…and no 'check your email' is shown", (await h1()) === "Create account", await h1());
    check("…with 'Log in instead' as a link inside the message", await js(`(() => { const b = [...document.querySelectorAll('[role="alert"] button')].find((b) => b.textContent.trim() === "Log in instead"); return Boolean(b) && b.type === "button"; })()`));
    await shot("x-signup-existing");
    await click("Log in instead");
    check("the link goes to Log in, and she chose to go", (await path()) === "/" && (await h1()) === "Log in", `${await path()} ${await h1()}`);
    check("…with the address she tried already filled in", (await value("Email")) === "taken@example.com", String(await value("Email")));
    await js(`history.back()`); await sleep(300);
    check("Back returns to the sign-up form", (await path()) === "/signup" && (await h1()) === "Create account");
    await type("Password", "long-enough-passw0rd");
    await type("Invite code", "any-code");

    // ---- a new address: check your email, with the wait ----
    signupReply = user("new@example.com", [{ identity_id: "i", provider: "email", identity_data: { email: "new@example.com" } }]);
    await type("Email", "new@example.com");
    await click("Create account");
    check("a new address gets 'Check your email'", await waitFor(`document.querySelector("h1")?.textContent === "Check your email"`, "the sent screen"), await bodyText());
    check("…with advice about spam", await has("look in spam"));
    const waitLine = (await bodyText()).match(/You can send it again in (\d+) seconds?\./);
    check("the wait is shown, about a minute", waitLine !== null && Number(waitLine[1]) >= 58 && Number(waitLine[1]) <= 60, String(waitLine?.[0]));
    const disabled = () => js(`[...document.querySelectorAll("button")].find((b) => b.textContent.trim() === "Send it again").disabled`);
    check("Send it again is switched off meanwhile", (await disabled()) === true);
    await click("Send it again");
    check("…and a tap on it sends nothing", resends === 0, `resends: ${resends}`);
    await shot("x-check-email-waiting");
    await sleep(2100);
    const later = (await bodyText()).match(/You can send it again in (\d+) seconds?\./);
    check("the wait counts down", later !== null && Number(later[1]) < Number(waitLine[1]), `${waitLine?.[1]} then ${later?.[1]}`);
    // Jump the page's clock a minute ahead instead of waiting one.
    await js(`(() => { const real = Date.now.bind(Date); Date.now = () => real() + 61000; })()`);
    check("after a minute Send it again is available", await waitFor(`![...document.querySelectorAll("button")].find((b) => b.textContent.trim() === "Send it again").disabled`, "the button to switch on", 5000));
    check("…and the wait line is gone", !(await has("You can send it again in")));
    await click("Send it again");
    check("sending again says so", await waitFor(`document.body.innerText.includes("Sent again.")`, "the sent-again note"), await bodyText());
    check("…exactly one email was asked for", resends === 1, `resends: ${resends}`);
    check("…and the wait starts over", (await disabled()) === true && (await has("You can send it again in")));
    await shot("x-check-email-sent-again");

    // ---- reset: the same answer for every address, with advice ----
    await goto("/forgot-password");
    await type("Email", "nobody-here@example.com");
    await click("Send reset link");
    await waitFor(`document.querySelector("h1")?.textContent === "Check your email"`, "the sent screen");
    check("reset still says 'If there's an account for…'", await has("If there's an account for"));
    check("…with advice about spam and the address", await has("look in spam, and check the address above"));
    check("reset: one email was asked for", recovers === 1, `recovers: ${recovers}`);
    check("reset: Send it again is there, switched off, with the wait", (await disabled()) === true && (await has("You can send it again in")));
    await click("Send it again");
    check("reset: a tap during the wait sends nothing", recovers === 1, `recovers: ${recovers}`);
    await shot("x-forgot-waiting");
    // The page's clock is already a minute ahead from the sign-up part; a
    // new screen started a new wait from that clock, so jump once more.
    await js(`(() => { const shifted = Date.now.bind(Date); Date.now = () => shifted() + 61000; })()`);
    check("reset: after a minute it is available", await waitFor(`![...document.querySelectorAll("button")].find((b) => b.textContent.trim() === "Send it again").disabled`, "the button to switch on", 5000));
    await click("Send it again");
    check("reset: sending again says so", await waitFor(`document.body.innerText.includes("Sent again.")`, "the sent-again note"), await bodyText());
    check("reset: exactly one more email was asked for", recovers === 2, `recovers: ${recovers}`);
    check("reset: and the wait starts over", (await disabled()) === true && (await has("You can send it again in")));
  }
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
