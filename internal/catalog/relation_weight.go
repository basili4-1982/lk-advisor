package catalog

// Relation performance is smoothed with a Bayesian prior so a relation never
// jumps from full weight to zero after a short unlucky streak. A new relation
// starts at 1.0; each impression without a click pulls it down gradually.
const (
	// relationCTRTarget is the click-through rate a relation must hold to keep
	// its full weight.
	relationCTRTarget = 0.03
	// relationPriorShows is the strength of the prior, in impressions.
	relationPriorShows = 20.0
	// relationRejectWeight is the weight below which a relation may be rejected.
	relationRejectWeight = 0.10
	// relationRejectShown is the minimum evidence before rejection.
	relationRejectShown = 100
)

// RelationPerformanceMultiplier returns a smooth 0..1 multiplier for a relation.
// With no observations it returns 1. A relation ignored for a long time decays
// towards zero instead of being switched off abruptly.
func RelationPerformanceMultiplier(shown, clicks, purchases int) float64 {
	if purchases > 0 {
		return 1
	}
	alpha := relationCTRTarget * relationPriorShows
	beta := relationPriorShows - alpha
	smoothed := (float64(clicks) + alpha) / (float64(shown) + alpha + beta)
	m := smoothed / relationCTRTarget
	if m > 1 {
		return 1
	}
	if m < 0 {
		return 0
	}
	return m
}
