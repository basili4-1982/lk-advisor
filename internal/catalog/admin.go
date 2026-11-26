package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lib/pq"

	"github.com/basili4-1982/lk-advisor/internal/model"
)

// AllRelations returns every product relation for the admin editor.
func AllRelations(ctx context.Context, dbc *sql.DB) ([]model.Relation, error) {
	rows, err := dbc.QueryContext(ctx, `
		SELECT from_id, to_id, type, direction, confidence, status, source,
		       reason, evidence, reviewed_by, reviewed_at, review_note
		FROM product_relations
		ORDER BY from_id, to_id, type`)
	if err != nil {
		return nil, fmt.Errorf("all relations: %w", err)
	}
	defer rows.Close()

	var out []model.Relation
	for rows.Next() {
		r, err := scanRelationReview(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRelationReview(scanner rowScanner) (model.Relation, error) {
	var r model.Relation
	var evidence []byte
	var reviewedAt sql.NullTime
	if err := scanner.Scan(&r.FromID, &r.ToID, &r.Type, &r.Direction,
		&r.Confidence, &r.Status, &r.Source, &r.Reason, &evidence,
		&r.ReviewedBy, &reviewedAt, &r.ReviewNote); err != nil {
		return model.Relation{}, err
	}
	if len(evidence) > 0 {
		_ = json.Unmarshal(evidence, &r.Evidence)
	}
	if reviewedAt.Valid {
		t := reviewedAt.Time
		r.ReviewedAt = &t
	}
	return r, nil
}

// UpsertRelation creates or updates one relation as an admin change.
func UpsertRelation(ctx context.Context, dbc *sql.DB, r model.Relation) error {
	evidence, err := json.Marshal(r.Evidence)
	if err != nil {
		return fmt.Errorf("relation evidence: %w", err)
	}
	if len(evidence) == 0 || string(evidence) == "null" {
		evidence = []byte("[]")
	}
	_, err = dbc.ExecContext(ctx, `
		INSERT INTO product_relations
			(from_id, to_id, type, direction, confidence, status, source, reason, evidence)
		VALUES ($1, $2, $3, $4, $5, $6, $9, $7, $8)
		ON CONFLICT (from_id, to_id, type) DO UPDATE
		SET direction = EXCLUDED.direction,
		    confidence = EXCLUDED.confidence,
		    status = EXCLUDED.status,
		    source = EXCLUDED.source,
		    reason = EXCLUDED.reason,
		    evidence = EXCLUDED.evidence,
		    reviewed_by = CASE WHEN EXCLUDED.status IN ('quarantine', 'llm_suggested')
		                        THEN '' ELSE product_relations.reviewed_by END,
		    reviewed_at = CASE WHEN EXCLUDED.status IN ('quarantine', 'llm_suggested')
		                        THEN NULL ELSE product_relations.reviewed_at END,
		    review_note = CASE WHEN EXCLUDED.status IN ('quarantine', 'llm_suggested')
		                        THEN '' ELSE product_relations.review_note END`,
		r.FromID, r.ToID, r.Type, r.Direction, r.Confidence, r.Status,
		r.Reason, string(evidence), relationSource(r.Source))
	if err != nil {
		return fmt.Errorf("upsert relation: %w", err)
	}
	return nil
}

// DeleteRelation removes one relation by its natural key.
func DeleteRelation(ctx context.Context, dbc *sql.DB, fromID, toID, typ string) (bool, error) {
	res, err := dbc.ExecContext(ctx, `
		DELETE FROM product_relations WHERE from_id = $1 AND to_id = $2 AND type = $3`,
		fromID, toID, typ)
	if err != nil {
		return false, fmt.Errorf("delete relation: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// InsertRule stores one business rule.
func InsertRule(ctx context.Context, dbc *sql.DB, r model.Rule) error {
	_, err := dbc.ExecContext(ctx, `
		INSERT INTO rules (id, type, condition, action) VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE SET type = EXCLUDED.type,
			condition = EXCLUDED.condition, action = EXCLUDED.action`,
		r.ID, r.Type, r.Condition, r.Action)
	if err != nil {
		return fmt.Errorf("insert rule: %w", err)
	}
	return nil
}

// DeleteRule removes one rule by id.
func DeleteRule(ctx context.Context, dbc *sql.DB, id string) (bool, error) {
	res, err := dbc.ExecContext(ctx, `DELETE FROM rules WHERE id = $1`, id)
	if err != nil {
		return false, fmt.Errorf("delete rule: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// AllClients returns client ids and names for the preview picker.
func AllClients(ctx context.Context, dbc *sql.DB) ([]model.Client, error) {
	rows, err := dbc.QueryContext(ctx, `SELECT id, name FROM clients ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("clients: %w", err)
	}
	defer rows.Close()

	var out []model.Client
	for rows.Next() {
		var c model.Client
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// LogAdminChange records a rule change for audit.
func LogAdminChange(ctx context.Context, dbc *sql.DB, actor, action string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("audit encode: %w", err)
	}
	_, err = dbc.ExecContext(ctx, `
		INSERT INTO admin_audit (actor, action, payload) VALUES ($1, $2, $3)`,
		actor, action, raw)
	if err != nil {
		return fmt.Errorf("audit: %w", err)
	}
	return nil
}

// AdminAuditEntry is one recorded rule change.
type AdminAuditEntry struct {
	ID        int64           `json:"id"`
	Actor     string          `json:"actor"`
	Action    string          `json:"action"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"created_at"`
}

// AdminAudit returns the latest rule changes.
func AdminAudit(ctx context.Context, dbc *sql.DB, limit int) ([]AdminAuditEntry, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := dbc.QueryContext(ctx, `
		SELECT id, actor, action, payload, created_at
		FROM admin_audit ORDER BY id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("admin audit: %w", err)
	}
	defer rows.Close()
	var out []AdminAuditEntry
	for rows.Next() {
		var e AdminAuditEntry
		if err := rows.Scan(&e.ID, &e.Actor, &e.Action, &e.Payload, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// RelationTypes returns all editable relation templates.
func RelationTypes(ctx context.Context, dbc *sql.DB) ([]model.RelationType, error) {
	rows, err := dbc.QueryContext(ctx, `
		SELECT code, title, description, examples, behavior, direction,
		       default_confidence, status, builtin, updated_at
		FROM relation_types
		ORDER BY builtin DESC, title`)
	if err != nil {
		return nil, fmt.Errorf("relation types: %w", err)
	}
	defer rows.Close()
	var out []model.RelationType
	for rows.Next() {
		var rt model.RelationType
		if err := rows.Scan(&rt.Code, &rt.Title, &rt.Description, &rt.Examples,
			&rt.Behavior, &rt.Direction, &rt.DefaultConfidence, &rt.Status,
			&rt.Builtin, &rt.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, rt)
	}
	return out, rows.Err()
}

// RelationType returns one template by code.
func RelationType(ctx context.Context, dbc *sql.DB, code string) (model.RelationType, error) {
	var rt model.RelationType
	err := dbc.QueryRowContext(ctx, `
		SELECT code, title, description, examples, behavior, direction,
		       default_confidence, status, builtin, updated_at
		FROM relation_types WHERE code = $1`, code).
		Scan(&rt.Code, &rt.Title, &rt.Description, &rt.Examples,
			&rt.Behavior, &rt.Direction, &rt.DefaultConfidence, &rt.Status,
			&rt.Builtin, &rt.UpdatedAt)
	if err != nil {
		return model.RelationType{}, err
	}
	return rt, nil
}

// UpsertRelationType creates or updates a template.
func UpsertRelationType(ctx context.Context, dbc *sql.DB, rt model.RelationType) error {
	_, err := dbc.ExecContext(ctx, `
		INSERT INTO relation_types
			(code, title, description, examples, behavior, direction,
			 default_confidence, status, builtin, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now())
		ON CONFLICT (code) DO UPDATE SET
			title = EXCLUDED.title,
			description = EXCLUDED.description,
			examples = EXCLUDED.examples,
			behavior = EXCLUDED.behavior,
			direction = EXCLUDED.direction,
			default_confidence = EXCLUDED.default_confidence,
			status = EXCLUDED.status,
			updated_at = now()`,
		rt.Code, rt.Title, rt.Description, rt.Examples, rt.Behavior, rt.Direction,
		rt.DefaultConfidence, rt.Status, rt.Builtin)
	if err != nil {
		return fmt.Errorf("upsert relation type: %w", err)
	}
	return nil
}

// DeleteRelationType removes a custom template.
func DeleteRelationType(ctx context.Context, dbc *sql.DB, code string) (bool, error) {
	res, err := dbc.ExecContext(ctx, `DELETE FROM relation_types WHERE code = $1 AND builtin = false`, code)
	if err != nil {
		return false, fmt.Errorf("delete relation type: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// RelationTypeMap returns active and draft templates keyed by code for the engine.
func RelationTypeMap(ctx context.Context, dbc *sql.DB) (map[string]model.RelationType, error) {
	types, err := RelationTypes(ctx, dbc)
	if err != nil {
		return nil, err
	}
	out := make(map[string]model.RelationType, len(types))
	for _, rt := range types {
		out[rt.Code] = rt
	}
	return out, nil
}

// ReviewRelations returns relations waiting for a human decision.
func ReviewRelations(ctx context.Context, dbc *sql.DB, statuses []string, limit int) ([]model.Relation, error) {
	if len(statuses) == 0 {
		statuses = []string{"quarantine", "llm_suggested"}
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := dbc.QueryContext(ctx, `
		SELECT from_id, to_id, type, direction, confidence, status, source,
		       reason, evidence, reviewed_by, reviewed_at, review_note
		FROM product_relations
		WHERE status = ANY($1)
		ORDER BY confidence ASC, from_id, to_id, type
		LIMIT $2`, pq.Array(statuses), limit)
	if err != nil {
		return nil, fmt.Errorf("review relations: %w", err)
	}
	defer rows.Close()
	var out []model.Relation
	for rows.Next() {
		r, err := scanRelationReview(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ReviewCounts returns how many relations are waiting in each review status.
func ReviewCounts(ctx context.Context, dbc *sql.DB) (map[string]int, error) {
	rows, err := dbc.QueryContext(ctx, `
		SELECT status, count(*) FROM product_relations
		WHERE status IN ('quarantine', 'llm_suggested')
		GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("review counts: %w", err)
	}
	defer rows.Close()
	out := map[string]int{"quarantine": 0, "llm_suggested": 0}
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return nil, err
		}
		out[status] = n
	}
	return out, rows.Err()
}

// ReviewRelation applies a human decision to one relation.
func ReviewRelation(ctx context.Context, dbc *sql.DB, fromID, toID, typ, status, note string) (bool, error) {
	res, err := dbc.ExecContext(ctx, `
		UPDATE product_relations
		SET status = $4, reviewed_by = 'admin', reviewed_at = now(), review_note = $5
		WHERE from_id = $1 AND to_id = $2 AND type = $3`,
		fromID, toID, typ, status, note)
	if err != nil {
		return false, fmt.Errorf("review relation: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func relationSource(source string) string {
	if source == "" {
		return "admin"
	}
	return source
}

// SoloProducts returns active products without any relation.
func SoloProducts(ctx context.Context, dbc *sql.DB, limit int) ([]model.Product, int, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var total int
	if err := dbc.QueryRowContext(ctx, `
		SELECT count(*) FROM products p
		WHERE p.status = 'active'
		  AND NOT EXISTS (SELECT 1 FROM product_relations r WHERE r.from_id = p.id OR r.to_id = p.id)`).
		Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("solo count: %w", err)
	}
	rows, err := dbc.QueryContext(ctx, `
		SELECT `+productColumns+` FROM products p
		WHERE p.status = 'active'
		  AND NOT EXISTS (SELECT 1 FROM product_relations r WHERE r.from_id = p.id OR r.to_id = p.id)
		ORDER BY p.title
		LIMIT $1`, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("solo products: %w", err)
	}
	defer rows.Close()
	var out []model.Product
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}

// FlagRelationsForReview marks incoming relations of low-click products for human review.
func FlagRelationsForReview(ctx context.Context, dbc *sql.DB, productIDs []string, note string) (int, error) {
	if len(productIDs) == 0 {
		return 0, nil
	}
	res, err := dbc.ExecContext(ctx, `
		UPDATE product_relations
		SET review_note = $1,
		    reviewed_by = '',
		    reviewed_at = NULL
		WHERE to_id = ANY($2)
		  AND status IN ('confirmed', 'llm_suggested')`,
		note, pq.Array(productIDs))
	if err != nil {
		return 0, fmt.Errorf("flag relations: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}
