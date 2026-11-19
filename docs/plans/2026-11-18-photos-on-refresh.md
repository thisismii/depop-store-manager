# Plan: Photos on Refresh (cut phone↔laptop hell)

- **Status:** draft
- **Backlog ref:** Should #3 — Less phone↔laptop photo hell
- **Owner:** cloud-agent
- **Branch / PR:** (none yet)
- **Started:** 2026-11-18
- **Last updated:** 2026-11-18

## Goal

Seller can refresh a stale listing with **all photo URLs** available in the Refresh pack (copy / open) without manually hunting images on phone and laptop.

## Slice (in scope)

- Ensure Sync / store path preserves `photo_urls` (largest per picture group) when the hub payload includes them.
- Refresh pack UI shows every photo URL (or a clear “only cover available” warning) and one-click copy helpers.
- Unit/fixture coverage for multi-photo products in Refresh pack JSON.
- Docs: note Cloud vs Human verification in this plan + AGENTS if behavior changes.

## Out of scope

- Auto-upload photos to Depop / silent create or delete
- Downloading a ZIP of binaries (Later / follow-on)
- Boost, auto price cuts, messaging buyers
- Committing live catalog or real seller photo dumps to git (use fixtures only)

## Checkpoints

### Cloud (agent must pass)

| ID | Check | How to verify | Result |
|----|-------|---------------|--------|
| C1 | Unit tests green | `export PATH=/usr/local/go/bin:$PATH && go test ./...` | |
| C2 | Server builds | `go build -o /dev/null ./cmd/server` | |
| C3 | Multi-photo fixture → Refresh pack includes all URLs | Add/extend store test with synthetic product `photo_urls` length ≥ 2; assert pack exposes them | |
| C4 | Refresh UI copy path exists for photos | Static review of `web/refresh.js` / pack markup: multi-URL copy or list; no silent Depop write | |

### Human-later (owner / laptop)

| ID | Check | Blocked on | Result |
|----|-------|------------|--------|
| H1 | Live Sync of a multi-photo listing fills `photo_urls` in `data/catalog.json` | Chrome extension + Selling Hub | |
| H2 | Refresh pack on a real stale item: copy photos → create listing on Depop feels faster than phone↔laptop | Laptop + Depop logged in | |
| H3 | If Sync still only returns cover: document gap and stopgap (gather links) without fake “done” | Laptop Sync sample | |

## Demo checkpoint

With fixture/demo data: open Refresh pack for a sample item → see **more than one** photo URL (or explicit cover-only warning) → Copy photos works. Human confirms on a real listing when available.

## Notes / failures

Next cloud agent: set status to `in_progress`, implement smallest slice that passes C1–C4, open PR linking this plan. Leave H1–H3 open with `needs_human` if Cloud checks pass.
