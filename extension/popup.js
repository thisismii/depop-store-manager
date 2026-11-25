const syncBtn = document.getElementById("sync");
const recordBtn = document.getElementById("record");
const saveRecordBtn = document.getElementById("save-record");
const openRefreshBtn = document.getElementById("open-refresh");
const openStaleLink = document.getElementById("open-stale");
const staleSummaryEl = document.getElementById("stale-summary");
const statusEl = document.getElementById("status");
const versionEl = document.getElementById("version");

const LOCAL_ORIGIN = "http://127.0.0.1:8787";
const REFRESH_API = `${LOCAL_ORIGIN}/api/refresh`;
const REFRESH_URL = `${LOCAL_ORIGIN}/refresh.html`;
const STALE_TAB_URL = `${LOCAL_ORIGIN}/?tab=stale`;

function setStatus(message, kind) {
  statusEl.textContent = message || "";
  statusEl.dataset.kind = kind || "";
}

function shortError(text) {
  const raw = String(text || "unknown error");
  const noHtml = raw.replace(/<[^>]+>/g, " ").replace(/\s+/g, " ").trim();
  if (noHtml.length > 220) return noHtml.slice(0, 217) + "…";
  return noHtml;
}

async function showVersion() {
  try {
    const man = chrome.runtime.getManifest();
    versionEl.textContent = `Extension v${man.version} — Record Post is a capture test, not auto-repost`;
  } catch {
    versionEl.textContent = "Extension version unknown";
  }
}

async function openLocalUrl(url) {
  const tabs = await chrome.tabs.query({ url: `${LOCAL_ORIGIN}/*` });
  if (tabs.length > 0) {
    const tab = tabs[0];
    await chrome.tabs.update(tab.id, { active: true, url });
    if (tab.windowId != null) {
      await chrome.windows.update(tab.windowId, { focused: true });
    }
    return;
  }
  await chrome.tabs.create({ url, active: true });
}

function renderStaleSummary(data) {
  const count = typeof data?.count === "number" ? data.count : 0;
  const days = typeof data?.stale_days === "number" ? data.stale_days : 60;
  if (count === 0) {
    staleSummaryEl.textContent = `0 stale (≥${days}d) — Sync first if empty`;
    return;
  }
  staleSummaryEl.textContent = `${count} stale (≥${days}d)`;
}

async function loadStaleSummary() {
  staleSummaryEl.textContent = "Checking stale…";
  openRefreshBtn.disabled = true;
  try {
    const res = await fetch(REFRESH_API);
    if (!res.ok) {
      throw new Error(`${res.status}: ${await res.text()}`);
    }
    const data = await res.json();
    renderStaleSummary(data);
    openRefreshBtn.disabled = false;
  } catch (err) {
    staleSummaryEl.textContent = "Server offline — start ./start on :8787";
    openRefreshBtn.disabled = false;
    console.warn("stale summary failed", err);
  }
}

async function openRefresh() {
  openRefreshBtn.disabled = true;
  try {
    await openLocalUrl(REFRESH_URL);
    setStatus("Opened Refresh queue", "ok");
  } catch (err) {
    setStatus(shortError(err.message || err), "error");
  }
  openRefreshBtn.disabled = false;
}

async function openStaleTab(event) {
  event.preventDefault();
  try {
    await openLocalUrl(STALE_TAB_URL);
    setStatus("Opened Stale tab", "ok");
  } catch (err) {
    setStatus(shortError(err.message || err), "error");
  }
}

async function runSync() {
  syncBtn.disabled = true;
  setStatus(
    "Syncing Active + Sold… scrolling Selling Hub. Wait for dashboard."
  );

  let response;
  try {
    response = await chrome.runtime.sendMessage({ type: "depop-manager-sync" });
  } catch (err) {
    // Focusing Selling Hub closes the popup; Sync still runs in the service worker.
    setStatus(
      "Popup closed while Sync runs — wait for dashboard, or reopen popup.\nhttp://127.0.0.1:8787",
      "ok"
    );
    syncBtn.disabled = false;
    return;
  }

  if (!response?.ok) {
    setStatus(
      `Sync failed: ${shortError(response?.error)}\nStay logged in on Depop. Server on :8787?`,
      "error"
    );
    syncBtn.disabled = false;
    return;
  }

  const n = response.product_count;
  const covers = response.cover_count;
  const coverBit =
    typeof covers === "number" ? ` · ${covers} covers` : "";
  const extra = response.partial
    ? ` (partial — ${shortError(response.partial_error)})`
    : "";
  setStatus(
    `Synced ${n} active${coverBit}${extra}. Opening dashboard…`,
    "ok"
  );
  syncBtn.disabled = false;
  await loadStaleSummary();
}

async function startRecord() {
  recordBtn.disabled = true;
  setStatus("Opening Depop create. Post one listing, then click Save capture.");
  try {
    const response = await chrome.runtime.sendMessage({
      type: "depop-manager-record-start",
    });
    if (!response?.ok) {
      throw new Error(response?.error || "record start failed");
    }
    setStatus(
      "Create tab is open. Post one listing (same price is fine). Then reopen this popup and click Save capture.",
      "ok"
    );
  } catch (err) {
    setStatus(shortError(err.message || err), "error");
  }
  recordBtn.disabled = false;
}

async function saveRecord() {
  saveRecordBtn.disabled = true;
  setStatus("Saving captured writes…");
  try {
    const response = await chrome.runtime.sendMessage({
      type: "depop-manager-record-save",
    });
    if (!response?.ok) {
      throw new Error(response?.error || "save failed");
    }
    const lines = (response.lines || []).slice(0, 12).join("\n");
    setStatus(
      `Saved ${response.event_count} calls → ${response.path}\n${lines}`,
      "ok"
    );
  } catch (err) {
    setStatus(shortError(err.message || err), "error");
  }
  saveRecordBtn.disabled = false;
}

showVersion();
loadStaleSummary();
openRefreshBtn.addEventListener("click", openRefresh);
openStaleLink.addEventListener("click", openStaleTab);
syncBtn.addEventListener("click", runSync);
recordBtn.addEventListener("click", startRecord);
saveRecordBtn.addEventListener("click", saveRecord);
