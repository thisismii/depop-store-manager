package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const DefaultStaleDays = 60

const (
	StatusActive = "active"
	StatusSold   = "sold"
	StatusGone   = "gone"
)

// ErrNotSynced means no catalog file exists yet.
var ErrNotSynced = errors.New("catalog not synced")

// Product is a listing row in local snake_case store shape.
type Product struct {
	ItemID             string     `json:"item_id"`
	ID                 int64      `json:"id"`
	Slug               string     `json:"slug"`
	Description        string     `json:"description"`
	PendingDescription string     `json:"pending_description,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	FirstListedAt      time.Time  `json:"first_listed_at"`
	SoldAt             *time.Time `json:"sold_at,omitempty"`
	Status             string     `json:"status"`
	ListedFor          string     `json:"listed_for"` // Source in her books
	Location           string     `json:"location"`
	Gross              string     `json:"gross,omitempty"` // net from her books when known
	BooksTitle         string     `json:"books_title,omitempty"`
	BooksReviewOK      bool       `json:"books_review_ok,omitempty"` // human confirmed nickname match
	RefreshCount       int        `json:"refresh_count"`
	CoverURL           string     `json:"cover_url"`
	PhotoURLs          []string   `json:"photo_urls,omitempty"`
	PendingRefreshSlug string     `json:"pending_refresh_slug,omitempty"`
	PendingPrice       string     `json:"pending_price,omitempty"`
	DepopIDHistory     []int64    `json:"depop_id_history,omitempty"`
	Price              Price      `json:"price"`
}

type Price struct {
	PriceAmount  string `json:"price_amount"`
	CurrencyName string `json:"currency_name"`
}

// Catalog is the on-disk live store under data/catalog.json.
type Catalog struct {
	SyncedAt     time.Time `json:"synced_at"`
	ProductCount int       `json:"product_count"` // active only
	Products     []Product `json:"products"`
}

// SyncRequest is a parsed POST /api/sync body.
type SyncRequest struct {
	Products  []Product
	Reconcile bool
}

type catalogFile struct {
	SyncedAt     string           `json:"synced_at"`
	ProductCount int              `json:"product_count"`
	Products     []productPayload `json:"products"`
}

type productPayload struct {
	ItemID             string   `json:"item_id"`
	ID                 int64    `json:"id"`
	Slug               string   `json:"slug"`
	Description        string   `json:"description"`
	PendingDescription string   `json:"pending_description,omitempty"`
	CreatedAt          string   `json:"created_at"`
	FirstListedAt      string   `json:"first_listed_at"`
	SoldAt             string   `json:"sold_at,omitempty"`
	Status             string   `json:"status"`
	ListedFor          string   `json:"listed_for"`
	Location           string   `json:"location"`
	Gross              string   `json:"gross,omitempty"`
	BooksTitle         string   `json:"books_title,omitempty"`
	BooksReviewOK      bool     `json:"books_review_ok,omitempty"`
	RefreshCount       int      `json:"refresh_count"`
	CoverURL           string   `json:"cover_url"`
	PhotoURLs          []string `json:"photo_urls,omitempty"`
	PendingRefreshSlug string   `json:"pending_refresh_slug,omitempty"`
	PendingPrice       string   `json:"pending_price,omitempty"`
	DepopIDHistory     []int64  `json:"depop_id_history,omitempty"`
	Price              Price    `json:"price"`
}

// CatalogStore reads and merges data/catalog.json.
type CatalogStore struct {
	path string
}

func NewCatalogStore(path string) *CatalogStore {
	return &CatalogStore{path: path}
}

func (s *CatalogStore) Path() string {
	return s.path
}

// Load returns the catalog, or ErrNotSynced if the file is missing.
// Migrates legacy rows missing item_id / first_listed_at / status.
func (s *CatalogStore) Load() (*Catalog, error) {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotSynced
		}
		return nil, fmt.Errorf("read catalog: %w", err)
	}
	var file catalogFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("parse catalog: %w", err)
	}
	syncedAt, err := time.Parse(time.RFC3339, file.SyncedAt)
	if err != nil {
		return nil, fmt.Errorf("synced_at: %w", err)
	}
	products, err := parseProducts(file.Products)
	if err != nil {
		return nil, err
	}
	migrated := false
	for i := range products {
		if migrateProduct(&products[i]) {
			migrated = true
		}
	}
	cat := &Catalog{
		SyncedAt:     syncedAt.UTC(),
		ProductCount: countActive(products),
		Products:     products,
	}
	if migrated {
		if err := s.write(cat); err != nil {
			return nil, err
		}
	} else if file.ProductCount != cat.ProductCount {
		// Fix stale product_count from pre-merge catalogs.
		cat.ProductCount = countActive(products)
	}
	return cat, nil
}

// Merge upserts incoming products by Depop id, preserving local identity fields.
// When reconcile is true, any local product whose Depop id is not in incoming
// is marked status=gone (row kept).
//
// Refresh: if incoming slug matches a local pending_refresh_slug, retarget that
// row's Depop id (keep item_id / first_listed_at). Incoming ids that only appear
// in depop_id_history are skipped so the old listing does not mint a new row.
func (s *CatalogStore) Merge(incoming []Product, syncedAt time.Time, reconcile bool) (*Catalog, error) {
	existing := map[int64]Product{}
	var orderedIDs []int64
	prev, err := s.Load()
	if err != nil && !errors.Is(err, ErrNotSynced) {
		return nil, err
	}
	if prev != nil {
		for _, p := range prev.Products {
			existing[p.ID] = p
			orderedIDs = append(orderedIDs, p.ID)
		}
	}

	pendingBySlug := map[string]int64{}
	historyIDs := map[int64]struct{}{}
	rebuildIndexes := func() {
		pendingBySlug = map[string]int64{}
		historyIDs = map[int64]struct{}{}
		for _, p := range existing {
			for _, hid := range p.DepopIDHistory {
				historyIDs[hid] = struct{}{}
			}
			if slug := NormalizeSlug(p.PendingRefreshSlug); slug != "" {
				pendingBySlug[slug] = p.ID
			}
		}
	}
	rebuildIndexes()

	replaceOrderedID := func(oldID, newID int64) {
		for i, id := range orderedIDs {
			if id == oldID {
				orderedIDs[i] = newID
				return
			}
		}
		orderedIDs = append(orderedIDs, newID)
	}

	seen := map[int64]struct{}{}
	for _, in := range incoming {
		if in.ID == 0 {
			return nil, fmt.Errorf("product missing id")
		}
		seen[in.ID] = struct{}{}
		status := normalizeStatus(in.Status)
		if status == "" {
			status = StatusActive
		}
		if old, ok := existing[in.ID]; ok {
			old.Slug = in.Slug
			old.Description = in.Description
			old.CreatedAt = in.CreatedAt.UTC()
			old.Price = in.Price
			old.Status = status
			if in.CoverURL != "" {
				old.CoverURL = in.CoverURL
			}
			if len(in.PhotoURLs) > 0 {
				old.PhotoURLs = append([]string(nil), in.PhotoURLs...)
			}
			// Keep item_id, first_listed_at, listed_for, location, gross,
			// books_title, books_review_ok, refresh_count, pending_description,
			// pending_refresh_slug, pending_price, history, sold_at.
			if old.ItemID == "" {
				old.ItemID = newItemID()
			}
			if old.FirstListedAt.IsZero() {
				old.FirstListedAt = old.CreatedAt
			}
			if status == StatusSold && old.SoldAt == nil {
				t := syncedAt.UTC()
				old.SoldAt = &t
			}
			existing[in.ID] = old
			continue
		}
		if ownerID, ok := pendingBySlug[NormalizeSlug(in.Slug)]; ok {
			old := existing[ownerID]
			retargeted := retargetProduct(old, in, status, syncedAt)
			delete(existing, ownerID)
			existing[retargeted.ID] = retargeted
			replaceOrderedID(ownerID, retargeted.ID)
			rebuildIndexes()
			continue
		}
		if _, retired := historyIDs[in.ID]; retired {
			continue
		}
		p := Product{
			ItemID:        newItemID(),
			ID:            in.ID,
			Slug:          in.Slug,
			Description:   in.Description,
			CreatedAt:     in.CreatedAt.UTC(),
			FirstListedAt: in.CreatedAt.UTC(),
			Status:        status,
			ListedFor:     "",
			Location:      "",
			RefreshCount:  0,
			CoverURL:      in.CoverURL,
			PhotoURLs:     append([]string(nil), in.PhotoURLs...),
			Price:         in.Price,
		}
		if status == StatusSold {
			t := syncedAt.UTC()
			p.SoldAt = &t
		}
		existing[in.ID] = p
		orderedIDs = append(orderedIDs, in.ID)
		rebuildIndexes()
	}

	if reconcile {
		for id, p := range existing {
			if _, ok := seen[id]; !ok && p.Status != StatusGone {
				if p.PendingRefreshSlug != "" {
					continue
				}
				if p.RefreshCount > 0 && syncedAt.Sub(p.CreatedAt) < 48*time.Hour {
					continue
				}
				p.Status = StatusGone
				existing[id] = p
			}
		}
	}

	products := make([]Product, 0, len(existing))
	emitted := map[int64]struct{}{}
	for _, id := range orderedIDs {
		if cur, ok := existing[id]; ok {
			products = append(products, cur)
			emitted[id] = struct{}{}
		}
	}
	for _, in := range incoming {
		if _, ok := emitted[in.ID]; ok {
			continue
		}
		if cur, ok := existing[in.ID]; ok {
			products = append(products, cur)
			emitted[in.ID] = struct{}{}
		}
	}
	for id, p := range existing {
		if _, ok := emitted[id]; !ok {
			products = append(products, p)
		}
	}

	cat := &Catalog{
		SyncedAt:     syncedAt.UTC(),
		ProductCount: countActive(products),
		Products:     products,
	}
	if err := s.write(cat); err != nil {
		return nil, err
	}
	return cat, nil
}

func retargetProduct(old Product, in Product, status string, syncedAt time.Time) Product {
	hist := append([]int64(nil), old.DepopIDHistory...)
	hist = append(hist, old.ID)
	if len(hist) > maxDepopIDHistory {
		hist = hist[len(hist)-maxDepopIDHistory:]
	}
	old.DepopIDHistory = hist
	old.ID = in.ID
	old.Slug = in.Slug
	old.Description = in.Description
	old.CreatedAt = in.CreatedAt.UTC()
	old.Price = in.Price
	old.Status = status
	if old.Status == "" {
		old.Status = StatusActive
	}
	if in.CoverURL != "" {
		old.CoverURL = in.CoverURL
	}
	if len(in.PhotoURLs) > 0 {
		old.PhotoURLs = append([]string(nil), in.PhotoURLs...)
	}
	old.RefreshCount++
	old.PendingRefreshSlug = ""
	old.PendingPrice = ""
	old.PendingDescription = ""
	if old.ItemID == "" {
		old.ItemID = newItemID()
	}
	if old.FirstListedAt.IsZero() {
		old.FirstListedAt = old.CreatedAt
	}
	if status == StatusSold && old.SoldAt == nil {
		t := syncedAt.UTC()
		old.SoldAt = &t
	}
	return old
}

// FindByItemID returns the product with the given local item_id.
func (s *CatalogStore) FindByItemID(itemID string) (*Product, error) {
	if itemID == "" {
		return nil, fmt.Errorf("item_id required")
	}
	cat, err := s.Load()
	if err != nil {
		return nil, err
	}
	for i := range cat.Products {
		if cat.Products[i].ItemID == itemID {
			p := cat.Products[i]
			return &p, nil
		}
	}
	return nil, fmt.Errorf("item_id not found")
}

// FindByDepopID returns the product with the given Depop product id.
func (s *CatalogStore) FindByDepopID(id int64) (*Product, error) {
	if id == 0 {
		return nil, fmt.Errorf("product_id required")
	}
	cat, err := s.Load()
	if err != nil {
		return nil, err
	}
	for i := range cat.Products {
		if cat.Products[i].ID == id {
			p := cat.Products[i]
			return &p, nil
		}
	}
	return nil, fmt.Errorf("product_id not found")
}

// SetPendingDescription stores an approved rewrite draft on the local row.
// Does not change description (Depop-synced field) or post to Depop.
func (s *CatalogStore) SetPendingDescription(itemID string, draft string) (*Product, error) {
	itemID = strings.TrimSpace(itemID)
	draft = strings.TrimSpace(draft)
	if itemID == "" {
		return nil, fmt.Errorf("item_id required")
	}
	if draft == "" {
		return nil, fmt.Errorf("draft required")
	}
	cat, err := s.Load()
	if err != nil {
		return nil, err
	}
	idx := -1
	for i := range cat.Products {
		if cat.Products[i].ItemID == itemID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, fmt.Errorf("item_id not found")
	}
	cat.Products[idx].PendingDescription = draft
	if err := s.write(cat); err != nil {
		return nil, err
	}
	p := cat.Products[idx]
	return &p, nil
}

// Replace is kept for tests/fixtures; prefers Merge for live Sync.
func (s *CatalogStore) Replace(products []Product, syncedAt time.Time) (*Catalog, error) {
	for i := range products {
		migrateProduct(&products[i])
		if products[i].Status == "" {
			products[i].Status = StatusActive
		}
	}
	cat := &Catalog{
		SyncedAt:     syncedAt.UTC(),
		ProductCount: countActive(products),
		Products:     products,
	}
	if err := s.write(cat); err != nil {
		return nil, err
	}
	return cat, nil
}

func (s *CatalogStore) write(cat *Catalog) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("mkdir data: %w", err)
	}
	file := catalogFile{
		SyncedAt:     cat.SyncedAt.UTC().Format(time.RFC3339),
		ProductCount: cat.ProductCount,
		Products:     make([]productPayload, 0, len(cat.Products)),
	}
	for _, p := range cat.Products {
		file.Products = append(file.Products, toPayload(p))
	}
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("encode catalog: %w", err)
	}
	raw = append(raw, '\n')
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return fmt.Errorf("write catalog tmp: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("rename catalog: %w", err)
	}
	return nil
}

func toPayload(p Product) productPayload {
	hist := p.DepopIDHistory
	if len(hist) == 0 {
		hist = nil
	}
	var soldAt string
	if p.SoldAt != nil && !p.SoldAt.IsZero() {
		soldAt = p.SoldAt.UTC().Format(time.RFC3339)
	}
	return productPayload{
		ItemID:             p.ItemID,
		ID:                 p.ID,
		Slug:               p.Slug,
		Description:        p.Description,
		PendingDescription: p.PendingDescription,
		CreatedAt:          p.CreatedAt.UTC().Format(time.RFC3339),
		FirstListedAt:      p.FirstListedAt.UTC().Format(time.RFC3339),
		SoldAt:             soldAt,
		Status:             p.Status,
		ListedFor:          p.ListedFor,
		Location:           p.Location,
		Gross:              p.Gross,
		BooksTitle:         p.BooksTitle,
		BooksReviewOK:      p.BooksReviewOK,
		RefreshCount:       p.RefreshCount,
		CoverURL:           p.CoverURL,
		PhotoURLs:          append([]string(nil), p.PhotoURLs...),
		PendingRefreshSlug: p.PendingRefreshSlug,
		PendingPrice:       p.PendingPrice,
		DepopIDHistory:     hist,
		Price:              p.Price,
	}
}

// ParseSyncPayload maps a Depop page (or our own shape) into a SyncRequest.
// Accepts either "products" (normalized) or Depop's "objects".
// Top-level "status" applies to products that omit status.
// Top-level "reconcile" marks missing local ids as gone after merge.
func ParseSyncPayload(raw []byte) (*SyncRequest, error) {
	var envelope struct {
		Products  []json.RawMessage `json:"products"`
		Objects   []json.RawMessage `json:"objects"`
		Status    string            `json:"status"`
		Reconcile bool              `json:"reconcile"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("parse sync payload: %w", err)
	}
	list := envelope.Products
	if list == nil {
		list = envelope.Objects
	}
	if list == nil {
		return nil, fmt.Errorf("sync payload missing products/objects array")
	}
	defaultStatus := normalizeStatus(envelope.Status)
	payloads := make([]productPayload, 0, len(list))
	for i, item := range list {
		p, err := normalizeProductJSON(item)
		if err != nil {
			return nil, fmt.Errorf("product[%d]: %w", i, err)
		}
		if p.Status == "" {
			p.Status = defaultStatus
		}
		payloads = append(payloads, p)
	}
	products, err := parseProducts(payloads)
	if err != nil {
		return nil, err
	}
	return &SyncRequest{Products: products, Reconcile: envelope.Reconcile}, nil
}

func normalizeProductJSON(raw json.RawMessage) (productPayload, error) {
	// Accept local snake_case, Selling Hub camelCase, and older presentation shapes.
	var live struct {
		ID          int64           `json:"id"`
		Slug        string          `json:"slug"`
		Description string          `json:"description"`
		CreatedAt   string          `json:"created_at"`
		DateCreated string          `json:"dateCreated"`
		Status      string          `json:"status"`
		CoverURL    string          `json:"cover_url"`
		Preview     string          `json:"preview"`
		ImageURL    string          `json:"imageUrl"`
		Thumbnail   string          `json:"thumbnail"`
		Currency    string          `json:"currency_name"`
		PhotoURLs   []string        `json:"photo_urls"`
		Pictures    json.RawMessage `json:"pictures"`
		Photos      json.RawMessage `json:"photos"`
		Images      json.RawMessage `json:"images"`
		Price       struct {
			PriceAmountSnake  string `json:"price_amount"`
			CurrencyNameSnake string `json:"currency_name"`
			PriceAmountCamel  string `json:"priceAmount"`
			CurrencyNameCamel string `json:"currencyName"`
		} `json:"price"`
		CurrentPrice struct {
			TotalPrice     string `json:"total_price"`
			PriceBreakdown struct {
				Price struct {
					Amount string `json:"amount"`
				} `json:"price"`
			} `json:"price_breakdown"`
		} `json:"current_price"`
	}
	if err := json.Unmarshal(raw, &live); err != nil {
		return productPayload{}, err
	}
	createdAt := live.CreatedAt
	if createdAt == "" {
		createdAt = live.DateCreated
	}
	amount := live.Price.PriceAmountSnake
	if amount == "" {
		amount = live.Price.PriceAmountCamel
	}
	if amount == "" {
		amount = live.CurrentPrice.PriceBreakdown.Price.Amount
	}
	if amount == "" {
		amount = live.CurrentPrice.TotalPrice
	}
	cur := live.Price.CurrencyNameSnake
	if cur == "" {
		cur = live.Price.CurrencyNameCamel
	}
	if cur == "" {
		cur = live.Currency
	}
	if cur == "" {
		cur = "USD"
	}
	cover := live.CoverURL
	if cover == "" {
		cover = live.Preview
	}
	if cover == "" {
		cover = live.ImageURL
	}
	if cover == "" {
		cover = live.Thumbnail
	}
	if cover == "" {
		cover = firstPictureURL(live.Pictures)
	}
	if cover == "" {
		cover = firstPictureURL(live.Photos)
	}
	if cover == "" {
		cover = firstPictureURL(live.Images)
	}
	photos := uniqueNonEmpty(live.PhotoURLs)
	if len(photos) == 0 {
		photos = pictureURLs(live.Pictures)
	}
	if len(photos) == 0 {
		photos = pictureURLs(live.Photos)
	}
	if len(photos) == 0 {
		photos = pictureURLs(live.Images)
	}
	if cover == "" && len(photos) > 0 {
		cover = photos[0]
	}
	return productPayload{
		ID:          live.ID,
		Slug:        live.Slug,
		Description: live.Description,
		CreatedAt:   createdAt,
		Status:      normalizeStatus(live.Status),
		CoverURL:    cover,
		PhotoURLs:   photos,
		Price: Price{
			PriceAmount:  amount,
			CurrencyName: cur,
		},
	}, nil
}

func firstPictureURL(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var asStrings []string
	if err := json.Unmarshal(raw, &asStrings); err == nil {
		for _, s := range asStrings {
			if s != "" {
				return s
			}
		}
	}
	var asObjects []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &asObjects); err != nil || len(asObjects) == 0 {
		urls := pictureURLs(raw)
		if len(urls) > 0 {
			return urls[0]
		}
		return ""
	}
	p := asObjects[0]
	if u := rawString(p["url"]); u != "" {
		return u
	}
	if u := rawString(p["preview"]); u != "" {
		return u
	}
	if formatsRaw, ok := p["formats"]; ok {
		var formats map[string]map[string]any
		if err := json.Unmarshal(formatsRaw, &formats); err == nil {
			for _, key := range []string{"P5", "P4", "P2", "P1", "P0", "P8", "P7", "P6", "P9"} {
				if f, ok := formats[key]; ok {
					if u, ok := f["url"].(string); ok && u != "" {
						return u
					}
				}
			}
			for _, f := range formats {
				if u, ok := f["url"].(string); ok && u != "" {
					return u
				}
			}
		}
	}
	return ""
}

// pictureURLs returns one largest URL per photo group.
func pictureURLs(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var asStrings []string
	if err := json.Unmarshal(raw, &asStrings); err == nil {
		if looksLikeURLList(asStrings) {
			return uniqueNonEmpty(asStrings)
		}
	}
	var elems []json.RawMessage
	if err := json.Unmarshal(raw, &elems); err != nil {
		return nil
	}
	var out []string
	for _, el := range elems {
		if u := bestPictureURL(el); u != "" {
			out = append(out, u)
		}
	}
	return uniqueNonEmpty(out)
}

func looksLikeURLList(ss []string) bool {
	if len(ss) == 0 {
		return false
	}
	for _, s := range ss {
		if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") || strings.Contains(s, "/") {
			return true
		}
	}
	return false
}

func bestPictureURL(raw json.RawMessage) string {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return ""
	}
	if s[0] == '"' {
		var u string
		if json.Unmarshal(raw, &u) == nil {
			return u
		}
	}
	if s[0] == '[' {
		var elems []json.RawMessage
		if json.Unmarshal(raw, &elems) != nil {
			return ""
		}
		best, bestW := "", -1
		for _, el := range elems {
			u, w := urlAndWidth(el)
			if u != "" && w >= bestW {
				best, bestW = u, w
			}
		}
		return best
	}
	if s[0] == '{' {
		u, _ := urlAndWidth(raw)
		return u
	}
	return ""
}

func urlAndWidth(raw json.RawMessage) (string, int) {
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return "", 0
	}
	u := rawString(obj["url"])
	if u == "" {
		u = rawString(obj["preview"])
	}
	w := rawInt(obj["width"])
	if formatsRaw, ok := obj["formats"]; ok {
		var formats map[string]map[string]any
		if json.Unmarshal(formatsRaw, &formats) == nil {
			for _, f := range formats {
				fu, _ := f["url"].(string)
				if fu == "" {
					continue
				}
				fw := 0
				switch n := f["width"].(type) {
				case float64:
					fw = int(n)
				}
				if fw >= w {
					u, w = fu, fw
				}
			}
		}
	}
	return u, w
}

func rawInt(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	var n float64
	if json.Unmarshal(raw, &n) == nil {
		return int(n)
	}
	return 0
}

func uniqueNonEmpty(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func rawString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return ""
}

func parseProducts(in []productPayload) ([]Product, error) {
	out := make([]Product, 0, len(in))
	for _, p := range in {
		if p.ID == 0 {
			return nil, fmt.Errorf("product missing id")
		}
		ts, err := parseCreatedAt(p.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("product %d created_at: %w", p.ID, err)
		}
		first := time.Time{}
		if p.FirstListedAt != "" {
			first, err = parseCreatedAt(p.FirstListedAt)
			if err != nil {
				return nil, fmt.Errorf("product %d first_listed_at: %w", p.ID, err)
			}
			first = first.UTC()
		}
		var soldAt *time.Time
		if p.SoldAt != "" {
			st, err := parseCreatedAt(p.SoldAt)
			if err != nil {
				return nil, fmt.Errorf("product %d sold_at: %w", p.ID, err)
			}
			st = st.UTC()
			soldAt = &st
		}
		status := normalizeStatus(p.Status)
		out = append(out, Product{
			ItemID:             p.ItemID,
			ID:                 p.ID,
			Slug:               p.Slug,
			Description:        p.Description,
			PendingDescription: p.PendingDescription,
			CreatedAt:          ts.UTC(),
			FirstListedAt:      first,
			SoldAt:             soldAt,
			Status:             status,
			ListedFor:          p.ListedFor,
			Location:           p.Location,
			Gross:              p.Gross,
			BooksTitle:         p.BooksTitle,
			BooksReviewOK:      p.BooksReviewOK,
			RefreshCount:       p.RefreshCount,
			CoverURL:           p.CoverURL,
			PhotoURLs:          append([]string(nil), p.PhotoURLs...),
			PendingRefreshSlug: p.PendingRefreshSlug,
			PendingPrice:       p.PendingPrice,
			DepopIDHistory:     append([]int64(nil), p.DepopIDHistory...),
			Price:              p.Price,
		})
	}
	return out, nil
}

func migrateProduct(p *Product) bool {
	changed := false
	if p.ItemID == "" {
		p.ItemID = newItemID()
		changed = true
	}
	if p.FirstListedAt.IsZero() {
		p.FirstListedAt = p.CreatedAt
		changed = true
	}
	if p.Status == "" {
		p.Status = StatusActive
		changed = true
	}
	if p.RefreshCount < 0 {
		p.RefreshCount = 0
		changed = true
	}
	return changed
}

func normalizeStatus(s string) string {
	switch s {
	case StatusActive, StatusSold, StatusGone:
		return s
	case "selling":
		return StatusActive
	default:
		return ""
	}
}

func newItemID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Extremely unlikely; fall back to time-based uniqueness.
		return fmt.Sprintf("item-%d", time.Now().UnixNano())
	}
	// UUID v4-ish layout without importing an external package.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

func countActive(products []Product) int {
	n := 0
	for _, p := range products {
		if p.Status == StatusActive || p.Status == "" {
			n++
		}
	}
	return n
}

func parseCreatedAt(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty")
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	}
	var last error
	for _, layout := range layouts {
		ts, err := time.Parse(layout, s)
		if err == nil {
			return ts, nil
		}
		last = err
	}
	return time.Time{}, last
}

// Active returns only active products.
func Active(products []Product) []Product {
	var out []Product
	for _, p := range products {
		if p.Status == StatusActive || p.Status == "" {
			out = append(out, p)
		}
	}
	return out
}

// WithStatus returns products matching status.
func WithStatus(products []Product, status string) []Product {
	var out []Product
	for _, p := range products {
		if p.Status == status {
			out = append(out, p)
		}
	}
	return out
}

// Stale returns active products at least staleDays old (by posting created_at), oldest first.
func Stale(products []Product, now time.Time, staleDays int) []Product {
	cutoff := now.UTC().AddDate(0, 0, -staleDays)
	var out []Product
	for _, p := range products {
		if p.Status != StatusActive && p.Status != "" {
			continue
		}
		if !p.CreatedAt.After(cutoff) {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

// SortByFirstListedOldest sorts by first_listed_at ascending (in-system age).
func SortByFirstListedOldest(products []Product) []Product {
	out := append([]Product(nil), products...)
	sort.Slice(out, func(i, j int) bool {
		ai := out[i].FirstListedAt
		if ai.IsZero() {
			ai = out[i].CreatedAt
		}
		aj := out[j].FirstListedAt
		if aj.IsZero() {
			aj = out[j].CreatedAt
		}
		if ai.Equal(aj) {
			return out[i].ID < out[j].ID
		}
		return ai.Before(aj)
	})
	return out
}

// SortSoldNewest sorts sold items by sold_at descending (fallback created_at).
func SortSoldNewest(products []Product) []Product {
	out := append([]Product(nil), products...)
	sort.Slice(out, func(i, j int) bool {
		ti := soldSortTime(out[i])
		tj := soldSortTime(out[j])
		if ti.Equal(tj) {
			return out[i].ID > out[j].ID
		}
		return ti.After(tj)
	})
	return out
}

func soldSortTime(p Product) time.Time {
	if p.SoldAt != nil && !p.SoldAt.IsZero() {
		return p.SoldAt.UTC()
	}
	return p.CreatedAt.UTC()
}

// SortOldestFirst sorts products by created_at ascending (posting age).
func SortOldestFirst(products []Product) []Product {
	out := append([]Product(nil), products...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

func (p Product) ListingURL() string {
	return fmt.Sprintf("https://www.depop.com/products/%s/", p.Slug)
}

func (p Product) PriceLabel() string {
	cur := p.Price.CurrencyName
	if cur == "" {
		cur = "USD"
	}
	return p.Price.PriceAmount + " " + cur
}

func (p Product) AgeDays(now time.Time) int {
	return ageDays(p.CreatedAt, now)
}

func (p Product) FirstListedAgeDays(now time.Time) int {
	t := p.FirstListedAt
	if t.IsZero() {
		t = p.CreatedAt
	}
	return ageDays(t, now)
}

func ageDays(from, now time.Time) int {
	d := now.UTC().Sub(from.UTC())
	days := int(d.Hours() / 24)
	if days < 0 {
		return 0
	}
	return days
}
