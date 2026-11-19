# Plan: &lt;short title&gt;

- **Status:** draft | in_progress | needs_human | done | cancelled
- **Backlog ref:** Must/Should/Later — item summary
- **Owner:** cloud-agent | human
- **Branch / PR:** (fill when opened)
- **Started:** YYYY-MM-DD
- **Last updated:** YYYY-MM-DD

## Goal

One sentence: what the seller can do when this is done.

## Slice (in scope)

- …

## Out of scope

- … (esp. silent Depop write, Boost, live catalog in git)

## Checkpoints

### Cloud (agent must pass)

| ID | Check | How to verify | Result |
|----|-------|---------------|--------|
| C1 | | `go test ./...` | |
| C2 | | `go build -o /dev/null ./cmd/server` | |
| C3 | | (add fixture / HTTP / UI-static check) | |

### Human-later (owner / laptop)

| ID | Check | Blocked on | Result |
|----|-------|------------|--------|
| H1 | | Chrome Sync / real Depop / … | |

## Demo checkpoint

What someone should see or click when the slice is done (even if Human-later still open).

## Notes / failures

Log what failed, what you tried, what the next agent should retry.
