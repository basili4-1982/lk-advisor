// Package strategy chooses a recommendation strategy for a segment.
package strategy

import "math/rand"

const (
	exploration = 0.1
	minObserved = 20
)

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
	want := Default(audience)
	var defaultArm *Stat
	for i := range arms {
		if arms[i].Strategy == want {
			defaultArm = &arms[i]
			break
		}
	}
	if defaultArm == nil {
		best := arms[0]
		for _, s := range arms[1:] {
			if Score(s) > Score(best) {
				best = s
			}
		}
		return best.Strategy
	}
	// Do not switch away from the safe default until it has enough impressions.
	if defaultArm.Shown < minObserved {
		return want
	}
	// Warm up each challenger before comparing it with the default.
	under := make([]Stat, 0)
	for _, s := range arms {
		if s.Strategy != want && s.Shown < minObserved {
			under = append(under, s)
		}
	}
	if len(under) > 0 {
		return under[int(random()*float64(len(under)))%len(under)].Strategy
	}
	if random() < exploration {
		return arms[int(random()*float64(len(arms)))%len(arms)].Strategy
	}
	best := *defaultArm
	bestScore := Score(*defaultArm)
	for _, s := range arms {
		if s.Strategy == want || s.Shown < minObserved {
			continue
		}
		if Score(s) > bestScore+0.02 {
			best, bestScore = s, Score(s)
		}
	}
	return best.Strategy
}
