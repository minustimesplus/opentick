// Reads a LinkedIn profile from the logged-in session. Loaded as a content
// script on linkedin.com (and by Node for tests).
//
// Strategies, in order:
//   1. LinkedIn's own JSON API (Voyager) with several known response
//      shapes. It returns every position and school, including grouped roles.
//   2. The JSON blobs LinkedIn embeds in its HTML (<code> elements).
//   3. Text lines read from the rendered /details/experience/ and
//      /details/education/ pages. background.js navigates there when 1 and 2 fail.
//      The OpenTick server parses these lines.
(function (root) {
  "use strict";

  const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

  function str(v) {
    if (typeof v === "string") {
      const t = v.trim();
      return t ? t : null;
    }
    if (v && typeof v === "object" && typeof v.text === "string") return str(v.text);
    return null;
  }

  function vanityFromUrl(url) {
    const m = String(url || "").match(/linkedin\.com\/in\/([^/?#]+)/i);
    return m ? decodeURIComponent(m[1]) : null;
  }

  function fmtDate(d) {
    if (!d || typeof d !== "object" || !d.year) return null;
    return d.month ? `${MONTHS[d.month - 1] || d.month} ${d.year}` : String(d.year);
  }

  function range(e) {
    const r = e.dateRange || e.timePeriod || {};
    const start = r.start || r.startDate;
    const end = r.end || r.endDate;
    return { start: fmtDate(start), end: end ? fmtDate(end) : start ? "Present" : null, startKey: start ? start.year * 12 + (start.month || 0) : 0, current: !end };
  }

  function typeOf(o) {
    return String(o.$type || o["$type"] || "");
  }

  function endsWithType(o, name) {
    const t = typeOf(o);
    return t.endsWith("." + name);
  }

  function pictureUrl(p) {
    const vec =
      (p.profilePicture && p.profilePicture.displayImageReference && p.profilePicture.displayImageReference.vectorImage) ||
      (p.profilePicture && p.profilePicture.displayImageReferenceResolutionResult && p.profilePicture.displayImageReferenceResolutionResult.vectorImage) ||
      (p.picture && (p.picture["com.linkedin.common.VectorImage"] || p.picture)) ||
      null;
    if (!vec || !vec.rootUrl || !Array.isArray(vec.artifacts) || !vec.artifacts.length) return null;
    const best = vec.artifacts.slice().sort((a, b) => (b.width || 0) - (a.width || 0))[0];
    return vec.rootUrl + (best.fileIdentifyingUrlPathSegment || "");
  }

  // Collect entities from any Voyager payload shape: {included:[...]},
  // {data:{...}, included:[...]}, or nested *elements arrays.
  function entities(json) {
    const out = [];
    const seen = new Set();
    const visit = (v, depth) => {
      if (!v || typeof v !== "object" || depth > 12) return;
      if (Array.isArray(v)) {
        v.forEach((x) => visit(x, depth + 1));
        return;
      }
      if (typeOf(v)) {
        const key = v.entityUrn || v.$id || JSON.stringify(v).slice(0, 200);
        if (!seen.has(key)) {
          seen.add(key);
          out.push(v);
        }
      }
      for (const k of Object.keys(v)) {
        if (k === "$type") continue;
        visit(v[k], depth + 1);
      }
    };
    visit(json, 0);
    return out;
  }

  // Turns Voyager entities into the OpenTick payload, or null when the
  // payload has no profile in it.
  function parseVoyager(json, vanity) {
    const ents = entities(json);
    const profiles = ents.filter((e) => endsWithType(e, "Profile") && (e.firstName || e.lastName));
    const want = String(vanity || "").toLowerCase();
    const profile = profiles.find((p) => String(p.publicIdentifier || "").toLowerCase() === want) || (profiles.length === 1 ? profiles[0] : null);
    if (!profile) return null;

    const posUrns = new Set();
    const positions = ents
      .filter((e) => endsWithType(e, "Position") && (e.title || e.companyName))
      .filter((e) => {
        const k = e.entityUrn || `${e.title}|${e.companyName}|${JSON.stringify(e.dateRange || e.timePeriod || {})}`;
        if (posUrns.has(k)) return false;
        posUrns.add(k);
        return true;
      })
      .map((e) => {
        const r = range(e);
        return {
          title: str(e.title),
          company: str(e.companyName) || str(e.company && e.company.name),
          location: str(e.locationName) || str(e.geoLocationName),
          start_date: r.start,
          end_date: r.end,
          description: str(e.description),
          _k: r.startKey,
          _c: r.current,
        };
      })
      .sort((a, b) => (b._c - a._c) || (b._k - a._k));

    const eduUrns = new Set();
    const educations = ents
      .filter((e) => endsWithType(e, "Education") && (e.schoolName || e.school))
      .filter((e) => {
        const k = e.entityUrn || `${e.schoolName}|${e.degreeName}`;
        if (eduUrns.has(k)) return false;
        eduUrns.add(k);
        return true;
      })
      .map((e) => {
        const r = e.dateRange || e.timePeriod || {};
        const s = r.start || r.startDate;
        const en = r.end || r.endDate;
        return {
          school: str(e.schoolName) || str(e.school && e.school.name),
          degree: str(e.degreeName),
          field_of_study: str(e.fieldOfStudy),
          start_year: s && s.year ? String(s.year) : null,
          end_year: en && en.year ? String(en.year) : null,
          _k: s && s.year ? s.year : 0,
        };
      })
      .sort((a, b) => b._k - a._k);

    positions.forEach((p) => { delete p._k; delete p._c; });
    educations.forEach((e) => { delete e._k; });

    return {
      profile: {
        first_name: str(profile.firstName),
        last_name: str(profile.lastName),
        headline: str(profile.headline) || str(profile.occupation),
        location: str(profile.geoLocationName) || str(profile.locationName) || str(profile.geoLocation && profile.geoLocation.geo && profile.geoLocation.geo.defaultLocalizedName),
        about: str(profile.summary),
        avatar_url: pictureUrl(profile),
      },
      public_identifier: str(profile.publicIdentifier),
      experiences: positions,
      educations,
    };
  }

  /* ---------------- Rendered page (DOM) ---------------- */

  function spanLines(el, skip) {
    const lines = [];
    const spans = el.querySelectorAll('span[aria-hidden="true"]');
    spans.forEach((s) => {
      if (skip && skip.some((x) => x !== el && x.contains(s))) return;
      const t = (s.innerText || s.textContent || "").trim();
      if (t) t.split(/\n+/).forEach((x) => x.trim() && lines.push(x.trim()));
    });
    if (lines.length) return lines;
    return (el.innerText || "").split(/\n+/).map((x) => x.trim()).filter(Boolean);
  }

  // Items of a /details/experience/ or /details/education/ page, as
  // [{lines, sub}], where `sub` holds grouped roles at one company.
  function domItems(doc) {
    const main = doc.querySelector("main") || doc.body;
    let items = Array.from(main.querySelectorAll('[data-view-name="profile-component-entity"]'));
    // Keep only top-level entities; nested ones are sub-positions.
    items = items.filter((el) => !items.some((o) => o !== el && o.contains(el)));
    if (!items.length) {
      items = Array.from(main.querySelectorAll("li.pvs-list__paged-list-item, li.artdeco-list__item")).filter(
        (el, _, all) => !all.some((o) => o !== el && o.contains(el)),
      );
    }
    return items
      .map((el) => {
        let subs = Array.from(el.querySelectorAll('[data-view-name="profile-component-entity"]')).filter((s) => s !== el);
        if (!subs.length) {
          const box = el.querySelector(".pvs-entity__sub-components");
          if (box) subs = Array.from(box.querySelectorAll(":scope li")).filter((li) => li.querySelector("span[aria-hidden]") && /\d{4}/.test(li.innerText || ""));
        }
        subs = subs.filter((s, _, all) => !all.some((o) => o !== s && o.contains(s)));
        return { lines: spanLines(el, subs), sub: subs.map((s) => spanLines(s)) };
      })
      .filter((i) => i.lines.length);
  }

  /* ---------------- Network ---------------- */

  const DECORATIONS = [
    "com.linkedin.voyager.dash.deco.identity.profile.FullProfileWithEntities-93",
    "com.linkedin.voyager.dash.deco.identity.profile.FullProfileWithEntities-91",
    "com.linkedin.voyager.dash.deco.identity.profile.FullProfileWithEntities-85",
    "com.linkedin.voyager.dash.deco.identity.profile.FullProfileWithEntities-76",
  ];

  class HttpError extends Error {
    constructor(status, message, kind) {
      super(message || `HTTP ${status}`);
      this.status = status;
      this.kind = kind || null;
    }
  }

  // Why a LinkedIn URL means we must stop: "checkpoint" for security checks
  // and captchas, "logged_out" for the login wall, null for a normal page.
  // These are never bypassed; the user has to deal with them.
  function blockReason(url) {
    let u;
    try {
      u = new URL(String(url || ""));
    } catch (_) {
      return null;
    }
    if (!/(^|\.)linkedin\.com$/i.test(u.hostname)) return null;
    const p = u.pathname.toLowerCase();
    if (/^\/(checkpoint|captcha)(\/|$)/.test(p) || p.includes("/challenge") || p.startsWith("/uas/consumer-captcha")) return "checkpoint";
    if (/^\/(login|uas\/login|authwall|signup)(\/|$)/.test(p) || p.startsWith("/uas/login")) return "logged_out";
    return null;
  }

  // What a failed request means for a batch: "logged_out", "rate_limited",
  // "checkpoint", "not_found" or null for a failure worth retrying later.
  function errorKind(e) {
    if (!e) return null;
    if (e.kind) return e.kind;
    if (e.status === 401 || e.status === 403) return "logged_out";
    if (e.status === 429 || e.status === 999) return "rate_limited";
    if (e.status === 404) return "not_found";
    return null;
  }

  function checkRedirect(res) {
    if (!res.redirected) return;
    const reason = blockReason(res.url);
    if (reason) throw new HttpError(403, reason === "checkpoint" ? "LinkedIn wants a security check" : "Not logged in to LinkedIn", reason);
  }

  function csrf() {
    const m = document.cookie.match(/JSESSIONID="?([^";]+)"?/);
    return m ? m[1] : null;
  }

  async function voyager(path) {
    const token = csrf();
    if (!token) throw new HttpError(401, "Not logged in to LinkedIn", "logged_out");
    const res = await fetch(path, {
      credentials: "include",
      headers: {
        "csrf-token": token,
        "x-restli-protocol-version": "2.0.0",
        accept: "application/vnd.linkedin.normalized+json+2.1",
        "x-li-lang": "en_US",
      },
    });
    checkRedirect(res);
    if (!res.ok) throw new HttpError(res.status);
    return res.json();
  }

  function fatal(e) {
    // 401/403: logged out or blocked; 429/999: rate limited. Stop, don't retry.
    return !!e && (!!e.kind || [401, 403, 429, 999].includes(e.status));
  }

  async function fetchViaApi(vanity) {
    const errors = [];
    const v = encodeURIComponent(vanity);
    for (const deco of DECORATIONS) {
      try {
        const json = await voyager(`/voyager/api/identity/dash/profiles?q=memberIdentity&memberIdentity=${v}&decorationId=${deco}`);
        const parsed = parseVoyager(json, vanity);
        if (parsed) return { ...parsed, method: "voyager-dash" };
      } catch (e) {
        if (fatal(e)) throw e;
        if (e.status === 404) throw new HttpError(404, "Profile not found");
        errors.push(e.message);
      }
    }
    try {
      const json = await voyager(`/voyager/api/identity/profiles/${v}/profileView`);
      const parsed = parseVoyager(json, vanity);
      if (parsed) return { ...parsed, method: "voyager-profileView" };
    } catch (e) {
      if (fatal(e)) throw e;
      errors.push(e.message);
    }
    return { error: errors.join("; ") || "no profile in API response" };
  }

  async function fetchViaHtml(vanity) {
    const merged = { included: [] };
    for (const path of [`/in/${vanity}/`, `/in/${vanity}/details/experience/`, `/in/${vanity}/details/education/`]) {
      try {
        const res = await fetch(path, { credentials: "include" });
        checkRedirect(res);
        if (res.status === 404) throw new HttpError(404, "Profile not found");
        if (!res.ok) {
          if ([429, 999].includes(res.status)) throw new HttpError(res.status);
          continue;
        }
        const html = await res.text();
        const doc = new DOMParser().parseFromString(html, "text/html");
        doc.querySelectorAll("code").forEach((c) => {
          try {
            const j = JSON.parse(c.textContent);
            merged.included.push(j);
          } catch (_) {
            /* not JSON */
          }
        });
      } catch (e) {
        if (fatal(e) || e.status === 404) throw e;
      }
    }
    const parsed = parseVoyager(merged, vanity);
    return parsed ? { ...parsed, method: "embedded-json" } : null;
  }

  // Full profile from the network. Throws HttpError on fatal/404; returns
  // {error} when every network strategy failed (caller can try the DOM).
  async function fetchProfile(vanity) {
    const api = await fetchViaApi(vanity);
    if (!api.error && (api.experiences.length || api.educations.length)) return api;
    const html = await fetchViaHtml(vanity);
    if (html && (html.experiences.length || html.educations.length)) return html;
    if (!api.error) return api; // genuinely empty profile
    return html || api;
  }

  // Items of the rendered details page, scrolling to load lazy ones. Empty
  // when nothing appears within timeoutMs.
  async function waitForDomItems(timeoutMs) {
    const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
    const until = Date.now() + timeoutMs;
    while (Date.now() < until) {
      window.scrollTo(0, document.body.scrollHeight);
      if (domItems(document).length) {
        await sleep(1200);
        return domItems(document);
      }
      await sleep(700);
    }
    return [];
  }

  // Runs in the page for background.js (chrome.scripting): reads a profile
  // and returns a structured-clone-safe result instead of throwing.
  async function readProfile(vanity) {
    const blocked = blockReason(location.href);
    if (blocked) return { ok: false, kind: blocked, error: "LinkedIn shows a " + (blocked === "checkpoint" ? "security check" : "login page") };
    try {
      const data = await fetchProfile(vanity);
      if (data.error) return { ok: false, kind: "unreadable", error: data.error };
      return { ok: true, data };
    } catch (e) {
      return { ok: false, kind: errorKind(e), status: e.status || 0, error: e.message || String(e) };
    }
  }

  const api = { parseVoyager, vanityFromUrl, domItems, waitForDomItems, fetchProfile, readProfile, HttpError, fatal, blockReason, errorKind };
  root.OpenTickExtract = api;
  if (typeof module !== "undefined") module.exports = api;
})(typeof globalThis !== "undefined" ? globalThis : this);
