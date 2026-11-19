package store

import (
	"encoding/csv"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
	"unicode"
)

const BooksMatchThreshold = 0.45

// BooksRow is one line from a Depop sheet (CSV export).
type BooksRow struct {
	Item     string
	Location string
	Qty      string
	Month    time.Time // first of month when known
	Source   string
	Gross    string
}

// BooksSeedResult summarizes ApplyBooksSeed.
type BooksSeedResult struct {
	Rows      int
	Matched   int
	Applied   int
	Skipped   int
	Weak      []string // her item titles under threshold
	Ambiguous []string
}

var (
	hashtagRe = regexp.MustCompile(`#\w+`)
	nonAlnum  = regexp.MustCompile(`[^a-z0-9\s]+`)
)

// ReadBooksCSV reads a CSV with her Depop headers (Item, Location, …).
func ReadBooksCSV(r io.Reader) ([]BooksRow, error) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true
	cr.FieldsPerRecord = -1
	records, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read books csv: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("books csv empty")
	}
	idx := mapHeaders(records[0])
	if _, ok := idx["item"]; !ok {
		return nil, fmt.Errorf("books csv missing Item column")
	}
	out := make([]BooksRow, 0, len(records)-1)
	for _, rec := range records[1:] {
		item := strings.TrimSpace(col(rec, idx, "item"))
		if item == "" {
			continue
		}
		row := BooksRow{
			Item:     item,
			Location: strings.TrimSpace(col(rec, idx, "location")),
			Qty:      strings.TrimSpace(col(rec, idx, "number of items")),
			Source:   strings.TrimSpace(col(rec, idx, "source")),
			Gross:    strings.TrimSpace(col(rec, idx, "gross")),
		}
		if m := strings.TrimSpace(col(rec, idx, "month")); m != "" {
			if t, ok := parseBooksMonth(m); ok {
				row.Month = t
			}
		}
		out = append(out, row)
	}
	return out, nil
}

// ApplyBooksSeed fuzzy-matches books rows onto sold catalog products and
// writes Source/Location/Gross/BooksTitle + sold_at month when confident.
func (s *CatalogStore) ApplyBooksSeed(rows []BooksRow, threshold float64) (*BooksSeedResult, error) {
	if threshold <= 0 {
		threshold = BooksMatchThreshold
	}
	cat, err := s.Load()
	if err != nil {
		return nil, err
	}
	res := &BooksSeedResult{Rows: len(rows)}
	soldIdx := make([]int, 0)
	for i, p := range cat.Products {
		if p.Status == StatusSold {
			soldIdx = append(soldIdx, i)
		}
	}

	used := map[int64]struct{}{}
	for _, row := range rows {
		score, pi, ambiguous := bestBooksMatch(row.Item, cat.Products, soldIdx, used)
		if ambiguous {
			res.Ambiguous = append(res.Ambiguous, row.Item)
		}
		if score < threshold || pi < 0 {
			res.Skipped++
			res.Weak = append(res.Weak, fmt.Sprintf("%.2f %s", score, row.Item))
			continue
		}
		res.Matched++
		p := &cat.Products[pi]
		used[p.ID] = struct{}{}
		if applyBooksRow(p, row) {
			res.Applied++
		}
	}
	if res.Applied > 0 {
		if err := s.write(cat); err != nil {
			return res, err
		}
	}
	return res, nil
}

// BooksLink force-assigns a books sheet row (by Item title) onto a Depop product id.
type BooksLink struct {
	ProductID int64
	Item      string
}

// ApplyBooksLinks writes books fields from CSV rows onto products by explicit id.
// Always replaces existing books fields on the target (manual review override).
func (s *CatalogStore) ApplyBooksLinks(rows []BooksRow, links []BooksLink) (*BooksSeedResult, error) {
	cat, err := s.Load()
	if err != nil {
		return nil, err
	}
	byItem := map[string]BooksRow{}
	for _, row := range rows {
		byItem[normalizeBooks(row.Item)] = row
		byItem[strings.ToLower(strings.TrimSpace(row.Item))] = row
	}
	byID := map[int64]int{}
	for i := range cat.Products {
		byID[cat.Products[i].ID] = i
	}

	res := &BooksSeedResult{Rows: len(links)}
	for _, link := range links {
		row, ok := byItem[normalizeBooks(link.Item)]
		if !ok {
			row, ok = byItem[strings.ToLower(strings.TrimSpace(link.Item))]
		}
		if !ok {
			res.Skipped++
			res.Weak = append(res.Weak, "missing books row: "+link.Item)
			continue
		}
		pi, ok := byID[link.ProductID]
		if !ok {
			res.Skipped++
			res.Weak = append(res.Weak, fmt.Sprintf("missing product id %d for %s", link.ProductID, link.Item))
			continue
		}
		res.Matched++
		if applyBooksRow(&cat.Products[pi], row) {
			res.Applied++
		}
	}
	if res.Applied > 0 {
		if err := s.write(cat); err != nil {
			return res, err
		}
	}
	return res, nil
}

// applyBooksRow overwrites books fields on p from row. Returns true if anything changed.
func applyBooksRow(p *Product, row BooksRow) bool {
	changed := false
	if row.Source != "" && p.ListedFor != row.Source {
		p.ListedFor = row.Source
		changed = true
	}
	if row.Location != "" && p.Location != row.Location {
		p.Location = row.Location
		changed = true
	}
	if row.Gross != "" && p.Gross != row.Gross {
		p.Gross = row.Gross
		changed = true
	}
	if row.Item != "" && p.BooksTitle != row.Item {
		p.BooksTitle = row.Item
		changed = true
	}
	if !row.Month.IsZero() {
		m := row.Month.UTC()
		if p.SoldAt == nil || p.SoldAt.IsZero() || !sameMonth(*p.SoldAt, m) || p.SoldAt.Day() != 1 {
			p.SoldAt = &m
			changed = true
		}
	}
	return changed
}

func sameMonth(a, b time.Time) bool {
	a, b = a.UTC(), b.UTC()
	return a.Year() == b.Year() && a.Month() == b.Month()
}

func mapHeaders(header []string) map[string]int {
	idx := map[string]int{}
	for i, h := range header {
		key := strings.ToLower(strings.TrimSpace(h))
		idx[key] = i
	}
	return idx
}

func col(rec []string, idx map[string]int, name string) string {
	i, ok := idx[name]
	if !ok || i < 0 || i >= len(rec) {
		return ""
	}
	return rec[i]
}

func parseBooksMonth(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	formats := []string{
		"2006-01-02",
		"2006-01-02T15:04:05Z",
		"1/2/2006",
		"01/02/2006",
		"1/2/06",
		time.RFC3339,
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			t = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
			return t, true
		}
	}
	// Excel serial (e.g. 45627)
	var serial float64
	if _, err := fmt.Sscanf(s, "%f", &serial); err == nil && serial > 20000 && serial < 80000 {
		// Excel 1900 date system: day 0 = 1899-12-30
		base := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
		t := base.Add(time.Duration(serial * float64(24*time.Hour)))
		t = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
		return t, true
	}
	return time.Time{}, false
}

func bestBooksMatch(title string, products []Product, soldIdx []int, used map[int64]struct{}) (float64, int, bool) {
	bestScore := -1.0
	bestIdx := -1
	second := -1.0
	for _, i := range soldIdx {
		p := products[i]
		if _, ok := used[p.ID]; ok {
			continue
		}
		sc := booksScore(title, p)
		if sc > bestScore {
			second = bestScore
			bestScore = sc
			bestIdx = i
		} else if sc > second {
			second = sc
		}
	}
	if bestIdx < 0 {
		return 0, -1, false
	}
	ambiguous := second >= 0 && bestScore-second < 0.05 && bestScore >= BooksMatchThreshold
	return bestScore, bestIdx, ambiguous
}

func booksScore(title string, p Product) float64 {
	fl := firstLine(p.Description)
	nt := normalizeBooks(title)
	nfl := normalizeBooks(fl)
	tt := tokens(title)
	a, b := setOf(tt), setOf(tokens(fl))
	jac := 0.0
	if len(a) > 0 && len(b) > 0 {
		inter := 0
		for t := range a {
			if b[t] {
				inter++
			}
		}
		union := len(a) + len(b) - inter
		if union > 0 {
			jac = float64(inter) / float64(union)
		}
	}
	seq := sequenceRatio(nt, nfl)
	desc := normalizeBooks(p.Description)
	cover := 0.0
	if len(tt) > 0 {
		hit := 0
		for _, t := range tt {
			if strings.Contains(desc, t) {
				hit++
			}
		}
		cover = float64(hit) / float64(len(tt))
	}
	score := 0.45*jac + 0.25*seq + 0.30*cover
	// Her titles are often a truncated first line.
	if nt != "" && (strings.HasPrefix(nfl, nt) || strings.HasPrefix(desc, nt)) {
		if score < 0.92 {
			score = 0.92
		}
	}
	return score
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	return s
}

func normalizeBooks(s string) string {
	s = strings.ToLower(s)
	s = hashtagRe.ReplaceAllString(s, " ")
	s = nonAlnum.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(s), " ")
}

var booksStop = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true, "with": true,
	"for": true, "in": true, "on": true, "to": true, "of": true, "size": true,
	"womens": true, "women": true, "mens": true, "men": true, "new": true,
	"bnwt": true, "nwot": true,
}

func tokens(s string) []string {
	var out []string
	for _, t := range strings.Fields(normalizeBooks(s)) {
		if len(t) <= 2 || booksStop[t] {
			continue
		}
		out = append(out, t)
	}
	return out
}

func setOf(tt []string) map[string]bool {
	m := make(map[string]bool, len(tt))
	for _, t := range tt {
		m[t] = true
	}
	return m
}

func sequenceRatio(a, b string) float64 {
	if a == "" || b == "" {
		return 0
	}
	if a == b {
		return 1
	}
	// Simple Dice on bigrams (cheap SequenceMatcher stand-in).
	bg := func(s string) map[string]int {
		m := map[string]int{}
		if len(s) < 2 {
			m[s] = 1
			return m
		}
		for i := 0; i < len(s)-1; i++ {
			m[s[i:i+2]]++
		}
		return m
	}
	ba, bb := bg(a), bg(b)
	overlap := 0
	for k, va := range ba {
		if vb, ok := bb[k]; ok {
			if va < vb {
				overlap += va
			} else {
				overlap += vb
			}
		}
	}
	total := 0
	for _, v := range ba {
		total += v
	}
	for _, v := range bb {
		total += v
	}
	if total == 0 {
		return 0
	}
	return 2 * float64(overlap) / float64(total)
}

// ShortBooksTitle mirrors her ~40-char item labels for export when no books_title.
func ShortBooksTitle(description string) string {
	line := firstLine(description)
	line = hashtagRe.ReplaceAllString(line, "")
	line = strings.Join(strings.Fields(line), " ")
	const max = 45
	if len(line) <= max {
		return line
	}
	// cut at last space before max when possible
	cut := line[:max]
	if i := strings.LastIndexFunc(cut, unicode.IsSpace); i > 20 {
		cut = cut[:i]
	}
	return strings.TrimSpace(cut)
}

// ExportGross prefers books net gross, else list price.
func ExportGross(p Product) string {
	if g := strings.TrimSpace(p.Gross); g != "" {
		return g
	}
	return strings.TrimSpace(p.Price.PriceAmount)
}

// ExportItemTitle prefers books title from her sheet.
func ExportItemTitle(p Product) string {
	if t := strings.TrimSpace(p.BooksTitle); t != "" {
		return t
	}
	return ShortBooksTitle(p.Description)
}
