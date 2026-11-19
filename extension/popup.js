const syncBtn = document.getElementById("sync");
const recordBtn = document.getElementById("record");
const saveRecordBtn = document.getElementById("save-record");
const statusEl = document.getElementById("status");
const versionEl = document.getElementById("version");

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
syncBtn.addEventListener("click", runSync);
recordBtn.addEventListener("click", startRecord);
saveRecordBtn.addEventListener("click", saveRecord);
