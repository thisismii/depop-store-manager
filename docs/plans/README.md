# Feature plans & checkpoints

Cloud Agents (and humans) keep **one plan file per active feature** here so work can pause, resume, and self-repair when the owner cannot test on a laptop.

## Rules for agents

1. **Before coding** a non-trivial change: create or update a plan under `docs/plans/YYYY-MM-DD-slug.md` (copy `_TEMPLATE.md`). Never leave **Checkpoints** empty.
2. **During work:** mark Cloud checkpoints as you pass them. If a Cloud checkpoint fails, **fix it in the same run** before opening/updating the PR.
3. **PR description** must link the plan and list:
   - Cloud checkpoints: pass / fail
   - Human-later checkpoints: still open (blocked on laptop / Sync / real Depop)
4. **Follow-up agents:** open any plan with status `in_progress` or `needs_human`. Re-run **all Cloud checkpoints** first. Then fix failures or address human notes in the plan.
5. **Do not invent product scope.** Align with [`BACKLOG.md`](../../BACKLOG.md). Grill / ask if the slice is unclear.
6. **Never** put shop secrets, live catalog dumps, or shipping addresses in a plan.

## Status values

| Status | Meaning |
|--------|---------|
| `draft` | Plan only; coding not started |
| `in_progress` | Agent or human actively working |
| `needs_human` | Cloud checks green (or N/A); waiting on laptop / Sync / Depop |
| `done` | Cloud + Human checkpoints complete |
| `cancelled` | Abandoned; say why |

## Checkpoint kinds

- **Cloud** — runnable without a logged-in Depop browser: `go test`, `go build`, fixture HTTP checks, mockup/docs review. Agents **must** pass these before claiming the slice is ready.
- **Human-later** — needs laptop, Chrome extension Sync, or real Depop create/delete. Leave open with notes; next agent does not invent a pass — owner marks done or files a fix request.

## Naming

`YYYY-MM-DD-short-slug.md` using the date the plan started (UTC or local is fine; stay consistent).

## Index

| Plan | Status | Backlog |
|------|--------|---------|
| [2026-11-18-photos-on-refresh](2026-11-18-photos-on-refresh.md) | draft | Should #3 — photo hell / photos in Refresh |
