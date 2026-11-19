package main

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"depop-store-manager/internal/rewrite"
	"depop-store-manager/internal/store"
)

const (
	staleDays   = store.DefaultStaleDays
	maxSyncBody = 8 << 20 // 8 MiB
)

// Shop identity from env only — never commit real values.
// Empty = demo/fixture mode; Sync still works from the logged-in extension.
var (
	shopID       string
	shopUsername string
)

type productRow struct {
	ItemID             string  `json:"item_id"`
	ID                 int64   `json:"id"`
	Slug               string  `json:"slug"`
	Description        string  `json:"description"`
	PendingDescription string  `json:"pending_description,omitempty"`
	CreatedAt          string  `json:"created_at"`
	FirstListedAt      string  `json:"first_listed_at"`
	SoldAt             *string `json:"sold_at"`
	Status             string  `json:"status"`
	ListedFor          string  `json:"listed_for"`
	Location           string  `json:"location"`
	Gross              string  `json:"gross,omitempty"`
	BooksTitle         string  `json:"books_title,omitempty"`
	RefreshCount       int     `json:"refresh_count"`
	CoverURL           string  `json:"cover_url"`
	PriceAmount        string  `json:"price_amount"`
	Currency           string  `json:"currency_name"`
	PriceLabel         string  `json:"price_label"`
	ListingURL         string  `json:"listing_url"`
	AgeDays            int     `json:"age_days"`
	FirstListedAgeDays int     `json:"first_listed_age_days"`
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8787", "listen address (local only)")
	root := flag.String("root", ".", "repo root (contains web/ and data/)")
	flag.Parse()

	shopID = strings.TrimSpace(os.Getenv("DEPOP_SHOP_ID"))
	shopUsername = strings.TrimSpace(os.Getenv("DEPOP_SHOP_USERNAME"))

	catalogPath := filepath.Join(*root, "data", "catalog.json")
	webDir := filepath.Join(*root, "web")
	cs := store.NewCatalogStore(catalogPath)
	rewriter := rewrite.NewProvider()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"status": "ok", "rewrite_provider": rewriter.Name()})
	})

	mux.HandleFunc("GET /api/catalog", func(w http.ResponseWriter, r *http.Request) {
		cat, err := cs.Load()
		if errors.Is(err, store.ErrNotSynced) {
			writeJSON(w, catalogResponse(nil, nil))
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, catalogResponse(cat, cat.Products))
	})

	mux.HandleFunc("POST /api/debug/raw-product", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		dir := filepath.Join(*root, "data")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		path := filepath.Join(dir, "last-raw-product.json")
		if err := os.WriteFile(path, append(body, '\n'), 0o644); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "path": path, "bytes": len(body)})
	})

	mux.HandleFunc("POST /api/debug/create-capture", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		dir := filepath.Join(*root, "data")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		path := filepath.Join(dir, "last-create-capture.json")
		if err := os.WriteFile(path, append(body, '\n'), 0o644); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "path": path, "bytes": len(body)})
	})

	mux.HandleFunc("POST /api/sync", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, maxSyncBody+1))
		if err != nil {
			http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
			return
		}
		if len(body) > maxSyncBody {
			http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
			return
		}
		req, err := store.ParseSyncPayload(body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		cat, err := cs.Merge(req.Products, time.Now().UTC(), req.Reconcile)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{
			"ok":            true,
			"synced_at":     cat.SyncedAt.Format(time.RFC3339),
			"product_count": cat.ProductCount,
			"total_rows":    len(cat.Products),
			"reconcile":     req.Reconcile,
			"dashboard_url": "http://127.0.0.1:8787/",
		})
	})

	mux.HandleFunc("POST /api/rewrite", handleRewrite(cs, rewriter))
	mux.HandleFunc("POST /api/rewrite/approve", handleRewriteApprove(cs))
	mux.HandleFunc("GET /api/export/sold.csv", handleExportSoldCSV(cs))
	mux.HandleFunc("GET /api/refresh", handleRefreshQueue(cs))
	mux.HandleFunc("GET /api/refresh/item", handleRefreshItem(cs))
	mux.HandleFunc("POST /api/refresh/plan", handleRefreshPlan(cs))
	mux.HandleFunc("GET /books-review", handleBooksReview(cs, filepath.Join(*root, "data", "books-seed.csv")))
	mux.HandleFunc("POST /api/books-review/accept-suspects", handleAcceptBooksSuspects(cs))

	mux.Handle("GET /", http.FileServer(http.Dir(webDir)))

	handler := withLocalCORS(mux)

	if shopID == "" || shopUsername == "" {
		log.Printf("depop-store-manager listening on http://%s (catalog %s, rewrite=%s, shop=demo — set DEPOP_SHOP_ID / DEPOP_SHOP_USERNAME for your shop)", *addr, catalogPath, rewriter.Name())
	} else {
		log.Printf("depop-store-manager listening on http://%s (catalog %s, rewrite=%s, shop=@%s)", *addr, catalogPath, rewriter.Name(), shopUsername)
	}
	if err := http.ListenAndServe(*addr, handler); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func handleBooksReview(cs *store.CatalogStore, booksCSV string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cat, err := cs.Load()
		if errors.Is(err, store.ErrNotSynced) {
			http.Error(w, "catalog not synced", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		f, err := os.Open(booksCSV)
		if err != nil {
			http.Error(w, "books seed csv missing: "+err.Error()+" (expected data/books-seed.csv)", http.StatusNotFound)
			return
		}
		defer f.Close()
		rows, err := store.ReadBooksCSV(f)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		rev := store.BuildBooksReview(cat, rows, store.BooksMatchThreshold)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := store.WriteBooksReviewHTML(w, rev); err != nil {
			log.Printf("books review: %v", err)
		}
	}
}

func handleAcceptBooksSuspects(cs *store.CatalogStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		n, err := cs.AcceptBooksReviewSuspects()
		if errors.Is(err, store.ErrNotSynced) {
			http.Error(w, "catalog not synced", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf("books review: accepted %d suspects as matches", n)
		http.Redirect(w, r, "/books-review", http.StatusSeeOther)
	}
}

func handleRefreshQueue(cs *store.CatalogStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cat, err := cs.Load()
		if errors.Is(err, store.ErrNotSynced) {
			writeJSON(w, map[string]any{
				"shop_id":       shopID,
				"shop_username": shopUsername,
				"stale_days":    staleDays,
				"create_url":    store.CreateListingURL,
				"count":         0,
				"items":         []store.RefreshQueueItem{},
			})
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		items := store.RefreshQueue(cat.Products, time.Now().UTC(), staleDays)
		writeJSON(w, map[string]any{
			"shop_id":       shopID,
			"shop_username": shopUsername,
			"stale_days":    staleDays,
			"create_url":    store.CreateListingURL,
			"synced_at":     cat.SyncedAt.Format(time.RFC3339),
			"count":         len(items),
			"items":         items,
		})
	}
}

func handleRefreshItem(cs *store.CatalogStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		itemID := strings.TrimSpace(r.URL.Query().Get("item_id"))
		if itemID == "" {
			http.Error(w, "item_id required", http.StatusBadRequest)
			return
		}
		p, err := cs.FindByItemID(itemID)
		if err != nil {
			if errors.Is(err, store.ErrNotSynced) {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			status := http.StatusNotFound
			if strings.Contains(err.Error(), "required") {
				status = http.StatusBadRequest
			}
			http.Error(w, err.Error(), status)
			return
		}
		writeJSON(w, map[string]any{
			"ok":            true,
			"shop_username": shopUsername,
			"item":          store.RefreshPack(*p, time.Now().UTC()),
		})
	}
}

func handleRefreshPlan(cs *store.CatalogStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ItemID      string `json:"item_id"`
			PriceAmount string `json:"price_amount"`
			ListingURL  string `json:"listing_url"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
			http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
			return
		}
		p, err := cs.SetRefreshPlan(req.ItemID, req.PriceAmount, req.ListingURL)
		if err != nil {
			if errors.Is(err, store.ErrNotSynced) {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			status := http.StatusBadRequest
			if strings.Contains(err.Error(), "not found") {
				status = http.StatusNotFound
			}
			http.Error(w, err.Error(), status)
			return
		}
		writeJSON(w, map[string]any{
			"ok":                   true,
			"item_id":              p.ItemID,
			"pending_price":        p.PendingPrice,
			"pending_refresh_slug": p.PendingRefreshSlug,
			"note":                 "Saved locally. After the new listing is live, Sync retargets this item. Then delete the old listing on Depop.",
		})
	}
}

func handleExportSoldCSV(cs *store.CatalogStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cat, err := cs.Load()
		if errors.Is(err, store.ErrNotSynced) {
			http.Error(w, "catalog not synced", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		name := "depop-sold-" + time.Now().UTC().Format("2006-01-02") + ".csv"
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
		if _, err := store.WriteSoldCSV(w, cat.Products); err != nil {
			log.Printf("export sold csv: %v", err)
		}
	}
}

func handleRewrite(cs *store.CatalogStore, rewriter rewrite.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ItemID    string `json:"item_id"`
			ProductID int64  `json:"product_id"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
			http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
			return
		}
		var (
			p   *store.Product
			err error
		)
		switch {
		case strings.TrimSpace(req.ItemID) != "":
			p, err = cs.FindByItemID(strings.TrimSpace(req.ItemID))
		case req.ProductID != 0:
			p, err = cs.FindByDepopID(req.ProductID)
		default:
			http.Error(w, "item_id or product_id required", http.StatusBadRequest)
			return
		}
		if err != nil {
			if errors.Is(err, store.ErrNotSynced) {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			status := http.StatusNotFound
			if strings.Contains(err.Error(), "required") {
				status = http.StatusBadRequest
			}
			http.Error(w, err.Error(), status)
			return
		}
		if strings.TrimSpace(p.Description) == "" {
			http.Error(w, "listing has empty description", http.StatusBadRequest)
			return
		}
		draft, err := rewriter.Rewrite(r.Context(), p.Description)
		if err != nil {
			http.Error(w, "rewrite failed: "+err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, map[string]any{
			"ok":         true,
			"item_id":    p.ItemID,
			"product_id": p.ID,
			"original":   p.Description,
			"draft":      draft,
			"provider":   rewriter.Name(),
			"pending":    p.PendingDescription,
		})
	}
}

func handleRewriteApprove(cs *store.CatalogStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ItemID string `json:"item_id"`
			Draft  string `json:"draft"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
			http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
			return
		}
		p, err := cs.SetPendingDescription(req.ItemID, req.Draft)
		if err != nil {
			if errors.Is(err, store.ErrNotSynced) {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			status := http.StatusBadRequest
			if strings.Contains(err.Error(), "not found") {
				status = http.StatusNotFound
			}
			http.Error(w, err.Error(), status)
			return
		}
		writeJSON(w, map[string]any{
			"ok":                  true,
			"item_id":             p.ItemID,
			"product_id":          p.ID,
			"pending_description": p.PendingDescription,
			"note":                "Saved locally only. Does not update Depop. Use Copy to paste into Depop.",
		})
	}
}

func catalogResponse(cat *store.Catalog, products []store.Product) map[string]any {
	now := time.Now().UTC()
	var syncedAt any
	productCount := 0
	totalRows := 0
	if cat != nil {
		syncedAt = cat.SyncedAt.Format(time.RFC3339)
		productCount = cat.ProductCount
		totalRows = len(cat.Products)
	}
	ordered := store.SortByFirstListedOldest(products)
	rows := make([]productRow, 0, len(ordered))
	for _, p := range ordered {
		status := p.Status
		if status == "" {
			status = store.StatusActive
		}
		firstListed := p.FirstListedAt
		if firstListed.IsZero() {
			firstListed = p.CreatedAt
		}
		var soldAt *string
		if p.SoldAt != nil && !p.SoldAt.IsZero() {
			s := p.SoldAt.UTC().Format(time.RFC3339)
			soldAt = &s
		}
		rows = append(rows, productRow{
			ItemID:             p.ItemID,
			ID:                 p.ID,
			Slug:               p.Slug,
			Description:        p.Description,
			PendingDescription: p.PendingDescription,
			CreatedAt:          p.CreatedAt.Format(time.RFC3339),
			FirstListedAt:      firstListed.Format(time.RFC3339),
			SoldAt:             soldAt,
			Status:             status,
			ListedFor:          p.ListedFor,
			Location:           p.Location,
			Gross:              p.Gross,
			BooksTitle:         p.BooksTitle,
			RefreshCount:       p.RefreshCount,
			CoverURL:           p.CoverURL,
			PriceAmount:        p.Price.PriceAmount,
			Currency:           p.Price.CurrencyName,
			PriceLabel:         p.PriceLabel(),
			ListingURL:         p.ListingURL(),
			AgeDays:            p.AgeDays(now),
			FirstListedAgeDays: p.FirstListedAgeDays(now),
		})
	}
	return map[string]any{
		"shop_id":       shopID,
		"shop_username": shopUsername,
		"stale_days":    staleDays,
		"synced_at":     syncedAt,
		"product_count": productCount,
		"total_rows":    totalRows,
		"count":         len(rows),
		"products":      rows,
	}
}

func withLocalCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		log.Printf("write json: %v", err)
	}
}
