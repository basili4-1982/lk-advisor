package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"

	"github.com/lib/pq"
)

// GenerateOptions controls a registry candidate-generation run.
type GenerateOptions struct {
	MinCoPurchase   int
	Limit           int
	IncludeVariants bool
	OnlyItems       []string
}

// GenerateResult reports what one generation run produced and skipped.
type GenerateResult struct {
	CoPurchase      int `json:"co_purchase"`
	Variant         int `json:"variant"`
	SkippedRejected int `json:"skipped_rejected"`
	SkippedHub      int `json:"skipped_hub"`
}

// GenerateRelations creates structural variant edges and co-purchase candidates.
func GenerateRelations(ctx context.Context, dbc *sql.DB, opts GenerateOptions) (GenerateResult, error) {
	if opts.MinCoPurchase <= 0 {
		opts.MinCoPurchase = 3
	}
	if opts.Limit <= 0 || opts.Limit > 1000 {
		opts.Limit = 500
	}
	var res GenerateResult
	if opts.IncludeVariants {
		n, err := generateVariants(ctx, dbc, opts.OnlyItems)
		if err != nil {
			return res, err
		}
		res.Variant = n
	}
	cp, rejected, hub, err := generateCoPurchase(ctx, dbc, opts)
	if err != nil {
		return res, err
	}
	res.CoPurchase = cp
	res.SkippedRejected = rejected
	res.SkippedHub = hub
	return res, nil
}

func generateVariants(ctx context.Context, dbc *sql.DB, only []string) (int, error) {
	args := []any{}
	scopeGroups := ""
	if len(only) > 0 {
		args = append(args, pq.Array(only))
		scopeGroups = fmt.Sprintf(" AND item_id = ANY($%d)", len(args))
	}
	scopeOuter := ""
	if len(only) > 0 {
		scopeOuter = fmt.Sprintf(" AND i.item_id = ANY($%d)", len(args))
	}
	scopeUpdate := ""
	if len(only) > 0 {
		scopeUpdate = fmt.Sprintf(" AND from_item_id = ANY($%d)", len(args))
	}
	if _, err := dbc.ExecContext(ctx, `
		UPDATE relations r
		SET status = 'superseded', version = r.version + 1, updated_at = now()
		WHERE r.type = 'variant_of'
		  AND r.status = 'active'
		  AND r.to_item_id <> (
		      SELECT min(i2.item_id) FROM lk_items i2
		      WHERE i2.product_group = (
		          SELECT i3.product_group FROM lk_items i3
		          WHERE i3.item_id = r.from_item_id
		      )
		  )`+scopeUpdate, args...); err != nil {
		return 0, fmt.Errorf("supersede variants: %w", err)
	}

	query := `
		WITH groups AS (
			SELECT product_group, min(item_id) AS rep_id
			FROM lk_items
			WHERE product_group <> ''` + scopeGroups + `
			GROUP BY product_group
			HAVING count(*) > 1
		)
		INSERT INTO relations
			(from_item_id, to_item_id, type, confidence, status, reason, evidence,
			 source, provenance_kind, valid_from)
		SELECT i.item_id, g.rep_id, 'variant_of', 0.99, 'active',
		       'общий product_group',
		       jsonb_build_array(jsonb_build_object(
		           'kind', 'attribute',
		           'summary', 'общий product_group',
		           'details', jsonb_build_object('product_group', i.product_group))),
		       'attribute', 'attribute', now()
		FROM lk_items i
		JOIN groups g ON g.product_group = i.product_group
		WHERE i.item_id <> g.rep_id` + scopeOuter + `
		ON CONFLICT (from_item_id, to_item_id, type) DO NOTHING`
	res, err := dbc.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("generate variants: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func generateCoPurchase(ctx context.Context, dbc *sql.DB, opts GenerateOptions) (created, rejected, hub int, err error) {
	args := []any{opts.MinCoPurchase}
	scope := ""
	if len(opts.OnlyItems) > 0 {
		args = append(args, pq.Array(opts.OnlyItems))
		scope = fmt.Sprintf(" AND p.item_a = ANY($%d) AND p.item_b = ANY($%d)", len(args), len(args))
	}
	args = append(args, opts.Limit)
	limit := len(args)

	query := `
		WITH degrees AS (
			SELECT item_b AS item_id, count(*) AS degree
			FROM co_purchase_pairs
			GROUP BY item_b
		), hub AS (
			SELECT COALESCE(
			         percentile_cont(0.99) WITHIN GROUP (ORDER BY degree), 0
			       )::float8 AS threshold
			FROM degrees
		)
		SELECT p.item_a, p.item_b, p.count, p.b2b_count,
		       COALESCE(d.degree, 0)::int AS target_degree,
		       h.threshold,
		       EXISTS (
		           SELECT 1 FROM relations r
		           WHERE r.from_item_id = p.item_a
		             AND r.to_item_id = p.item_b
		             AND r.type = 'adjacent_task'
		             AND r.status = 'rejected'
		             AND r.rejected_until IS NOT NULL
		             AND r.rejected_until > now()
		       ) AS rejected
		FROM co_purchase_pairs p
		LEFT JOIN degrees d ON d.item_id = p.item_b
		CROSS JOIN hub h
		WHERE p.b2b_count >= $1` + scope + `
		ORDER BY p.count DESC, p.item_a, p.item_b
		LIMIT $` + fmt.Sprintf("%d", limit)

	rows, err := dbc.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("generate co-purchase: %w", err)
	}
	defer rows.Close()

	ttlDays := relationTTLDays(ctx, dbc, "adjacent_task")
	for rows.Next() {
		var from, to string
		var count, b2b, targetDegree int
		var threshold float64
		var wasRejected bool
		if err := rows.Scan(&from, &to, &count, &b2b, &targetDegree, &threshold, &wasRejected); err != nil {
			return 0, 0, 0, err
		}
		if wasRejected {
			rejected++
			continue
		}
		if float64(targetDegree) > threshold {
			hub++
			continue
		}
		confidence := math.Min(0.90, 0.55+float64(count)*0.03)
		evidence, _ := json.Marshal([]map[string]any{{
			"kind":    "co_purchase",
			"summary": fmt.Sprintf("%d заказов, B2B %d", count, b2b),
			"details": map[string]any{"orders": count, "b2b": b2b},
		}})
		res, err := dbc.ExecContext(ctx, `
			INSERT INTO relations
				(from_item_id, to_item_id, type, confidence, status, reason, evidence,
				 source, provenance_kind, metadata, revalidate_after)
			VALUES ($1, $2, 'adjacent_task', $3, 'candidate', $4, $5,
			        'co_purchase', 'co_purchase', '{"typing":"pending"}'::jsonb,
			        now() + ($6 || ' days')::interval)
			ON CONFLICT (from_item_id, to_item_id, type) DO UPDATE
			SET status = 'candidate',
			    version = relations.version + 1,
			    rejection_reason = '',
			    rejected_until = NULL,
			    updated_at = now()
			WHERE relations.status IN ('rejected', 'stale', 'expired')`,
			from, to, confidence, fmt.Sprintf("куплено вместе %d раз", count), evidence, ttlDays)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("insert co-purchase candidate: %w", err)
		}
		if n, _ := res.RowsAffected(); n > 0 {
			created++
		}
	}
	return created, rejected, hub, rows.Err()
}

// relationTTLDays returns the registry TTL for a relation type, with a safe default.
func relationTTLDays(ctx context.Context, dbc *sql.DB, code string) int {
	var days sql.NullInt64
	if err := dbc.QueryRowContext(ctx,
		`SELECT default_ttl_days FROM relation_types WHERE code = $1`, code).Scan(&days); err != nil ||
		!days.Valid || days.Int64 <= 0 {
		return 180
	}
	return int(days.Int64)
}
