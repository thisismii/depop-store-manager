package rewrite

import (
	"context"
	"strings"
	"testing"
)

func TestHeuristicKeepsFactsAndFormat(t *testing.T) {
	original := `Polo Ralph Lauren Navy Stripe Polo

Brand: Polo Ralph Lauren

Size: Small (S)

Condition: Like new

Material: Cotton blend

Classic short-sleeve polo with micro-stripe pattern.

#ralphlauren #preppy #oldmoney #extra1 #extra2 #extra3`

	got, err := Heuristic{}.Rewrite(context.Background(), original)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "Polo Ralph Lauren Navy Stripe Polo\n\n") {
		t.Fatalf("want headline + blank line, got:\n%s", got)
	}
	for _, want := range []string{
		"- Brand: Polo Ralph Lauren",
		"- Size: Small (S)",
		"- Condition: Like new",
		"- Material: Cotton blend",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	tags := hashRe.FindAllString(got, -1)
	if len(tags) > maxHashtags {
		t.Fatalf("hashtags %d > %d: %v", len(tags), maxHashtags, tags)
	}
}

func TestHeuristicSkipsInventedCondition(t *testing.T) {
	original := `Lucky Brand Jorts

Brand: Lucky Brand
Size: 26 inch waist, 18 inch inseam

Cute summer jorts.`
	got, err := Heuristic{}.Rewrite(context.Background(), original)
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(got)
	if strings.Contains(lower, "condition:") {
		t.Fatalf("should not invent condition:\n%s", got)
	}
	if !strings.Contains(got, "- Brand: Lucky Brand") {
		t.Fatalf("missing brand:\n%s", got)
	}
	if !strings.Contains(got, "- Size:") {
		t.Fatalf("missing size:\n%s", got)
	}
}

func TestClampHashtags(t *testing.T) {
	in := "Title\n\n- Brand: X\n\n#a #b #c #d #e #f #g"
	out := clampHashtags(in)
	tags := hashRe.FindAllString(out, -1)
	if len(tags) != maxHashtags {
		t.Fatalf("want %d tags got %v", maxHashtags, tags)
	}
}
