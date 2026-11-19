const metaEl = document.getElementById("meta");
const statusEl = document.getElementById("status");
const viewEl = document.getElementById("view");
const params = new URLSearchParams(location.search);
const itemID = params.get("item_id") || "";
const presetPrice = params.get("price") || "";

function showStatus(message, kind) {
  statusEl.hidden = !message;
  statusEl.textContent = message || "";
  statusEl.dataset.kind = kind || "";
}

function money(amount, currency) {
  const cur = currency || "USD";
  return `${amount || "—"} ${cur}`;
}

async function copyText(label, text) {
  try {
    await navigator.clipboard.writeText(text);
    showStatus(`Copied ${label}`, "ok");
  } catch (err) {
    showStatus(`Copy failed: ${err.message}`, "error");
  }
}

function discountLocal(amount, pct) {
  const cents = Math.round(Number(String(amount).replace(",", "")) * 100);
  if (!Number.isFinite(cents)) return amount;
  const p = Number(pct);
  if (!p) return (cents / 100).toFixed(2);
  const next = Math.round((cents * (100 - p) + 50) / 100);
  return (next / 100).toFixed(2);
}

function renderQueue(data) {
  const items = data.items || [];
  metaEl.textContent = items.length
    ? `@${data.shop_username} · ${items.length} stale (≥${data.stale_days}d)`
    : `@${data.shop_username} · no stale listings`;

  if (!items.length) {
    viewEl.innerHTML =
      `<p class="empty-refresh">No active listings ≥${data.stale_days} days old. Sync first, then come back.</p>`;
    return;
  }

  const rows = items
    .map((item) => {
      const sugClass = item.suggested_percent ? "cut" : "keep";
      const sugLabel = item.suggested_percent
        ? `${item.suggested_amount} (−${item.suggested_percent}%)`
        : "same";
      const chosen = item.pending_price || item.price_amount;
      const thumb = item.cover_url
        ? `<img class="thumb" src="${escapeAttr(item.cover_url)}" alt="" loading="lazy" referrerpolicy="no-referrer" />`
        : "";
      const waiting = item.pending_refresh_slug
        ? `<div class="keep">waiting on ${escapeHtml(item.pending_refresh_slug)}</div>`
        : "";
      return `<tr data-item-id="${escapeAttr(item.item_id)}" data-old="${escapeAttr(item.price_amount)}" data-suggested="${escapeAttr(item.suggested_amount)}" data-suggested-pct="${item.suggested_percent}">
        <td>${thumb}</td>
        <td>${item.age_days}d${waiting}</td>
        <td>${escapeHtml(item.price_amount)}</td>
        <td class="${sugClass}">${escapeHtml(sugLabel)}</td>
        <td><input type="text" inputmode="decimal" class="chosen" value="${escapeAttr(chosen)}" aria-label="New listing price" /></td>
        <td><a class="open-pack" href="/refresh.html?item_id=${encodeURIComponent(item.item_id)}">Open pack</a></td>
      </tr>`;
    })
    .join("");

  viewEl.innerHTML = `
    <div class="blanket">
      <label for="blanket-pct">Blanket discount %</label>
      <input id="blanket-pct" type="number" min="0" max="90" step="1" value="" placeholder="0" />
      <button type="button" id="apply-blanket">Apply to all chosen</button>
      <button type="button" id="reset-keep">Reset to keep</button>
      <span class="keep">Default is keep old price. Signals are Depop’s 15% / 25% presets.</span>
    </div>
    <table class="price-table">
      <thead>
        <tr>
          <th></th>
          <th>Age</th>
          <th>Old</th>
          <th>Signal</th>
          <th>New listing</th>
          <th></th>
        </tr>
      </thead>
      <tbody>${rows}</tbody>
    </table>`;

  document.getElementById("apply-blanket").addEventListener("click", () => {
    const pct = Number(document.getElementById("blanket-pct").value);
    if (!Number.isFinite(pct) || pct < 0) {
      showStatus("Enter a discount percent", "error");
      return;
    }
    document.querySelectorAll(".price-table tbody tr").forEach((tr) => {
      const input = tr.querySelector(".chosen");
      input.value = discountLocal(tr.dataset.old, pct);
    });
    showStatus(`Applied ${pct}% off to chosen prices. Open a pack to copy and post.`, "ok");
  });
  document.getElementById("reset-keep").addEventListener("click", () => {
    document.querySelectorAll(".price-table tbody tr").forEach((tr) => {
      tr.querySelector(".chosen").value = tr.dataset.old;
    });
    showStatus("Chosen prices reset to keep.", "ok");
  });
  viewEl.querySelectorAll(".open-pack").forEach((a) => {
    a.addEventListener("click", (e) => {
      const tr = a.closest("tr");
      const chosen = tr.querySelector(".chosen")?.value?.trim();
      if (!chosen) return;
      e.preventDefault();
      const url = new URL(a.href, location.origin);
      url.searchParams.set("price", chosen);
      location.href = url.pathname + url.search;
    });
  });
}

function renderPack(item) {
  metaEl.textContent = `${item.age_days} days old · ${money(item.price_amount, item.currency_name)}`;
  const photos = item.photo_urls || [];
  const photoHtml = photos.length
    ? photos
        .map(
          (url, i) =>
            `<a href="${escapeAttr(url)}" target="_blank" rel="noopener"><img src="${escapeAttr(url)}" alt="Photo ${i + 1}" loading="lazy" referrerpolicy="no-referrer" /></a>`
        )
        .join("")
    : `<p class="reason">No photo URLs stored yet. Sync again with extension 0.7.0+ or copy photos from the old listing.</p>`;

  const chosen = presetPrice || item.pending_price || item.price_amount;
  const pendingBit = item.pending_refresh_slug
    ? `<p class="reason">Waiting for Sync to see <code>${escapeHtml(item.pending_refresh_slug)}</code>. Then delete the old listing.</p>`
    : "";

  viewEl.innerHTML = `
    <p class="eyebrow"><a href="/refresh.html">← All stale</a></p>
    <div class="pack">
      <section>
        <h2>Photos</h2>
        <div class="photos">${photoHtml}</div>
        <div class="pack-actions" style="margin-top:0.6rem">
          <button type="button" id="copy-photos" ${photos.length ? "" : "disabled"}>Copy photo URLs</button>
        </div>
      </section>
      <section>
        <h2>Description to post</h2>
        <pre class="copy-block" id="post-desc">${escapeHtml(item.post_description || "")}</pre>
        <div class="pack-actions">
          <button type="button" id="copy-desc">Copy description</button>
        </div>
      </section>
      <section class="price-card">
        <h2>Price for the new listing</h2>
        <p class="reason">${escapeHtml(item.suggested_reason || "")}</p>
        <div class="price-row">
          <span><strong>Old</strong>${escapeHtml(money(item.price_amount, item.currency_name))}</span>
          <span><strong>Signal</strong>${
            item.suggested_percent
              ? escapeHtml(`${item.suggested_amount} (−${item.suggested_percent}%)`)
              : "keep"
          }</span>
        </div>
        <label>
          You will post
          <input id="chosen-price" type="text" inputmode="decimal" value="${escapeAttr(chosen)}" />
        </label>
        <div class="pack-actions" style="margin-top:0.65rem">
          <button type="button" id="use-keep">Keep old</button>
          <button type="button" id="use-signal" ${item.suggested_percent ? "" : "disabled"}>Use signal</button>
          <button type="button" id="copy-price">Copy price</button>
        </div>
      </section>
      <section>
        <h2>Repost steps</h2>
        <ol class="steps">
          <li>
            Copy description and price. Open photos and save them if needed.
            <div class="step-actions">
              <a class="primary" href="${escapeAttr(item.create_url)}" target="_blank" rel="noopener">Open Depop create</a>
            </div>
          </li>
          <li>
            After the new listing is live, paste its URL. This does not post or delete anything.
            <input id="new-url" type="url" placeholder="https://www.depop.com/products/…" value="${
              item.pending_refresh_slug
                ? escapeAttr("https://www.depop.com/products/" + item.pending_refresh_slug + "/")
                : ""
            }" />
            <div class="step-actions">
              <button type="button" class="primary" id="save-plan">Save plan</button>
            </div>
            ${pendingBit}
          </li>
          <li>
            Delete the old listing on Depop after the new one is up.
            <div class="step-actions">
              <a href="${escapeAttr(item.old_listing_url)}" target="_blank" rel="noopener">Open old listing</a>
            </div>
          </li>
          <li>Sync. Helper retargets this item to the new Depop id and keeps first-listed date.</li>
        </ol>
      </section>
    </div>`;

  const chosenEl = document.getElementById("chosen-price");
  document.getElementById("copy-desc").addEventListener("click", () =>
    copyText("description", item.post_description || "")
  );
  document.getElementById("copy-price").addEventListener("click", () =>
    copyText("price", chosenEl.value.trim())
  );
  document.getElementById("use-keep").addEventListener("click", () => {
    chosenEl.value = item.price_amount;
  });
  document.getElementById("use-signal").addEventListener("click", () => {
    chosenEl.value = item.suggested_amount;
  });
  const copyPhotos = document.getElementById("copy-photos");
  if (copyPhotos) {
    copyPhotos.addEventListener("click", () =>
      copyText("photo URLs", photos.join("\n"))
    );
  }
  document.getElementById("save-plan").addEventListener("click", async () => {
    showStatus("");
    try {
      const res = await fetch("/api/refresh/plan", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          item_id: item.item_id,
          price_amount: chosenEl.value.trim(),
          listing_url: document.getElementById("new-url").value.trim(),
        }),
      });
      const text = await res.text();
      if (!res.ok) throw new Error(text || `${res.status}`);
      showStatus(
        "Plan saved locally. Post if you have not yet, then delete the old listing, then Sync.",
        "ok"
      );
    } catch (err) {
      showStatus(err.message, "error");
    }
  });
}

function escapeHtml(s) {
  return String(s || "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}

function escapeAttr(s) {
  return escapeHtml(s).replaceAll("'", "&#39;");
}

async function load() {
  showStatus("");
  try {
    if (itemID) {
      const res = await fetch(`/api/refresh/item?item_id=${encodeURIComponent(itemID)}`);
      const text = await res.text();
      if (!res.ok) throw new Error(text || `${res.status}`);
      const data = JSON.parse(text);
      renderPack(data.item);
      return;
    }
    const res = await fetch("/api/refresh");
    const text = await res.text();
    if (!res.ok) throw new Error(text || `${res.status}`);
    renderQueue(JSON.parse(text));
  } catch (err) {
    metaEl.textContent = "Could not load refresh queue";
    showStatus(err.message, "error");
  }
}

load();
