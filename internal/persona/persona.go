// Package persona infers probable client segments from purchases.
package persona

import (
	"sort"
	"strings"

	"github.com/basili4-1982/lk-advisor/internal/model"
)

// Score is a probable persona and its weight.
type Score struct {
	Type  string  `json:"type"`
	Score float64 `json:"score"`
}

// Scores returns a normalized persona distribution for the client.
func Scores(p model.Profile, products map[string]model.Product) []Score {
	seen := map[string]bool{}
	for _, x := range p.Purchases {
		seen[x.ProductID] = true
	}
	for _, l := range p.Licenses {
		seen[l.ProductID] = true
	}

	business, student, consumer := 0.0, 0.0, 0.0
	for id := range seen {
		prod, ok := products[id]
		if !ok {
			continue
		}
		if isStudy(prod.Title) && prod.Audience != "business" {
			student++
			continue
		}
		if isLimited(prod.Title) {
			student += 0.5
			consumer += 0.5
			continue
		}
		switch prod.Audience {
		case "business":
			business++
		case "consumer":
			consumer++
		}
	}

	total := business + student + consumer
	if total == 0 {
		return []Score{}
	}

	all := []Score{
		{Type: "business", Score: business / total},
		{Type: "student", Score: student / total},
		{Type: "consumer", Score: consumer / total},
	}
	out := make([]Score, 0, 3)
	for _, s := range all {
		if s.Score > 0 {
			out = append(out, s)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return rank(out[i].Type) < rank(out[j].Type)
	})
	return out
}

func rank(t string) int {
	switch t {
	case "business":
		return 0
	case "student":
		return 1
	default:
		return 2
	}
}

func isStudy(title string) bool {
	t := strings.ToLower(title)
	return strings.Contains(t, "учёб") || strings.Contains(t, "учеб") ||
		strings.Contains(t, "study") || strings.Contains(t, "student") ||
		strings.Contains(t, "education")
}

func isLimited(title string) bool {
	t := strings.ToLower(title)
	return strings.Contains(t, "home") || strings.Contains(t, "essentials") ||
		strings.Contains(t, "visio standard") || strings.Contains(t, "компас-график")
}
