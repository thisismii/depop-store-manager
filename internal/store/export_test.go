package store

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
	"time"
)

func TestWriteSoldCSV(t *testing.T) {
	soldEarly := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	soldLate := time.Date(2026, 9, 14, 3, 0, 0, 0, time.UTC)
	products := []Product{
		{
			ID: 2, Status: StatusActive,
			Description: "Active should skip",
			CreatedAt:   soldLate,
			Price:       Price{PriceAmount: "99.00", CurrencyName: "USD"},
		},
		{
			ID: 4, Status: StatusSold,
			Description: "Unseeded sold should skip\nbody",
			CreatedAt:   soldLate,
			SoldAt:      &soldLate,
			Price:       Price{PriceAmount: "12.00", CurrencyName: "USD"},
		},
		{
			ID: 3, Status: StatusSold,
			Description: "Later sale\nbody",
			CreatedAt:   soldLate,
			SoldAt:      &soldLate,
			ListedFor:   "Alex",
			BooksTitle:  "Later sale",
			Gross:       "33.26",
			Price:       Price{PriceAmount: "35.00", CurrencyName: "USD"},
		},
		{
			ID: 1, Status: StatusSold,
			Description: "Early sale title\nmore",
			CreatedAt:   soldEarly,
			SoldAt:      &soldEarly,
			BooksTitle:  "Early sale title",
			Gross:       "16.53",
			Price:       Price{PriceAmount: "20.00", CurrencyName: "USD"},
		},
	}

	var buf bytes.Buffer
	n, err := WriteSoldCSV(&buf, products)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("rows = %d, want 2", n)
	}

	got := buf.String()
	wantHeader := strings.Join(SoldSheetHeaders, ",")
	if !strings.HasPrefix(got, wantHeader+"\n") {
		t.Fatalf("header:\n%s", got)
	}
	records, err := csv.NewReader(strings.NewReader(got)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 {
		t.Fatalf("record count %d:\n%s", len(records), got)
	}
	// Oldest sold first; books title + net gross + qty 1.0
	if records[1][0] != "Early sale title" {
		t.Fatalf("row1 item = %q", records[1][0])
	}
	if records[1][2] != "1.0" {
		t.Fatalf("row1 qty = %q", records[1][2])
	}
	if records[1][3] != "2026-04-01" {
		t.Fatalf("row1 month = %q", records[1][3])
	}
	if records[1][5] != "16.53" {
		t.Fatalf("row1 gross = %q", records[1][5])
	}
	if records[2][0] != "Later sale" {
		t.Fatalf("row2 item = %q", records[2][0])
	}
	if records[2][2] != "1.0" {
		t.Fatalf("row2 qty = %q", records[2][2])
	}
	if records[2][4] != "Alex" {
		t.Fatalf("row2 source = %q", records[2][4])
	}
	if records[2][5] != "33.26" {
		t.Fatalf("row2 gross = %q", records[2][5])
	}
	if strings.Contains(got, "Unseeded") {
		t.Fatalf("unseeded sold leaked:\n%s", got)
	}
}

func TestWriteSoldCSV_headerOnly(t *testing.T) {
	var buf bytes.Buffer
	n, err := WriteSoldCSV(&buf, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("n = %d", n)
	}
	got := strings.TrimSpace(buf.String())
	want := strings.Join(SoldSheetHeaders, ",")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
