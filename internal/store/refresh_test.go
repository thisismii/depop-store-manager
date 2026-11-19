package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestApplyDiscountPercent(t *testing.T) {
	got, err := ApplyDiscountPercent("20.00", 15)
	if err != nil {
		t.Fatal(err)
	}
	if got != "17.00" {
		t.Fatalf("15%% of 20.00 got %s", got)
	}
	got, err = ApplyDiscountPercent("20.00", 25)
	if err != nil {
		t.Fatal(err)
	}
	if got != "15.00" {
		t.Fatalf("25%% of 20.00 got %s", got)
	}
	got, err = ApplyDiscountPercent("19.99", 15)
	if err != nil {
		t.Fatal(err)
	}
	if got != "16.99" {
		t.Fatalf("15%% of 19.99 got %s", got)
	}
}

func TestSuggestRefreshPrice(t *testing.T) {
	keep := SuggestRefreshPrice("20.00", 10)
	if keep.Percent != 0 || keep.Suggested != "20.00" {
		t.Fatalf("fresh listing should keep price: %+v", keep)
	}
	stale := SuggestRefreshPrice("20.00", 60)
	if stale.Percent != 15 || stale.Suggested != "17.00" {
		t.Fatalf("60d suggest 15%%: %+v", stale)
	}
	very := SuggestRefreshPrice("20.00", 120)
	if very.Percent != 25 || very.Suggested != "15.00" {
		t.Fatalf("120d suggest 25%%: %+v", very)
	}
}

func TestParseListingRef(t *testing.T) {
	ref, err := ParseListingRef("https://www.depop.com/products/sampleseller-coco-pants-8463/")
	if err != nil || ref.Slug != "sampleseller-coco-pants-8463" {
		t.Fatalf("url slug got %+v err=%v", ref, err)
	}
	ref, err = ParseListingRef("sampleseller-coco-pants-8463")
	if err != nil || ref.Slug != "sampleseller-coco-pants-8463" {
		t.Fatalf("bare slug got %+v err=%v", ref, err)
	}
	if _, err := ParseListingRef("https://www.depop.com/products/create/"); err == nil {
		t.Fatal("create url should fail")
	}
}

func TestParseNestedSellingHubPictures(t *testing.T) {
	raw := []byte(`{
	  "products": [{
	    "id": 9,
	    "slug": "nested-pics",
	    "description": "x",
	    "dateCreated": "2025-01-15T12:00:00Z",
	    "price": { "priceAmount": "18.00", "currencyName": "USD" },
	    "pictures": [
	      [
	        {"url":"https://media-photos.depop.com/a/P5.jpg","width":320,"id":1},
	        {"url":"https://media-photos.depop.com/a/P0.jpg","width":1280,"id":1}
	      ],
	      [
	        {"url":"https://media-photos.depop.com/b/P5.jpg","width":320,"id":2},
	        {"url":"https://media-photos.depop.com/b/P0.jpg","width":1280,"id":2}
	      ]
	    ]
	  }]
	}`)
	req, err := ParseSyncPayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	p := req.Products[0]
	if len(p.PhotoURLs) != 2 {
		t.Fatalf("photo_urls=%v", p.PhotoURLs)
	}
	if p.PhotoURLs[0] != "https://media-photos.depop.com/a/P0.jpg" {
		t.Fatalf("want largest first photo, got %s", p.PhotoURLs[0])
	}
	if p.PhotoURLs[1] != "https://media-photos.depop.com/b/P0.jpg" {
		t.Fatalf("want largest second photo, got %s", p.PhotoURLs[1])
	}
}

func TestMergeRetargetsPendingRefreshSlug(t *testing.T) {
	dir := t.TempDir()
	cs := NewCatalogStore(filepath.Join(dir, "catalog.json"))
	t0 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	synced := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

	first, err := cs.Merge([]Product{{
		ID: 10, Slug: "old-slug", Description: "old", CreatedAt: t0, Status: StatusActive,
		Price: Price{PriceAmount: "20.00", CurrencyName: "USD"},
	}}, synced, false)
	if err != nil {
		t.Fatal(err)
	}
	itemID := first.Products[0].ItemID
	if _, err := cs.SetRefreshPlan(itemID, "17.00", "https://www.depop.com/products/new-slug/"); err != nil {
		t.Fatal(err)
	}

	t1 := time.Date(2026, 10, 31, 13, 0, 0, 0, time.UTC)
	cat, err := cs.Merge([]Product{
		{
			ID: 10, Slug: "old-slug", Description: "old", CreatedAt: t0, Status: StatusActive,
			Price: Price{PriceAmount: "20.00", CurrencyName: "USD"},
		},
		{
			ID: 99, Slug: "new-slug", Description: "fresh copy", CreatedAt: t1, Status: StatusActive,
			Price: Price{PriceAmount: "17.00", CurrencyName: "USD"},
		},
	}, synced.Add(time.Hour), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Products) != 1 {
		t.Fatalf("want 1 local row after retarget, got %d", len(cat.Products))
	}
	p := cat.Products[0]
	if p.ItemID != itemID {
		t.Fatalf("item_id changed: %s vs %s", p.ItemID, itemID)
	}
	if p.ID != 99 {
		t.Fatalf("depop id want 99 got %d", p.ID)
	}
	if p.RefreshCount != 1 {
		t.Fatalf("refresh_count=%d", p.RefreshCount)
	}
	if len(p.DepopIDHistory) != 1 || p.DepopIDHistory[0] != 10 {
		t.Fatalf("history=%v", p.DepopIDHistory)
	}
	if !p.FirstListedAt.Equal(t0) {
		t.Fatalf("first_listed_at wiped: %v", p.FirstListedAt)
	}
	if p.PendingRefreshSlug != "" {
		t.Fatalf("pending slug still set: %s", p.PendingRefreshSlug)
	}
}

func TestMergeSkipsRetiredDepopID(t *testing.T) {
	dir := t.TempDir()
	cs := NewCatalogStore(filepath.Join(dir, "catalog.json"))
	t0 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 10, 31, 13, 0, 0, 0, time.UTC)
	synced := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

	first, err := cs.Merge([]Product{{
		ID: 10, Slug: "old-slug", Description: "old", CreatedAt: t0, Status: StatusActive,
		Price: Price{PriceAmount: "20.00", CurrencyName: "USD"},
	}}, synced, false)
	if err != nil {
		t.Fatal(err)
	}
	itemID := first.Products[0].ItemID
	if _, err := cs.SetRefreshPlan(itemID, "17.00", "new-slug"); err != nil {
		t.Fatal(err)
	}
	if _, err := cs.Merge([]Product{{
		ID: 99, Slug: "new-slug", Description: "fresh", CreatedAt: t1, Status: StatusActive,
		Price: Price{PriceAmount: "17.00", CurrencyName: "USD"},
	}}, synced.Add(time.Hour), false); err != nil {
		t.Fatal(err)
	}

	cat, err := cs.Merge([]Product{
		{
			ID: 10, Slug: "old-slug", Description: "old still live", CreatedAt: t0, Status: StatusActive,
			Price: Price{PriceAmount: "20.00", CurrencyName: "USD"},
		},
		{
			ID: 99, Slug: "new-slug", Description: "fresh", CreatedAt: t1, Status: StatusActive,
			Price: Price{PriceAmount: "17.00", CurrencyName: "USD"},
		},
	}, synced.Add(2*time.Hour), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Products) != 1 {
		t.Fatalf("retired old id minted extra row: %d", len(cat.Products))
	}
	if cat.Products[0].ID != 99 || cat.Products[0].ItemID != itemID {
		t.Fatalf("row=%+v", cat.Products[0])
	}
}

func TestSetRefreshPlanRejectsSold(t *testing.T) {
	dir := t.TempDir()
	cs := NewCatalogStore(filepath.Join(dir, "catalog.json"))
	synced := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	t0 := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	first, err := cs.Merge([]Product{{
		ID: 1, Slug: "sold", Description: "x", CreatedAt: t0, Status: StatusSold,
		Price: Price{PriceAmount: "10.00", CurrencyName: "USD"},
	}}, synced, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cs.SetRefreshPlan(first.Products[0].ItemID, "8.00", ""); err == nil {
		t.Fatal("expected error for sold listing")
	}
}
