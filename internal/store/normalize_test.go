package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseSellingHubCamelCase(t *testing.T) {
	raw := []byte(`{
	  "products": [
	    {
	      "id": 111,
	      "slug": "stale-hub-item",
	      "description": "old listing",
	      "dateCreated": "2025-01-15T12:00:00Z",
	      "price": { "priceAmount": "18.00", "currencyName": "USD" }
	    },
	    {
	      "id": 222,
	      "slug": "fresh-hub-item",
	      "description": "new listing",
	      "dateCreated": "2026-08-20T12:00:00Z",
	      "price": { "priceAmount": "42.00", "currencyName": "USD" }
	    }
	  ]
	}`)
	req, err := ParseSyncPayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	products := req.Products
	if len(products) != 2 {
		t.Fatalf("want 2 got %d", len(products))
	}
	if products[0].CreatedAt.Format(time.RFC3339) != "2025-01-15T12:00:00Z" {
		t.Fatalf("dateCreated map failed: %v", products[0].CreatedAt)
	}
	if products[0].Price.PriceAmount != "18.00" || products[0].Price.CurrencyName != "USD" {
		t.Fatalf("price camelCase map failed: %+v", products[0].Price)
	}
	now := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
	stale := Stale(products, now, 60)
	if len(stale) != 1 || stale[0].ID != 111 {
		t.Fatalf("stale want id 111, got %+v", stale)
	}
}

func TestMergePreservesFirstListedAndMintsItemID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "catalog.json")
	cs := NewCatalogStore(path)

	t0 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	synced := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)

	first, err := cs.Merge([]Product{{
		ID:          10,
		Slug:        "a",
		Description: "v1",
		CreatedAt:   t0,
		Status:      StatusActive,
		Price:       Price{PriceAmount: "10.00", CurrencyName: "USD"},
	}}, synced, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Products) != 1 {
		t.Fatalf("want 1 got %d", len(first.Products))
	}
	itemID := first.Products[0].ItemID
	if itemID == "" {
		t.Fatal("expected item_id")
	}
	if !first.Products[0].FirstListedAt.Equal(t0) {
		t.Fatalf("first_listed_at want %v got %v", t0, first.Products[0].FirstListedAt)
	}

	second, err := cs.Merge([]Product{{
		ID:          10,
		Slug:        "a",
		Description: "v2",
		CreatedAt:   t1,
		Status:      StatusActive,
		Price:       Price{PriceAmount: "12.00", CurrencyName: "USD"},
	}}, synced.Add(time.Hour), false)
	if err != nil {
		t.Fatal(err)
	}
	p := second.Products[0]
	if p.ItemID != itemID {
		t.Fatalf("item_id changed: %s -> %s", itemID, p.ItemID)
	}
	if !p.FirstListedAt.Equal(t0) {
		t.Fatalf("first_listed_at overwritten: %v", p.FirstListedAt)
	}
	if p.Description != "v2" || p.Price.PriceAmount != "12.00" {
		t.Fatalf("depop fields not updated: %+v", p)
	}
	if !p.CreatedAt.Equal(t1) {
		t.Fatalf("created_at want %v got %v", t1, p.CreatedAt)
	}
}

func TestMergeSoldAndReconcileGone(t *testing.T) {
	dir := t.TempDir()
	cs := NewCatalogStore(filepath.Join(dir, "catalog.json"))
	synced := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	t0 := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)

	_, err := cs.Merge([]Product{
		{ID: 1,Slug: "keep", Description: "a", CreatedAt: t0, Status: StatusActive, Price: Price{PriceAmount: "1", CurrencyName: "USD"}},
		{ID: 2, Slug: "sell", Description: "b", CreatedAt: t0, Status: StatusActive, Price: Price{PriceAmount: "2", CurrencyName: "USD"}},
		{ID: 3, Slug: "drop", Description: "c", CreatedAt: t0, Status: StatusActive, Price: Price{PriceAmount: "3", CurrencyName: "USD"}},
	}, synced, false)
	if err != nil {
		t.Fatal(err)
	}

	cat, err := cs.Merge([]Product{
		{ID: 1, Slug: "keep", Description: "a", CreatedAt: t0, Status: StatusActive, Price: Price{PriceAmount: "1", CurrencyName: "USD"}},
		{ID: 2, Slug: "sell", Description: "b", CreatedAt: t0, Status: StatusSold, Price: Price{PriceAmount: "2", CurrencyName: "USD"}},
	}, synced.Add(time.Minute), true)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[int64]Product{}
	for _, p := range cat.Products {
		byID[p.ID] = p
	}
	if byID[1].Status != StatusActive {
		t.Fatalf("id 1 status %s", byID[1].Status)
	}
	if byID[2].Status != StatusSold {
		t.Fatalf("id 2 status %s", byID[2].Status)
	}
	if byID[3].Status != StatusGone {
		t.Fatalf("id 3 status %s", byID[3].Status)
	}
	if cat.ProductCount != 1 {
		t.Fatalf("product_count want 1 got %d", cat.ProductCount)
	}
}

func TestLoadMigratesLegacyCatalog(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "catalog.json")
	legacy := `{
  "synced_at": "2026-09-10T12:00:00Z",
  "product_count": 1,
  "products": [
    {
      "id": 99,
      "slug": "legacy",
      "description": "old shape",
      "created_at": "2025-06-01T00:00:00Z",
      "price": { "price_amount": "5.00", "currency_name": "USD" }
    }
  ]
}
`
	if err := writeFile(path, legacy); err != nil {
		t.Fatal(err)
	}
	cs := NewCatalogStore(path)
	cat, err := cs.Load()
	if err != nil {
		t.Fatal(err)
	}
	p := cat.Products[0]
	if p.ItemID == "" {
		t.Fatal("expected migrated item_id")
	}
	if p.Status != StatusActive {
		t.Fatalf("status %s", p.Status)
	}
	if p.FirstListedAt.Format(time.RFC3339) != "2025-06-01T00:00:00Z" {
		t.Fatalf("first_listed_at %v", p.FirstListedAt)
	}
	// Reload — stable item_id
	again, err := cs.Load()
	if err != nil {
		t.Fatal(err)
	}
	if again.Products[0].ItemID != p.ItemID {
		t.Fatal("item_id changed on second load")
	}
}

func TestMergeSoldSetsSoldAtAndPreservesRefreshCount(t *testing.T) {
	dir := t.TempDir()
	cs := NewCatalogStore(filepath.Join(dir, "catalog.json"))
	synced := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	t0 := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)

	first, err := cs.Merge([]Product{{
		ID: 1, Slug: "x", Description: "a", CreatedAt: t0, Status: StatusActive,
		Price: Price{PriceAmount: "1", CurrencyName: "USD"},
	}}, synced, false)
	if err != nil {
		t.Fatal(err)
	}
	first.Products[0].RefreshCount = 0
	// rewrite with refresh_count 0 explicit via second merge preserving field
	cat, err := cs.Merge([]Product{{
		ID: 1, Slug: "x", Description: "a", CreatedAt: t0, Status: StatusSold,
		Price: Price{PriceAmount: "1", CurrencyName: "USD"},
	}}, synced.Add(time.Hour), true)
	if err != nil {
		t.Fatal(err)
	}
	p := cat.Products[0]
	if p.Status != StatusSold {
		t.Fatalf("status %s", p.Status)
	}
	if p.SoldAt == nil {
		t.Fatal("expected sold_at")
	}
	if p.RefreshCount != 0 {
		t.Fatalf("refresh_count want 0 got %d", p.RefreshCount)
	}
	sorted := SortSoldNewest([]Product{
		{ID: 2, SoldAt: ptrTime(synced), CreatedAt: t0},
		{ID: 1, SoldAt: ptrTime(synced.Add(time.Hour)), CreatedAt: t0},
	})
	if sorted[0].ID != 1 {
		t.Fatalf("sold sort want newest id 1 first, got %d", sorted[0].ID)
	}
}

func ptrTime(t time.Time) *time.Time { return &t }

func TestParseCoverURLFromPicturesFormats(t *testing.T) {
	raw := []byte(`{
	  "products": [{
	    "id": 7,
	    "slug": "pic-item",
	    "description": "with photo",
	    "dateCreated": "2025-01-15T12:00:00Z",
	    "price": { "priceAmount": "18.00", "currencyName": "USD" },
	    "pictures": [{
	      "id": 1,
	      "formats": {
	        "P0": { "url": "https://cdn.example/full.jpg", "width": 1280 },
	        "P4": { "url": "https://cdn.example/thumb.jpg", "width": 150 }
	      }
	    }]
	  }]
	}`)
	req, err := ParseSyncPayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	if req.Products[0].CoverURL != "https://cdn.example/thumb.jpg" {
		t.Fatalf("cover_url got %q", req.Products[0].CoverURL)
	}
}

func TestMergePreservesPendingDescription(t *testing.T) {
	dir := t.TempDir()
	cs := NewCatalogStore(filepath.Join(dir, "catalog.json"))
	synced := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	t0 := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)

	first, err := cs.Merge([]Product{{
		ID: 1, Slug: "x", Description: "original", CreatedAt: t0, Status: StatusActive,
		Price: Price{PriceAmount: "1", CurrencyName: "USD"},
	}}, synced, false)
	if err != nil {
		t.Fatal(err)
	}
	itemID := first.Products[0].ItemID
	if _, err := cs.SetPendingDescription(itemID, "approved draft"); err != nil {
		t.Fatal(err)
	}

	cat, err := cs.Merge([]Product{{
		ID: 1, Slug: "x", Description: "synced from depop", CreatedAt: t0, Status: StatusActive,
		Price: Price{PriceAmount: "1", CurrencyName: "USD"},
	}}, synced.Add(time.Hour), false)
	if err != nil {
		t.Fatal(err)
	}
	p := cat.Products[0]
	if p.Description != "synced from depop" {
		t.Fatalf("description want synced got %q", p.Description)
	}
	if p.PendingDescription != "approved draft" {
		t.Fatalf("pending wiped: %q", p.PendingDescription)
	}
}

func writeFile(path, body string) error {
	return os.WriteFile(path, []byte(body), 0o644)
}


