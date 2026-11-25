# Plan: Extension popup stale count + Open Refresh

- **Status:** needs_human
- **Backlog ref:** North-star / exact-relist Phase 1 — stale visible from plugin
- **Owner:** cloud-agent
- **Branch / PR:** cursor/extension-stale-popup-dd5d — https://github.com/thisismii/depop-store-manager/pull/2
- **Started:** 2026-11-25
- **Last updated:** 2026-11-25

## Goal

From the Chrome extension popup, Keishi can see how many **stale** active listings exist in the local catalog and open the Refresh queue (or dashboard Stale tab) in one click.

## Slice (in scope)

- Popup fetches local `GET /api/refresh` (stale queue count; empty catalog → 0)
- Popup shows stale count + **Open Refresh** → `http://127.0.0.1:8787/refresh.html`
- Optional secondary link to dashboard Stale: `/?tab=stale` (dashboard honors `tab` query)
- Cloud + Human-later checkpoints in this plan
- No Depop write, no catalog dumps, no secrets

## Out of scope

- Assisted create/delete, photos pack fidelity, hold-age ledger
- Heavy in-popup stale list (v1 = count + open)
- Live Sync verification (Human-later)
- Committing `data/` or `.env`

## Checkpoints

### Cloud (agent must pass)

| ID | Check | How to verify | Result |
|----|-------|---------------|--------|
| C1 | Go tests | `export PATH=/usr/local/go/bin:$PATH && go test ./...` | **pass** (2026-11-25) |
| C2 | Go build | `export PATH=/usr/local/go/bin:$PATH && go build -o /dev/null ./cmd/server` | **pass** (2026-11-25) |
| C3 | Refresh API empty/offline-safe shape | Start server (empty catalog); `curl -s http://127.0.0.1:8787/api/refresh` returns JSON with `count: 0` (or equivalent empty items) | **pass** — `count: 0`, `items: []`, `stale_days: 60` |
| C4 | Popup static wiring | Review `extension/popup.html` + `popup.js`: fetches `/api/refresh`, renders count, Open Refresh opens `/refresh.html`; dashboard `web/app.js` reads `?tab=stale` | **pass** — static review of popup + `?tab=` init in `web/app.js` |

### Human-later (owner / laptop)

| ID | Check | Blocked on | Result |
|----|-------|------------|--------|
| H1 | After live Sync, popup stale count matches dashboard Stale / Refresh queue | Laptop + Chrome Sync + real catalog | open |
| H2 | Open Refresh from popup lands on queue; optional Open Stale lands on Stale tab | Same | open |
| H3 | Deep-link `/refresh.html?item_id=…` still works for a real stale row (unchanged path; smoke from dashboard or pack link) | Same | open |

## Demo checkpoint

Open extension popup with local server running → see “N stale (≥60d)” (or 0 / server offline) → click **Open Refresh** → `/refresh.html` queue loads.

## Notes / failures

Roadmap Phase 1 UX default: count + Open Refresh preferred over a heavy in-popup list.
