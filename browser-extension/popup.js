const $ = (id) => document.getElementById(id);
const send = (msg) => chrome.runtime.sendMessage(msg);

const STATUS_LABELS = { idle: "Idle", running: "Running", paused: "Paused", blocked: "Stopped", done: "Done" };

let activeTab = null;

function renderState(st) {
  $("setup").hidden = st.configured && st.hasPermission;
  $("problem").hidden = !st.serverProblem;
  $("problem").textContent = st.serverProblem ? st.serverProblem.message : "";

  const run = st.run;
  const status = $("status");
  status.textContent = run.status === "running" && run.waiting ? "Waiting" : STATUS_LABELS[run.status] || run.status;
  status.className = `pill ${run.status}`;
  $("runMsg").textContent = run.message || "";
  $("runMsg").className = run.status === "blocked" ? "err" : "muted";
  const left = st.queueInfo ? `${st.queueInfo.total} left in the queue · ` : "";
  const pending = st.outbox.length ? ` · ${st.outbox.length} waiting to sync` : "";
  $("counts").textContent = `${left}${st.daily}/${st.dailyCap} today${pending}`;
  $("start").textContent = run.status === "paused" || run.status === "blocked" ? "Resume" : "Start";
  $("start").disabled = run.status === "running" || !st.configured;
  $("pause").disabled = run.status !== "running";
}

async function refresh() {
  renderState(await send({ type: "state" }));
}

async function init() {
  [activeTab] = await chrome.tabs.query({ active: true, currentWindow: true });
  // The URL is only visible for linkedin.com tabs (our host permission).
  const url = (activeTab && activeTab.url) || "";
  const m = url.match(/^https:\/\/www\.linkedin\.com\/in\/([^/?#]+)/);
  if (m) {
    $("pageInfo").textContent = `Profile: ${decodeURIComponent(m[1])}`;
    $("save").disabled = false;
  }

  $("save").addEventListener("click", async () => {
    $("save").disabled = true;
    $("result").className = "muted";
    $("result").textContent = "Reading profile…";
    const r = await send({ type: "saveTab", tabId: activeTab.id });
    $("result").className = r.ok ? (r.queued ? "warn" : "ok") : "err";
    $("result").textContent = r.ok ? r.message : r.error;
    if (r.link) {
      const a = document.createElement("a");
      a.href = r.link;
      a.target = "_blank";
      a.textContent = "\nOpen in OpenTick";
      $("result").append(" ", a);
    }
    $("save").disabled = false;
    refresh();
  });
  $("start").addEventListener("click", async () => renderState(await send({ type: "start" })));
  $("pause").addEventListener("click", async () => renderState(await send({ type: "pause" })));
  $("dashboard").addEventListener("click", () => chrome.tabs.create({ url: chrome.runtime.getURL("dashboard.html") }));
  for (const id of ["options", "openOptions"]) {
    $(id).addEventListener("click", (e) => {
      e.preventDefault();
      chrome.runtime.openOptionsPage();
    });
  }
  await refresh();
  setInterval(refresh, 2000);
}

init();
