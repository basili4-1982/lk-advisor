// Package eval measures recommendation quality on the ground truth.
package eval

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/basili4-1982/lk-advisor/internal/catalog"
	"github.com/basili4-1982/lk-advisor/internal/model"
	"github.com/basili4-1982/lk-advisor/internal/profile"
	"github.com/basili4-1982/lk-advisor/internal/recommend"
)

// Report is the evaluation summary.
type Report struct {
	Total        int     `json:"total"`
	PrecisionAt1 float64 `json:"precision_at_1"`
	PrecisionAt3 float64 `json:"precision_at_3"`
	RecallAt3    float64 `json:"recall_at_3"`
	Violations   int     `json:"violations"`
	FallbackRate float64 `json:"fallback_rate"`
	EvidenceRate float64 `json:"evidence_rate"`
	Passed       bool    `json:"passed"`
}

// LoadGroundTruth reads the labelled cases.
func LoadGroundTruth(ctx context.Context, dbc *sql.DB) ([]model.GroundTruthCase, error) {
	rows, err := dbc.QueryContext(ctx, `
		SELECT client_id, event, expected_product_id, expected_type
		FROM ground_truth ORDER BY client_id, event, expected_product_id`)
	if err != nil {
		return nil, fmt.Errorf("ground truth: %w", err)
	}
	defer rows.Close()

	var out []model.GroundTruthCase
	for rows.Next() {
		var c model.GroundTruthCase
		if err := rows.Scan(&c.ClientID, &c.Event, &c.ExpectedProduct, &c.ExpectedType); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

type expectedGroup struct {
	products map[string]bool
}

// Run scores the engine on the given cases.
// Several rows per client are acceptable alternatives; a case is a hit
// when the top recommendation is any of them.
func Run(ctx context.Context, dbc *sql.DB, cases []model.GroundTruthCase, now time.Time) (Report, error) {
	products, err := catalog.AllProducts(ctx, dbc)
	if err != nil {
		return Report{}, err
	}
	rules, err := catalog.Rules(ctx, dbc)
	if err != nil {
		return Report{}, err
	}
	promotions, err := catalog.AllPromotions(ctx, dbc)
	if err != nil {
		return Report{}, err
	}
	weights, err := catalog.LoadWeights(ctx, dbc)
	if err != nil {
		return Report{}, err
	}
	relationTypes, err := catalog.RelationTypeMap(ctx, dbc)
	if err != nil {
		return Report{}, err
	}

	groups := map[string]*expectedGroup{}
	for _, c := range cases {
		g := groups[c.ClientID]
		if g == nil {
			g = &expectedGroup{products: map[string]bool{}}
			groups[c.ClientID] = g
		}
		g.products[c.ExpectedProduct] = true
	}

	var hits1, hits3, violations, fallbacks, withEvidence, recs int
	popB2B, _ := catalog.PopularityScores(ctx, dbc, "b2b", now, 8)
	popB2C, _ := catalog.PopularityScores(ctx, dbc, "b2c", now, 8)

	for clientID, g := range groups {
		p, err := profile.Get(ctx, dbc, clientID)
		if err != nil {
			continue
		}
		rels, err := catalog.RelationsScoped(ctx, dbc, ownedIDs(p), p.Audience, p.Industry, now)
		if err != nil {
			return Report{}, err
		}
		popularity := popB2B
		if p.Audience == "b2c" {
			popularity = popB2C
		}
		in := recommend.Input{
			Profile: p, Products: products, Relations: rels,
			Promotions: promotions, Rules: rules, RelationTypes: relationTypes,
			Weights: weights, Popularity: popularity, Now: now,
		}
		cands, rejected := recommend.Run(in)
		if len(cands) == 0 {
			fallbacks++
			continue
		}

		if g.products[cands[0].Product.ID] {
			hits1++
		}
		for _, c := range cands {
			if g.products[c.Product.ID] {
				hits3++
				break
			}
		}

		blocked := map[string]bool{}
		for _, r := range rejected {
			blocked[r.ProductID] = true
		}
		for _, c := range cands {
			if blocked[c.Product.ID] {
				violations++
			}
		}

		recs++
		if len(cands[0].Evidence) > 0 {
			withEvidence++
		}
	}

	total := len(groups)
	rep := Report{Total: total, Violations: violations}
	if total > 0 {
		rep.PrecisionAt1 = float64(hits1) / float64(total)
		rep.PrecisionAt3 = float64(hits3) / float64(total)
		rep.RecallAt3 = float64(hits3) / float64(total)
		rep.FallbackRate = float64(fallbacks) / float64(total)
	}
	if recs > 0 {
		rep.EvidenceRate = float64(withEvidence) / float64(recs)
	}
	rep.Passed = rep.PrecisionAt1 >= 0.70 &&
		rep.PrecisionAt3 >= 0.90 &&
		rep.RecallAt3 >= 0.80 &&
		rep.Violations == 0 &&
		rep.FallbackRate < 0.10 &&
		rep.EvidenceRate >= 1.0

	return rep, nil
}

func ownedIDs(p model.Profile) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range p.Purchases {
		if !seen[x.ProductID] {
			seen[x.ProductID] = true
			out = append(out, x.ProductID)
		}
	}
	for _, x := range p.Licenses {
		if !seen[x.ProductID] {
			seen[x.ProductID] = true
			out = append(out, x.ProductID)
		}
	}
	return out
}
