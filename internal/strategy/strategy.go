// Package strategy chooses a recommendation strategy for a segment.
package strategy

import "math/rand"

const exploration = 0.1

// Stat is one strategy arm with its prior and observed outcomes.
type Stat struct {
	Audience  string  `json:"audience"`
	Strategy  string  `json:"strategy"`
	Enabled   bool    `json:"enabled"`
	Alpha     float64 `json:"alpha"`
	Beta      float64 `json:"beta"`
	Shown     int     `json:"shown"`
	Clicks    int     `json:"clicks"`
	Purchases int     `json:"purchases"`
	CTR       float64 `json:"ctr"`
}

// Default returns the safe strategy when auto mode is off.
func Default(audience string) string {
	if audience == "b2c" {
		return "popular"
	}
	return "hybrid"
}

// Score is a Laplace-smoothed CTR with a Beta prior.
func Score(s Stat) float64 {
	a, b := s.Alpha, s.Beta
	if a <= 0 {
		a = 1
	}
	if b <= 0 {
		b = 1
	}
	return (float64(s.Clicks) + a) / (float64(s.Shown) + a + b)
}

// Choose returns a strategy using epsilon-greedy exploration.
func Choose(audience string, stats []Stat) string {
	return ChooseWith(audience, stats, rand.Float64)
}

// ChooseWith is Choose with injectable randomness for tests.
func ChooseWith(audience string, stats []Stat, random func() float64) string {
	arms := make([]Stat, 0, len(stats))
	for _, s := range stats {
		if s.Audience == audience && s.Enabled {
			if s.CTR == 0 && s.Shown > 0 {
				s.CTR = float64(s.Clicks) / float64(s.Shown)
			}
			arms = append(arms, s)
		}
	}
	if len(arms) == 0 {
		return Default(audience)
	}
	hasObservations := false
	for _, s := range arms {
		if s.Shown > 0 {
			hasObservations = true
			break
		}
	}
	if !hasObservations {
		want := Default(audience)
		for _, s := range arms {
			if s.Strategy == want {
				return want
			}
		}
		return arms[0].Strategy
	}
	if random() < exploration {
		return arms[int(random()*float64(len(arms)))%len(arms)].Strategy
	}
	best := arms[0]
	for _, s := range arms[1:] {
		if Score(s) > Score(best) {
			best = s
		}
	}
	return best.Strategy
}
