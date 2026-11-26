package catalog

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/basili4-1982/lk-advisor/internal/model"
)

// LoadRelationScopes returns scoped decisions with their TTL and titles.
func LoadRelationScopes(ctx context.Context, dbc *sql.DB, limit int) ([]model.RelationScope, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := dbc.QueryContext(ctx, `
		SELECT sc.id, sc.from_id, sc.to_id, sc.type,
		       COALESCE(a.title, sc.from_id), COALESCE(b.title, sc.to_id),
		       sc.audience, sc.industry, sc.status, sc.confidence::float8,
		       sc.reviewed_by, sc.reviewed_at, sc.valid_until, sc.last_tested_at,
		       sc.review_note,
		       (sc.valid_until IS NOT NULL AND sc.valid_until <= now())
		FROM relation_scopes sc
		LEFT JOIN products a ON a.id = sc.from_id
		LEFT JOIN products b ON b.id = sc.to_id
		ORDER BY sc.updated_at DESC, sc.id DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("relation scopes: %w", err)
	}
	defer rows.Close()
	out := []model.RelationScope{}
	for rows.Next() {
		var s model.RelationScope
		if err := rows.Scan(&s.ID, &s.FromID, &s.ToID, &s.Type, &s.TitleFrom, &s.TitleTo,
			&s.Audience, &s.Industry, &s.Status, &s.Confidence,
			&s.ReviewedBy, &s.ReviewedAt, &s.ValidUntil, &s.LastTestedAt,
			&s.ReviewNote, &s.Expired); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// UpsertRelationScope stores a decision for one audience/industry scope.
func UpsertRelationScope(ctx context.Context, dbc *sql.DB, from, to, typ, audience, industry,
	status string, confidence float64, reviewedBy, note string, ttl time.Duration) (int64, error) {
	if confidence <= 0 || confidence > 1 {
		confidence = 0.8
	}
	audience = strings.ToLower(strings.TrimSpace(audience))
	industry = strings.ToLower(strings.TrimSpace(industry))
	status = strings.ToLower(strings.TrimSpace(status))
	seconds := int64(ttl / time.Second)
	var id int64
	err := dbc.QueryRowContext(ctx, `
		INSERT INTO relation_scopes
			(from_id, to_id, type, audience, industry, status, confidence,
			 reviewed_by, reviewed_at, valid_until, last_tested_at, review_note, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now(),
		        CASE WHEN $9::bigint > 0 THEN now() + ($9 * interval '1 second') ELSE NULL END,
		        now(), $10, now())
		ON CONFLICT (from_id, to_id, type, audience, industry)
		DO UPDATE SET
			status = EXCLUDED.status,
			confidence = EXCLUDED.confidence,
			reviewed_by = EXCLUDED.reviewed_by,
			reviewed_at = EXCLUDED.reviewed_at,
			valid_until = EXCLUDED.valid_until,
			last_tested_at = EXCLUDED.last_tested_at,
			review_note = EXCLUDED.review_note,
			updated_at = now()
		RETURNING id`,
		from, to, typ, audience, industry, status, confidence,
		reviewedBy, seconds, note).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("upsert relation scope: %w", err)
	}
	return id, nil
}

// ReviewRelationScope applies a human decision to one scope.
func ReviewRelationScope(ctx context.Context, dbc *sql.DB, from, to, typ, audience, industry,
	status, note string, ttl time.Duration) (bool, error) {
	switch status {
	case "confirmed", "rejected", "quarantine", "testing", "expired":
	default:
		return false, fmt.Errorf("unknown scope status %q", status)
	}
	var exists bool
	if err := dbc.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM product_relations
			WHERE from_id = $1 AND to_id = $2 AND type = $3
		)`, from, to, typ).Scan(&exists); err != nil {
		return false, fmt.Errorf("relation scope relation: %w", err)
	}
	if !exists {
		return false, nil
	}
	if status == "confirmed" && ttl <= 0 {
		ttl = ScopeTTL(audience)
	}
	if status == "rejected" && ttl <= 0 {
		ttl = 90 * 24 * time.Hour
	}
	if _, err := UpsertRelationScope(ctx, dbc, from, to, typ, audience, industry,
		status, 0.8, "admin", note, ttl); err != nil {
		return false, err
	}
	return true, nil
}
