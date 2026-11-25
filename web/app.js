const queueEl = document.getElementById("queue");
const metaEl = document.getElementById("meta");
const statusEl = document.getElementById("status");
const exportSoldEl = document.getElementById("export-sold");
const tabButtons = document.querySelectorAll(".tab");
const rewriteDialog = document.getElementById("rewrite-dialog");
const rewriteOriginalEl = document.getElementById("rewrite-original");
const rewriteDraftEl = document.getElementById("rewrite-draft");
const rewriteMetaEl = document.getElementById("rewrite-meta");
const rewriteCopyBtn = document.getElementById("rewrite-copy");
const rewriteApproveBtn = document.getElementById("rewrite-approve");
const rewriteCloseBtn = document.getElementById("rewrite-close");

let catalog = null;
const initialTab = new URLSearchParams(location.search).get("tab");
let activeTab =
  initialTab === "stale" || initialTab === "sold" || initialTab === "active"
    ? initialTab
    : "active";
let rewriteState = null;

function showStatus(message, kind) {
  statusEl.hidden = !message;
  statusEl.textContent = message || "";
  statusEl.dataset.kind = kind || "";
}

function truncate(text, max) {
  const t = (text || "").trim();
  if (t.length <= max) return t;
  return t.slice(0, max - 1).trimEnd() + "…";
}

function formatSyncedAt(iso) {
  if (!iso) return null;
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  });
}

function formatListedSince(iso) {
  if (!iso) return null;
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleDateString(undefined, { dateStyle: "medium" });
}

function firstListedMs(p) {
  const t = Date.parse(p.first_listed_at || p.created_at || "");
  return Number.isNaN(t) ? 0 : t;
}

function soldMs(p) {
  const t = Date.parse(p.sold_at || p.created_at || "");
  return Number.isNaN(t) ? 0 : t;
}

async function copyText(label, text) {
  try {
    await navigator.clipboard.writeText(text);
    showStatus(`Copied ${label}`, "ok");
  } catch (err) {
    showStatus(`Copy failed: ${err.message}`, "error");
  }
}

function renderRow(product, index) {
  const li = document.createElement("li");
  li.className = "row";
  li.style.animationDelay = `${0.05 + index * 0.045}s`;

  if (product.cover_url) {
    const thumb = document.createElement("img");
    thumb.className = "thumb";
    thumb.src = product.cover_url;
    thumb.alt = "";
    thumb.loading = "lazy";
    thumb.decoding = "async";
    thumb.referrerPolicy = "no-referrer";
    thumb.addEventListener("error", () => {
      thumb.remove();
      li.classList.remove("has-thumb");
    });
    li.classList.add("has-thumb");
    li.append(thumb);
  }

  const body = document.createElement("div");
  body.className = "row-body";

  const top = document.createElement("div");
  top.className = "row-top";

  const status = document.createElement("span");
  status.className = "badge";
  status.dataset.status = product.status || "active";
  status.textContent = product.status || "active";

  top.append(status);

  const refreshCount = product.refresh_count || 0;
  if (refreshCount > 0) {
    const refresh = document.createElement("span");
    refresh.className = "badge refresh";
    refresh.textContent =
      refreshCount === 1 ? "1 refresh" : `${refreshCount} refreshes`;
    top.append(refresh);
  }

  if (product.pending_description) {
    const pending = document.createElement("span");
    pending.className = "badge pending";
    pending.textContent = "pending rewrite";
    top.append(pending);
  }

  const age = document.createElement("span");
  age.className = "age";
  if (activeTab === "sold") {
    age.textContent = product.sold_at
      ? `sold ${formatListedSince(product.sold_at)}`
      : `${product.age_days} days old`;
  } else {
    age.textContent = `${product.first_listed_age_days ?? product.age_days} days in system`;
  }

  const price = document.createElement("span");
  price.className = "price";
  price.textContent = product.price_label;

  top.append(age, price);
  body.append(top);

  if (
    product.first_listed_at &&
    product.created_at &&
    product.first_listed_at !== product.created_at
  ) {
    const listed = document.createElement("p");
    listed.className = "listed-since";
    const since = formatListedSince(product.first_listed_at);
    listed.textContent = since
      ? `First listed ${since} (${product.first_listed_age_days}d)`
      : "First listed unknown";
    body.append(listed);
  }

  const desc = document.createElement("p");
  desc.className = "desc";
  desc.textContent = truncate(product.description, 120);

  const actions = document.createElement("div");
  actions.className = "actions";

  const open = document.createElement("a");
  open.className = "primary";
  open.href = product.listing_url;
  open.target = "_blank";
  open.rel = "noopener noreferrer";
  open.textContent = "Open listing";

  const rewrite = document.createElement("button");
  rewrite.type = "button";
  rewrite.textContent = "Rewrite";
  rewrite.addEventListener("click", () => openRewrite(product));

  const copyDesc = document.createElement("button");
  copyDesc.type = "button";
  copyDesc.textContent = "Copy desc";
  copyDesc.addEventListener("click", () =>
    copyText("description", product.description)
  );

  const copyPrice = document.createElement("button");
  copyPrice.type = "button";
  copyPrice.textContent = "Copy price";
  copyPrice.addEventListener("click", () =>
    copyText("price", product.price_amount)
  );

  const refreshLink = document.createElement("a");
  refreshLink.href = `/refresh.html?item_id=${encodeURIComponent(product.item_id)}`;
  refreshLink.textContent = "Refresh";
  if (product.status && product.status !== "active") {
    refreshLink.hidden = true;
  }

  actions.append(open, rewrite, refreshLink, copyDesc, copyPrice);
  body.append(desc, actions);
  li.append(body);
  return li;
}

function visibleProducts() {
  const all = catalog?.products || [];
  let list;
  if (activeTab === "sold") {
    list = all.filter((p) => p.status === "sold");
    list.sort((a, b) => soldMs(b) - soldMs(a) || b.id - a.id);
    return list;
  }
  if (activeTab === "stale") {
    const days = catalog?.stale_days ?? 60;
    list = all.filter(
      (p) => (p.status === "active" || !p.status) && p.age_days >= days
    );
  } else {
    list = all.filter((p) => p.status === "active" || !p.status);
  }
  list.sort((a, b) => firstListedMs(a) - firstListedMs(b) || a.id - b.id);
  return list;
}

function renderMeta() {
  if (!catalog) {
    metaEl.textContent = "Loading…";
    return;
  }
  if (!catalog.synced_at) {
    metaEl.textContent = `@${catalog.shop_username} · never synced`;
    return;
  }
  const when = formatSyncedAt(catalog.synced_at);
  const n = catalog.product_count;
  const sold = (catalog.products || []).filter((p) => p.status === "sold").length;
  const soldBit = sold ? ` · ${sold} sold` : "";
  metaEl.textContent = `@${catalog.shop_username} · last synced ${when} · ${n} active${soldBit}`;
}

function renderEmpty(neverSynced, filteredEmpty) {
  const empty = document.createElement("li");
  empty.className = "empty";
  if (neverSynced) {
    empty.innerHTML =
      "No catalog yet. Open the <strong>Depop Manager Sync</strong> Chrome extension and click <strong>Sync</strong>.";
  } else if (filteredEmpty && activeTab === "stale") {
    empty.textContent = `No active listings ≥${catalog.stale_days} days old.`;
  } else if (filteredEmpty && activeTab === "sold") {
    empty.textContent =
      "No sold listings in catalog yet. Sync again (Sold hub) or check Sync soft-fail note.";
  } else {
    empty.textContent = "No listings in this view.";
  }
  queueEl.append(empty);
}

function renderQueue() {
  queueEl.replaceChildren();
  renderMeta();

  if (!catalog) return;

  if (!catalog.synced_at) {
    renderEmpty(true, false);
    return;
  }

  const products = visibleProducts();
  if (products.length === 0) {
    renderEmpty(false, true);
    return;
  }

  products.forEach((product, index) => {
    queueEl.append(renderRow(product, index));
  });
}

function setTab(tab) {
  activeTab = tab;
  tabButtons.forEach((btn) => {
    const selected = btn.dataset.tab === tab;
    btn.setAttribute("aria-selected", selected ? "true" : "false");
  });
  renderQueue();
}

tabButtons.forEach((btn) => {
  btn.addEventListener("click", () => setTab(btn.dataset.tab));
  btn.setAttribute(
    "aria-selected",
    btn.dataset.tab === activeTab ? "true" : "false"
  );
});

exportSoldEl.addEventListener("click", (e) => {
  if (catalog?.synced_at) return;
  e.preventDefault();
  showStatus("Sync catalog first, then export sold CSV.", "error");
});

function setRewriteBusy(busy) {
  rewriteCopyBtn.disabled = busy || !rewriteState?.draft;
  rewriteApproveBtn.disabled = busy || !rewriteState?.draft;
  rewriteCloseBtn.disabled = busy;
}

async function openRewrite(product) {
  rewriteState = {
    item_id: product.item_id,
    product_id: product.id,
    original: product.description || "",
    draft: "",
    provider: "",
  };
  rewriteOriginalEl.textContent = rewriteState.original || "(empty)";
  rewriteDraftEl.textContent = "Generating…";
  rewriteMetaEl.textContent = `Item ${product.item_id}`;
  setRewriteBusy(true);
  rewriteDialog.showModal();
  showStatus("");

  try {
    const res = await fetch("/api/rewrite", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ item_id: product.item_id }),
    });
    const text = await res.text();
    if (!res.ok) {
      throw new Error(text || `${res.status}`);
    }
    const data = JSON.parse(text);
    rewriteState.original = data.original || rewriteState.original;
    rewriteState.draft = data.draft || "";
    rewriteState.provider = data.provider || "";
    rewriteOriginalEl.textContent = rewriteState.original;
    rewriteDraftEl.textContent = rewriteState.draft;
    rewriteMetaEl.textContent = `Provider: ${rewriteState.provider} · local only until you Copy or Approve`;
    setRewriteBusy(false);
  } catch (err) {
    rewriteDraftEl.textContent = "";
    rewriteMetaEl.textContent = "Rewrite failed";
    setRewriteBusy(true);
    rewriteCopyBtn.disabled = true;
    rewriteApproveBtn.disabled = true;
    rewriteCloseBtn.disabled = false;
    showStatus(`Rewrite failed: ${err.message}`, "error");
  }
}

rewriteCopyBtn.addEventListener("click", async () => {
  if (!rewriteState?.draft) return;
  await copyText("rewrite draft", rewriteState.draft);
});

rewriteApproveBtn.addEventListener("click", async () => {
  if (!rewriteState?.draft || !rewriteState?.item_id) return;
  setRewriteBusy(true);
  try {
    const res = await fetch("/api/rewrite/approve", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        item_id: rewriteState.item_id,
        draft: rewriteState.draft,
      }),
    });
    const text = await res.text();
    if (!res.ok) {
      throw new Error(text || `${res.status}`);
    }
    showStatus(
      "Approved — saved as pending_description locally (not posted to Depop)",
      "ok"
    );
    rewriteDialog.close();
    await loadCatalog();
  } catch (err) {
    showStatus(`Approve failed: ${err.message}`, "error");
    setRewriteBusy(false);
  }
});

rewriteCloseBtn.addEventListener("click", () => {
  rewriteDialog.close();
});

rewriteDialog.addEventListener("close", () => {
  rewriteState = null;
  rewriteDraftEl.textContent = "";
  rewriteOriginalEl.textContent = "";
  rewriteMetaEl.textContent = "";
});

async function loadCatalog() {
  showStatus("");
  metaEl.textContent = "Loading…";
  queueEl.replaceChildren();

  try {
    const res = await fetch("/api/catalog");
    if (!res.ok) {
      throw new Error(`${res.status} ${await res.text()}`);
    }
    catalog = await res.json();
  } catch (err) {
    catalog = null;
    metaEl.textContent = "Could not load catalog";
    showStatus(err.message, "error");
    return;
  }

  renderQueue();
}

loadCatalog();
