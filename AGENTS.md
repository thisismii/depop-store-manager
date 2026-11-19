# Agent context — Depop Store Manager

Read this first. Shared understanding for humans and coding agents working in this repo.

## What this is

Open local tool for **any Depop seller**. Goal: sync active listings → find **stale** items (≥60 days since create) → help **manual** relist. **No silent auto-post.**

Shop identity comes from env (`DEPOP_SHOP_ID`, `DEPOP_SHOP_USERNAME`) — never hardcode a real shop in git.

## Must-read docs

| Doc | Why |
|-----|-----|
| [`README.md`](README.md) | Setup for sellers and contributors |
| [`ROADMAP.md`](ROADMAP.md) | Vertical-slice session plan and demo checkpoints |
| [`BACKLOG.md`](BACKLOG.md) | Ranked work (Must / Should / Later / Never) |
| [`docs/HANDOFF.md`](docs/HANDOFF.md) | Cloud Project handoff — architecture, local vs cloud, north-star relist+SKU |
| [`docs/plans/`](docs/plans/) | Active feature plans + Cloud / Human checkpoints |

## Hard product rules

- **No Boost** — do not build paid boost flows or depend on boost-only metrics.
- **Confirm before write** — delete / create / relist stay human-gated until explicitly changed.
- **Grill before features** — non-trivial new behavior needs alignment first.
- **Vertical slices** — each chunk ends in something demoable (input → visible output).
- **Plans + checkpoints** — non-trivial work gets a `docs/plans/*.md` with Cloud and Human-later checks; agents re-run Cloud checks on follow-up.
- **No personal shop data in git** — catalog, `.env`, addresses, recordings stay local / secrets.

## Architecture (current)

```
extension/     Chrome MV3 — Sync Active+Sold; Record Post captures one real create (no auto-write)
cmd/server/    Go HTTP on 127.0.0.1:8787 — /api/sync, /api/catalog, /api/rewrite, /api/refresh, static web/
web/           Dashboard — Active / Sold / Stale; Rewrite modal; Refresh stale queue + pack
internal/store Catalog JSON in data/catalog.json (gitignored; survives restart)
internal/rewrite Description rewrite (OpenAI-compatible or heuristic)
fixtures/      Synthetic offline data only
```

Start server: `./start` (or `go run ./cmd/server`). See `.env.example`.

## Sync source of truth

Prefer **logged-in Selling Hub**, not public shop presentation API:

- Active: `https://www.depop.com/sellinghub/selling/active/` — `statusFilter=selling`
- Sold: `https://www.depop.com/sellinghub/selling/sold/` — `statusFilter=sold` (soft-fail if hub unavailable)
- API: `GET https://webapi.depop.com/api/v1/shop/products/?lang=en&cursor=&limit=24&statusFilter=…&showProductInsights=true`
- Response: `{ meta: { cursor, end }, products: [...] }`
- **How Sync works:** inject MAIN-world hook → scroll hub (Load more if needed) → capture pages → **merge** into local catalog (not wipe). Do not forge pagination `fetch` (page SW / WAF often returns `Failed to fetch`).
- Local identity: opaque `item_id` (UUID) minted on first see; `first_listed_at` set once from first-seen `dateCreated` and preserved across Sync. Refresh retargets `item_id` to the new Depop id when `pending_refresh_slug` matches after Sync.
- Status: `active` | `sold` | `gone` (gone = missing after full Active+Sold reconcile)
- Stale age field: posting **`dateCreated`** → local `created_at` (not `first_listed_at`)
- Price: `price.priceAmount` / `price.currencyName`
- Photos: `photo_urls` (largest per picture group) for Refresh; `cover_url` for thumbs
- Reserved local fields: `listed_for`; `pending_description`; `pending_price`; `pending_refresh_slug`; future `sku`

Public presentation endpoints were flaky / blocked for Sync.

## Status

- Fixture UI → live Selling Hub Sync → full-catalog scroll capture + disk persistence
- Merge-safe store + Active/Sold status + `item_id` / `first_listed_at`
- Rewrite description for sales (draft → Copy / Approve local `pending_description`)
- Refresh helper — stale price-review queue + per-item pack (copy desc/photos/price, open Depop create, paste new URL, then delete old). No silent Depop write.
- **North-star:** exact stale relist + confirmed delete old + keep `item_id` (see [`docs/HANDOFF.md`](docs/HANDOFF.md)).
- **Next slices:** Photos-on-Refresh → exact pack fidelity → assisted create/delete (still human-gated). Then listed-for / payout.

## Scale / stale

Depop banners **60+ day** listings and offers native **Reprice and relist** (min 15% cut). Our stale queue is complementary; true delete+recreate is a later automation path.

## Language

Prefer **Go** for server code. Extension is JS (Chrome MV3). Dashboard is static HTML/CSS/JS.

## Cursor Cloud specific instructions

Cloud Agents use `.cursor/environment.json` (`install` installs Go 1.22 and runs `go test ./...`).

**Secrets** (Cursor Dashboard → Cloud Agents → Secrets — never commit):

| Name | Type | Notes |
|------|------|-------|
| `DEPOP_SHOP_ID` | Environment Variable | Optional; demo mode if empty |
| `DEPOP_SHOP_USERNAME` | Environment Variable | Optional; demo mode if empty |
| `OPENAI_API_KEY` | Runtime Secret | Optional; heuristic rewrite without it |

### Plans when the owner is away

1. Read [`docs/plans/README.md`](docs/plans/README.md). Open any `in_progress` / `needs_human` plan, or create one from `_TEMPLATE.md` for the next [`BACKLOG.md`](BACKLOG.md) item.
2. Implement only the plan’s slice. Keep **Cloud** checkpoints executable without a Depop login.
3. Before PR: run every Cloud checkpoint; on failure, fix in the same run. Update the plan table (pass/fail + notes).
4. PR body: link the plan; list Cloud results; list **Human-later** still open (`needs_human` is OK).
5. Follow-up agents: re-run Cloud checkpoints first, then fix or address human notes — do not mark Human-later done yourselves.

**Verify after changes (baseline Cloud checks):**

```bash
export PATH=/usr/local/go/bin:$PATH
go test ./...
go build -o /dev/null ./cmd/server
```

**Dashboard / Refresh UI:** `./start` then open `http://127.0.0.1:8787/` (and `/refresh.html`). Catalog lives in gitignored `data/` — Sync from Chrome extension is **local-only**; cloud agents should use fixtures / empty catalog / unit tests, not live Selling Hub.

**Do not:** commit `.env`, `data/`, real listing dumps, or shipping addresses. Prefer PRs from feature branches.
