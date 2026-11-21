// OpenTick LinkedIn enrichment: service worker.
//
// - Reads profiles in LinkedIn tabs (extract.js, injected with
//   chrome.scripting) from the user's own logged-in session.
// - Puts every capture into an outbox in chrome.storage.local first, then
//   pushes it to the OpenTick server's /api/v2 and retries with backoff, so
//   nothing that was read is lost while the server is unreachable.
// - Runs "Fill missing": pulls the server's queue and works through it in a
//   background tab, paced by chrome.alarms (45–90 s apart, 80 a day by
//   default). It stops on rate limits, security checks and logged-out
//   sessions and never tries to get around them.
importScripts("core.js", "extract.js");

const C = globalThis.OpenTickCore;
const X = globalThis.OpenTickExtract;
const VERSION = chrome.runtime.getManifest().version;
const FEED = "https://www.linkedin.com/feed/";
const LOG_MAX = 300;

/* ---------------- storage ---------------- */

const store = {
  get: async (key, fallback) => (await chrome.storage.local.get(key))[key] ?? fallback,
  set: (key, value) => chrome.storage.local.set({ [key]: value }),
};

// Serializes read-modify-write cycles; the service worker handles several
// events at once and chrome.storage has no transactions.
let chain = Promise.resolve();
function locked(fn) {
  const p = chain.then(fn, fn);
  chain = p.catch(() => {});
  return p;
}

function update(key, fallback, fn) {
  return locked(async () => {
    const cur = await store.get(key, fallback);
    const next = fn(structuredClone(cur));
    await store.set(key, next);
    return next;
  });
}

async function getSettings() {
  return C.normalizeSettings(await store.get("settings", {}));
}

const IDLE_RUN = { status: "idle", reason: null, message: "", waiting: null, nextAt: null, current: null, tabId: null, tried: {}, stats: { saved: 0, failed: 0 } };

async function getRun() {
  return { ...IDLE_RUN, ...(await store.get("run", {})) };
}

function patchRun(patch) {
  return update("run", IDLE_RUN, (r) => ({ ...IDLE_RUN, ...r, ...patch }));
}

async function dailyCount() {
  const d = await store.get("daily", null);
  return d && d.day === C.dayKey() ? d.count : 0;
}

function bumpDaily() {
  return update("daily", null, (d) => {
    const day = C.dayKey();
    return { day, count: d && d.day === day ? d.count + 1 : 1 };
  });
}

function addLog(entry) {
  return update("log", [], (log) => [{ at: Date.now(), ...entry }, ...log].slice(0, LOG_MAX));
}

/* ---------------- OpenTick server ---------------- */

class ServerError extends Error {
  constructor(status, message) {
    super(message);
    this.status = status;
  }
}

async function hasServerPermission(settings) {
  const pattern = C.originPattern(settings.serverUrl);
  if (!pattern) return false;
  try {
    return await chrome.permissions.contains({ origins: [pattern] });
  } catch (_) {
    return false;
  }
}

async function server(method, path, body) {
  const s = await getSettings();
  if (!s.serverUrl || !s.token) throw new ServerError(-1, "Set the OpenTick server and API token in the options.");
  let res;
  try {
    res = await fetch(C.apiUrl(s.serverUrl, path), {
      method,
      headers: { Authorization: `Bearer ${s.token}`, "Content-Type": "application/json", Accept: "application/json" },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  } catch (e) {
    throw new ServerError(0, (await hasServerPermission(s)) ? "OpenTick server not reachable" : "The extension has no access to the OpenTick server yet. Open the options and click Save.");
  }
  const data = await res.json().catch(() => null);
  if (!res.ok) {
    let msg = (data && (data.detail || data.message || data.title)) || `HTTP ${res.status}`;
    if (res.status === 401) msg = "OpenTick rejected the API token (expired, deleted, or missing the LinkedIn permissions).";
    throw new ServerError(res.status, msg);
  }
  return data;
}

// Retry later: the server is down, overloaded or the token needs fixing.
function retryable(e) {
  return e.status <= 0 || e.status === 401 || e.status === 403 || e.status === 408 || e.status === 429 || e.status >= 500;
}

/* ---------------- outbox ---------------- */

// An entry: {id, kind: "profile"|"failure", url, name, body, origin,
// created, attempts, nextAt, state: "pending"|"failed", error}.

async function enqueue(kind, body, meta) {
  const entry = {
    id: `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`,
    kind,
    url: body.linkedin_url,
    name: meta.name || C.profileName(body),
    origin: meta.origin,
    body,
    created: Date.now(),
    attempts: 0,
    nextAt: 0,
    state: "pending",
    error: null,
  };
  await update("outbox", [], (box) => [...box, entry]);
  return entry;
}

function patchEntry(id, patch) {
  return update("outbox", [], (box) => box.map((e) => (e.id === id ? { ...e, ...patch } : e)));
}

function removeEntry(id) {
  return update("outbox", [], (box) => box.filter((e) => e.id !== id));
}

// Sends one entry. Returns {ok, outcome} when the server took it,
// {ok:false, kept:true} when it stays queued for a retry.
async function sendEntry(entry) {
  const path = entry.kind === "profile" ? "/contacts/linkedin/profiles" : "/contacts/linkedin/failures";
  try {
    const outcome = await server("POST", path, entry.body);
    await removeEntry(entry.id);
    await clearServerProblem();
    if (entry.kind === "profile") {
      await addLog({ name: entry.name, url: entry.url, contactId: outcome.contact_id, result: outcome.created ? "created" : "saved", detail: C.describeOutcome(outcome), origin: entry.origin });
    } else {
      await addLog({ name: entry.name, url: entry.url, result: entry.body.unavailable ? "unavailable" : "failed", detail: entry.body.detail || "", origin: entry.origin });
    }
    return { ok: true, outcome };
  } catch (e) {
    if (e.status === 404) {
      // Nobody in OpenTick has this URL (any more); nothing to retry.
      await removeEntry(entry.id);
      await addLog({ name: entry.name, url: entry.url, result: "skipped", detail: "Not in your OpenTick contacts", origin: entry.origin });
      return { ok: false, notContact: true, error: "Not in your OpenTick contacts." };
    }
    if (retryable(e)) {
      const attempts = entry.attempts + 1;
      await patchEntry(entry.id, { attempts, nextAt: Date.now() + C.backoffMs(attempts), error: e.message });
      if (e.status === 401 || e.status === -1) await noteServerProblem(e.message);
      return { ok: false, kept: true, error: e.message };
    }
    // Rejected (e.g. 400): keep it for the user to retry or discard.
    await patchEntry(entry.id, { state: "failed", attempts: entry.attempts + 1, error: e.message });
    return { ok: false, error: e.message };
  }
}

let flushing = false;
async function flushOutbox(force) {
  if (flushing) return;
  flushing = true;
  try {
    const box = await store.get("outbox", []);
    const now = Date.now();
    for (const entry of box) {
      if (entry.state !== "pending" || (!force && entry.nextAt > now)) continue;
      const r = await sendEntry(entry);
      // Server down: the rest would fail the same way.
      if (!r.ok && r.kept) break;
    }
  } finally {
    flushing = false;
    await scheduleOutbox();
  }
}

async function scheduleOutbox() {
  const box = await store.get("outbox", []);
  const pending = box.filter((e) => e.state === "pending");
  if (!pending.length) {
    await chrome.alarms.clear("outbox");
    return;
  }
  const when = Math.max(Date.now() + 30 * 1000, Math.min(...pending.map((e) => e.nextAt || 0)));
  await chrome.alarms.create("outbox", { when });
}

async function noteServerProblem(message) {
  await store.set("serverProblem", { message, at: Date.now() });
}

async function clearServerProblem() {
  if (await store.get("serverProblem", null)) await chrome.storage.local.remove("serverProblem");
}

/* ---------------- LinkedIn tabs ---------------- */

function withTimeout(promise, ms, what) {
  let t;
  return Promise.race([promise, new Promise((_, reject) => (t = setTimeout(() => reject(new Error(`${what} timed out`)), ms)))]).finally(() => clearTimeout(t));
}

// Keeps the service worker alive while a long read is in flight; Chrome
// stops idle workers after 30 s and resets the timer on extension API calls.
async function keepAlive(promise) {
  const t = setInterval(() => chrome.runtime.getPlatformInfo().catch(() => {}), 20 * 1000);
  try {
    return await promise;
  } finally {
    clearInterval(t);
  }
}

function waitForTab(tabId, timeoutMs) {
  return withTimeout(
    new Promise((resolve, reject) => {
      const done = (tab) => {
        chrome.tabs.onUpdated.removeListener(onUpdated);
        chrome.tabs.onRemoved.removeListener(onRemoved);
        resolve(tab);
      };
      const onUpdated = (id, info, tab) => {
        if (id === tabId && info.status === "complete") done(tab);
      };
      const onRemoved = (id) => {
        if (id !== tabId) return;
        chrome.tabs.onUpdated.removeListener(onUpdated);
        chrome.tabs.onRemoved.removeListener(onRemoved);
        reject(new Error("The LinkedIn tab was closed"));
      };
      chrome.tabs.onUpdated.addListener(onUpdated);
      chrome.tabs.onRemoved.addListener(onRemoved);
      chrome.tabs.get(tabId).then((tab) => tab.status === "complete" && done(tab), () => onRemoved(tabId));
    }),
    timeoutMs,
    "Loading LinkedIn",
  );
}

async function navigate(tabId, url) {
  await chrome.tabs.update(tabId, { url });
  // Let the navigation start before waiting for it to complete.
  await new Promise((r) => setTimeout(r, 500));
  return waitForTab(tabId, 45 * 1000);
}

// The background tab "Fill missing" reads in; created when missing.
async function workerTab() {
  const run = await getRun();
  if (run.tabId != null) {
    try {
      const tab = await chrome.tabs.get(run.tabId);
      if (/^https:\/\/www\.linkedin\.com\//.test(tab.url || tab.pendingUrl || "")) return waitForTab(tab.id, 45 * 1000);
    } catch (_) {
      /* closed */
    }
  }
  let tab;
  try {
    tab = await chrome.tabs.create({ url: FEED, active: false });
  } catch (_) {
    // No browser window open: use a minimized one.
    const win = await chrome.windows.create({ url: FEED, focused: false, state: "minimized" });
    tab = win.tabs[0];
  }
  await patchRun({ tabId: tab.id });
  return waitForTab(tab.id, 45 * 1000);
}

async function closeWorkerTab() {
  const run = await getRun();
  if (run.tabId == null) return;
  await patchRun({ tabId: null });
  try {
    await chrome.tabs.remove(run.tabId);
  } catch (_) {
    /* already closed */
  }
}

async function inject(tabId) {
  await chrome.scripting.executeScript({ target: { tabId }, files: ["extract.js"] });
}

// Reads a profile through the LinkedIn session of the page in tabId.
async function readInTab(tabId, vanity) {
  await inject(tabId);
  const [res] = await withTimeout(
    chrome.scripting.executeScript({ target: { tabId }, func: (v) => globalThis.OpenTickExtract.readProfile(v), args: [vanity] }),
    90 * 1000,
    "Reading the profile",
  );
  return res && res.result ? res.result : { ok: false, kind: null, error: "No result from the LinkedIn tab" };
}

async function readRenderedItems(tabId, timeoutMs) {
  await inject(tabId);
  const [res] = await chrome.scripting.executeScript({ target: { tabId }, func: (ms) => globalThis.OpenTickExtract.waitForDomItems(ms), args: [timeoutMs] });
  return (res && res.result) || [];
}

// The fallback when LinkedIn's API returns nothing usable: open the rendered
// details pages in the worker tab and read their text.
async function readDetailsPages(tabId, vanity) {
  const out = {};
  for (const section of ["experience", "education"]) {
    const tab = await navigate(tabId, `${C.profileUrl(vanity)}/details/${section}/`);
    const blocked = X.blockReason(tab.url);
    if (blocked) return { blocked };
    out[section] = await readRenderedItems(tabId, 15 * 1000);
  }
  await navigate(tabId, FEED).catch(() => {});
  return out;
}

const BLOCK_MESSAGES = {
  rate_limited: "LinkedIn is rate limiting this account. Fill missing stopped; wait several hours (ideally a day) before resuming.",
  checkpoint: "LinkedIn is showing a security check or captcha. Solve it yourself in the LinkedIn tab, then resume.",
  logged_out: "You're logged out of LinkedIn. Log in, then resume.",
  not_configured: "Set the OpenTick server and API token in the options.",
  server_auth: "OpenTick rejected the API token. Create a new one (People → Import) and paste it in the options.",
};

/* ---------------- Fill missing ---------------- */

async function setBadge() {
  const run = await getRun();
  const box = await store.get("outbox", []);
  let text = "";
  let color = "#4772FA";
  if (run.status === "blocked") {
    text = "!";
    color = "#B42318";
  } else if (run.status === "running") {
    text = run.waiting ? "…" : "▶";
  } else if (box.some((e) => e.state === "failed")) {
    text = "!";
    color = "#B54708";
  } else if (box.length) {
    text = String(box.length);
  }
  await chrome.action.setBadgeText({ text });
  await chrome.action.setBadgeBackgroundColor({ color });
}

async function scheduleTick(at) {
  const when = Math.max(Date.now() + 1000, at);
  await chrome.alarms.create("tick", { when });
  await patchRun({ nextAt: when });
}

async function start(trigger) {
  const run = await getRun();
  if (run.status === "running") return run;
  if (trigger !== "user") {
    // Automatic starts leave hard stops alone: a security check needs the
    // user, and a rate limit gets a day to cool down.
    if (run.status === "blocked" && run.reason === "checkpoint") return run;
    if (run.status === "blocked" && run.reason === "rate_limited" && Date.now() - (run.blockedAt || 0) < 24 * 3600 * 1000) return run;
  }
  await patchRun({ status: "running", reason: null, waiting: null, message: "Starting…", startedAt: Date.now(), current: null, stats: { saved: 0, failed: 0 } });
  await addLog({ result: "info", detail: trigger === "user" ? "Fill missing started" : `Fill missing started automatically (${trigger})` });
  await scheduleTick(Date.now());
  return getRun();
}

async function pause() {
  await chrome.alarms.clear("tick");
  await patchRun({ status: "paused", waiting: null, nextAt: null, current: null, message: "Paused" });
  await closeWorkerTab();
  await addLog({ result: "info", detail: "Fill missing paused" });
}

async function block(reason, message) {
  await chrome.alarms.clear("tick");
  await patchRun({ status: "blocked", reason, waiting: null, nextAt: null, current: null, blockedAt: Date.now(), message: message || BLOCK_MESSAGES[reason] || reason });
  await addLog({ result: "blocked", detail: message || BLOCK_MESSAGES[reason] || reason });
}

async function finish(message) {
  await chrome.alarms.clear("tick");
  await patchRun({ status: "done", waiting: null, nextAt: null, current: null, message });
  await closeWorkerTab();
  await addLog({ result: "info", detail: message });
}

async function wait(waiting, at, message) {
  await patchRun({ waiting, current: null, message });
  await scheduleTick(at.getTime ? at.getTime() : at);
}

let ticking = false;
async function tick() {
  if (ticking) return;
  ticking = true;
  try {
    await keepAlive(step());
  } catch (e) {
    // Unexpected: keep going after a pause rather than dying silently.
    await addLog({ result: "failed", detail: `Internal error: ${e.message || e}` });
    const s = await getSettings();
    if ((await getRun()).status === "running") await scheduleTick(Date.now() + C.randomDelayMs(s));
  } finally {
    ticking = false;
    await setBadge();
  }
}

async function step() {
  const run = await getRun();
  if (run.status !== "running") return;
  const s = await getSettings();
  if (!s.serverUrl || !s.token || !(await hasServerPermission(s))) return block("not_configured");

  const now = new Date();
  if (!C.inActiveHours(s, now)) {
    const at = C.nextActiveStart(s, now);
    return wait("active_hours", at, `Outside active hours; continues at ${at.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}.`);
  }
  if ((await dailyCount()) >= s.dailyCap) {
    const at = C.nextDayStart(s, now);
    return wait("daily_cap", at, `Daily limit of ${s.dailyCap} profiles reached; continues tomorrow.`);
  }

  // Push captures first, so the queue reflects them.
  await flushOutbox();
  let q;
  try {
    q = await server("GET", "/contacts/linkedin/queue?limit=200");
  } catch (e) {
    if (e.status === 401 || e.status === -1) return block(e.status === 401 ? "server_auth" : "not_configured");
    const attempts = (run.serverAttempts || 0) + 1;
    await patchRun({ serverAttempts: attempts });
    return wait("server", Date.now() + C.backoffMs(attempts), `${e.message}. Retrying…`);
  }
  await store.set("queueInfo", { total: q.total, coverage: q.coverage, at: Date.now() });

  const day = C.dayKey();
  const tried = Object.fromEntries(Object.entries(run.tried || {}).filter(([, d]) => d === day));
  const box = await store.get("outbox", []);
  const inOutbox = new Set(box.map((e) => String(e.url || "").toLowerCase()));
  const item = (q.items || []).find((i) => !tried[i.linkedin_url.toLowerCase()] && !inOutbox.has(i.linkedin_url.toLowerCase()));
  if (!item) {
    return finish(q.total ? `Every profile in the queue was tried today; ${q.total} left for another day.` : "Done: every contact with a LinkedIn URL has history.");
  }

  tried[item.linkedin_url.toLowerCase()] = day;
  await patchRun({ tried, serverAttempts: 0, waiting: null, current: { name: item.name, url: item.linkedin_url, contactId: item.contact_id }, message: `Reading ${item.name || item.linkedin_url}…` });
  await bumpDaily();

  const res = await processItem(item);
  if (res.blocked) return block(res.blocked, res.message);
  await patchRun({ stats: { saved: (run.stats?.saved || 0) + (res.saved ? 1 : 0), failed: (run.stats?.failed || 0) + (res.saved ? 0 : 1) } });

  const next = Date.now() + C.randomDelayMs(s);
  await patchRun({ current: null, message: `Waiting before the next profile.` });
  await scheduleTick(next);
}

// Reads one queued contact and queues the result for the server.
async function processItem(item) {
  const vanity = X.vanityFromUrl(item.linkedin_url);
  const meta = { name: item.name, origin: "batch" };
  if (!vanity) {
    await enqueue("failure", { linkedin_url: item.linkedin_url, unavailable: true, detail: "not a profile URL" }, meta);
    await flushOutbox();
    return {};
  }
  let tab;
  try {
    tab = await workerTab();
  } catch (e) {
    return { error: e.message };
  }
  const blocked = X.blockReason(tab.url);
  if (blocked) return { blocked };

  const r = await readInTab(tab.id, vanity);
  if (r.ok) {
    await enqueue("profile", C.profilePayload(item.linkedin_url, r.data, { version: VERSION, create: false }), meta);
    await flushOutbox();
    return { saved: true };
  }
  if (["rate_limited", "checkpoint", "logged_out"].includes(r.kind)) return { blocked: r.kind };
  if (r.kind === "not_found") {
    await enqueue("failure", { linkedin_url: item.linkedin_url, unavailable: true, detail: "profile not found" }, meta);
    await flushOutbox();
    return {};
  }
  if (r.kind === "unreadable") {
    const dom = await readDetailsPages(tab.id, vanity).catch((e) => ({ error: e.message }));
    if (dom.blocked) return { blocked: dom.blocked };
    if (!dom.error && ((dom.experience || []).length || (dom.education || []).length)) {
      await enqueue("profile", C.domPayload(item.linkedin_url, dom.experience, dom.education, { version: VERSION, create: false }), meta);
      await flushOutbox();
      return { saved: true };
    }
    r.error = dom.error || "nothing readable on the profile pages";
  }
  await enqueue("failure", { linkedin_url: item.linkedin_url, unavailable: false, detail: String(r.error || "unknown error").slice(0, 500) }, meta);
  await flushOutbox();
  return {};
}

/* ---------------- manual and automatic saves ---------------- */

// Reads the profile open in tabId and saves it. Used by the popup and the
// page button. Returns a result for the UI.
async function saveTab(tabId, origin) {
  const s = await getSettings();
  if (!s.serverUrl || !s.token) return { ok: false, error: BLOCK_MESSAGES.not_configured };
  const tab = await chrome.tabs.get(tabId);
  const vanity = X.vanityFromUrl(tab.url);
  if (!vanity) return { ok: false, error: "Open a LinkedIn profile (linkedin.com/in/…) first." };
  const url = C.profileUrl(vanity);

  const r = await keepAlive(readInTab(tabId, vanity));
  let body;
  if (r.ok) {
    body = C.profilePayload(url, r.data, { version: VERSION, create: s.createContacts });
  } else if (r.kind === "unreadable") {
    // Read what this page renders; works on the "Show all" details pages.
    const items = await readRenderedItems(tabId, 4000);
    if (!items.length) return { ok: false, error: 'Couldn\'t read this profile. Open "Show all experiences" and try again.' };
    const onEdu = /\/details\/education/.test(new URL(tab.url).pathname);
    body = C.domPayload(url, onEdu ? [] : items, onEdu ? items : [], { version: VERSION, create: s.createContacts });
    const name = C.nameFromTitle(tab.title);
    if (name) body.profile = { full_name: name };
  } else {
    return { ok: false, error: BLOCK_MESSAGES[r.kind] || r.error || "Couldn't read this profile." };
  }

  const entry = await enqueue("profile", body, { origin });
  const sent = await sendEntry(entry);
  await scheduleOutbox();
  await setBadge();
  if (sent.ok) return { ok: true, outcome: sent.outcome, message: C.describeOutcome(sent.outcome), link: C.contactLink(s.serverUrl, sent.outcome.contact_id) };
  if (sent.notContact) return { ok: false, error: "This person isn't in your OpenTick contacts. Turn on \"Add new people\" in the options to add them." };
  if (sent.kept) return { ok: true, queued: true, message: `Saved in the extension; it will reach OpenTick when the server is reachable (${sent.error}).` };
  return { ok: false, error: sent.error };
}

// Profiles of existing contacts are saved while the user browses, when that
// option is on. Each profile once a day at most, within the daily cap.
async function profileVisited(tabId, pageUrl) {
  const s = await getSettings();
  if (!s.autoSaveBrowsing || !s.serverUrl || !s.token) return;
  const run = await getRun();
  if (run.tabId === tabId) return;
  const vanity = X.vanityFromUrl(pageUrl);
  if (!vanity) return;
  const url = C.profileUrl(vanity);
  const key = url.toLowerCase();
  const recent = await store.get("autoSaved", {});
  if (recent[key] && Date.now() - recent[key] < 24 * 3600 * 1000) return;
  if ((await dailyCount()) >= s.dailyCap) return;

  let match;
  try {
    match = await server("GET", `/contacts/linkedin/lookup?url=${encodeURIComponent(url)}`);
  } catch (e) {
    return; // Not a contact, or the server is unreachable: nothing to do.
  }
  await update("autoSaved", {}, (m) => {
    const cutoff = Date.now() - 24 * 3600 * 1000;
    const out = Object.fromEntries(Object.entries(m).filter(([, t]) => t > cutoff));
    out[key] = Date.now();
    return out;
  });
  await bumpDaily();
  const r = await keepAlive(readInTab(tabId, vanity));
  if (!r.ok) {
    if (["rate_limited", "checkpoint", "logged_out"].includes(r.kind)) await addLog({ name: match.name, url, result: "blocked", detail: BLOCK_MESSAGES[r.kind], origin: "auto" });
    return;
  }
  await enqueue("profile", C.profilePayload(url, r.data, { version: VERSION, create: false }), { name: match.name, origin: "auto" });
  await flushOutbox();
  chrome.tabs.sendMessage(tabId, { type: "toast", text: `Saved ${match.name} to OpenTick`, kind: "ok" }).catch(() => {});
}

/* ---------------- auto-start ---------------- */

async function scheduleAutostart() {
  const s = await getSettings();
  await chrome.alarms.clear("autostart");
  if (s.autoStartDaily) {
    await chrome.alarms.create("autostart", { when: C.nextTimeOfDay(s.autoStartTime).getTime(), periodInMinutes: 24 * 60 });
  }
}

// After the worker restarts: make sure a running batch still has its alarm.
async function resume() {
  const run = await getRun();
  if (run.status === "running" && !(await chrome.alarms.get("tick"))) await scheduleTick(Math.max(Date.now() + 5000, run.nextAt || 0));
  await scheduleOutbox();
  await setBadge();
}

chrome.runtime.onInstalled.addListener(async () => {
  await scheduleAutostart();
  await resume();
  const s = await getSettings();
  if (!s.serverUrl || !s.token) chrome.runtime.openOptionsPage();
});

chrome.runtime.onStartup.addListener(async () => {
  await scheduleAutostart();
  await resume();
  const s = await getSettings();
  if (s.autoStartOnBrowser) {
    // A browser that was closed mid-run continues anyway; this only starts idle ones.
    await start("browser start");
  }
  flushOutbox();
});

chrome.alarms.onAlarm.addListener(async (alarm) => {
  if (alarm.name === "tick") await tick();
  else if (alarm.name === "outbox") await flushOutbox();
  else if (alarm.name === "autostart") {
    const s = await getSettings();
    if (s.autoStartDaily) await start("daily schedule");
  }
  await setBadge();
});

chrome.storage.onChanged.addListener((changes, area) => {
  if (area !== "local") return;
  if (changes.settings) scheduleAutostart();
  if (changes.run || changes.outbox) setBadge();
});

/* ---------------- messages ---------------- */

async function state() {
  const [settings, run, daily, outbox, log, queueInfo, serverProblem] = await Promise.all([
    getSettings(),
    getRun(),
    dailyCount(),
    store.get("outbox", []),
    store.get("log", []),
    store.get("queueInfo", null),
    store.get("serverProblem", null),
  ]);
  return {
    version: VERSION,
    configured: !!(settings.serverUrl && settings.token),
    hasPermission: await hasServerPermission(settings),
    serverUrl: settings.serverUrl,
    dailyCap: settings.dailyCap,
    run: { ...run, tried: undefined, triedToday: Object.values(run.tried || {}).filter((d) => d === C.dayKey()).length },
    daily,
    outbox: outbox.map(({ body, ...e }) => e),
    log,
    queueInfo,
    serverProblem,
  };
}

async function handle(msg, sender) {
  switch (msg && msg.type) {
    case "state":
      return state();
    case "start":
      await start("user");
      return state();
    case "pause":
      await pause();
      return state();
    case "runNow": {
      const run = await getRun();
      if (run.status === "running") {
        await patchRun({ waiting: null });
        await scheduleTick(Date.now());
      }
      return state();
    }
    case "resetBlock":
      await patchRun({ status: "idle", reason: null, message: "" });
      return state();
    case "retry": {
      await update("outbox", [], (box) => box.map((e) => (msg.id == null || e.id === msg.id ? { ...e, state: "pending", nextAt: 0 } : e)));
      await flushOutbox(true);
      return state();
    }
    case "discard":
      await removeEntry(msg.id);
      await addLog({ result: "info", detail: `Discarded the pending save of ${msg.name || "a profile"}` });
      return state();
    case "clearLog":
      await store.set("log", []);
      return state();
    case "saveTab": {
      const tabId = msg.tabId ?? (sender.tab && sender.tab.id);
      return saveTab(tabId, sender.tab ? "page button" : "popup");
    }
    case "profileVisited":
      if (sender.tab) profileVisited(sender.tab.id, msg.url);
      return { ok: true };
    case "test": {
      try {
        const q = await server("GET", "/contacts/linkedin/queue?limit=1");
        await store.set("queueInfo", { total: q.total, coverage: q.coverage, at: Date.now() });
        await clearServerProblem();
        flushOutbox(true);
        return { ok: true, total: q.total, coverage: q.coverage };
      } catch (e) {
        return { ok: false, error: e.message, status: e.status };
      }
    }
    case "settingsSaved":
      await scheduleAutostart();
      await resume();
      return { ok: true };
    case "whoami":
      return { tabId: sender.tab ? sender.tab.id : null };
    default:
      return { ok: false, error: "unknown message" };
  }
}

chrome.runtime.onMessage.addListener((msg, sender, reply) => {
  handle(msg, sender).then(reply, (e) => reply({ ok: false, error: String((e && e.message) || e) }));
  return true; // async reply
});

resume();
