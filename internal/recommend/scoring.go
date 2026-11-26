package recommend

import (
	"math"
	"sort"
	"time"

	"github.com/basili4-1982/lk-advisor/internal/model"
	"github.com/basili4-1982/lk-advisor/internal/persona"
)

// Score applies the default weighted formula and the promotion boost.
func Score(c model.Candidate, now time.Time) model.Candidate {
	return ScoreWith(c, now, model.DefaultWeights())
}

// ScoreWith applies the configured weighted formula and the promotion boost.
func ScoreWith(c model.Candidate, now time.Time, weights model.Weights) model.Candidate {
	urgency := c.Urgency
	if c.Promo != nil {
		switch d := daysUntil(c.Promo.ValidTo, now); {
		case d <= 7:
			urgency += 0.2
		case d <= 30:
			urgency += 0.1
		}
		if urgency > 1 {
			urgency = 1
		}
	}
	c.Urgency = urgency

	w := weights.Normalize()
	score := w.Relevance*c.Relevance +
		w.Urgency*urgency +
		w.Affinity*c.Affinity +
		w.Margin*c.Margin +
		w.Confidence*c.Confidence
	c.Score = math.Round(score*100) / 100
	return c
}

// Sort orders by score, then by promotion, urgency and product id.
func Sort(cands []model.Candidate) {
	sort.SliceStable(cands, func(i, j int) bool {
		a, b := cands[i], cands[j]
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		ap, bp := a.Promo != nil, b.Promo != nil
		if ap != bp {
			return ap
		}
		if a.Urgency != b.Urgency {
			return a.Urgency > b.Urgency
		}
		return a.Product.ID < b.Product.ID
	})
}

// Diversify keeps at most one recommendation per type and per product.
func Diversify(cands []model.Candidate, limit int) []model.Candidate {
	seenProduct := map[string]bool{}
	seenTask := map[string]bool{}
	out := make([]model.Candidate, 0, limit)

	for _, c := range cands {
		if seenProduct[c.Product.ID] {
			continue
		}
		task := c.Product.Subcategory
		if task == "" {
			task = c.Product.Category
		}
		if task == "" || c.Type == "alternative" || c.Type == "renewal" || c.Type == "analog" {
			task = c.Type
		}
		if seenTask[task] {
			continue
		}
		seenProduct[c.Product.ID] = true
		seenTask[task] = true
		out = append(out, c)
		if len(out) >= limit {
			break
		}
	}
	return out
}

// Run executes the pipeline and returns the top-3 portfolio plus rejections.
func Run(in Input) ([]model.Candidate, []model.Rejection) {
	allowed, rejected := Rank(in)
	out := FillPopular(in, Diversify(allowed, 3), 3)
	return ForceExperiment(in, out, 3), rejected
}

// ForceExperiment guarantees the treatment product is in the list, so the
// experiment gets an exposure. Control arms are untouched.
func ForceExperiment(in Input, picked []model.Candidate, limit int) []model.Candidate {
	if in.Experiment == nil || in.Experiment.TargetProductID == "" || limit <= 0 {
		return picked
	}
	target := in.Experiment.TargetProductID
	for _, c := range picked {
		if c.Product.ID == target {
			return picked
		}
	}
	if ownedIDs(in.Profile)[target] {
		return picked
	}
	p, ok := in.Product(target)
	if !ok || p.Status == "discontinued" {
		return picked
	}
	typ := "cross_sell"
	if in.Experiment.RelationType == "upgrade" {
		typ = "upsell"
	}
	c := model.Candidate{
		Product:         p,
		Type:            typ,
		Relevance:       0.55,
		Affinity:        0.8,
		Margin:          normalizeMargin(p.Price),
		Confidence:      0.85,
		Evidence:        []model.Evidence{{Source: "experiment", Value: "показ в эксперименте: проверяем гипотезу"}},
		SourceProductID: in.Experiment.FromID,
		RelationType:    in.Experiment.RelationType,
		CandidateID:     in.Experiment.FromID + "|" + in.Experiment.RelationType + "|" + target,
	}
	c = ScoreWith(c, in.Now, in.Weights)
	if len(picked) >= limit {
		picked[limit-1] = c
		return picked
	}
	return append(picked, c)
}

// Rank executes candidates, rules and scoring and returns every allowed item
// in ranked order. Callers decide how many items to show.
func Rank(in Input) ([]model.Candidate, []model.Rejection) {
	intent := Intent(in)
	scores := persona.Scores(in.Profile, in.Products)
	studentWeight, businessWeight := 0.0, 0.0
	for _, s := range scores {
		switch s.Type {
		case "student":
			studentWeight = s.Score
		case "business":
			businessWeight = s.Score
		}
	}
	studentPriority := studentWeight > businessWeight && studentWeight > 0
	cands := Candidates(in)
	allowed, rejected := ApplyRules(in, cands)
	maxPopularity := 0.0
	for _, score := range in.Popularity {
		if score > maxPopularity {
			maxPopularity = score
		}
	}
	for i := range allowed {
		allowed[i] = ScoreWith(allowed[i], in.Now, in.Weights)
		if allowed[i].Type == intent {
			boost := 0.05
			switch intent {
			case "renewal":
				boost = 0.15
			case "upsell":
				boost = 0.10
			}
			allowed[i].Score = math.Round((allowed[i].Score+boost)*100) / 100
			if allowed[i].Score > 1 {
				allowed[i].Score = 1
			}
		}
		if maxPopularity > 0 {
			if demand, ok := in.Popularity[allowed[i].Product.ID]; ok {
				allowed[i].Score = math.Round((allowed[i].Score+0.05*demand/maxPopularity)*100) / 100
			}
		}
		if studentPriority && allowed[i].Product.Category == "training" {
			allowed[i].Score = math.Round((allowed[i].Score+0.15)*100) / 100
			if allowed[i].Score > 1 {
				allowed[i].Score = 1
			}
		}
	}
	Sort(allowed)
	return allowed, rejected
}

// ExplorationSlot picks one extra candidate for the third slot with a small
// probability. The random value is injected so the policy is testable.
func ExplorationSlot(allowed, chosen []model.Candidate, random float64) (model.Candidate, bool) {
	if random >= 0.1 || len(allowed) <= len(chosen) {
		return model.Candidate{}, false
	}
	seen := make(map[string]bool, len(chosen))
	for _, c := range chosen {
		seen[c.Product.ID] = true
	}
	for i := len(allowed) - 1; i >= 0; i-- {
		if seen[allowed[i].Product.ID] {
			continue
		}
		out := allowed[i]
		out.Exploration = true
		return out, true
	}
	return model.Candidate{}, false
}
