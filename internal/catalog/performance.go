package catalog

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// RelationPerformance is the observed effect of one concrete relation in one scope.
type RelationPerformance struct {
	FromID     string     `json:"from_id"`
	ToID       string     `json:"to_id"`
	Type       string     `json:"type"`
	TitleFrom  string     `json:"title_from"`
	TitleTo    string     `json:"title_to"`
	Status     string     `json:"status"`
	Audience   string     `json:"audience"`
	Industry   string     `json:"industry"`
	ValidUntil *time.Time `json:"valid_until,omitempty"`
	Shown      int        `json:"shown"`
	Clicks     int        `json:"clicks"`
	Purchases  int        `json:"purchases"`
	CTR        float64    `json:"ctr"`
	Conversion float64    `json:"conversion"`
}

// LoadRelationPerformance returns scoped relation-level CTR and conversion.
func LoadRelationPerformance(ctx context.Context, dbc *sql.DB, limit int) ([]RelationPerformance, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := dbc.QueryContext(ctx, `
		SELECT e.source_product_id, e.product_id, e.relation_type,
		       COALESCE(a.title, e.source_product_id), COALESCE(b.title, e.product_id),
		       COALESCE(s.status, r.status, 'unknown'),
		       lower(e.audience), lower(e.industry), s.valid_until,
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
		LEFT JOIN LATERAL (
			SELECT sc.status, sc.valid_until
			FROM relation_scopes sc
			WHERE sc.from_id = e.source_product_id
			  AND sc.to_id = e.product_id
			  AND sc.type = e.relation_type
			  AND (sc.audience = lower(e.audience) OR sc.audience = '')
			  AND (sc.industry = lower(e.industry) OR sc.industry = '')
			ORDER BY (sc.audience = lower(e.audience) AND sc.industry = lower(e.industry)) DESC,
			         (sc.audience = lower(e.audience) AND sc.industry = '') DESC,
			         (sc.audience = '' AND sc.industry = lower(e.industry)) DESC,
			         (sc.audience = '') DESC,
			         sc.updated_at DESC
			LIMIT 1
		) s ON true
		WHERE e.candidate_id <> ''
		GROUP BY e.source_product_id, e.product_id, e.relation_type,
		         a.title, b.title, r.status, lower(e.audience), lower(e.industry),
		         s.status, s.valid_until
		ORDER BY count(*) FILTER (WHERE e.event = 'shown') DESC,
		         e.source_product_id, e.product_id, lower(e.audience), lower(e.industry)
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("relation performance: %w", err)
	}
	defer rows.Close()
	out := []RelationPerformance{}
	for rows.Next() {
		var p RelationPerformance
		if err := rows.Scan(&p.FromID, &p.ToID, &p.Type, &p.TitleFrom, &p.TitleTo,
			&p.Status, &p.Audience, &p.Industry, &p.ValidUntil,
			&p.Shown, &p.Clicks, &p.Purchases); err != nil {
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

// ScopeTTL is how long a confirmed link lives before it must be re-tested.
// Low-traffic B2C needs a longer window to gather evidence.
func ScopeTTL(audience string) time.Duration {
	if audience == "b2c" {
		return 60 * 24 * time.Hour
	}
	return 30 * 24 * time.Hour
}

// AutoReviewRelations makes decisions per audience+industry scope.
// Only llm_suggested/quarantine links are touched: semantic links stay human-owned.
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
		if p.Shown < minShown ||
			(p.Status != "llm_suggested" && p.Status != "quarantine" && p.Status != "testing") {
			continue
		}
		status := ""
		note := ""
		switch {
		case p.CTR >= minCTR || p.Purchases > 0:
			status = "confirmed"
			note = fmt.Sprintf("bandit: ctr=%.2f purchases=%d shown=%d", p.CTR, p.Purchases, p.Shown)
			promoted++
		case p.Clicks == 0:
			status = "rejected"
			note = fmt.Sprintf("bandit: 0 clicks at %d impressions", p.Shown)
			rejected++
		default:
			continue
		}
		ttl := ScopeTTL(p.Audience)
		if status == "rejected" {
			ttl = 90 * 24 * time.Hour
		}
		if _, err := UpsertRelationScope(ctx, dbc, p.FromID, p.ToID, p.Type,
			p.Audience, p.Industry, status, 0.8, "bandit", note, ttl); err != nil {
			return promoted, rejected, err
		}
	}
	return promoted, rejected, nil
}
