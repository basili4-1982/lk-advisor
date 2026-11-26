// Package feedback adjusts candidate scores from observed impressions.
package feedback

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"

	"github.com/lib/pq"

	"github.com/basili4-1982/lk-advisor/internal/model"
)

const (
	minShown = 10
	baseline = 0.05
	maxBoost = 1.35
	minBoost = 0.65
)

// Stat is one product's observed performance.
type Stat struct {
	ProductID  string
	Shown      int
	Clicks     int
	Purchases  int
	CTR        float64
	Multiplier float64
}

// Load reads product feedback for the candidate set.
func Load(ctx context.Context, dbc *sql.DB, productIDs []string, now time.Time) (map[string]Stat, error) {
	if len(productIDs) == 0 {
		return map[string]Stat{}, nil
	}
	since := now.AddDate(0, 0, -30)
	rows, err := dbc.QueryContext(ctx, `
		SELECT product_id,
		       count(*) FILTER (WHERE event = 'shown'),
		       count(*) FILTER (WHERE event = 'clicked'),
		       count(*) FILTER (WHERE event = 'purchased')
		FROM recommendation_events
		WHERE created_at >= $1 AND product_id = ANY($2)
		GROUP BY product_id`, since, pq.Array(productIDs))
	if err != nil {
		return nil, fmt.Errorf("feedback: %w", err)
	}
	defer rows.Close()
	out := map[string]Stat{}
	for rows.Next() {
		var s Stat
		if err := rows.Scan(&s.ProductID, &s.Shown, &s.Clicks, &s.Purchases); err != nil {
			return nil, err
		}
		// Beta(1, 19) prior: a product with no clicks starts below baseline
		// and grows only after real evidence.
		s.CTR = (float64(s.Clicks) + 1) / (float64(s.Shown) + 20)
		s.Multiplier = 1 + (s.CTR-baseline)*3
		s.Multiplier = math.Max(minBoost, math.Min(maxBoost, s.Multiplier))
		if s.Shown < minShown {
			s.Multiplier = 1
		}
		out[s.ProductID] = s
	}
	return out, rows.Err()
}

// Apply multiplies candidate scores by the feedback factor and exposes it.
func Apply(cands []model.Candidate, stats map[string]Stat) {
	for i := range cands {
		s, ok := stats[cands[i].Product.ID]
		if !ok || s.Shown < minShown {
			continue
		}
		cands[i].FeedbackMultiplier = s.Multiplier
		cands[i].Score = math.Round(math.Max(0, math.Min(1, cands[i].Score*s.Multiplier))*100) / 100
	}
}
