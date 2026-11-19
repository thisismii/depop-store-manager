const HUB_ACTIVE_URL = "https://www.depop.com/sellinghub/selling/active/";
const HUB_SOLD_URL = "https://www.depop.com/sellinghub/selling/sold/";
const PRODUCTS_API_ACTIVE =
  "https://webapi.depop.com/api/v1/shop/products/?lang=en&cursor=&limit=24&statusFilter=selling&showProductInsights=true";
const PRODUCTS_API_SOLD =
  "https://webapi.depop.com/api/v1/shop/products/?lang=en&cursor=&limit=24&statusFilter=sold&showProductInsights=true";
const SYNC_URL = "http://127.0.0.1:8787/api/sync";
const DASHBOARD_URL = "http://127.0.0.1:8787/";
const CREATE_URL = "https://www.depop.com/products/create/";
const WRITE_CAPTURE_URL = "http://127.0.0.1:8787/api/debug/create-capture";
const WRITE_KEY = "__depop_mgr_writes";
const CAPTURE_KEY = "__depop_mgr_capture";
const MAX_PAGES = 50;

function urlAndWidth(el) {
  if (typeof el === "string") return { url: el, width: 0 };
  if (!el || typeof el !== "object") return { url: "", width: 0 };
  let url = el.url || el.preview || el.uri || el.src || "";
  let width = Number(el.width) || 0;
  const formats = el.formats || el.imageFormats || el.sizes || {};
  const values = Object.values(formats);
  for (let i = 0; i < values.length; i++) {
    const f = values[i];
    if (!f || !f.url) continue;
    const w = Number(f.width) || 0;
    if (w >= width) {
      url = f.url;
      width = w;
    }
  }
  return { url: String(url || ""), width };
}

function bestUrlFromGroup(group) {
  if (typeof group === "string") return group;
  if (Array.isArray(group)) {
    let best = "";
    let bestW = -1;
    for (let i = 0; i < group.length; i++) {
      const got = urlAndWidth(group[i]);
      if (got.url && got.width >= bestW) {
        best = got.url;
        bestW = got.width;
      }
    }
    return best;
  }
  return urlAndWidth(group).url;
}

function photoUrlsFromRaw(raw) {
  if (!raw || typeof raw !== "object") return [];
  if (Array.isArray(raw.photo_urls) && raw.photo_urls.length) {
    return raw.photo_urls.filter(Boolean).map(String);
  }
  const pics =
    raw.pictures ||
    raw.photos ||
    raw.images ||
    raw.productPictures ||
    raw.product_pictures;
  if (!Array.isArray(pics) || !pics.length) return [];
  const out = [];
  const seen = {};
  for (let i = 0; i < pics.length; i++) {
    const u = bestUrlFromGroup(pics[i]);
    if (!u || seen[u]) continue;
    seen[u] = true;
    out.push(u);
  }
  return out;
}

function coverUrlFromRaw(raw) {
  if (!raw || typeof raw !== "object") return "";
  if (raw.cover_url) return String(raw.cover_url);
  if (raw.preview) return String(raw.preview);
  if (raw.imageUrl) return String(raw.imageUrl);
  if (raw.thumbnail) return String(raw.thumbnail);
  if (raw.image) {
    if (typeof raw.image === "string") return raw.image;
    if (raw.image.url) return String(raw.image.url);
  }
  if (raw.primaryPicture) {
    const u = coverUrlFromRaw({ pictures: [raw.primaryPicture] });
    if (u) return u;
  }
  if (raw.picture) {
    const u = coverUrlFromRaw({ pictures: [raw.picture] });
    if (u) return u;
  }
  const pics =
    raw.pictures ||
    raw.photos ||
    raw.images ||
    raw.productPictures ||
    raw.product_pictures;
  if (!Array.isArray(pics) || !pics.length) {
    // Last resort: any nested http URL that looks like a Depop media image.
    return deepFindImageUrl(raw);
  }
  const p = pics[0];
  if (typeof p === "string") return p;
  if (p && typeof p === "object") {
    if (p.url) return String(p.url);
    if (p.preview) return String(p.preview);
    if (p.uri) return String(p.uri);
    if (p.src) return String(p.src);
    const formats = p.formats || p.imageFormats || p.sizes || {};
    for (const key of ["P5", "P4", "P2", "P1", "P0", "P8", "P7", "P6", "P9"]) {
      if (formats[key] && formats[key].url) return String(formats[key].url);
    }
    for (const f of Object.values(formats)) {
      if (f && f.url) return String(f.url);
    }
  }
  return deepFindImageUrl(raw);
}

function deepFindImageUrl(obj, depth) {
  if (depth == null) depth = 0;
  if (depth > 5 || obj == null) return "";
  if (typeof obj === "string") {
    if (
      /^https?:\/\//i.test(obj) &&
      /(media-photos|img\.depop|\.jpe?g($|\?)|\.webp($|\?)|\.png($|\?))/i.test(
        obj
      )
    ) {
      return obj;
    }
    return "";
  }
  if (Array.isArray(obj)) {
    for (let i = 0; i < obj.length; i++) {
      const u = deepFindImageUrl(obj[i], depth + 1);
      if (u) return u;
    }
    return "";
  }
  if (typeof obj === "object") {
    const keys = Object.keys(obj);
    for (let i = 0; i < keys.length; i++) {
      const u = deepFindImageUrl(obj[keys[i]], depth + 1);
      if (u) return u;
    }
  }
  return "";
}

function normalizeProduct(raw) {
  const amount =
    (raw.price && (raw.price.priceAmount || raw.price.price_amount)) ||
    (raw.current_price &&
      raw.current_price.price_breakdown &&
      raw.current_price.price_breakdown.price &&
      raw.current_price.price_breakdown.price.amount) ||
    (raw.current_price && raw.current_price.total_price) ||
    "";
  const currency =
    (raw.price && (raw.price.currencyName || raw.price.currency_name)) ||
    raw.currency_name ||
    raw.currency ||
    "USD";
  return {
    id: raw.id,
    slug: raw.slug || "",
    description: raw.description || "",
    created_at: raw.dateCreated || raw.created_at || "",
    cover_url: coverUrlFromRaw(raw),
    photo_urls: photoUrlsFromRaw(raw),
    price: {
      price_amount: String(amount),
      currency_name: String(currency),
    },
  };
}

function productsFromList(list) {
  if (!Array.isArray(list) || !list.length) return null;
  return list.map(normalizeProduct);
}

async function injectPageHook(tabId) {
  await chrome.scripting.executeScript({
    target: { tabId },
    world: "MAIN",
    files: ["page-hook.js"],
    injectImmediately: true,
  });
}

/**
 * Hub tab must be focused during pagination — Chrome throttles background-tab
 * network and forged fetch returns "Failed to fetch".
 */
async function openHubCaptureTab(hubUrl) {
  const tab = await chrome.tabs.create({ url: hubUrl, active: true });
  const tabId = tab.id;

  await new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      chrome.tabs.onUpdated.removeListener(onUpdated);
      reject(new Error("Depop tab load timed out"));
    }, 45000);

    async function onUpdated(id, info) {
      if (id !== tabId) return;
      if (info.status === "loading") {
        try {
          await injectPageHook(tabId);
        } catch {
          /* retry */
        }
      }
      if (info.status === "complete") {
        clearTimeout(timer);
        chrome.tabs.onUpdated.removeListener(onUpdated);
        try {
          await injectPageHook(tabId);
        } catch {
          /* ignore */
        }
        resolve();
      }
    }

    chrome.tabs.onUpdated.addListener(onUpdated);
    chrome.tabs.get(tabId).then(async (t) => {
      if (t.status === "complete") {
        clearTimeout(timer);
        chrome.tabs.onUpdated.removeListener(onUpdated);
        try {
          await injectPageHook(tabId);
        } catch {
          /* ignore */
        }
        resolve();
      } else {
        try {
          await injectPageHook(tabId);
        } catch {
          /* ignore */
        }
      }
    });
  });

  return tabId;
}

async function readCapture(tabId) {
  const [{ result }] = await chrome.scripting.executeScript({
    target: { tabId },
    world: "MAIN",
    args: [CAPTURE_KEY],
    func: (key) => {
      try {
        return sessionStorage.getItem(key);
      } catch {
        return null;
      }
    },
  });
  return result;
}

async function clearCapture(tabId) {
  try {
    await chrome.scripting.executeScript({
      target: { tabId },
      world: "MAIN",
      args: [CAPTURE_KEY],
      func: (key) => {
        try {
          sessionStorage.removeItem(key);
        } catch {
          /* ignore */
        }
      },
    });
  } catch {
    /* ignore */
  }
}

async function waitForCapture(tabId, timeoutMs) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    let raw;
    try {
      raw = await readCapture(tabId);
    } catch {
      await new Promise((r) => setTimeout(r, 250));
      continue;
    }
    if (typeof raw === "string" && raw) {
      try {
        return JSON.parse(raw);
      } catch {
        throw new Error("corrupt Selling Hub capture");
      }
    }
    await new Promise((r) => setTimeout(r, 250));
  }
  return null;
}

/**
 * Cursor pagination via XMLHttpRequest (not fetch).
 * Depop's page service worker intercepts fetch → "Failed to fetch" on our calls;
 * XHR bypasses the SW and still sends cookies.
 */
async function paginateWithXHR(tabId, capture, productsApi, statusFilter) {
  const [{ result }] = await chrome.scripting.executeScript({
    target: { tabId },
    world: "MAIN",
    args: [productsApi, capture, MAX_PAGES, statusFilter || "selling"],
    func: async (baseUrl, capture, maxPages, statusFilter) => {
      function pageUrl(cursor) {
        const template = capture.url || baseUrl;
        try {
          const u = new URL(template, "https://webapi.depop.com");
          u.searchParams.set("cursor", cursor || "");
          u.searchParams.set("limit", "24");
          if (!u.searchParams.get("lang")) u.searchParams.set("lang", "en");
          u.searchParams.set("statusFilter", statusFilter);
          if (!u.searchParams.get("showProductInsights")) {
            u.searchParams.set("showProductInsights", "true");
          }
          return u.toString();
        } catch {
          return (
            "https://webapi.depop.com/api/v1/shop/products/?lang=en&cursor=" +
            encodeURIComponent(cursor || "") +
            "&limit=24&statusFilter=" +
            encodeURIComponent(statusFilter) +
            "&showProductInsights=true"
          );
        }
      }

      function xhrGet(url, headers) {
        return new Promise((resolve, reject) => {
          const XHR = window.__depop_mgr_XHR || XMLHttpRequest;
          const xhr = new XHR();
          xhr.open("GET", url, true);
          xhr.withCredentials = true;
          Object.keys(headers || {}).forEach((k) => {
            try {
              xhr.setRequestHeader(k, headers[k]);
            } catch {
              /* forbidden header name */
            }
          });
          xhr.onload = () => {
            resolve({ status: xhr.status, text: xhr.responseText || "" });
          };
          xhr.onerror = () => reject(new Error("XHR network error"));
          xhr.ontimeout = () => reject(new Error("XHR timeout"));
          xhr.timeout = 30000;
          xhr.send();
        });
      }

      const headers = { Accept: "application/json" };
      const src = capture.headers || {};
      Object.keys(src).forEach((k) => {
        if (/^authorization$/i.test(k) && src[k]) {
          headers.Authorization = src[k];
        }
      });

      const seed =
        Array.isArray(capture.products) && capture.products.length
          ? capture.products
          : capture.page && capture.page.products
            ? capture.page.products
            : [];
      if (!seed.length) {
        return { ok: false, error: "first page missing products" };
      }

      const all = seed.slice();
      const seen = {};
      for (let i = 0; i < all.length; i++) seen[all[i].id] = true;

      let cursor =
        capture.cursor ||
        (capture.page && capture.page.meta && capture.page.meta.cursor) ||
        "";
      let ended = !!(
        capture.end ||
        (capture.page && capture.page.meta && capture.page.meta.end)
      );
      let pages = capture.pages || 1;

      while (!ended && cursor && pages < maxPages) {
        let res;
        try {
          res = await xhrGet(pageUrl(cursor), headers);
        } catch (e) {
          return {
            ok: false,
            error:
              "pagination XHR failed after " +
              all.length +
              " items: " +
              String(e && e.message ? e.message : e),
            products: all,
            pages,
          };
        }
        if (res.status < 200 || res.status >= 300) {
          return {
            ok: false,
            error:
              "pagination HTTP " +
              res.status +
              " after " +
              all.length +
              " items: " +
              String(res.text).slice(0, 120),
            products: all,
            pages,
          };
        }
        let page;
        try {
          page = JSON.parse(res.text);
        } catch {
          return {
            ok: false,
            error: "pagination invalid JSON after " + all.length + " items",
            products: all,
            pages,
          };
        }
        const list = page.products || [];
        for (let i = 0; i < list.length; i++) {
          const p = list[i];
          if (p && p.id != null && !seen[p.id]) {
            seen[p.id] = true;
            all.push(p);
          }
        }
        pages += 1;
        ended = !!(page.meta && page.meta.end);
        cursor = (page.meta && page.meta.cursor) || "";
        if (!list.length) break;
      }

      return { ok: true, products: all, pages, ended };
    },
  });
  return result;
}

/**
 * Primary pagination: keep scrolling Selling Hub so Depop fires cursor pages;
 * page-hook accumulates every shop/products response. Also clicks Load more if present.
 */
async function paginateByScrolling(tabId) {
  await chrome.scripting.executeScript({
    target: { tabId },
    world: "MAIN",
    func: () => {
      window.__depop_mgr_accumulate = true;
    },
  });

  let lastCount = 0;
  let stableRounds = 0;
  const maxRounds = 80; // ~398/24 ≈ 17 pages; extra room for slow loads

  for (let i = 0; i < maxRounds; i++) {
    const rawBefore = await readCapture(tabId);
    let countBefore = 0;
    let endedBefore = false;
    try {
      const s = rawBefore ? JSON.parse(rawBefore) : null;
      countBefore = Array.isArray(s?.products) ? s.products.length : 0;
      endedBefore = !!s?.end;
    } catch {
      /* ignore */
    }
    if (endedBefore) {
      const s = JSON.parse(rawBefore);
      return {
        ok: true,
        products: s.products,
        pages: s.pages || 1,
        ended: true,
      };
    }

    await chrome.scripting.executeScript({
      target: { tabId },
      world: "MAIN",
      func: () => {
        function visible(el) {
          if (!el) return false;
          const r = el.getBoundingClientRect();
          const st = window.getComputedStyle(el);
          return (
            r.width > 0 &&
            r.height > 0 &&
            st.visibility !== "hidden" &&
            st.display !== "none"
          );
        }
        function clickLoadMoreIfPresent() {
          const nodes = document.querySelectorAll(
            'button, a, [role="button"]'
          );
          for (let i = 0; i < nodes.length; i++) {
            const el = nodes[i];
            const t = (el.textContent || "")
              .replace(/\s+/g, " ")
              .trim()
              .toLowerCase();
            if (!t.includes("load more")) continue;
            if (el.disabled || el.getAttribute("aria-disabled") === "true") {
              continue;
            }
            if (!visible(el)) continue;
            el.scrollIntoView({ block: "center", inline: "nearest" });
            el.click();
            return true;
          }
          return false;
        }

        // Scroll window + every scrollable container to the bottom.
        const step = Math.max(window.innerHeight * 1.2, 1000);
        window.scrollBy(0, step);
        document.documentElement.scrollTop = document.documentElement.scrollHeight;
        document.body.scrollTop = document.body.scrollHeight;

        const nodes = document.querySelectorAll("div, main, section, ul");
        for (let i = 0; i < nodes.length; i++) {
          const el = nodes[i];
          if (el.scrollHeight > el.clientHeight + 40) {
            el.scrollTop = el.scrollHeight;
          }
        }

        // Some hubs show an explicit control after scroll.
        clickLoadMoreIfPresent();
      },
    });

    // Give Depop time to fire the next cursor request + paint.
    await new Promise((r) => setTimeout(r, 700));

    const waitDeadline = Date.now() + 5000;
    let grew = false;
    let state = null;
    while (Date.now() < waitDeadline) {
      const raw = await readCapture(tabId);
      if (raw) {
        try {
          state = JSON.parse(raw);
        } catch {
          state = null;
        }
      }
      const count = Array.isArray(state?.products) ? state.products.length : 0;
      if (state?.end) {
        return {
          ok: true,
          products: state.products,
          pages: state.pages || 1,
          ended: true,
        };
      }
      if (count > countBefore) {
        grew = true;
        lastCount = count;
        stableRounds = 0;
        break;
      }
      await new Promise((r) => setTimeout(r, 300));
    }

    if (!grew) {
      // One more nudge: click Load more only, then wait.
      await chrome.scripting.executeScript({
        target: { tabId },
        world: "MAIN",
        func: () => {
          const nodes = document.querySelectorAll(
            'button, a, [role="button"]'
          );
          for (let i = 0; i < nodes.length; i++) {
            const el = nodes[i];
            const t = (el.textContent || "")
              .replace(/\s+/g, " ")
              .trim()
              .toLowerCase();
            if (t.includes("load more") && !el.disabled) {
              el.click();
              return;
            }
          }
        },
      });
      await new Promise((r) => setTimeout(r, 1200));
      const raw = await readCapture(tabId);
      try {
        state = raw ? JSON.parse(raw) : null;
      } catch {
        state = null;
      }
      const count = Array.isArray(state?.products) ? state.products.length : 0;
      if (state?.end) {
        return {
          ok: true,
          products: state.products,
          pages: state.pages || 1,
          ended: true,
        };
      }
      if (count > countBefore) {
        lastCount = count;
        stableRounds = 0;
        continue;
      }
      stableRounds += 1;
      if (stableRounds >= 5) {
        return {
          ok: count > 0,
          products: state?.products || [],
          pages: state?.pages || 1,
          ended: false,
          partial: true,
          error: `scroll stalled at ${count} items`,
        };
      }
    }
  }

  const raw = await readCapture(tabId);
  const state = raw ? JSON.parse(raw) : null;
  return {
    ok: (state?.products?.length || 0) > 0,
    products: state?.products || [],
    pages: state?.pages || 1,
    ended: !!state?.end,
    partial: !state?.end,
    error: state?.end ? undefined : "hit max scroll rounds",
  };
}

async function collectAllProducts(tabId, _capture, productsApi, statusFilter) {
  try {
    await chrome.tabs.update(tabId, { active: true });
  } catch {
    /* ignore */
  }

  const scrolled = await paginateByScrolling(tabId);
  if (scrolled?.products?.length && (scrolled.ended || scrolled.products.length > 24)) {
    return {
      ok: true,
      products: scrolled.products,
      pages: scrolled.pages,
      partial: !scrolled.ended,
      error: scrolled.ended ? undefined : scrolled.error,
    };
  }

  // Last resort: forged XHR cursor loop.
  const xhr = await paginateWithXHR(tabId, _capture, productsApi, statusFilter);
  if (xhr?.ok && xhr.products?.length) return xhr;

  throw new Error(
    `scroll: ${scrolled?.error || "failed"} (${scrolled?.products?.length || 0} items) | XHR: ${xhr?.error || "failed"}`
  );
}

async function fetchViaPageCapture(hubUrl, productsApi, statusFilter) {
  const tabId = await openHubCaptureTab(hubUrl);
  try {
    let capture = await waitForCapture(tabId, 25000);
    if (!capture) {
      await clearCapture(tabId);
      const reloadDone = new Promise((resolve, reject) => {
        const timer = setTimeout(() => {
          chrome.tabs.onUpdated.removeListener(onUpdated);
          reject(new Error("Depop reload timed out"));
        }, 45000);
        async function onUpdated(id, info) {
          if (id !== tabId) return;
          if (info.status === "loading") {
            try {
              await injectPageHook(tabId);
            } catch {
              /* ignore */
            }
          }
          if (info.status === "complete") {
            clearTimeout(timer);
            chrome.tabs.onUpdated.removeListener(onUpdated);
            try {
              await injectPageHook(tabId);
            } catch {
              /* ignore */
            }
            resolve();
          }
        }
        chrome.tabs.onUpdated.addListener(onUpdated);
      });
      await chrome.tabs.reload(tabId);
      await reloadDone;
      capture = await waitForCapture(tabId, 25000);
    }
    if (!capture?.page && !(capture?.products && capture.products.length)) {
      throw new Error(
        "No Selling Hub products capture — log in, open the hub once manually, retry Sync"
      );
    }

    const result = await collectAllProducts(
      tabId,
      capture,
      productsApi,
      statusFilter
    );
    if (!result.ok && (!result.products || result.products.length <= 24)) {
      throw new Error(result.error || "pagination failed");
    }
    const products = productsFromList(result.products);
    if (!products) throw new Error("pagination produced 0 products");
    return {
      products,
      rawSample: result.products[0] || null,
      pages: result.pages,
      partial: !result.ok || !!result.partial,
      error: result.error,
    };
  } finally {
    try {
      await chrome.tabs.remove(tabId);
    } catch {
      /* already gone */
    }
  }
}

async function postRawProductSample(raw) {
  if (!raw) return;
  try {
    await fetch("http://127.0.0.1:8787/api/debug/raw-product", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(raw),
    });
  } catch {
    /* debug only */
  }
}

async function postCatalog(products, { status, reconcile } = {}) {
  const body = { products };
  if (status) body.status = status;
  if (reconcile) body.reconcile = true;
  let res;
  try {
    res = await fetch(SYNC_URL, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
  } catch (e) {
    throw new Error(
      `Cannot reach Depop Manager at ${SYNC_URL} (${e.message}). Run ./start`
    );
  }
  if (!res.ok) {
    throw new Error(`${res.status}: ${await res.text()}`);
  }
  return res.json();
}

async function openDashboard() {
  const tabs = await chrome.tabs.query({ url: "http://127.0.0.1:8787/*" });
  if (tabs.length > 0) {
    const tab = tabs[0];
    await chrome.tabs.update(tab.id, { active: true, url: DASHBOARD_URL });
    if (tab.windowId != null) {
      await chrome.windows.update(tab.windowId, { focused: true });
    }
    return;
  }
  await chrome.tabs.create({ url: DASHBOARD_URL, active: true });
}

async function runSync() {
  const active = await fetchViaPageCapture(
    HUB_ACTIVE_URL,
    PRODUCTS_API_ACTIVE,
    "selling"
  );
  if (!Array.isArray(active.products) || active.products.length === 0) {
    throw new Error("Depop returned 0 active products");
  }
  if (active.rawSample) {
    await postRawProductSample(active.rawSample);
  }

  let sold = null;
  let soldError = null;
  try {
    sold = await fetchViaPageCapture(HUB_SOLD_URL, PRODUCTS_API_SOLD, "sold");
  } catch (e) {
    soldError = String(e?.message || e);
  }

  const byId = new Map();
  for (const p of active.products) {
    byId.set(p.id, { ...p, status: "active" });
  }
  if (sold?.products?.length) {
    for (const p of sold.products) {
      byId.set(p.id, { ...p, status: "sold" });
    }
  }

  const combined = Array.from(byId.values());
  const withCover = combined.filter((p) => p.cover_url).length;
  const withPhotos = combined.filter((p) => p.photo_urls && p.photo_urls.length).length;
  const reconcile = !soldError; // only mark gone when both hubs synced
  const result = await postCatalog(combined, {
    reconcile,
  });
  await openDashboard();

  const pages = (active.pages || 0) + (sold?.pages || 0);
  const partial = !!active.partial || !!sold?.partial || !!soldError || withCover === 0;
  const partialBits = [];
  if (active.partial && active.error) partialBits.push(`active: ${active.error}`);
  if (sold?.partial && sold.error) partialBits.push(`sold: ${sold.error}`);
  if (soldError) partialBits.push(`sold skipped: ${soldError}`);
  if (withCover === 0) {
    partialBits.push("no cover urls mapped — see data/last-raw-product.json");
  }

  const out = {
    product_count: result.product_count ?? active.products.length,
    total_rows: result.total_rows,
    cover_count: withCover,
    photo_count: withPhotos,
    pages,
    partial,
    partial_error: partialBits.length ? partialBits.join(" | ") : undefined,
    sold_ok: !soldError,
    sold_count: sold?.products?.length || 0,
  };
  try {
    await chrome.storage.local.set({ lastSync: { ...out, at: Date.now() } });
  } catch {
    /* storage optional */
  }
  return out;
}

async function injectWriteHook(tabId) {
  await chrome.scripting.executeScript({
    target: { tabId, allFrames: true },
    world: "MAIN",
    files: ["write-hook.js"],
    injectImmediately: true,
  });
}

async function readWriteCapture(tabId, clear) {
  const [{ result }] = await chrome.scripting.executeScript({
    target: { tabId },
    world: "MAIN",
    func: (key, shouldClear) => {
      try {
        const raw = sessionStorage.getItem(key);
        if (shouldClear) sessionStorage.removeItem(key);
        return raw;
      } catch (err) {
        return JSON.stringify({ error: String(err && err.message ? err.message : err) });
      }
    },
    args: [WRITE_KEY, !!clear],
  });
  if (!result) return { at: Date.now(), events: [] };
  try {
    return JSON.parse(result);
  } catch {
    return { at: Date.now(), events: [], parse_error: true };
  }
}

function summarizeWrites(state) {
  const events = Array.isArray(state?.events) ? state.events : [];
  return events.map((e) => {
    const path = String(e.url || "").replace(/^https?:\/\/[^/]+/i, "");
    return `${e.method || "?"} ${e.status || "—"} ${path.slice(0, 120)}`;
  });
}

async function startRecordPost() {
  const existing = await chrome.tabs.query({ url: CREATE_URL + "*" });
  let tab =
    existing.find((t) => t.url && t.url.indexOf("/products/create") !== -1) ||
    null;
  if (tab) {
    await chrome.tabs.update(tab.id, { active: true, url: CREATE_URL });
  } else {
    tab = await chrome.tabs.create({ url: CREATE_URL, active: true });
  }
  await chrome.storage.local.set({
    recordPost: { tabId: tab.id, startedAt: Date.now() },
  });
  try {
    await injectWriteHook(tab.id);
  } catch {
    /* tab still loading; onUpdated will inject */
  }
  return { tabId: tab.id };
}

async function saveRecordPost() {
  const stored = await chrome.storage.local.get("recordPost");
  const tabId = stored.recordPost && stored.recordPost.tabId;
  if (!tabId) {
    throw new Error("Click Record Post first, then Post one listing on Depop.");
  }
  const peek = await readWriteCapture(tabId, false);
  const events = peek.events || [];
  if (!events.length) {
    throw new Error(
      "No write traffic yet. Stay on the create tab, click Post, then Save capture."
    );
  }
  const payload = {
    captured_at: new Date().toISOString(),
    event_count: events.length,
    events,
  };
  let res;
  try {
    res = await fetch(WRITE_CAPTURE_URL, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
  } catch (e) {
    throw new Error(
      `Cannot reach helper at ${WRITE_CAPTURE_URL} (${e.message}). Run ./start`
    );
  }
  if (!res.ok) {
    throw new Error(`${res.status}: ${await res.text()}`);
  }
  const saved = await res.json();
  await readWriteCapture(tabId, true);
  await chrome.storage.local.remove("recordPost");
  return {
    event_count: events.length,
    lines: summarizeWrites(peek),
    path: saved.path || "data/last-create-capture.json",
  };
}

chrome.tabs.onUpdated.addListener((tabId, info) => {
  if (info.status !== "loading" && info.status !== "complete") return;
  chrome.storage.local.get("recordPost").then((stored) => {
    if (!stored.recordPost || stored.recordPost.tabId !== tabId) return;
    injectWriteHook(tabId).catch(() => {});
  });
});

chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
  if (message?.type === "depop-manager-sync") {
    runSync()
      .then((result) => sendResponse({ ok: true, ...result }))
      .catch((err) =>
        sendResponse({ ok: false, error: String(err?.message || err) })
      );
    return true;
  }
  if (message?.type === "depop-manager-record-start") {
    startRecordPost()
      .then((result) => sendResponse({ ok: true, ...result }))
      .catch((err) =>
        sendResponse({ ok: false, error: String(err?.message || err) })
      );
    return true;
  }
  if (message?.type === "depop-manager-record-save") {
    saveRecordPost()
      .then((result) => sendResponse({ ok: true, ...result }))
      .catch((err) =>
        sendResponse({ ok: false, error: String(err?.message || err) })
      );
    return true;
  }
});
