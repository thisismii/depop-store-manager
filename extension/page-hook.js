/**
 * MAIN-world hook. Injected via chrome.scripting.executeScript (bypasses page CSP).
 * Captures shop/products pages. Exposes __depop_mgr_origFetch for pagination.
 */
(function () {
  if (window.__depop_mgr_hooked) return;
  window.__depop_mgr_hooked = true;

  const KEY = "__depop_mgr_capture";
  const MATCH = "/api/v1/shop/products/";

  function headersToObject(h) {
    const out = {};
    if (!h) return out;
    if (typeof Headers !== "undefined" && h instanceof Headers) {
      h.forEach(function (v, k) {
        out[k] = v;
      });
      return out;
    }
    if (Array.isArray(h)) {
      h.forEach(function (pair) {
        if (pair && pair.length >= 2) out[pair[0]] = pair[1];
      });
      return out;
    }
    if (typeof h === "object") {
      Object.keys(h).forEach(function (k) {
        out[k] = h[k];
      });
    }
    return out;
  }

  function mergeHeaders(a, b) {
    const out = {};
    Object.keys(a || {}).forEach(function (k) {
      out[k] = a[k];
    });
    Object.keys(b || {}).forEach(function (k) {
      out[k] = b[k];
    });
    return out;
  }

  function authOnly(headers) {
    const out = { Accept: "application/json" };
    if (!headers) return out;
    Object.keys(headers).forEach(function (k) {
      if (/^authorization$/i.test(k) && headers[k]) {
        out.Authorization = headers[k];
      }
    });
    return out;
  }

  function readState() {
    try {
      const raw = sessionStorage.getItem(KEY);
      if (!raw) return null;
      return JSON.parse(raw);
    } catch {
      return null;
    }
  }

  function store(text, requestHeaders, requestUrl) {
    try {
      const page = JSON.parse(text);
      if (!page || !Array.isArray(page.products)) return;

      const accumulate = !!window.__depop_mgr_accumulate;
      let state = readState();
      if (!state) {
        state = {
          at: Date.now(),
          products: [],
          pages: 0,
          end: false,
          cursor: "",
          headers: authOnly(requestHeaders),
          url: requestUrl || "",
          page: null,
        };
      }

      if (!accumulate && state.pages > 0) return;

      const seen = {};
      for (let i = 0; i < state.products.length; i++) {
        seen[state.products[i].id] = true;
      }
      for (let i = 0; i < page.products.length; i++) {
        const p = page.products[i];
        if (p && p.id != null && !seen[p.id]) {
          seen[p.id] = true;
          state.products.push(p);
        }
      }

      state.pages += 1;
      state.end = !!(page.meta && page.meta.end);
      state.cursor = (page.meta && page.meta.cursor) || "";
      state.headers = authOnly(requestHeaders);
      state.url = requestUrl || state.url;
      state.at = Date.now();
      // Keep first raw page for callers that expect capture.page
      if (!state.page) state.page = page;

      sessionStorage.setItem(KEY, JSON.stringify(state));
    } catch {
      /* ignore */
    }
  }

  const origFetch = window.fetch.bind(window);
  window.__depop_mgr_origFetch = origFetch;
  // Native XHR (pre-hook) — page service workers intercept fetch, not XHR.
  window.__depop_mgr_XHR = XMLHttpRequest;

  window.fetch = function (...args) {
    let url = "";
    let reqHeaders = {};
    try {
      if (typeof args[0] === "string") {
        url = args[0];
        reqHeaders = headersToObject(args[1] && args[1].headers);
      } else if (args[0] && typeof args[0].url === "string") {
        url = args[0].url;
        reqHeaders = mergeHeaders(
          headersToObject(args[0].headers),
          headersToObject(args[1] && args[1].headers)
        );
      }
    } catch {
      /* ignore */
    }

    const p = origFetch.apply(this, args);
    if (url.indexOf(MATCH) !== -1) {
      const capturedHeaders = reqHeaders;
      const capturedUrl = url;
      p.then(function (res) {
        res
          .clone()
          .text()
          .then(function (text) {
            store(text, capturedHeaders, capturedUrl);
          })
          .catch(function () {});
      }).catch(function () {});
    }
    return p;
  };

  const origOpen = XMLHttpRequest.prototype.open;
  const origSend = XMLHttpRequest.prototype.send;
  const origSetHeader = XMLHttpRequest.prototype.setRequestHeader;
  XMLHttpRequest.prototype.open = function (method, url) {
    this.__depop_mgr_url = url;
    this.__depop_mgr_headers = {};
    return origOpen.apply(this, arguments);
  };
  XMLHttpRequest.prototype.setRequestHeader = function (key, value) {
    if (!this.__depop_mgr_headers) this.__depop_mgr_headers = {};
    this.__depop_mgr_headers[key] = value;
    return origSetHeader.apply(this, arguments);
  };
  XMLHttpRequest.prototype.send = function () {
    if (
      this.__depop_mgr_url &&
      String(this.__depop_mgr_url).indexOf(MATCH) !== -1
    ) {
      const capturedHeaders = this.__depop_mgr_headers || {};
      const capturedUrl = String(this.__depop_mgr_url);
      this.addEventListener("load", function () {
        if (typeof this.responseText === "string") {
          store(this.responseText, capturedHeaders, capturedUrl);
        }
      });
    }
    return origSend.apply(this, arguments);
  };
})();
