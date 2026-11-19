package store

import (
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// SoldSheetHeaders mirrors a typical Depop Google Sheet column order.
var SoldSheetHeaders = []string{
	"Item",
	"Location",
	"Number of Items",
	"Month",
	"Source",
	"Gross",
	"Average",
	"Paid Out",
}

// WriteSoldCSV writes sold rows in her Depop-sheet shape (UTF-8 CSV).
// Only seeded sold rows (non-empty books_title). Uses books fields for
// title, location, source, gross, sold month. Number of Items is "1.0".
// Average / Paid Out stay blank. Sorted oldest sold month first.
func WriteSoldCSV(w io.Writer, products []Product) (int, error) {
	cw := csv.NewWriter(w)
	if err := cw.Write(SoldSheetHeaders); err != nil {
		return 0, fmt.Errorf("write header: %w", err)
	}

	sold := make([]Product, 0, len(products))
	for _, p := range products {
		if p.Status == StatusSold && strings.TrimSpace(p.BooksTitle) != "" {
			sold = append(sold, p)
		}
	}
	sort.Slice(sold, func(i, j int) bool {
		ti := soldSortTime(sold[i])
		tj := soldSortTime(sold[j])
		if ti.Equal(tj) {
			return sold[i].ID < sold[j].ID
		}
		return ti.Before(tj)
	})

	for _, p := range sold {
		row := []string{
			ExportItemTitle(p),
			strings.TrimSpace(p.Location),
			"1.0",
			monthBucket(p),
			strings.TrimSpace(p.ListedFor),
			ExportGross(p),
			"", // Average
			"", // Paid Out
		}
		if err := cw.Write(row); err != nil {
			return 0, fmt.Errorf("write row: %w", err)
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return 0, err
	}
	return len(sold), nil
}

func monthBucket(p Product) string {
	t := p.CreatedAt
	if p.SoldAt != nil && !p.SoldAt.IsZero() {
		t = *p.SoldAt
	}
	t = t.UTC()
	if t.IsZero() {
		return ""
	}
	first := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	return first.Format("2006-01-02")
}
