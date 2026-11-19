/**
 * MAIN-world write capture. Record Post test only.
 * Steals Depop's own fetch/XHR (same idea as Sync). Does not send writes.
 */
(function () {
  if (window.__depop_mgr_write_hooked) return;
  window.__depop_mgr_write_hooked = true;

  const KEY = "__depop_mgr_writes";
  const MAX = 200;
  const BODY_CAP = 4000;
  const SKIP_PATH =
    /\/features\/experiments\/|\/performance\/collect\/|\/events\/|\/vwdj\/|google\.com\/|\/referral-event\/|\/analytics\/|\/checkin\/|\/session\//;

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

  function redactHeaders(h) {
    const out = {};
    const src = h || {};
    Object.keys(src).forEach(function (k) {
      if (/^(authorization|cookie|set-cookie|x-csrf|x-xsrf|x-api-key)/i.test(k)) {
        out[k] = "[redacted]";
      } else {
        out[k] = src[k];
      }
    });
    return out;
  }

  function redactUrl(url) {
    try {
      const u = new URL(url, location.href);
      Array.from(u.searchParams.keys()).forEach(function (q) {
        if (
          /token|signature|csrf|credential|amz|AWSAccessKeyId/i.test(q)
        ) {
          u.searchParams.set(q, "REDACTED");
        }
      });
      return u.toString();
    } catch {
      return String(url || "");
    }
  }

  function interestingUrl(url) {
    const u = String(url || "");
    if (!u) return false;
    if (/depop\.com/i.test(u)) return true;
    if (/amazonaws|cloudfront|cloudinary|filestack|upload/i.test(u)) return true;
    return false;
  }

  function shouldCapture(method, url) {
    if (!interestingUrl(url)) return false;
    const u = String(url || "");
    if (SKIP_PATH.test(u)) return false;
    const m = String(method || "GET").toUpperCase();
    if (m !== "GET" && m !== "HEAD" && m !== "OPTIONS") return true;
    return /upload|picture|photo|media|product|presign|signed|listing/i.test(u);
  }

  function previewBody(body) {
    if (body == null) return null;
    if (typeof body === "string") {
      const s = body.length > BODY_CAP ? body.slice(0, BODY_CAP) + "…" : body;
      try {
        return { kind: "json", json: JSON.parse(body), truncated: body.length > BODY_CAP };
      } catch {
        return { kind: "text", text: s, bytes: body.length };
      }
    }
    if (typeof URLSearchParams !== "undefined" && body instanceof URLSearchParams) {
      return { kind: "form", text: body.toString().slice(0, BODY_CAP) };
    }
    if (typeof FormData !== "undefined" && body instanceof FormData) {
      const keys = [];
      body.forEach(function (_v, k) {
        if (keys.indexOf(k) === -1) keys.push(k);
      });
      return { kind: "formdata", keys: keys };
    }
    if (typeof Blob !== "undefined" && body instanceof Blob) {
      return {
        kind: "blob",
        type: body.type || "",
        size: body.size,
        name: body.name || "",
      };
    }
    if (typeof body === "object") {
      try {
        return { kind: "object", json: JSON.parse(JSON.stringify(body)) };
      } catch {
        return { kind: "object", note: "unserializable" };
      }
    }
    return { kind: typeof body };
  }

  function previewResponse(text) {
    if (typeof text !== "string" || !text) return null;
    const slice = text.length > BODY_CAP ? text.slice(0, BODY_CAP) + "…" : text;
    try {
      return { kind: "json", json: JSON.parse(text), truncated: text.length > BODY_CAP };
    } catch {
      return { kind: "text", text: slice, bytes: text.length };
    }
  }

  function readState() {
    try {
      const raw = sessionStorage.getItem(KEY);
      if (!raw) return { at: Date.now(), events: [] };
      return JSON.parse(raw);
    } catch {
      return { at: Date.now(), events: [] };
    }
  }

  function append(event) {
    if (window !== window.top) {
      try {
        window.top.postMessage({ __depop_mgr_write: true, event: event }, "*");
      } catch {
        /* ignore */
      }
      return;
    }
    const state = readState();
    if (state.events.length >= MAX) return;
    state.events.push(event);
    state.at = Date.now();
    try {
      sessionStorage.setItem(KEY, JSON.stringify(state));
    } catch {
      /* quota */
    }
  }

  window.addEventListener("message", function (e) {
    if (!e || !e.data || !e.data.__depop_mgr_write) return;
    if (window !== window.top) return;
    if (e.data.event) append(e.data.event);
  });

  const origFetch = window.fetch.bind(window);
  window.fetch = function (...args) {
    let url = "";
    let method = "GET";
    let reqHeaders = {};
    let reqBody = null;
    try {
      if (typeof args[0] === "string") {
        url = args[0];
        method = (args[1] && args[1].method) || "GET";
        reqHeaders = headersToObject(args[1] && args[1].headers);
        if (args[1] && "body" in args[1]) reqBody = previewBody(args[1].body);
      } else if (args[0] && typeof args[0].url === "string") {
        url = args[0].url;
        method = args[0].method || "GET";
        reqHeaders = headersToObject(args[0].headers);
        if (args[1] && args[1].headers) {
          const extra = headersToObject(args[1].headers);
          Object.keys(extra).forEach(function (k) {
            reqHeaders[k] = extra[k];
          });
        }
        if (args[1] && "body" in args[1]) reqBody = previewBody(args[1].body);
      }
    } catch {
      /* ignore */
    }

    const p = origFetch.apply(this, args);
    if (shouldCapture(method, url)) {
      const captured = {
        at: Date.now(),
        via: "fetch",
        method: String(method || "GET").toUpperCase(),
        url: redactUrl(url),
        req_headers: redactHeaders(reqHeaders),
        req_body: reqBody,
      };
      p.then(function (res) {
        captured.status = res.status;
        res
          .clone()
          .text()
          .then(function (text) {
            captured.resp = previewResponse(text);
            append(captured);
          })
          .catch(function () {
            append(captured);
          });
      }).catch(function (err) {
        captured.error = String(err && err.message ? err.message : err);
        append(captured);
      });
    }
    return p;
  };

  const origOpen = XMLHttpRequest.prototype.open;
  const origSend = XMLHttpRequest.prototype.send;
  const origSetHeader = XMLHttpRequest.prototype.setRequestHeader;
  XMLHttpRequest.prototype.open = function (method, url) {
    this.__depop_mgr_write_method = method;
    this.__depop_mgr_write_url = url;
    this.__depop_mgr_write_headers = {};
    return origOpen.apply(this, arguments);
  };
  XMLHttpRequest.prototype.setRequestHeader = function (key, value) {
    if (!this.__depop_mgr_write_headers) this.__depop_mgr_write_headers = {};
    this.__depop_mgr_write_headers[key] = value;
    return origSetHeader.apply(this, arguments);
  };
  XMLHttpRequest.prototype.send = function (body) {
    const method = this.__depop_mgr_write_method || "GET";
    const url = String(this.__depop_mgr_write_url || "");
    if (shouldCapture(method, url)) {
      const captured = {
        at: Date.now(),
        via: "xhr",
        method: String(method).toUpperCase(),
        url: redactUrl(url),
        req_headers: redactHeaders(this.__depop_mgr_write_headers || {}),
        req_body: previewBody(body),
      };
      this.addEventListener("load", function () {
        captured.status = this.status;
        if (typeof this.responseText === "string") {
          captured.resp = previewResponse(this.responseText);
        }
        append(captured);
      });
      this.addEventListener("error", function () {
        captured.error = "xhr error";
        append(captured);
      });
    }
    return origSend.apply(this, arguments);
  };
})();
