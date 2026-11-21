const C = globalThis.OpenTickCore;
const $ = (id) => document.getElementById(id);

const NUMBERS = ["minDelay", "maxDelay", "dailyCap"];
const TIMES = ["activeFrom", "activeTo", "autoStartTime"];
const CHECKS = ["activeHoursEnabled", "autoStartOnBrowser", "autoStartDaily", "createContacts", "autoSaveBrowsing", "showPageButton"];

async function load() {
  const { settings } = await chrome.storage.local.get("settings");
  return C.normalizeSettings(settings);
}

async function saveSettings(patch) {
  const next = C.normalizeSettings({ ...(await load()), ...patch });
  await chrome.storage.local.set({ settings: next });
  await chrome.runtime.sendMessage({ type: "settingsSaved" });
  return next;
}

function fill(s) {
  $("serverUrl").value = s.serverUrl;
  $("token").value = s.token;
  for (const k of [...NUMBERS, ...TIMES]) $(k).value = s[k];
  for (const k of CHECKS) $(k).checked = s[k];
}

function flash(text) {
  $("saved").textContent = text;
  clearTimeout(flash.t);
  flash.t = setTimeout(() => ($("saved").textContent = ""), 2500);
}

function status(text, kind) {
  $("conn").textContent = text;
  $("conn").className = kind || "muted";
}

async function connect() {
  const serverUrl = C.normalizeServerUrl($("serverUrl").value);
  const token = $("token").value.trim();
  if (!serverUrl) return status("Enter the address of your OpenTick server.", "err");
  if (!token) return status("Paste an API token from OpenTick.", "err");
  // Ask for access to just this server. Must run directly in the click handler.
  const origin = C.originPattern(serverUrl);
  let granted = false;
  try {
    granted = await chrome.permissions.request({ origins: [origin] });
  } catch (e) {
    return status(`Chrome refused access to ${origin}: ${e.message}`, "err");
  }
  if (!granted) return status("The extension needs access to your server to send profiles.", "err");
  await saveSettings({ serverUrl, token });
  $("serverUrl").value = serverUrl;
  status("Checking…");
  const r = await chrome.runtime.sendMessage({ type: "test" });
  if (r.ok) {
    const c = r.coverage || {};
    status(`Connected. ${r.total} contacts to fill · ${c.with_url ?? "?"} of ${c.total ?? "?"} contacts have a LinkedIn URL.`, "ok");
  } else {
    status(r.status === 401 ? "OpenTick rejected the token. Is it complete, unexpired, and created for the extension?" : r.error, "err");
  }
}

async function init() {
  const s = await load();
  fill(s);
  if (s.serverUrl && s.token) {
    const r = await chrome.runtime.sendMessage({ type: "test" });
    status(r.ok ? `Connected to ${s.serverUrl}.` : r.error, r.ok ? "ok" : "err");
  }
  $("connect").addEventListener("click", connect);
  for (const k of NUMBERS) {
    $(k).addEventListener("change", async () => {
      const next = await saveSettings({ [k]: Number($(k).value) });
      fill({ ...next, serverUrl: $("serverUrl").value, token: $("token").value });
      flash("Saved");
    });
  }
  for (const k of TIMES) {
    $(k).addEventListener("change", async () => {
      await saveSettings({ [k]: $(k).value });
      flash("Saved");
    });
  }
  for (const k of CHECKS) {
    $(k).addEventListener("change", async () => {
      await saveSettings({ [k]: $(k).checked });
      flash("Saved");
    });
  }
}

init();
