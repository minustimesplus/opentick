// Runs on linkedin.com. Adds the "Save to OpenTick" button to profiles,
// shows toasts, and tells the service worker when a profile is opened (for
// the optional auto-save of existing contacts). The reading itself is done
// by extract.js, which background.js runs in this tab.
(function () {
  "use strict";
  if (globalThis.__openTickContent) return;
  globalThis.__openTickContent = true;

  const X = globalThis.OpenTickExtract;
  let settings = {};

  function toast(text, kind) {
    let el = document.getElementById("opentick-toast");
    if (!el) {
      el = document.createElement("div");
      el.id = "opentick-toast";
      el.setAttribute("role", "status");
      el.style.cssText =
        "position:fixed;right:20px;bottom:76px;z-index:2147483647;max-width:340px;padding:10px 14px;border-radius:8px;" +
        "font:13px/1.4 system-ui,sans-serif;color:#fff;box-shadow:0 4px 16px rgba(0,0,0,.25);white-space:pre-line";
      document.documentElement.appendChild(el);
    }
    el.style.background = kind === "error" ? "#b42318" : kind === "busy" ? "#475467" : "#067647";
    el.textContent = text;
    clearTimeout(el._t);
    if (kind !== "busy") el._t = setTimeout(() => el.remove(), 7000);
  }

  // Top-level profile pages only, not /details/… or /recent-activity/.
  function isProfilePage() {
    return /^\/in\/[^/]+\/?$/.test(location.pathname);
  }

  function ensureButton() {
    const want = settings.showPageButton !== false && /^\/in\/[^/]+/.test(location.pathname);
    let btn = document.getElementById("opentick-save");
    if (!want) {
      if (btn) btn.remove();
      return;
    }
    if (btn) return;
    btn = document.createElement("button");
    btn.id = "opentick-save";
    btn.type = "button";
    btn.textContent = "Save to OpenTick";
    btn.style.cssText =
      "position:fixed;right:20px;bottom:24px;z-index:2147483647;padding:9px 16px;border:0;border-radius:20px;cursor:pointer;" +
      "background:#4772FA;color:#fff;font:600 14px system-ui,sans-serif;box-shadow:0 4px 14px rgba(0,0,0,.25)";
    btn.addEventListener("click", async () => {
      btn.disabled = true;
      toast("Reading profile…", "busy");
      try {
        const r = await chrome.runtime.sendMessage({ type: "saveTab" });
        toast(r.ok ? r.message : r.error, r.ok ? "ok" : "error");
      } catch (e) {
        toast("The extension was updated. Reload this page.", "error");
      } finally {
        btn.disabled = false;
      }
    });
    document.documentElement.appendChild(btn);
  }

  // LinkedIn is a single-page app: watch for profile changes.
  let lastProfile = null;
  let visitTimer = null;
  function watchNavigation() {
    ensureButton();
    const vanity = isProfilePage() ? X.vanityFromUrl(location.href) : null;
    if (vanity === lastProfile) return;
    lastProfile = vanity;
    clearTimeout(visitTimer);
    if (vanity && settings.autoSaveBrowsing) {
      // Wait until the user has actually stayed on the profile.
      visitTimer = setTimeout(() => {
        if (X.vanityFromUrl(location.href) === vanity) chrome.runtime.sendMessage({ type: "profileVisited", url: location.href }).catch(() => {});
      }, 4000);
    }
  }

  chrome.runtime.onMessage.addListener((msg) => {
    if (msg && msg.type === "toast") toast(msg.text, msg.kind);
  });

  chrome.storage.onChanged.addListener((changes, area) => {
    if (area === "local" && changes.settings) {
      settings = changes.settings.newValue || {};
      ensureButton();
    }
  });

  chrome.storage.local.get("settings").then(({ settings: s }) => {
    settings = s || {};
    watchNavigation();
    setInterval(watchNavigation, 1000);
  });
})();
