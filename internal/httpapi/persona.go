package httpapi

import (
	"fmt"
	"strings"

	"github.com/basili4-1982/lk-advisor/internal/model"
	"github.com/basili4-1982/lk-advisor/internal/persona"
)

// personaFor returns the top persona, a readable distribution and all scores.
func personaFor(p model.Profile, products map[string]model.Product) (string, string, []persona.Score) {
	scores := persona.Scores(p, products)
	if len(scores) == 0 {
		return "unknown", "", scores
	}
	parts := make([]string, 0, len(scores))
	for _, s := range scores {
		parts = append(parts, fmt.Sprintf("%s %.0f%%", s.Type, s.Score*100))
	}
	return scores[0].Type, strings.Join(parts, " · "), scores
}
