package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Review errors. They are sentinel values so the HTTP layer can map them to
// 404/409/422 without parsing messages.
var (
	ErrRelationNotFound  = errors.New("relation not found")
	ErrVersionConflict   = errors.New("relation version conflict")
	ErrNoteRequired      = errors.New("review note required")
	ErrCriterionRequired = errors.New("bulk review requires a criterion")
)

// decisionSpec is the per-decision SQL fragment of a review.
// $1 = id, $2 = actor, $3 = note; the shared columns are added by the caller.
type decisionSpec struct {
	status string
	extra  string
}

// ReviewRegistryRelation applies one human decision to a registry edge.
//
// The row is locked with SELECT ... FOR UPDATE, so expectedVersion is checked
// against the locked row: any mismatch is ErrVersionConflict and nothing is
// written. Every accepted decision bumps version, records the reviewer and
// writes one relation_events row with action='review', actor_kind='human' and
// the full row before/after as JSON.
func ReviewRegistryRelation(ctx context.Context, dbc *sql.DB, id int64, decision, note, actor string, expectedVersion int) (RegistryRelation, error) {
	note = strings.TrimSpace(note)
	spec, err := reviewDecision(decision, note)
	if err != nil {
		return RegistryRelation{}, err
	}

	tx, err := dbc.BeginTx(ctx, nil)
	if err != nil {
		return RegistryRelation{}, fmt.Errorf("review relation: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var currentVersion int
	if err := tx.QueryRowContext(ctx,
		`SELECT version FROM relations r WHERE r.id = $1 FOR UPDATE`,
		id).Scan(&currentVersion); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RegistryRelation{}, ErrRelationNotFound
		}
		return RegistryRelation{}, fmt.Errorf("review relation: lock: %w", err)
	}
	if currentVersion != expectedVersion {
		return RegistryRelation{}, ErrVersionConflict
	}

	beforeRel, err := loadRegistryRelation(ctx, tx, id)
	if err != nil {
		return RegistryRelation{}, err
	}
	before, err := json.Marshal(beforeRel)
	if err != nil {
		return RegistryRelation{}, fmt.Errorf("review relation: encode before: %w", err)
	}

	query := fmt.Sprintf(`UPDATE relations
		SET status = $4,
		    version = version + 1,
		    updated_at = now(),
		    reviewed_by = $2,
		    reviewed_at = now(),
		    review_note = $3%s
		WHERE id = $1`, spec.extra)

	res, err := tx.ExecContext(ctx, query, id, actor, note, spec.status)
	if err != nil {
		return RegistryRelation{}, fmt.Errorf("review relation: update: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return RegistryRelation{}, fmt.Errorf("review relation: affected: %w", err)
	} else if n != 1 {
		return RegistryRelation{}, ErrRelationNotFound
	}

	rel, err := loadRegistryRelation(ctx, tx, id)
	if err != nil {
		return RegistryRelation{}, err
	}

	after, err := json.Marshal(rel)
	if err != nil {
		return RegistryRelation{}, fmt.Errorf("review relation: encode after: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO relation_events
			(relation_id, actor_kind, actor_id, action, payload_before, payload_after)
		VALUES ($1, 'human', $2, 'review', $3::jsonb, $4::jsonb)`,
		id, actor, string(before), string(after)); err != nil {
		return RegistryRelation{}, fmt.Errorf("review relation: event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return RegistryRelation{}, fmt.Errorf("review relation: commit: %w", err)
	}
	return rel, nil
}

// reviewDecision builds the decision-specific SET fragment.
func reviewDecision(decision, note string) (decisionSpec, error) {
	switch decision {
	case "confirm":
		return decisionSpec{
			status: "active",
			extra: `,
			    approved_by = $2,
			    approved_at = now(),
			    rejection_reason = '',
			    rejected_until = NULL`,
		}, nil
	case "reject":
		if strings.TrimSpace(note) == "" {
			return decisionSpec{}, ErrNoteRequired
		}
		return decisionSpec{
			status: "rejected",
			extra: `,
			    approved_by = '',
			    approved_at = NULL,
			    rejection_reason = $3,
			    rejected_until = now() + interval '90 days'`,
		}, nil
	case "defer":
		return decisionSpec{
			status: "candidate",
			extra: `,
			    approved_by = '',
			    approved_at = NULL`,
		}, nil
	default:
		return decisionSpec{}, fmt.Errorf("review relation: unknown decision %q", decision)
	}
}

// queryer is the subset of *sql.DB / *sql.Tx used by the registry readers.
type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// loadRegistryRelation reads one edge with both item cards inside a tx.
func loadRegistryRelation(ctx context.Context, q queryer, id int64) (RegistryRelation, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+registryColumns+`, 0 AS total
		FROM relations r
		LEFT JOIN lk_items f ON f.item_id = r.from_item_id
		LEFT JOIN lk_items t ON t.item_id = r.to_item_id
		WHERE r.id = $1`, id)
	if err != nil {
		return RegistryRelation{}, fmt.Errorf("load relation: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return RegistryRelation{}, fmt.Errorf("load relation: %w", err)
		}
		return RegistryRelation{}, ErrRelationNotFound
	}
	rel, _, err := scanRegistryRelation(rows)
	if err != nil {
		return RegistryRelation{}, fmt.Errorf("load relation: scan: %w", err)
	}
	return rel, nil
}

// BulkReviewRelations applies one decision to every edge matching f.
//
// A filter without Types and MinConfidence is refused: bulk review must never
// mean "all rows". At most 500 edges are selected; they are reviewed one by one
// so each one gets its own version check and audit event. Concurrent edits are
// skipped rather than aborting the whole batch, and only successes are counted.
func BulkReviewRelations(ctx context.Context, dbc *sql.DB, f RelationFilter, decision, note, actor string) (int, error) {
	if decision != "confirm" && decision != "reject" && decision != "defer" {
		return 0, fmt.Errorf("bulk review: unknown decision %q", decision)
	}
	if len(f.IDs) == 0 && len(f.Types) == 0 && !(f.MinConfidence > 0) {
		return 0, ErrCriterionRequired
	}
	if len(f.Statuses) == 0 && len(f.IDs) == 0 {
		f.Statuses = []string{"candidate"}
	}
	note = strings.TrimSpace(note)
	if decision == "reject" && note == "" {
		return 0, ErrNoteRequired
	}

	where, args := buildRelationWhere(f)
	query := `SELECT r.id, r.version
		FROM relations r
		LEFT JOIN lk_items f ON f.item_id = r.from_item_id
		LEFT JOIN lk_items t ON t.item_id = r.to_item_id` + where +
		` ORDER BY r.id LIMIT 500`

	rows, err := dbc.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("bulk review: select: %w", err)
	}
	type target struct {
		id      int64
		version int
	}
	var targets []target
	for rows.Next() {
		var t target
		if err := rows.Scan(&t.id, &t.version); err != nil {
			rows.Close()
			return 0, fmt.Errorf("bulk review: scan: %w", err)
		}
		targets = append(targets, t)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, fmt.Errorf("bulk review: rows: %w", err)
	}
	rows.Close()

	done := 0
	for _, t := range targets {
		if _, err := ReviewRegistryRelation(ctx, dbc, t.id, decision, note, actor, t.version); err != nil {
			if errors.Is(err, ErrVersionConflict) || errors.Is(err, ErrRelationNotFound) {
				continue
			}
			return done, err
		}
		done++
	}
	return done, nil
}

// SweepRelations moves overdue edges to stale and expired, in that order.
//
// Stale covers candidate/active edges whose revalidate_after has passed;
// expired covers active edges whose valid_until has passed. Both steps run in
// one transaction and leave a single aggregate relation_events row.
func SweepRelations(ctx context.Context, dbc *sql.DB, now time.Time) (stale, expired int, err error) {
	tx, err := dbc.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("sweep relations: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `
		UPDATE relations SET status = 'stale', version = version + 1, updated_at = now()
		WHERE status IN ('candidate', 'active')
		  AND revalidate_after IS NOT NULL
		  AND revalidate_after < $1`, now)
	if err != nil {
		return 0, 0, fmt.Errorf("sweep relations: stale: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, 0, fmt.Errorf("sweep relations: stale affected: %w", err)
	}
	stale = int(n)

	res, err = tx.ExecContext(ctx, `
		UPDATE relations SET status = 'expired', version = version + 1, updated_at = now()
		WHERE status = 'active'
		  AND valid_until IS NOT NULL
		  AND valid_until < $1`, now)
	if err != nil {
		return 0, 0, fmt.Errorf("sweep relations: expired: %w", err)
	}
	n, err = res.RowsAffected()
	if err != nil {
		return 0, 0, fmt.Errorf("sweep relations: expired affected: %w", err)
	}
	expired = int(n)

	if stale == 0 && expired == 0 {
		if err := tx.Commit(); err != nil {
			return 0, 0, fmt.Errorf("sweep relations: commit: %w", err)
		}
		return 0, 0, nil
	}

	payload, err := json.Marshal(map[string]int{"stale": stale, "expired": expired})
	if err != nil {
		return 0, 0, fmt.Errorf("sweep relations: encode payload: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO relation_events
			(relation_id, actor_kind, actor_id, action, payload_after)
		VALUES (0, 'system', 'sweep', 'sweep', $1::jsonb)`, string(payload)); err != nil {
		return 0, 0, fmt.Errorf("sweep relations: event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("sweep relations: commit: %w", err)
	}
	return stale, expired, nil
}
