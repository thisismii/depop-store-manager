# Depop Store Manager — Session Roadmap

Tentative. Revise after each demo. Principle: **thin vertical slice per session**.

Shop identity: `DEPOP_SHOP_ID` + `DEPOP_SHOP_USERNAME` in `.env` (never commit real values).  
v0.1 goal: Sync → stale queue (≥60d) → manual relist helper. No auto-post.

**Repo:** mono-repo (`cmd/server`, `web/`, `extension/`).  
**Data:** live catalog + secrets stay out of git (`data/`, `.env`). Committed fixtures are synthetic only.

**Agent / shared context:** start at [`AGENTS.md`](AGENTS.md). No Boost; write actions stay confirmed.

---

## Session 0 (done)
- Discovery: presentation API has `created_at`, pagination, photos, description
- Plan locked

**Demo:** N/A (research)

---

## Session 1 — Fixture → stale UI (done)
**Slice:** Fake data in, useful list out. No extension yet.

- Go server serves dashboard + loads synthetic fixture JSON
- Show items where age ≥ 60 days (`created_at`)
- Actions: Open listing URL, Copy description, Copy price
- Shop / stale threshold configurable
- Sorted oldest first

**Demo:** Open dashboard → see stale fixture rows → copy/open works offline.

---

## Session 2 — Live Sync one page (done)
**Slice:** Extension captures one Selling Hub page into the same UI.

**Demo:** Logged-in hub → Sync → rows appear on dashboard.

---

## Session 3 — Full catalog scroll + persistence (done)
**Slice:** Scroll / Load more → merge full Active catalog to disk.

- `POST /api/sync` merges into `data/catalog.json`
- **Persistence checked:** stop server, restart `./start` → last sync still on dashboard

**Demo:** One Sync pulls full Active catalog; All + Stale tabs; last-synced survives restart.

**Risk killed:** Full-volume live sync; scroll-driven capture works where direct API replay does not.

**Deferred:** default UI to Stale tab; ignore/done marks; README polish (README now shipped for public repo).

---

## Session 4 — Grill shop owner → backlog (done)
**Slice:** Learn real Depop seller workflow and pain points. **No feature build** this session.

**Outcomes:** Ranked [`BACKLOG.md`](BACKLOG.md). Refresh path = copy photos → rewrite description → new listing → delete old. Wants sales description rewrite first, then confirmed Refresh.

**Demo / done when:** Ranked backlog; next build = Session 5 Rewrite.

---

## Session 4.5 — Identity + status merge store (done)
**Slice:** Sync stops wiping local memory. Stable `item_id`, `first_listed_at`, Active+Sold status.

- `POST /api/sync` **merges** by Depop id (preserve `item_id`, `first_listed_at`, `listed_for`)
- Extension Sync: Active then Sold; combined POST with `reconcile` when Sold ok
- Dashboard: Active / Sold / Stale
- Stale still uses posting `created_at`

**Demo:** Sync → restart → same `item_id` / `first_listed_at`; Sold tab when hub works.

---

## Session 5 — Rewrite description for sales (done)
**Slice:** Generate a sales-optimized description (headline + blank line + bullets). Seller approves before apply.

- `POST /api/rewrite` → `{ original, draft, provider }`
- Provider: OpenAI-compatible when `OPENAI_API_KEY` set; else heuristic
- `POST /api/rewrite/approve` stores `pending_description` locally only

**Demo:** Stale → Rewrite → Copy / Approve → pending survives Sync.

---

## Session 6 — Refresh listing helper (done)
**Slice:** Stale items → price review → copy pack → seller posts on Depop → helper retargets after Sync.

- `/refresh.html` queue + per-item pack
- `POST /api/refresh/plan` stores pending price + slug
- Next Sync retargets `item_id` on slug match
- **No silent Depop write.**

**Demo:** Full manual refresh loop with Sync retarget.

---

## Later (unordered)
- Partner / official API if approved → auto-relist path
- Photo download pack
- Cross-platform
- Multi-shop productization

---

## Session checklist (every time)
1. Name the slice and **demo checkpoint**
2. Stub anything outside the slice
3. Stop when demo works — polish only if it blocks dogfood
4. Update this roadmap with what changed
5. After shop-owner feedback: update [`BACKLOG.md`](BACKLOG.md) before writing code
