const C = globalThis.OpenTickCore;
const $ = (id) => document.getElementById(id);
const send = (msg) => chrome.runtime.sendMessage(msg);

const STATUS_LABELS = { idle: "Idle", running: "Running", paused: "Paused", blocked: "Stopped", done: "Done" };
const WAITING_LABELS = { active_hours: "Waiting for active hours", daily_cap: "Daily limit reached", server: "Waiting for OpenTick" };
const RESULT_LABELS = { saved: "Saved", created: "Added", failed: "Failed", unavailable: "Unavailable", skipped: "Skipped", blocked: "Stopped", info: "" };

let st = null;

function el(tag, props, ...children) {
  const e = document.createElement(tag);
  Object.assign(e, props || {});
  for (const c of children) if (c != null) e.append(c);
  return e;
}

function fmtTime(ms) {
  const d = new Date(ms);
  const sameDay = C.dayKey(d) === C.dayKey();
  return sameDay ? d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" }) : d.toLocaleString([], { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" });
}

function fmtCountdown(ms) {
  const s = Math.max(0, Math.round(ms / 1000));
  if (s < 90) return `${s}s`;
  const m = Math.round(s / 60);
  if (m < 90) return `${m} min`;
  return `${Math.floor(m / 60)} h ${m % 60} min`;
}

function profileCell(name, url, contactId) {
  const cell = el("td");
  const link = contactId && st ? C.contactLink(st.serverUrl, contactId) : null;
  cell.append(link ? el("a", { href: link, target: "_blank", textContent: name || "Contact" }) : el("span", { textContent: name || "" }));
  if (url) cell.append(el("div", {}, el("a", { href: url, target: "_blank", className: "muted", textContent: url.replace(/^https:\/\/www\./, "") })));
  return cell;
}

function renderOutbox() {
  const body = $("outbox");
  body.replaceChildren();
  $("outboxCount").textContent = st.outbox.length ? `(${st.outbox.length})` : "";
  $("retryAll").disabled = !st.outbox.length;
  if (!st.outbox.length) {
    body.append(el("tr", {}, el("td", { colSpan: 5, className: "empty", textContent: "Nothing waiting. Everything read is in OpenTick." })));
    return;
  }
  for (const e of st.outbox) {
    const state = e.state === "failed" ? el("span", { className: "err", textContent: `Rejected: ${e.error || ""}` }) : el("span", { className: e.error ? "warn" : "muted", textContent: e.error ? `Retrying ${e.nextAt ? "at " + fmtTime(e.nextAt) : "soon"}: ${e.error}` : "Sending…" });
    const retry = el("button", { textContent: "Retry" });
    retry.addEventListener("click", async () => render(await send({ type: "retry", id: e.id })));
    const discard = el("button", { textContent: "Discard" });
    discard.addEventListener("click", async () => {
      if (confirm(`Discard the capture of ${e.name}? It won't reach OpenTick.`)) render(await send({ type: "discard", id: e.id, name: e.name }));
    });
    body.append(
      el(
        "tr",
        {},
        profileCell(e.name, e.url),
        el("td", { textContent: e.kind === "profile" ? "Profile" : "Failure note" }),
        el("td", {}, state),
        el("td", { textContent: String(e.attempts) }),
        el("td", {}, el("div", { className: "row" }, retry, discard)),
      ),
    );
  }
}

function renderLog() {
  const body = $("log");
  body.replaceChildren();
  if (!st.log.length) {
    body.append(el("tr", {}, el("td", { colSpan: 4, className: "empty", textContent: "No activity yet." })));
    return;
  }
  for (const l of st.log.slice(0, 150)) {
    const origin = l.origin && l.origin !== "batch" ? ` (${l.origin})` : "";
    body.append(
      el(
        "tr",
        {},
        el("td", { className: "when", textContent: fmtTime(l.at) }),
        l.name || l.url ? profileCell(l.name, l.url, l.contactId) : el("td"),
        el("td", { className: `res ${l.result}`, textContent: (RESULT_LABELS[l.result] ?? l.result) + origin }),
        el("td", { textContent: l.detail || "" }),
      ),
    );
  }
}

function renderClock() {
  if (!st) return;
  const run = st.run;
  if (run.status === "running" && run.nextAt && !run.current) {
    $("next").textContent = fmtCountdown(run.nextAt - Date.now());
    $("nextAt").textContent = `at ${fmtTime(run.nextAt)}`;
  } else if (run.status === "running" && run.current) {
    $("next").textContent = "now";
    $("nextAt").textContent = "";
  } else {
    $("next").textContent = "–";
    $("nextAt").textContent = "";
  }
}

function render(next) {
  if (next) st = next;
  if (!st) return;
  const run = st.run;
  $("server").textContent = st.serverUrl ? `Syncing to ${st.serverUrl} · extension ${st.version}` : `Extension ${st.version}`;
  $("setup").hidden = st.configured && st.hasPermission;
  $("problem").hidden = !st.serverProblem;
  $("problem").textContent = st.serverProblem ? `OpenTick: ${st.serverProblem.message}` : "";

  const status = $("status");
  status.textContent = run.status === "running" && run.waiting ? WAITING_LABELS[run.waiting] || "Waiting" : STATUS_LABELS[run.status] || run.status;
  status.className = `pill ${run.status}`;
  $("message").textContent = run.message || (run.status === "idle" ? "Start to fill in contacts that lack experience or education." : "");
  $("message").className = run.status === "blocked" ? "err" : "muted";
  $("current").textContent = run.current ? `Now: ${run.current.name || run.current.url}` : "";

  $("start").textContent = run.status === "paused" || run.status === "blocked" ? "Resume" : "Start";
  $("start").disabled = run.status === "running" || !st.configured;
  $("pause").disabled = run.status !== "running";
  $("runNow").disabled = !(run.status === "running" && !run.current);

  const q = st.queueInfo;
  $("queue").textContent = q ? String(q.total) : "–";
  $("queueAt").textContent = q ? `checked ${fmtTime(q.at)}` : "";
  $("today").textContent = `${st.daily} / ${st.dailyCap}`;
  $("todayBar").style.width = `${Math.min(100, (100 * st.daily) / st.dailyCap)}%`;
  $("run").textContent = `${run.stats?.saved || 0} · ${run.stats?.failed || 0}`;
  if (q && q.coverage) {
    const c = q.coverage;
    const full = c.with_url - c.missing_any;
    $("coverage").textContent = c.with_url ? `${Math.round((100 * full) / c.with_url)}%` : "–";
    $("coverageNote").textContent = `${full} of ${c.with_url} with a LinkedIn URL have full history`;
  }
  renderClock();
  renderOutbox();
  renderLog();
}

async function refresh() {
  render(await send({ type: "state" }));
}

$("start").addEventListener("click", async () => render(await send({ type: "start" })));
$("pause").addEventListener("click", async () => render(await send({ type: "pause" })));
$("runNow").addEventListener("click", async () => render(await send({ type: "runNow" })));
$("retryAll").addEventListener("click", async () => render(await send({ type: "retry" })));
$("clearLog").addEventListener("click", async () => render(await send({ type: "clearLog" })));

let pending = null;
chrome.storage.onChanged.addListener((changes, area) => {
  if (area !== "local") return;
  clearTimeout(pending);
  pending = setTimeout(refresh, 150);
});
setInterval(renderClock, 1000);
refresh();
