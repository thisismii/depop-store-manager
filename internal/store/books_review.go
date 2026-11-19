package store

import (
	"fmt"
	"html"
	"io"
	"sort"
	"strings"
)

// ReviewCandidate is a possible catalog match for a books row.
type ReviewCandidate struct {
	Score      float64
	ProductID  int64
	ItemID     string
	FirstLine  string
	ListPrice  string
	SoldAt     string
	ListingURL string
	AlreadySeeded bool
	BooksTitle string
	ListedFor  string
	Gross      string
	Location   string
}

// UnmatchedReview is a books row with no (or weak) automatic seed.
type UnmatchedReview struct {
	Num        int
	Item       string
	Location   string
	Source     string
	Month      string
	Gross      string
	Candidates []ReviewCandidate
}

// SuspectSeed is a seeded product whose books_title looks unlike the listing.
type SuspectSeed struct {
	Num        int
	Score      float64
	ProductID  int64
	ItemID     string
	BooksTitle string
	FirstLine  string
	ListedFor  string
	Location   string
	Gross      string
	ListPrice  string
	SoldAt     string
	ListingURL string
}

// BooksReview is data for the HTML confirmation page.
type BooksReview struct {
	Suspects  []SuspectSeed
	Unmatched []UnmatchedReview
	SeededN   int
	SoldN     int
	BooksN    int
}

// BuildBooksReview compares books CSV rows to the catalog.
func BuildBooksReview(cat *Catalog, rows []BooksRow, threshold float64) BooksReview {
	if threshold <= 0 {
		threshold = BooksMatchThreshold
	}
	if cat == nil {
		return BooksReview{}
	}
	sold := make([]Product, 0)
	seededTitles := map[string]struct{}{}
	seededN := 0
	for _, p := range cat.Products {
		if p.Status != StatusSold {
			continue
		}
		sold = append(sold, p)
		if strings.TrimSpace(p.BooksTitle) != "" || strings.TrimSpace(p.Gross) != "" || strings.TrimSpace(p.ListedFor) != "" {
			seededN++
		}
		if t := normalizeBooks(p.BooksTitle); t != "" {
			seededTitles[t] = struct{}{}
		}
	}

	var suspects []SuspectSeed
	for _, p := range sold {
		bt := strings.TrimSpace(p.BooksTitle)
		if bt == "" || p.BooksReviewOK {
			continue
		}
		if !isBooksSuspect(p) {
			continue
		}
		fl := firstLine(p.Description)
		jac := titleDescJaccard(bt, fl)
		suspects = append(suspects, SuspectSeed{
			Score:      jac,
			ProductID:  p.ID,
			ItemID:     p.ItemID,
			BooksTitle: bt,
			FirstLine:  fl,
			ListedFor:  p.ListedFor,
			Location:   p.Location,
			Gross:      p.Gross,
			ListPrice:  p.Price.PriceAmount,
			SoldAt:     formatSoldDay(p),
			ListingURL: p.ListingURL(),
		})
	}
	sort.Slice(suspects, func(i, j int) bool {
		if suspects[i].Score == suspects[j].Score {
			return suspects[i].ProductID < suspects[j].ProductID
		}
		return suspects[i].Score < suspects[j].Score
	})
	for i := range suspects {
		suspects[i].Num = i + 1
	}

	var unmatched []UnmatchedReview
	num := 0
	for _, row := range rows {
		if _, ok := seededTitles[normalizeBooks(row.Item)]; ok {
			continue
		}
		num++
		cands := topCandidates(row.Item, sold, 3)
		ur := UnmatchedReview{
			Num:      num,
			Item:     row.Item,
			Location: row.Location,
			Source:   row.Source,
			Month:    "",
			Gross:    row.Gross,
			Candidates: cands,
		}
		if !row.Month.IsZero() {
			ur.Month = row.Month.Format("2006-01-02")
		}
		unmatched = append(unmatched, ur)
	}

	return BooksReview{
		Suspects:  suspects,
		Unmatched: unmatched,
		SeededN:   seededN,
		SoldN:     len(sold),
		BooksN:    len(rows),
	}
}

// AcceptBooksReviewSuspects marks currently flagged suspects as human-OK matches.
// They keep books fields and drop out of the Suspect list on next review.
func (s *CatalogStore) AcceptBooksReviewSuspects() (int, error) {
	cat, err := s.Load()
	if err != nil {
		return 0, err
	}
	n := 0
	for i := range cat.Products {
		p := &cat.Products[i]
		if p.Status != StatusSold || p.BooksReviewOK {
			continue
		}
		if !isBooksSuspect(*p) {
			continue
		}
		p.BooksReviewOK = true
		n++
	}
	if n == 0 {
		return 0, nil
	}
	if err := s.write(cat); err != nil {
		return n, err
	}
	return n, nil
}

func isBooksSuspect(p Product) bool {
	bt := strings.TrimSpace(p.BooksTitle)
	if bt == "" {
		return false
	}
	jac := titleDescJaccard(bt, firstLine(p.Description))
	self := booksScore(bt, p)
	return jac < 0.28 || self < 0.55
}

func titleDescJaccard(title, desc string) float64 {
	a, b := setOf(tokens(title)), setOf(tokens(desc))
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter := 0
	for t := range a {
		if b[t] {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func topCandidates(title string, sold []Product, n int) []ReviewCandidate {
	type scored struct {
		sc float64
		p  Product
	}
	all := make([]scored, 0, len(sold))
	for _, p := range sold {
		all = append(all, scored{booksScore(title, p), p})
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].sc == all[j].sc {
			return all[i].p.ID < all[j].p.ID
		}
		return all[i].sc > all[j].sc
	})
	if n > len(all) {
		n = len(all)
	}
	out := make([]ReviewCandidate, 0, n)
	for _, s := range all[:n] {
		p := s.p
		seeded := strings.TrimSpace(p.BooksTitle) != "" || strings.TrimSpace(p.Gross) != "" || strings.TrimSpace(p.ListedFor) != ""
		out = append(out, ReviewCandidate{
			Score:         s.sc,
			ProductID:     p.ID,
			ItemID:        p.ItemID,
			FirstLine:     firstLine(p.Description),
			ListPrice:     p.Price.PriceAmount,
			SoldAt:        formatSoldDay(p),
			ListingURL:    p.ListingURL(),
			AlreadySeeded: seeded,
			BooksTitle:    p.BooksTitle,
			ListedFor:     p.ListedFor,
			Gross:         p.Gross,
			Location:      p.Location,
		})
	}
	return out
}

func formatSoldDay(p Product) string {
	if p.SoldAt == nil || p.SoldAt.IsZero() {
		return ""
	}
	return p.SoldAt.UTC().Format("2006-01-02")
}

func displayMoney(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "—"
	}
	return s
}

type priceHint struct {
	text      string
	className string
}

// priceCompareHint compares her sheet Gross (often ~net) to Depop list price.
func priceCompareHint(gross, list string) priceHint {
	g, okG := parseMoney(gross)
	l, okL := parseMoney(list)
	if !okG || !okL || l <= 0 {
		return priceHint{text: "", className: "price-na"}
	}
	ratio := g / l
	switch {
	case ratio >= 0.95 && ratio <= 1.05:
		return priceHint{text: "≈ same as list", className: "price-match"}
	case ratio >= 0.75 && ratio <= 0.95:
		return priceHint{
			text:      fmt.Sprintf("~%.0f%% of list (typical fees)", ratio*100),
			className: "price-ok",
		}
	case ratio > 1.05:
		return priceHint{
			text:      fmt.Sprintf("gross > list (%.0f%%) — check", ratio*100),
			className: "price-bad",
		}
	default:
		return priceHint{
			text:      fmt.Sprintf("only %.0f%% of list — weak", ratio*100),
			className: "price-bad",
		}
	}
}

func parseMoney(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "$")
	s = strings.ReplaceAll(s, ",", "")
	if s == "" {
		return 0, false
	}
	var v float64
	_, err := fmt.Sscanf(s, "%f", &v)
	if err != nil {
		return 0, false
	}
	return v, true
}

// WriteBooksReviewHTML writes a numbered confirmation page.
func WriteBooksReviewHTML(w io.Writer, rev BooksReview) error {
	esc := html.EscapeString
	var b strings.Builder
	b.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Books match review</title>
  <link rel="preconnect" href="https://fonts.googleapis.com" />
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
  <link href="https://fonts.googleapis.com/css2?family=IBM+Plex+Sans:wght@400;500;600&family=IBM+Plex+Serif:wght@500&display=swap" rel="stylesheet" />
  <link rel="stylesheet" href="/styles.css" />
  <link rel="stylesheet" href="/books-review.css" />
</head>
<body>
  <main class="review">
    <header class="page-header">
      <p class="eyebrow"><a href="/">← Dashboard</a></p>
      <h1 class="brand">Books match review</h1>
      <p class="subtitle">Confirm remaining gaps. Seeded nicknames that look different from Depop copy can be accepted as matches.</p>
      <p class="meta">books ` + fmt.Sprintf("%d", rev.BooksN) + ` · sold ` + fmt.Sprintf("%d", rev.SoldN) + ` · seeded ` + fmt.Sprintf("%d", rev.SeededN) + ` · suspects ` + fmt.Sprintf("%d", len(rev.Suspects)) + ` · gaps ` + fmt.Sprintf("%d", len(rev.Unmatched)) + `</p>
    </header>
`)

	b.WriteString(`    <section>
      <h2>S — Suspicious seeds</h2>
      <p class="hint">books_title looks unlike listing text. If they are still the right match, accept them so they leave this list.</p>
`)
	if len(rev.Suspects) == 0 {
		b.WriteString(`      <p class="empty">None flagged.</p>
`)
	} else {
		b.WriteString(`      <form method="post" action="/api/books-review/accept-suspects" class="accept-form">
        <button type="submit" class="primary">Accept all ` + fmt.Sprintf("%d", len(rev.Suspects)) + ` as matches</button>
      </form>
`)
	}
	for _, s := range rev.Suspects {
		b.WriteString(`      <article class="card suspect" id="S` + fmt.Sprintf("%d", s.Num) + `">
        <h3><span class="tag">S` + fmt.Sprintf("%d", s.Num) + `</span> Jaccard ` + fmt.Sprintf("%.2f", s.Score) + `</h3>
        <p><strong>Books title:</strong> ` + esc(s.BooksTitle) + `</p>
        <p><strong>Listing:</strong> ` + esc(s.FirstLine) + `</p>
        <p class="meta">` + esc(s.Location) + ` · ` + esc(s.ListedFor) + ` · sold ` + esc(s.SoldAt) + ` · id ` + fmt.Sprintf("%d", s.ProductID) + `</p>
        <p class="price-row"><strong>List:</strong> ` + esc(displayMoney(s.ListPrice)) +
			` · <strong>Sheet Gross (seeded):</strong> ` + esc(displayMoney(s.Gross)) +
			` <span class="` + priceCompareHint(s.Gross, s.ListPrice).className + `">` + esc(priceCompareHint(s.Gross, s.ListPrice).text) + `</span></p>
        <p><a class="depop" href="` + esc(s.ListingURL) + `" target="_blank" rel="noopener">Open on Depop</a></p>
      </article>
`)
	}
	b.WriteString(`    </section>

    <section>
      <h2>U — Unmatched books rows</h2>
      <p class="hint">No seed for this sheet line. Candidates C1–C3. Prefer unused (not already seeded).</p>
`)
	if len(rev.Unmatched) == 0 {
		b.WriteString(`      <p class="empty">All books rows seeded.</p>
`)
	}
	for _, u := range rev.Unmatched {
		b.WriteString(`      <article class="card gap" id="U` + fmt.Sprintf("%d", u.Num) + `">
        <h3><span class="tag">U` + fmt.Sprintf("%d", u.Num) + `</span> ` + esc(u.Item) + `</h3>
        <p class="meta">` + esc(u.Location) + ` · ` + esc(u.Source) + ` · ` + esc(u.Month) + `</p>
        <p class="price-row"><strong>Sheet Gross:</strong> ` + esc(displayMoney(u.Gross)) + ` <span class="hint-inline">(her books net)</span></p>
        <ol class="cands">
`)
		for i, c := range u.Candidates {
			label := fmt.Sprintf("C%d", i+1)
			seedNote := ""
			if c.AlreadySeeded {
				seedNote = ` <span class="warn">already seeded as ` + esc(c.BooksTitle) + `</span>`
			}
			priceHint := priceCompareHint(u.Gross, c.ListPrice)
			b.WriteString(`          <li id="U` + fmt.Sprintf("%d", u.Num) + label + `">
            <div><span class="tag">` + label + `</span> score ` + fmt.Sprintf("%.2f", c.Score) + seedNote + `</div>
            <div>` + esc(c.FirstLine) + `</div>
            <div class="price-row"><strong>List:</strong> ` + esc(displayMoney(c.ListPrice)) +
				` · <strong>Sheet Gross:</strong> ` + esc(displayMoney(u.Gross)) +
				` <span class="` + priceHint.className + `">` + esc(priceHint.text) + `</span></div>
            <div class="meta">sold ` + esc(c.SoldAt) + ` · id ` + fmt.Sprintf("%d", c.ProductID) + `</div>
            <div><a class="depop" href="` + esc(c.ListingURL) + `" target="_blank" rel="noopener">Open on Depop</a></div>
          </li>
`)
		}
		b.WriteString(`        </ol>
      </article>
`)
	}
	b.WriteString(`    </section>
  </main>
</body>
</html>
`)
	_, err := io.WriteString(w, b.String())
	return err
}
