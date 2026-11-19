package store

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReadBooksCSVAndApplySeed(t *testing.T) {
	dir := t.TempDir()
	catalogPath := filepath.Join(dir, "catalog.json")
	soldAt := time.Date(2026, 9, 14, 3, 0, 0, 0, time.UTC)
	created := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	cs := NewCatalogStore(catalogPath)
	cat := &Catalog{
		SyncedAt:     time.Now().UTC(),
		ProductCount: 0,
		Products: []Product{
			{
				ItemID: "a", ID: 1, Status: StatusSold,
				Description: "ADIDAS Grey and black athletic shorts\nmore",
				CreatedAt:   created,
				SoldAt:      &soldAt,
				Price:       Price{PriceAmount: "6.00", CurrencyName: "USD"},
			},
			{
				ItemID: "b", ID: 2, Status: StatusActive,
				Description: "Should ignore active",
				CreatedAt:   created,
				Price:       Price{PriceAmount: "1.00", CurrencyName: "USD"},
			},
		},
	}
	if err := cs.write(cat); err != nil {
		t.Fatal(err)
	}

	csvBody := "Item,Location,Number of Items,Month,Source,Gross,Average,Paid Out\n" +
		"ADIDAS Grey and black athletic shorts,Dallas - CC,1,2025-07-01,Evan,4.41,,\n"
	rows, err := ReadBooksCSV(strings.NewReader(csvBody))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows=%d", len(rows))
	}

	res, err := cs.ApplyBooksSeed(rows, BooksMatchThreshold)
	if err != nil {
		t.Fatal(err)
	}
	if res.Matched != 1 || res.Applied != 1 {
		t.Fatalf("res=%+v", res)
	}

	got, err := cs.Load()
	if err != nil {
		t.Fatal(err)
	}
	var p Product
	for _, x := range got.Products {
		if x.ID == 1 {
			p = x
		}
	}
	if p.ListedFor != "Evan" || p.Location != "Dallas - CC" || p.Gross != "4.41" {
		t.Fatalf("books fields: %+v", p)
	}
	if p.BooksTitle != "ADIDAS Grey and black athletic shorts" {
		t.Fatalf("title %q", p.BooksTitle)
	}
	if p.SoldAt == nil || p.SoldAt.Format("2006-01") != "2025-07" {
		t.Fatalf("sold_at %v", p.SoldAt)
	}

	var buf bytes.Buffer
	n, err := WriteSoldCSV(&buf, got.Products)
	if err != nil || n != 1 {
		t.Fatalf("export n=%d err=%v", n, err)
	}
	out := buf.String()
	if !strings.Contains(out, "Evan") || !strings.Contains(out, "4.41") || !strings.Contains(out, "Dallas - CC") {
		t.Fatalf("csv:\n%s", out)
	}
	if !strings.Contains(out, "2025-07-01") {
		t.Fatalf("month missing:\n%s", out)
	}
}

func TestShortBooksTitle(t *testing.T) {
	long := "UNIQLO dark green flannel buttondown with red details and more words #uniqlo #flannel"
	got := ShortBooksTitle(long)
	if strings.Contains(got, "#") {
		t.Fatalf("hashtag left: %q", got)
	}
	if len(got) > 45 {
		t.Fatalf("too long: %q (%d)", got, len(got))
	}
}

func TestApplyBooksLinks_replace(t *testing.T) {
	dir := t.TempDir()
	catalogPath := filepath.Join(dir, "catalog.json")
	soldAt := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	created := time.Date(2025, 5, 1, 12, 0, 0, 0, time.UTC)
	cs := NewCatalogStore(catalogPath)
	cat := &Catalog{
		SyncedAt: time.Now().UTC(),
		Products: []Product{
			{
				ItemID: "a", ID: 570717532, Status: StatusSold,
				Description: "ADIDAS Grey and black athletic shorts\nmore",
				CreatedAt:   created,
				SoldAt:      &soldAt,
				ListedFor:   "Alex",
				Location:    "NYC",
				Gross:       "4.47",
				BooksTitle:  "Grey Northyard athletic shorts",
				Price:       Price{PriceAmount: "6.00", CurrencyName: "USD"},
			},
		},
	}
	if err := cs.write(cat); err != nil {
		t.Fatal(err)
	}

	rows := []BooksRow{{
		Item: "ADIDAS Grey and black athletic shorts", Location: "Dallas - CC",
		Source: "Evan", Gross: "4.41", Month: time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC),
	}}
	res, err := cs.ApplyBooksLinks(rows, []BooksLink{{
		ProductID: 570717532,
		Item:      "ADIDAS Grey and black athletic shorts",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Applied != 1 {
		t.Fatalf("applied=%d", res.Applied)
	}
	got, err := cs.Load()
	if err != nil {
		t.Fatal(err)
	}
	p := got.Products[0]
	if p.BooksTitle != "ADIDAS Grey and black athletic shorts" || p.ListedFor != "Evan" || p.Gross != "4.41" {
		t.Fatalf("got %+v", p)
	}
	if p.Location != "Dallas - CC" {
		t.Fatalf("location %q", p.Location)
	}
}

func TestAcceptBooksReviewSuspects(t *testing.T) {
	dir := t.TempDir()
	cs := NewCatalogStore(filepath.Join(dir, "catalog.json"))
	created := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	soldAt := time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC)
	cat := &Catalog{
		SyncedAt: time.Now().UTC(),
		Products: []Product{
			{
				ItemID: "s1", ID: 10, Status: StatusSold,
				Description: "Completely different listing text about a coat",
				BooksTitle:  "multicolor stripe scarf",
				CreatedAt:   created,
				SoldAt:      &soldAt,
				Price:       Price{PriceAmount: "10.00", CurrencyName: "USD"},
			},
			{
				ItemID: "ok", ID: 11, Status: StatusSold,
				Description: "ADIDAS Grey and black athletic shorts for running",
				BooksTitle:  "ADIDAS Grey and black athletic shorts",
				CreatedAt:   created,
				SoldAt:      &soldAt,
				Price:       Price{PriceAmount: "6.00", CurrencyName: "USD"},
			},
		},
	}
	if err := cs.write(cat); err != nil {
		t.Fatal(err)
	}

	rev := BuildBooksReview(cat, nil, BooksMatchThreshold)
	if len(rev.Suspects) != 1 || rev.Suspects[0].ProductID != 10 {
		t.Fatalf("suspects before: %+v", rev.Suspects)
	}

	n, err := cs.AcceptBooksReviewSuspects()
	if err != nil || n != 1 {
		t.Fatalf("accept n=%d err=%v", n, err)
	}
	got, err := cs.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !got.Products[0].BooksReviewOK {
		t.Fatal("expected books_review_ok on suspect")
	}
	if got.Products[1].BooksReviewOK {
		t.Fatal("good match should not need accept flag")
	}
	rev2 := BuildBooksReview(got, nil, BooksMatchThreshold)
	if len(rev2.Suspects) != 0 {
		t.Fatalf("suspects after accept: %+v", rev2.Suspects)
	}
}
