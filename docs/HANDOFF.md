# Cloud Project Handoff — Depop Store Manager

**Audience:** Cursor Cloud Project / Cloud Agents  
**Repo:** `thisismii/depop-store-manager` (public)  
**Private local twin:** `~/code/depop-manager` may hold live `data/`, notes, recordings — **never copy those into git**.

Read this first in the Cloud Project. Then [`AGENTS.md`](../AGENTS.md), [`BACKLOG.md`](../BACKLOG.md), [`docs/plans/`](plans/).

---

## North-star (owner priority)

**Strict exact relisting of stale items**, driven as much as possible from the **Chrome extension / helper**, then **delete the old Depop listing**, while **keeping our common SKU / identity** across the new post.

In practice that means:

1. Pick a stale active listing (≥60 days on posting `created_at`).
2. Recreate it on Depop with the **same commercial content** (photos, description or approved `pending_description`, price default **keep**, sizes/brand/etc. when we have them) — not Depop’s forced “reprice ≥15%” path unless the seller chooses a cut.
3. Delete the **old** Depop product after the new one is live.
4. Keep **one stable local identity** for that physical item so history, listed-for, rewrite drafts, and future SKU stay attached.

### Identity = our SKU spine

| Field | Role |
|-------|------|
| `item_id` | **Stable local UUID** — minted once, survives Sync and Refresh. This is today’s “common SKU” / identity. **Must not change** when Depop id/slug changes after relist. |
| Depop `id` / `slug` | Ephemeral platform ids. Change on create+delete. |
| `first_listed_at` | First time we saw the item; preserved across retarget. |
| `pending_refresh_slug` | After seller pastes new listing URL, next Sync **retargets** the row: new Depop id, **same** `item_id`. |
| `sku` (reserved) | Future seller-facing code; do not invent write-through to Depop SKU without a plan. |

**Exact relist** = field-faithful clone where possible + identity continuity via `item_id`. Not “similar listing.” Not silent Boost. Not auto price cut unless chosen.

---

## What exists today (honest)

| Piece | Status |
|-------|--------|
| Extension Sync (Active + Sold) from Selling Hub | Works — laptop + logged-in Chrome |
| Local catalog merge (`data/catalog.json`) | Works — gitignored |
| Dashboard Active / Sold / Stale | Works |
| Description rewrite → local `pending_description` | Works (LLM optional) |
| Refresh helper (queue + pack: copy desc/photos/price, open create URL) | Works — **human** creates on Depop |
| Paste new URL → `pending_refresh_slug` → Sync retargets `item_id` | Works in store |
| **Automated** create + delete from extension | **Not done** — Record Post can capture one create for research; no silent write |
| Full create-form parity (condition, brand, sizes, shipping address ids, all photos) | Partial — Sync has hub fields; Refresh pack is mainly desc/photos/price |

So the **north-star path is partially manual today**. Cloud work should close gaps toward exact pack + safer delete confirmation + identity tests — **without** turning on silent Depop writes until explicitly planned and human-gated.

---

## Architecture (cloud vs local)

```
┌──────────────────────────── LOCAL (laptop) ─────────────────────────────┐
│  Chrome + extension/                                                     │
│    Sync → POST http://127.0.0.1:8787/api/sync                            │
│    (optional) Record Post → capture create network for research          │
│  ./start → Go server → web/ dashboard + /refresh.html                    │
│  data/catalog.json  ← live shop data (NEVER commit)                      │
└──────────────────────────────────────────────────────────────────────────┘

┌──────────────────────────── CLOUD AGENT VM ─────────────────────────────┐
│  Clone public repo                                                       │
│  Secrets: DEPOP_SHOP_ID, DEPOP_SHOP_USERNAME, optional OPENAI_API_KEY    │
│  go test / go build / fixture or empty catalog                           │
│  Cannot Sync Selling Hub or delete real listings                         │
│  Opens PRs; Human-later checkpoints wait for laptop                      │
└──────────────────────────────────────────────────────────────────────────┘
```

**Must stay local:** live Sync, real create/delete on Depop, `data/`, `.env`, shipping addresses, screen recordings.

**Cloud can do:** store/API/UI/extension code, unit tests, fixtures, docs, plans/checkpoints, PR review loops.

---

## Relist flow (current manual) → target

### Current (Session 6 helper)

1. Stale tab / `/refresh.html` → choose price (default keep).
2. Open pack → copy description (prefer `pending_description` if approved), photo URLs, price.
3. Open `https://www.depop.com/products/create/` — seller fills form (manual).
4. Paste new listing URL into helper → stores `pending_refresh_slug` (+ pending price).
5. Seller **deletes old** listing on Depop (manual).
6. Sync → slug match → **same `item_id`**, new Depop id, history updated.

### Target (north-star)

Same identity rules. Stricter **exact** field copy. Extension assists create + **confirmed** delete. Still **no silent** write until product rules change. Prefer confirm UI / Record Post learnings over forged pagination `fetch` (WAF).

---

## Hard product rules (do not violate)

- **No Boost** flows or boost-only metrics.
- **Confirm before write** — create/delete/relist human-gated until an explicit plan says otherwise.
- **No personal shop data in git.**
- Prefer **Selling Hub** sync over public presentation APIs.
- Stale age = posting **`dateCreated`** → `created_at`, not `first_listed_at`.

---

## Secrets (Cursor environment)

| Name | Notes |
|------|-------|
| `DEPOP_SHOP_ID` | Env var; demo if empty |
| `DEPOP_SHOP_USERNAME` | Env var; demo if empty |
| `OPENAI_API_KEY` | Runtime Secret; optional |

Baseline verify:

```bash
export PATH=/usr/local/go/bin:$PATH
go test ./...
go build -o /dev/null ./cmd/server
```

---

## Plans & checkpoints

See [`docs/plans/README.md`](plans/README.md).

- **Cloud** checkpoints: agent must pass before PR.
- **Human-later**: Sync / real Depop — leave `needs_human`; do not fake pass.
- Active related plan: [`2026-11-18-photos-on-refresh.md`](plans/2026-11-18-photos-on-refresh.md) (feeds exact relist photo fidelity).
- New work toward auto/assisted create+delete: **new plan required** with grill/confirm gates; link this handoff.

---

## Suggested Project Note (paste into Cursor Project)

> **Depop Store Manager** — public repo `thisismii/depop-store-manager`.  
> **North-star:** exact stale relist + delete old Depop listing + keep stable local `item_id` (our SKU spine).  
> **Today:** Sync + Refresh pack are human-assisted; retarget already preserves `item_id`. Auto create/delete not shipped.  
> **Cloud:** code/tests/docs/PRs only. Live Sync/delete stay on laptop.  
> **Read:** `docs/HANDOFF.md`, `AGENTS.md`, `docs/plans/`.  
> **Rules:** no Boost, confirm-before-write, no live catalog in git.

---

## Quick file map

| Path | Why |
|------|-----|
| `extension/` | Sync + optional create capture |
| `cmd/server/` | Local API + static UI |
| `internal/store/` | Catalog merge, refresh pack, retarget |
| `internal/rewrite/` | Description drafts |
| `web/` | Dashboard + refresh UI |
| `fixtures/` | Synthetic products only |
| `docs/mockups/` | Static UI previews |
| `docs/plans/` | Feature plans + checkpoints |
