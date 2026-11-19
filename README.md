# Depop Store Manager

Local helper for Depop sellers: sync your active (and sold) listings, spot **stale** items (60+ days), rewrite descriptions, and **manually** refresh listings.

**No silent auto-post.** Delete / create / relist stay human-gated. **No Boost** flows.

Your shop identity and API keys live in `.env` (or Cursor environment secrets) — never in git.

## Cloud Agents (phone / away from laptop)

1. Connect GitHub at [Cursor Integrations](https://cursor.com/dashboard/integrations) and grant access to **thisismii/depop-store-manager**.
2. Open [Cloud Agents](https://cursor.com/dashboard/cloud-agents) → create/setup environment for this repo (guided install is fine; repo ships `.cursor/environment.json`).
3. Add secrets (same names as `.env.example`): `DEPOP_SHOP_ID`, `DEPOP_SHOP_USERNAME`, optional `OPENAI_API_KEY` as a **Runtime Secret**.
4. From phone: [cursor.com/agents](https://cursor.com/agents) (or Cursor iOS) → pick this repo → describe the feature → agent opens a PR.

Live Depop Sync still needs your laptop + Chrome extension. Cloud work = code, tests, docs, UI against fixtures.

**Cloud handoff:** [`docs/HANDOFF.md`](docs/HANDOFF.md) — how the app works, what stays local, north-star (exact relist + delete old + keep `item_id`).

**Plans / checkpoints:** [`docs/plans/`](docs/plans/) — agents leave Cloud-verifiable checks and Human-later (Sync / Depop) so they can pause and self-repair when you cannot test locally.

Cloud smoke test OK.

## Preview (mockups)

Static HTML with **fake sample listings** (no live shop data):

- [Mockup index](docs/mockups/index.html) — Stale dashboard, Rewrite modal, Refresh queue

Open the files in a browser after clone (they load `web/styles.css`). Real screenshots can land later under `docs/screenshots/` once Cloud Agents / a demo shop are set up.

## Who this is for

Any Depop store owner who wants a private dashboard + Chrome extension to:

1. Sync from the logged-in **Selling Hub** (not the flaky public shop API)
2. See Active / Sold / Stale queues
3. Draft sales-oriented descriptions (optional OpenAI-compatible key; heuristic fallback)
4. Run a Refresh pack: copy description / photos / price, open Depop create, paste the new URL, then delete the old listing yourself

## Quick start

### Requirements

- Go 1.22+
- Chrome (for the Sync extension)
- A Depop seller account (logged in)

### Configure

```bash
cp .env.example .env
```

Edit `.env`:

| Variable | Purpose |
|----------|---------|
| `DEPOP_SHOP_ID` | Numeric shop / user id (from Depop network calls or profile) |
| `DEPOP_SHOP_USERNAME` | Your `@username` without `@` |
| `OPENAI_API_KEY` | Optional; enables LLM description rewrite |

Leave shop vars blank to run in **demo mode** (fixtures / empty catalog). Cloud Agents and contributors can build and test without a live shop.

For Cursor Cloud Agents, set the same keys as environment secrets on the agent — do not put real values in the repo.

### Run the dashboard

```bash
./start
# → http://127.0.0.1:8787
```

Or: `go run ./cmd/server -root .`

### Load the extension

1. Chrome → `chrome://extensions` → Developer mode
2. Load unpacked → select `extension/`
3. Open Selling Hub (Active), click **Sync** in the extension popup
4. Refresh the dashboard

## Privacy

- `data/` (live catalog) is gitignored
- `.env` is gitignored; only `.env.example` is committed
- Do not commit shipping addresses, captures with tokens, or screen recordings

## Docs for agents / contributors

- [`AGENTS.md`](AGENTS.md) — architecture and product rules
- [`BACKLOG.md`](BACKLOG.md) — ranked work
- [`ROADMAP.md`](ROADMAP.md) — session history / next slices

## License

Use at your own risk. Unofficial; not affiliated with Depop.
