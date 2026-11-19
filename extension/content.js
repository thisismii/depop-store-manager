/**
 * Isolated-world fallback only. No DOM <script> injection —
 * Selling Hub CSP blocks unsafe-inline (script-src).
 * Preferred path is chrome.scripting MAIN world in background.js.
 */

function normalizeProduct(raw) {
  const amount =
    raw?.price?.priceAmount ||
    raw?.price?.price_amount ||
    raw?.current_price?.price_breakdown?.price?.amount ||
    raw?.current_price?.total_price ||
    "";
  const currency =
    raw?.price?.currencyName ||
    raw?.price?.currency_name ||
    raw?.currency_name ||
    raw?.currency ||
    "USD";
  return {
    id: raw.id,
    slug: raw.slug || "",
    description: raw.description || "",
    created_at: raw.dateCreated || raw.created_at || "",
    cover_url: raw.cover_url || "",
    photo_urls: Array.isArray(raw.photo_urls) ? raw.photo_urls : [],
    price: {
      price_amount: String(amount),
      currency_name: String(currency),
    },
  };
}

function bearerFromValue(v) {
  if (!v || typeof v !== "string") return null;
  const trimmed = v.trim();
  if (/^Bearer\s+/i.test(trimmed)) return trimmed;
  if (/^eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+/.test(trimmed)) {
    return "Bearer " + trimmed;
  }
  try {
    const obj = JSON.parse(trimmed);
    const cand =
      obj.access_token ||
      obj.accessToken ||
      obj.token ||
      obj.id_token ||
      obj.idToken ||
      (obj.auth && (obj.auth.access_token || obj.auth.token));
    if (typeof cand === "string" && cand) {
      return /^Bearer\s+/i.test(cand) ? cand : "Bearer " + cand;
    }
  } catch {
    /* not JSON */
  }
  return null;
}

function findAuthHeader() {
  // Isolated world cannot read page localStorage; cookies still go with credentials.
  try {
    const stores = [localStorage, sessionStorage];
    const keyRe = /token|auth|jwt|session|access|bearer|oidc|cognito/i;
    for (const store of stores) {
      for (let i = 0; i < store.length; i++) {
        const k = store.key(i);
        if (!k || !keyRe.test(k)) continue;
        const hdr = bearerFromValue(store.getItem(k));
        if (hdr) return hdr;
      }
    }
  } catch {
    /* storage blocked */
  }
  return null;
}

function slimProducts(page) {
  const list = page?.products || page?.objects;
  if (!Array.isArray(list)) return null;
  return list.map(normalizeProduct);
}

async function fetchIsolated(url) {
  const headers = { Accept: "application/json" };
  const auth = findAuthHeader();
  if (auth) headers.Authorization = auth;

  const res = await fetch(url, {
    credentials: "include",
    headers,
  });
  const text = await res.text();
  if (!res.ok) {
    throw new Error(`Depop ${res.status}: ${text.slice(0, 180)}`);
  }
  const page = JSON.parse(text);
  const products = slimProducts(page);
  if (!products) throw new Error("missing products/objects");
  return products;
}

chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
  if (message?.type !== "depop-manager-fetch") return;

  (async () => {
    try {
      const products = await fetchIsolated(message.url);
      sendResponse({ ok: true, products });
    } catch (err) {
      sendResponse({ ok: false, error: String(err?.message || err) });
    }
  })();

  return true;
});
