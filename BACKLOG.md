# Depop Store Manager — Backlog

Living list from shop-owner interviews + build constraints.  
Agents: do not invent items the owner did not endorse.

**Last updated:** 2026-11-18 (public repo scrub; shop identity via env)

## How to read

| Column | Meaning |
|--------|---------|
| Must | Next slices; unblocks weekly pain |
| Should | Soon after must |
| Later | Real, but not next |
| Never | Explicitly out (or “not unless the owner changes their mind”) |

## Must

1. **Rewrite description for sales** — ✅ Done. Dashboard Rewrite → draft (seller format) → Copy / Approve (`pending_description` local only; no Depop write). Optional `OPENAI_API_KEY` for LLM; heuristic fallback otherwise.
2. **Refresh listing** — ✅ Helper done. Stale queue reviews old vs new price (keep by default; 15%/25% signals; optional blanket %). Pack copies description (pending rewrite if approved), photo URLs, price; opens Depop create. Paste new listing URL → local plan; delete old on Depop; Sync retargets `item_id`. **Does not post or delete on Depop.** True auto create/delete still blocked until Network capture.

## Should

3. **Less phone↔laptop photo hell** — Prefer: photos ride along inside Refresh so the seller doesn’t manually copy them. Stopgap if needed: gather photo links for one listing in one place.

## Later

- Clearer “what do I do on this page” / guidance around Sync
- Default dashboard to old-listings (Stale) tab
- Mark skip / done on a row until next sync
- **Listed-for / payout view** — tag listing with a person/consignor; on Sold show grouped “owe X for a,b,c” with prices (seller totals; no auto % yet). Schema field `listed_for` already reserved.
- Full photo set for Refresh — look up live from Depop at Refresh time (dashboard only stores `cover_url` for ID)
- **Local SKU** — code for books / consignor / variant tracking. Depop SKU is drafts-table only (not on Sell now); optional later write-through.
- Shipping, buyer reviews, orders — stay on Depop for early versions

## Never (until the owner says otherwise)

- Silent delete (no confirm)
- Boost purchase or boost-only metrics
- Auto-message buyers
- Auto price cuts
- Silent auto-post without confirmation

## Interview notes (anonymized)

**Weekly rhythm:** Maintenance beyond messages at least once a week, ~1 hour. Mix: post new listings, refresh old descriptions, re-upload old listings, ship, write buyer reviews.

**How sellers spot stale:** Listings older than ~2 months. Often do not care about accumulated likes. Optimizing for conversion/sale.

**Typical refresh steps today:** (1) Copy all photos (2) Edit listing in place with new description + small tweaks (3) Make a copy / new listing and paste photos + updated description (4) After fresh upload is live, delete the original stale listing. Pricing rarely changes in the same step.

**Time sinks:** Physical clicking to copy photos; prompting AI for a new description within Depop rules (max 5 hashtags); making a copy of the original listing; reframing photos; deleting the original; hopping phone ↔ laptop.

**Wants first:** (1) Button to auto-update description optimized for sales (2) “Refresh” that completes copy of description, price, listing, and photos into a new listing (then delete old, with confirms).

**Description rules (common):** Start with a headline title, blank line, then body with bullets for brand, size, condition, measurements, and/or material. Condition/material only if already in original. Always size, measurements (if present), brand at minimum.

**Scope:** Listings and refresh only for early versions (not shipping/reviews in the helper).
