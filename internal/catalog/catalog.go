// Package catalog reads products, relations and promotions.
package catalog

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"

	"github.com/basili4-1982/lk-advisor/internal/model"
)

// Relations returns outgoing edges for the given products.
// Only confirmed and llm_suggested edges are shown to clients.
func Relations(ctx context.Context, dbc *sql.DB, ids []string) ([]model.Relation, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := dbc.QueryContext(ctx, `
		SELECT from_id, to_id, type, direction, confidence, status, source
		FROM product_relations
		WHERE (from_id = ANY($1) OR (to_id = ANY($1) AND type = 'upgrade'))
		  AND status IN ('confirmed', 'llm_suggested')
		ORDER BY from_id, to_id, type`, pq.Array(ids))
	if err != nil {
		return nil, fmt.Errorf("relations: %w", err)
	}
	defer rows.Close()

	var out []model.Relation
	for rows.Next() {
		var r model.Relation
		if err := rows.Scan(&r.FromID, &r.ToID, &r.Type, &r.Direction,
			&r.Confidence, &r.Status, &r.Source); err != nil {
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
