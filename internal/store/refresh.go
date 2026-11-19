package store

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	CreateListingURL    = "https://www.depop.com/products/create/"
	SuggestPctStale     = 15
	SuggestPctVeryStale = 25
	VeryStaleDays       = 120
	maxDepopIDHistory   = 8
)

// PriceSuggestion is a human-gated hint. Never auto-apply.
type PriceSuggestion struct {
	Current   string
	Suggested string
	Percent   int
	Reason    string
}

// PostDescription returns the copy to paste on the new listing.
func (p Product) PostDescription() string {
	if s := strings.TrimSpace(p.PendingDescription); s != "" {
		return s
	}
	return p.Description
}

// SuggestRefreshPrice keeps current price as the default action.
// 15% / 25% match Depop's native reprice presets and are hints only.
func SuggestRefreshPrice(amount string, ageDays int) PriceSuggestion {
	cur := strings.TrimSpace(amount)
	out := PriceSuggestion{
		Current:   cur,
		Suggested: cur,
		Percent:   0,
		Reason:    "Keep current price unless you choose a cut. No auto discount.",
	}
	if ageDays >= VeryStaleDays {
		if cut, err := ApplyDiscountPercent(cur, SuggestPctVeryStale); err == nil {
			out.Suggested = cut
			out.Percent = SuggestPctVeryStale
			out.Reason = "Listed 120+ days. Depop native reprice offers 25% off."
		}
	} else if ageDays >= DefaultStaleDays {
		if cut, err := ApplyDiscountPercent(cur, SuggestPctStale); err == nil {
			out.Suggested = cut
			out.Percent = SuggestPctStale
			out.Reason = "Listed 60+ days. Depop native reprice floor is 15% off."
		}
	}
	return out
}

// ApplyDiscountPercent returns amount * (100-pct)/100, rounded to cents.
func ApplyDiscountPercent(amount string, pct int) (string, error) {
	cents, err := ParseCents(amount)
	if err != nil {
		return "", err
	}
	if pct <= 0 {
		return FormatCents(cents), nil
	}
	if pct >= 100 {
		return FormatCents(0), nil
	}
	discounted := (cents*int64(100-pct) + 50) / 100
	return FormatCents(discounted), nil
}

func ParseCents(amount string) (int64, error) {
	s := strings.TrimSpace(strings.ReplaceAll(amount, ",", ""))
	if s == "" {
		return 0, fmt.Errorf("empty amount")
	}
	neg := false
	if strings.HasPrefix(s, "-") {
		neg = true
		s = strings.TrimPrefix(s, "-")
	}
	parts := strings.SplitN(s, ".", 2)
	dollars, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("amount: %w", err)
	}
	var cents int64
	if len(parts) == 2 {
		frac := parts[1]
		if len(frac) == 1 {
			frac += "0"
		}
		if len(frac) > 2 {
			frac = frac[:2]
		}
		cents, err = strconv.ParseInt(frac, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("amount cents: %w", err)
		}
	}
	total := dollars*100 + cents
	if neg {
		total = -total
	}
	return total, nil
}

func FormatCents(cents int64) string {
	neg := cents < 0
	if neg {
		cents = -cents
	}
	s := fmt.Sprintf("%d.%02d", cents/100, cents%100)
	if neg {
		return "-" + s
	}
	return s
}

// ListingRef is a pasted Depop product URL, slug, or numeric id.
type ListingRef struct {
	Slug string
	ID   int64
}

func ParseListingRef(raw string) (ListingRef, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ListingRef{}, fmt.Errorf("listing url required")
	}
	if !strings.Contains(s, "://") && strings.Contains(s, ".") && strings.Contains(s, "/") {
		s = "https://" + s
	}
	if strings.Contains(s, "://") {
		u, err := url.Parse(s)
		if err != nil {
			return ListingRef{}, fmt.Errorf("listing url: %w", err)
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		for i, p := range parts {
			if p == "products" && i+1 < len(parts) {
				next := parts[i+1]
				if next == "create" || next == "edit" {
					return ListingRef{}, fmt.Errorf("not a listing url")
				}
				return ListingRef{Slug: NormalizeSlug(next)}, nil
			}
		}
		return ListingRef{}, fmt.Errorf("listing url missing /products/<slug>")
	}
	if id, err := strconv.ParseInt(s, 10, 64); err == nil && id > 0 {
		return ListingRef{ID: id}, nil
	}
	slug := NormalizeSlug(s)
	if slug == "" || slug == "create" || slug == "edit" {
		return ListingRef{}, fmt.Errorf("invalid listing slug")
	}
	return ListingRef{Slug: slug}, nil
}

func NormalizeSlug(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "/")
	if i := strings.LastIndex(s, "/"); i >= 0 {
		s = s[i+1:]
	}
	return s
}

// SetRefreshPlan records the price you will post and the new listing slug
// (from a pasted URL). Next Sync retargets item_id when that slug appears.
func (s *CatalogStore) SetRefreshPlan(itemID, priceAmount, listingRaw string) (*Product, error) {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return nil, fmt.Errorf("item_id required")
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
	p := &cat.Products[idx]
	if status := normalizeStatus(p.Status); status != "" && status != StatusActive {
		return nil, fmt.Errorf("refresh only for active listings")
	}
	priceAmount = strings.TrimSpace(priceAmount)
	if priceAmount != "" {
		if _, err := ParseCents(priceAmount); err != nil {
			return nil, err
		}
		p.PendingPrice = FormatCentsMust(priceAmount)
	}
	if strings.TrimSpace(listingRaw) != "" {
		ref, err := ParseListingRef(listingRaw)
		if err != nil {
			return nil, err
		}
		if ref.Slug == "" {
			return nil, fmt.Errorf("paste the new listing URL (needs the slug)")
		}
		p.PendingRefreshSlug = ref.Slug
	}
	if p.PendingRefreshSlug == "" && p.PendingPrice == "" {
		return nil, fmt.Errorf("price or new listing url required")
	}
	if err := s.write(cat); err != nil {
		return nil, err
	}
	out := *p
	return &out, nil
}

func FormatCentsMust(amount string) string {
	c, err := ParseCents(amount)
	if err != nil {
		return strings.TrimSpace(amount)
	}
	return FormatCents(c)
}

// RefreshQueueItem is one stale row for the price-review / refresh page.
type RefreshQueueItem struct {
	ItemID             string   `json:"item_id"`
	ProductID          int64    `json:"product_id"`
	Slug               string   `json:"slug"`
	CoverURL           string   `json:"cover_url"`
	PhotoURLs          []string `json:"photo_urls"`
	Description        string   `json:"description"`
	PostDescription    string   `json:"post_description"`
	PendingDescription string   `json:"pending_description,omitempty"`
	AgeDays            int      `json:"age_days"`
	PriceAmount        string   `json:"price_amount"`
	Currency           string   `json:"currency_name"`
	SuggestedAmount    string   `json:"suggested_amount"`
	SuggestedPercent   int      `json:"suggested_percent"`
	SuggestedReason    string   `json:"suggested_reason"`
	PendingPrice       string   `json:"pending_price,omitempty"`
	PendingRefreshSlug string   `json:"pending_refresh_slug,omitempty"`
	RefreshCount       int      `json:"refresh_count"`
	ListingURL         string   `json:"listing_url"`
	CreateURL          string   `json:"create_url"`
	OldListingURL      string   `json:"old_listing_url"`
}

func RefreshQueue(products []Product, now time.Time, staleDays int) []RefreshQueueItem {
	stale := Stale(products, now, staleDays)
	out := make([]RefreshQueueItem, 0, len(stale))
	for _, p := range stale {
		out = append(out, RefreshPack(p, now))
	}
	return out
}

func RefreshPack(p Product, now time.Time) RefreshQueueItem {
	age := p.AgeDays(now)
	sug := SuggestRefreshPrice(p.Price.PriceAmount, age)
	photos := append([]string(nil), p.PhotoURLs...)
	return RefreshQueueItem{
		ItemID:             p.ItemID,
		ProductID:          p.ID,
		Slug:               p.Slug,
		CoverURL:           p.CoverURL,
		PhotoURLs:          photos,
		Description:        p.Description,
		PostDescription:    p.PostDescription(),
		PendingDescription: p.PendingDescription,
		AgeDays:            age,
		PriceAmount:        p.Price.PriceAmount,
		Currency:           p.Price.CurrencyName,
		SuggestedAmount:    sug.Suggested,
		SuggestedPercent:   sug.Percent,
		SuggestedReason:    sug.Reason,
		PendingPrice:       p.PendingPrice,
		PendingRefreshSlug: p.PendingRefreshSlug,
		RefreshCount:       p.RefreshCount,
		ListingURL:         p.ListingURL(),
		CreateURL:          CreateListingURL,
		OldListingURL:      p.ListingURL(),
	}
}
