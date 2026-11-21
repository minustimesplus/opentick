// Pure helpers shared by the service worker and the extension pages (and
// loaded by Node for tests). No chrome.* calls in here.
(function (root) {
  "use strict";

  const DEFAULT_SETTINGS = {
    serverUrl: "",
    token: "",
    // Pacing for "Fill missing". The defaults are the ones that kept the old
    // Personal CRM extension clear of LinkedIn's limits.
    minDelay: 45,
    maxDelay: 90,
    dailyCap: 80,
    activeHoursEnabled: false,
    activeFrom: "08:00",
    activeTo: "22:00",
    // Start "Fill missing" when the browser starts, and/or every day at a time.
    autoStartOnBrowser: false,
    autoStartDaily: false,
    autoStartTime: "09:00",
    // Manual saves add people who are no contact yet.
    createContacts: true,
    // Save profiles of existing contacts while you browse LinkedIn.
    autoSaveBrowsing: false,
    // Show a "Save to OpenTick" button on LinkedIn profile pages.
    showPageButton: true,
  };

  const LIMITS = { minDelay: [30, 3600], maxDelay: [30, 7200], dailyCap: [1, 500] };

  function clampNumber(v, [lo, hi], fallback) {
    const n = Math.round(Number(v));
    if (!Number.isFinite(n)) return fallback;
    return Math.min(hi, Math.max(lo, n));
  }

  const HM = /^([01]?\d|2[0-3]):([0-5]\d)$/;

  // Settings with defaults filled in and numbers clamped. The minimum delay
  // stays at 30 s: chrome.alarms cannot fire sooner, and faster is unsafe.
  function normalizeSettings(s) {
    const out = { ...DEFAULT_SETTINGS, ...(s || {}) };
    for (const k of Object.keys(LIMITS)) out[k] = clampNumber(out[k], LIMITS[k], DEFAULT_SETTINGS[k]);
    if (out.maxDelay < out.minDelay) out.maxDelay = out.minDelay;
    for (const k of ["activeFrom", "activeTo", "autoStartTime"]) if (!HM.test(String(out[k]))) out[k] = DEFAULT_SETTINGS[k];
    for (const k of ["activeHoursEnabled", "autoStartOnBrowser", "autoStartDaily", "createContacts", "autoSaveBrowsing", "showPageButton"]) out[k] = !!out[k];
    out.serverUrl = normalizeServerUrl(out.serverUrl) || "";
    out.token = String(out.token || "").trim();
    return out;
  }

  // "https://tick.example.com/api/v2/" -> "https://tick.example.com". Null
  // when it isn't an http(s) URL.
  function normalizeServerUrl(raw) {
    let t = String(raw || "").trim();
    if (!t) return null;
    if (/^[a-z][a-z0-9+.-]*:\/\//i.test(t)) {
      if (!/^https?:\/\//i.test(t)) return null;
    } else {
      t = "https://" + t;
    }
    let u;
    try {
      u = new URL(t);
    } catch (_) {
      return null;
    }
    if (u.protocol !== "https:" && u.protocol !== "http:") return null;
    let path = u.pathname.replace(/\/+$/, "").replace(/\/api\/v[12]$/i, "");
    return u.origin + path;
  }

  // The host permission pattern for the server, e.g. "https://tick.example.com/*".
  function originPattern(serverUrl) {
    const n = normalizeServerUrl(serverUrl);
    return n ? new URL(n).origin + "/*" : null;
  }

  function apiUrl(serverUrl, path) {
    return normalizeServerUrl(serverUrl) + "/api/v2" + path;
  }

  function contactLink(serverUrl, contactId) {
    const n = normalizeServerUrl(serverUrl);
    return n && contactId ? `${n}/people/all/${contactId}` : null;
  }

  function pad(n) {
    return String(n).padStart(2, "0");
  }

  // Local calendar day, so the daily cap resets at local midnight.
  function dayKey(date) {
    const d = date || new Date();
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
  }

  function minutesOf(hm) {
    const m = HM.exec(String(hm));
    return m ? Number(m[1]) * 60 + Number(m[2]) : 0;
  }

  // Whether date falls inside the active hours. A window like 22:00–06:00
  // runs over midnight; from == to means all day.
  function inActiveHours(settings, date) {
    const s = normalizeSettings(settings);
    if (!s.activeHoursEnabled) return true;
    const from = minutesOf(s.activeFrom);
    const to = minutesOf(s.activeTo);
    if (from === to) return true;
    const d = date || new Date();
    const now = d.getHours() * 60 + d.getMinutes();
    return from < to ? now >= from && now < to : now >= from || now < to;
  }

  // The next moment at or after date when local time is hm.
  function nextTimeOfDay(hm, date) {
    const d = new Date((date || new Date()).getTime());
    const m = minutesOf(hm);
    const at = new Date(d.getFullYear(), d.getMonth(), d.getDate(), Math.floor(m / 60), m % 60, 0, 0);
    if (at.getTime() <= d.getTime()) at.setDate(at.getDate() + 1);
    return at;
  }

  // When the active hours open next (date itself when they are open).
  function nextActiveStart(settings, date) {
    const d = date || new Date();
    if (inActiveHours(settings, d)) return d;
    return nextTimeOfDay(normalizeSettings(settings).activeFrom, d);
  }

  // When a new day's quota is available: the first active moment of tomorrow.
  function nextDayStart(settings, date) {
    const d = date || new Date();
    const midnight = new Date(d.getFullYear(), d.getMonth(), d.getDate() + 1, 0, 0, 30, 0);
    return nextActiveStart(settings, midnight);
  }

  function randomDelayMs(settings, rnd) {
    const s = normalizeSettings(settings);
    const r = typeof rnd === "number" ? rnd : Math.random();
    return Math.round((s.minDelay + r * (s.maxDelay - s.minDelay)) * 1000);
  }

  // Outbox retry delay: 30 s, 1 min, 2 min … capped at one hour.
  function backoffMs(attempts) {
    return Math.min(60 * 60 * 1000, 30 * 1000 * Math.pow(2, Math.max(0, attempts - 1)));
  }

  function profileUrl(vanity) {
    return `https://www.linkedin.com/in/${vanity}`;
  }

  // The body of POST /contacts/linkedin/profiles for a structured capture.
  function profilePayload(url, data, opts) {
    return {
      linkedin_url: url,
      extension_version: (opts && opts.version) || "",
      method: data.method || "",
      create: !!(opts && opts.create),
      complete: true,
      profile: data.profile || {},
      experiences: data.experiences || [],
      educations: data.educations || [],
    };
  }

  // The body for lines read from the rendered details pages. Partial: it only
  // replaces lists that are poorer than what OpenTick already has.
  function domPayload(url, experienceItems, educationItems, opts) {
    return {
      linkedin_url: url,
      extension_version: (opts && opts.version) || "",
      method: "dom",
      create: !!(opts && opts.create),
      complete: false,
      experience_items: experienceItems || [],
      education_items: educationItems || [],
    };
  }

  function profileName(payload) {
    const p = (payload && payload.profile) || {};
    const name = [p.first_name, p.last_name].filter(Boolean).join(" ") || p.full_name;
    if (name) return name;
    const m = String((payload && payload.linkedin_url) || "").match(/\/in\/([^/?#]+)/);
    return m ? decodeURIComponent(m[1]) : "Profile";
  }

  // The person's name from a LinkedIn tab title such as "(3) Jane Doe | LinkedIn"
  // or "Experience | Jane Doe | LinkedIn". Used when only the rendered page
  // could be read, so a new contact isn't nameless.
  function nameFromTitle(title) {
    const SECTIONS = /^(experience|education|linkedin|skills|activity|licenses.*|projects)$/i;
    const parts = String(title || "")
      .replace(/^\(\d+\+?\)\s*/, "")
      .split("|")
      .map((p) => p.trim())
      .filter((p) => p && !SECTIONS.test(p));
    return parts[0] || "";
  }

  // One line describing what the server did with a saved profile.
  function describeOutcome(r) {
    if (!r) return "";
    const what = `${r.experiences} ${r.experiences === 1 ? "job" : "jobs"}, ${r.educations} ${r.educations === 1 ? "school" : "schools"}`;
    const changed = r.experiences_written || r.educations_written ? "updated" : "already up to date";
    return `${r.created ? "Added to OpenTick" : "Saved"}: ${what} (${changed})${r.photo_saved ? ", photo added" : ""}`;
  }

  const api = {
    DEFAULT_SETTINGS,
    normalizeSettings,
    normalizeServerUrl,
    originPattern,
    apiUrl,
    contactLink,
    dayKey,
    inActiveHours,
    nextActiveStart,
    nextDayStart,
    nextTimeOfDay,
    randomDelayMs,
    backoffMs,
    profileUrl,
    profilePayload,
    domPayload,
    profileName,
    describeOutcome,
    nameFromTitle,
  };
  root.OpenTickCore = api;
  if (typeof module !== "undefined") module.exports = api;
})(typeof globalThis !== "undefined" ? globalThis : this);
