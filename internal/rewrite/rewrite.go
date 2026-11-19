package rewrite

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode"
)

const (
	maxHashtags = 5
	systemPrompt = `You rewrite Depop listing descriptions for sales conversion.

Output ONLY the rewritten listing text. No preamble, no markdown fences.

Format (exact):
1. One headline title line
2. A blank line
3. Body with bullets (use "- " prefix). Include Brand, Size, and measurements when present in the original. Include Condition and Material ONLY if they already appear in the original. You may add a short sales-oriented blurb as bullets or short paragraphs after the fact bullets.
4. Optional hashtags at the end — maximum 5 total.

Rules:
- Same facts only. Fresher wording and trendy keywords are OK.
- Do NOT invent brand, size, measurements, condition, material, flaws, or other facts.
- Always keep brand, size, and measurements if they were in the original.
- Never mention Boost or price cuts.
- Keep tone suitable for a fashion resale shop.`
)

// Provider generates a sales-oriented description draft from an original.
type Provider interface {
	Name() string
	Rewrite(ctx context.Context, original string) (string, error)
}

// NewProvider picks OpenAI-compatible API when OPENAI_API_KEY is set, else heuristic.
func NewProvider() Provider {
	if strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) != "" {
		return NewOpenAIProviderFromEnv()
	}
	return Heuristic{}
}

// Heuristic reformats without an LLM — demo-safe offline fallback.
type Heuristic struct{}

func (Heuristic) Name() string { return "heuristic" }

func (Heuristic) Rewrite(_ context.Context, original string) (string, error) {
	original = strings.TrimSpace(original)
	if original == "" {
		return "", fmt.Errorf("empty description")
	}
	return heuristicRewrite(original), nil
}

var (
	labelRe = regexp.MustCompile(`(?i)^(brand|size|condition|material|color|colour|style|details|measurements?)\s*:\s*(.+)$`)
	hashRe  = regexp.MustCompile(`#([\p{L}\p{N}_]+)`)
	measRe  = regexp.MustCompile(`(?i)\b(\d+(?:\.\d+)?)\s*(?:["”]|in(?:ch(?:es)?)?|cm)\b`)
)

func heuristicRewrite(original string) string {
	lines := splitLines(original)
	var headline string
	var facts []string
	var body []string
	var hashtags []string
	seenHash := map[string]bool{}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Collect trailing hashtag-only lines / inline tags.
		if strings.HasPrefix(trimmed, "#") || isMostlyHashtags(trimmed) {
			for _, m := range hashRe.FindAllString(trimmed, -1) {
				key := strings.ToLower(m)
				if !seenHash[key] && len(hashtags) < maxHashtags {
					seenHash[key] = true
					hashtags = append(hashtags, m)
				}
			}
			continue
		}
		if m := labelRe.FindStringSubmatch(trimmed); m != nil {
			label := titleCase(strings.ToLower(m[1]))
			if strings.EqualFold(label, "measurements") || strings.EqualFold(label, "measurement") {
				label = "Measurements"
			}
			facts = append(facts, "- "+label+": "+strings.TrimSpace(m[2]))
			continue
		}
		// Size/measurement facts without labels.
		if looksLikeMeasurementLine(trimmed) {
			facts = append(facts, "- "+trimmed)
			continue
		}
		// Pull hashtags out of prose lines.
		clean := trimmed
		for _, m := range hashRe.FindAllString(trimmed, -1) {
			key := strings.ToLower(m)
			if !seenHash[key] && len(hashtags) < maxHashtags {
				seenHash[key] = true
				hashtags = append(hashtags, m)
			}
			clean = strings.ReplaceAll(clean, m, "")
		}
		clean = strings.TrimSpace(clean)
		if clean == "" {
			continue
		}
		if headline == "" {
			headline = stripEmoji(clean)
			continue
		}
		if len(clean) < 80 && !strings.Contains(clean, ". ") {
			body = append(body, "- "+clean)
			continue
		}
		body = append(body, clean)
	}

	if headline == "" {
		headline = "Listing refresh"
	}

	var b strings.Builder
	b.WriteString(headline)
	b.WriteString("\n\n")
	for _, f := range facts {
		b.WriteString(f)
		b.WriteByte('\n')
	}
	if len(body) > 0 {
		if len(facts) > 0 {
			b.WriteByte('\n')
		}
		for _, para := range body {
			b.WriteString(para)
			b.WriteString("\n\n")
		}
	}
	out := strings.TrimRight(b.String(), "\n")
	if len(hashtags) > 0 {
		out += "\n\n" + strings.Join(hashtags, " ")
	}
	return out
}

func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.Split(s, "\n")
}

func isMostlyHashtags(s string) bool {
	without := hashRe.ReplaceAllString(s, "")
	without = strings.TrimSpace(without)
	return without == ""
}

func looksLikeMeasurementLine(s string) bool {
	lower := strings.ToLower(s)
	if strings.Contains(lower, "waist") || strings.Contains(lower, "inseam") ||
		strings.Contains(lower, "bust") || strings.Contains(lower, "length") ||
		strings.Contains(lower, "rise") {
		return measRe.MatchString(s)
	}
	return false
}

func stripEmoji(s string) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if r == '\uFE0F' || r == '\u200D' {
			return -1
		}
		// Keep basic punctuation and letters; drop common emoji blocks.
		if r >= 0x1F300 && r <= 0x1FAFF {
			return -1
		}
		if r >= 0x2600 && r <= 0x27BF {
			return -1
		}
		return r
	}, s))
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// OpenAIProvider calls an OpenAI-compatible chat completions endpoint.
type OpenAIProvider struct {
	APIKey  string
	BaseURL string
	Model   string
	Client  *http.Client
}

func NewOpenAIProviderFromEnv() *OpenAIProvider {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("OPENAI_BASE_URL")), "/")
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	model := strings.TrimSpace(os.Getenv("OPENAI_MODEL"))
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &OpenAIProvider{
		APIKey:  strings.TrimSpace(os.Getenv("OPENAI_API_KEY")),
		BaseURL: base,
		Model:   model,
		Client:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (p *OpenAIProvider) Name() string { return "openai:" + p.Model }

func (p *OpenAIProvider) Rewrite(ctx context.Context, original string) (string, error) {
	original = strings.TrimSpace(original)
	if original == "" {
		return "", fmt.Errorf("empty description")
	}
	if p.APIKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY missing")
	}
	payload := map[string]any{
		"model": p.Model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": "Original listing description:\n\n" + original},
		},
		"temperature": 0.4,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)

	resp, err := p.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("openai request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("openai HTTP %d: %s", resp.StatusCode, truncate(string(raw), 300))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("openai parse: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("openai empty choices")
	}
	draft := strings.TrimSpace(parsed.Choices[0].Message.Content)
	draft = strings.TrimPrefix(draft, "```")
	draft = strings.TrimSuffix(draft, "```")
	draft = strings.TrimSpace(draft)
	if draft == "" {
		return "", fmt.Errorf("openai empty draft")
	}
	return clampHashtags(draft), nil
}

func clampHashtags(text string) string {
	lines := splitLines(text)
	var out []string
	var tags []string
	seen := map[string]bool{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && (strings.HasPrefix(trimmed, "#") || isMostlyHashtags(trimmed)) {
			for _, m := range hashRe.FindAllString(trimmed, -1) {
				key := strings.ToLower(m)
				if !seen[key] && len(tags) < maxHashtags {
					seen[key] = true
					tags = append(tags, m)
				}
			}
			continue
		}
		// Strip excess inline hashtags from body lines.
		clean := trimmed
		inline := hashRe.FindAllString(trimmed, -1)
		if len(inline) > 0 {
			for _, m := range inline {
				key := strings.ToLower(m)
				if !seen[key] && len(tags) < maxHashtags {
					seen[key] = true
					tags = append(tags, m)
				}
				clean = strings.ReplaceAll(clean, m, "")
			}
			clean = strings.TrimSpace(clean)
			if clean == "" {
				continue
			}
			out = append(out, clean)
			continue
		}
		out = append(out, line)
	}
	result := strings.TrimRight(strings.Join(out, "\n"), "\n")
	if len(tags) > 0 {
		result += "\n\n" + strings.Join(tags, " ")
	}
	return result
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
