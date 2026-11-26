// Package catalog reads products, relations and promotions.
package catalog

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"

	"github.com/basili4-1982/lk-advisor/internal/model"
)

// Relations returns outgoing edges for the given products.
// Only confirmed and llm_suggested edges are shown to clients.
func Relations(ctx context.Context, dbc *sql.DB, ids []string) ([]model.Relation, error) {
	return RelationsScoped(ctx, dbc, ids, "", "", time.Now())
}

// RelationsScoped resolves the effective status per audience/industry scope.
// A scope can confirm a link for one segment and reject it for another.
func RelationsScoped(ctx context.Context, dbc *sql.DB, ids []string, audience, industry string, at time.Time) ([]model.Relation, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	audience = strings.ToLower(strings.TrimSpace(audience))
	industry = strings.ToLower(strings.TrimSpace(industry))
	rows, err := dbc.QueryContext(ctx, `
		SELECT r.from_id, r.to_id, r.type, r.direction, r.confidence,
		       COALESCE(s.status, r.status), r.source,
		       COALESCE(s.audience, ''), COALESCE(s.industry, ''),
		       s.valid_until, s.id IS NOT NULL
		FROM product_relations r
		LEFT JOIN LATERAL (
			SELECT sc.id, sc.status, sc.audience, sc.industry, sc.valid_until
			FROM relation_scopes sc
			WHERE sc.from_id = r.from_id
			  AND sc.to_id = r.to_id
			  AND sc.type = r.type
			  AND (sc.audience = $2 OR sc.audience = '')
			  AND (sc.industry = $3 OR sc.industry = '')
			ORDER BY (sc.audience = $2 AND sc.industry = $3) DESC,
			         (sc.audience = $2 AND sc.industry = '') DESC,
			         (sc.audience = '' AND sc.industry = $3) DESC,
			         (sc.audience = '') DESC,
			         sc.updated_at DESC
			LIMIT 1
		) s ON true
		WHERE (r.from_id = ANY($1) OR (r.to_id = ANY($1) AND r.type = 'upgrade'))
		  AND COALESCE(s.status, r.status) IN ('confirmed', 'llm_suggested', 'testing')
		  AND (s.valid_until IS NULL OR s.valid_until > $4)
		ORDER BY r.from_id, r.to_id, r.type`, pq.Array(ids), audience, industry, at)
	if err != nil {
		return nil, fmt.Errorf("relations: %w", err)
	}
	defer rows.Close()

	var out []model.Relation
	for rows.Next() {
		var r model.Relation
		if err := rows.Scan(&r.FromID, &r.ToID, &r.Type, &r.Direction,
			&r.Confidence, &r.Status, &r.Source,
			&r.ScopeAudience, &r.ScopeIndustry, &r.ValidUntil, &r.Scoped); err != nil {
			return nil, fmt.Errorf("scan relation: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Promotions returns promotions active at the given moment.
func Promotions(ctx context.Context, dbc *sql.DB, ids []string, at time.Time) ([]model.Promotion, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := dbc.QueryContext(ctx, `
		SELECT id, product_id, discount, valid_from, valid_to, conditions
		FROM promotions
		WHERE product_id = ANY($1)
		  AND valid_from <= $2
		  AND valid_to >= $2
		ORDER BY id`, pq.Array(ids), at)
	if err != nil {
		return nil, fmt.Errorf("promotions: %w", err)
	}
	defer rows.Close()

	var out []model.Promotion
	for rows.Next() {
		var p model.Promotion
		if err := rows.Scan(&p.ID, &p.ProductID, &p.Discount,
			&p.ValidFrom, &p.ValidTo, &p.Conditions); err != nil {
			return nil, fmt.Errorf("scan promotion: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
