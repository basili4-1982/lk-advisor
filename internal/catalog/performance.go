package catalog

import (
	"context"
	"database/sql"
	"fmt"
)

// RelationPerformance is the observed effect of one concrete relation.
type RelationPerformance struct {
	FromID     string  `json:"from_id"`
	ToID       string  `json:"to_id"`
	Type       string  `json:"type"`
	TitleFrom  string  `json:"title_from"`
	TitleTo    string  `json:"title_to"`
	Status     string  `json:"status"`
	Shown      int     `json:"shown"`
	Clicks     int     `json:"clicks"`
	Purchases  int     `json:"purchases"`
	CTR        float64 `json:"ctr"`
	Conversion float64 `json:"conversion"`
}

// LoadRelationPerformance returns relation-level CTR and conversion.
func LoadRelationPerformance(ctx context.Context, dbc *sql.DB, limit int) ([]RelationPerformance, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := dbc.QueryContext(ctx, `
		SELECT e.source_product_id, e.product_id, e.relation_type,
		       COALESCE(a.title, e.source_product_id), COALESCE(b.title, e.product_id),
		       COALESCE(r.status, 'unknown'),
		       count(*) FILTER (WHERE e.event = 'shown'),
		       count(*) FILTER (WHERE e.event = 'clicked'),
		       count(*) FILTER (WHERE e.event = 'purchased')
		FROM recommendation_events e
		LEFT JOIN products a ON a.id = e.source_product_id
		LEFT JOIN products b ON b.id = e.product_id
		LEFT JOIN product_relations r
		  ON r.from_id = e.source_product_id
		 AND r.to_id = e.product_id
		 AND r.type = e.relation_type
		WHERE e.candidate_id <> ''
		GROUP BY e.source_product_id, e.product_id, e.relation_type, a.title, b.title, r.status
		ORDER BY count(*) FILTER (WHERE e.event = 'shown') DESC, e.source_product_id, e.product_id
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("relation performance: %w", err)
	}
	defer rows.Close()
	var out []RelationPerformance
	for rows.Next() {
		var p RelationPerformance
		if err := rows.Scan(&p.FromID, &p.ToID, &p.Type, &p.TitleFrom, &p.TitleTo,
			&p.Status, &p.Shown, &p.Clicks, &p.Purchases); err != nil {
			return nil, err
		}
		if p.Shown > 0 {
			p.CTR = float64(p.Clicks) / float64(p.Shown)
			p.Conversion = float64(p.Purchases) / float64(p.Shown)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// AutoReviewRelations promotes converting links and rejects links with no clicks.
// Only llm_suggested/quarantine relations are touched.
func AutoReviewRelations(ctx context.Context, dbc *sql.DB, minShown int, minCTR float64) (int, int, error) {
	if minShown <= 0 {
		minShown = 20
	}
	if minCTR <= 0 {
		minCTR = 0.03
	}
	perf, err := LoadRelationPerformance(ctx, dbc, 500)
	if err != nil {
		return 0, 0, err
	}
	promoted, rejected := 0, 0
	for _, p := range perf {
		if p.Shown < minShown || (p.Status != "llm_suggested" && p.Status != "quarantine") {
			continue
		}
		status := ""
		note := ""
		if p.CTR >= minCTR || p.Purchases > 0 {
			status = "confirmed"
			note = fmt.Sprintf("bandit: ctr=%.2f purchases=%d", p.CTR, p.Purchases)
			promoted++
		} else if p.Clicks == 0 {
			status = "rejected"
			note = fmt.Sprintf("bandit: 0 clicks at %d impressions", p.Shown)
			rejected++
		}
		if status == "" {
			continue
		}
		_, err := dbc.ExecContext(ctx, `
			UPDATE product_relations
			SET status=$4, reviewed_by='bandit', reviewed_at=now(), review_note=$5
			WHERE from_id=$1 AND to_id=$2 AND type=$3`,
			p.FromID, p.ToID, p.Type, status, note)
		if err != nil {
			return promoted, rejected, fmt.Errorf("auto review relation: %w", err)
		}
	}
	return promoted, rejected, nil
}
