# Plan: Exact stale relist + delete old + keep item_id (SKU spine)

- **Status:** draft
- **Backlog ref:** North-star (owner) — beyond current Should #3 photos; builds on Refresh helper
- **Owner:** cloud-agent / human
- **Branch / PR:** (none yet)
- **Started:** 2026-11-18
- **Last updated:** 2026-11-18

## Goal

Seller can **exactly** relist a stale item (same commercial content), **delete the old Depop listing** with confirmation, and keep one stable local **`item_id`** (common SKU / identity) across the new post.

## Slice phases (do not skip gates)

### Phase A — Exact pack fidelity (Cloud-heavy)

- Refresh pack exposes everything we already store that create needs (desc / pending rewrite, all `photo_urls`, price keep-by-default).
- Document gaps vs full create form (brand, size, condition, shipping) — capture only with fixtures, no live dumps in git.
- Depends on / pairs with [`2026-11-18-photos-on-refresh.md`](2026-11-18-photos-on-refresh.md).

### Phase B — Assisted create (Human-gated)

- Extension helps fill or paste create flow using pack data.
- **Still confirm before submit.** No silent auto-post.
- Learn from existing Record Post capture; do not forge hub pagination `fetch`.

### Phase C — Confirmed delete old

- After new listing live + `pending_refresh_slug` set (or equivalent), helper/extension offers **confirm delete** of old Depop id.
- Sync retarget keeps **`item_id`** / `first_listed_at`.

## Out of scope (until owner changes rules)

- Silent create or delete
- Boost
- Forced price cuts
- Committing live catalog / addresses

## Checkpoints

### Cloud (agent must pass)

| ID | Check | How to verify | Result |
|----|-------|---------------|--------|
| C1 | Tests green | `go test ./...` | |
| C2 | Server builds | `go build -o /dev/null ./cmd/server` | |
| C3 | Retarget keeps `item_id` | Existing refresh/store tests still assert identity | |
| C4 | Plan + HANDOFF linked from PR | PR body references `docs/HANDOFF.md` and this plan | |

### Human-later (owner / laptop)

| ID | Check | Blocked on | Result |
|----|-------|------------|--------|
| H1 | Exact pack on real multi-photo stale item | Sync + Refresh | |
| H2 | Create new listing from pack; paste URL; Sync; `item_id` unchanged | Laptop + Depop | |
| H3 | Delete old listing (manual or confirmed assist); catalog healthy | Depop + Sync | |

## Demo checkpoint

Cloud: tests prove retarget identity. Human: one real item survives create+delete with same `item_id`.

## Notes / failures

Read [`docs/HANDOFF.md`](../HANDOFF.md) before coding. Prefer thin PRs per phase. Do not mark Phase B/C done from Cloud alone.
