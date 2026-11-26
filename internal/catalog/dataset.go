package catalog

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lib/pq"
)

// DatasetCoverage is the relation-completion report for the real dataset.
type DatasetCoverage struct {
	Items           int `json:"items"`
	Clients         int `json:"clients"`
	Orders          int `json:"orders"`
	OrderInstances  int `json:"order_instances"`
	MultiWeekOrders int `json:"multi_week_orders"`
	CrossCutOrders  int `json:"cross_cut_orders"`
	Pairs           int `json:"pairs"`
	FrequentPairs   int `json:"frequent_pairs"`
	ItemsInPairs    int `json:"items_in_pairs"`
	SoloItems       int `json:"solo_items"`
	Candidates      int `json:"candidates"`
}

// Dataset train/test cut used by analysis/evaluate.py.
const (
	datasetTrainEnd  = "2026-08-31"
	datasetTestStart = "2026-09-07"
)

// CoPurchase is one frequent co-purchase pair.
type CoPurchase struct {
	ItemA     string `json:"item_a"`
	ItemB     string `json:"item_b"`
	TitleA    string `json:"title_a"`
	TitleB    string `json:"title_b"`
	CategoryA string `json:"category_a"`
	CategoryB string `json:"category_b"`
	Count     int    `json:"count"`
	B2B       int    `json:"b2b_count"`
	B2C       int    `json:"b2c_count"`
}

// LoadDatasetCoverage returns counts for the imported v3 dataset.
func LoadDatasetCoverage(ctx context.Context, dbc *sql.DB) (DatasetCoverage, error) {
	var c DatasetCoverage
	if err := dbc.QueryRowContext(ctx, `SELECT count(*) FROM lk_items`).Scan(&c.Items); err != nil {
		return c, fmt.Errorf("coverage items: %w", err)
	}
	if err := dbc.QueryRowContext(ctx, `SELECT count(*) FROM lk_clients`).Scan(&c.Clients); err != nil {
		return c, err
	}
	if err := dbc.QueryRowContext(ctx, `SELECT count(DISTINCT order_id) FROM lk_orders`).Scan(&c.Orders); err != nil {
		return c, err
	}
	if err := dbc.QueryRowContext(ctx, `
		SELECT count(*) FROM (
			SELECT DISTINCT order_id, order_week FROM lk_orders
		) x`).Scan(&c.OrderInstances); err != nil {
		return c, err
	}
	if err := dbc.QueryRowContext(ctx, `
		SELECT count(*) FROM (
			SELECT order_id FROM lk_orders
			GROUP BY order_id
			HAVING count(DISTINCT order_week) > 1
		) x`).Scan(&c.MultiWeekOrders); err != nil {
		return c, err
	}
	if err := dbc.QueryRowContext(ctx, `
		SELECT count(*) FROM (
			SELECT order_id FROM lk_orders
			GROUP BY order_id
			HAVING min(order_week) <= $1::date AND max(order_week) >= $2::date
		) x`, datasetTrainEnd, datasetTestStart).Scan(&c.CrossCutOrders); err != nil {
		return c, err
	}
	if err := dbc.QueryRowContext(ctx, `SELECT count(*) FROM co_purchase_pairs`).Scan(&c.Pairs); err != nil {
		return c, err
	}
	if err := dbc.QueryRowContext(ctx, `SELECT count(*) FROM co_purchase_pairs WHERE count >= 2`).Scan(&c.FrequentPairs); err != nil {
		return c, err
	}
	if err := dbc.QueryRowContext(ctx, `
		SELECT count(*) FROM (
			SELECT item_a AS item_id FROM co_purchase_pairs
			UNION
			SELECT item_b AS item_id FROM co_purchase_pairs
		) x`).Scan(&c.ItemsInPairs); err != nil {
		return c, err
	}
	if err := dbc.QueryRowContext(ctx, `SELECT count(*) FROM lk_relation_candidates`).Scan(&c.Candidates); err != nil {
		return c, err
	}
	c.SoloItems = c.Items - c.ItemsInPairs
	return c, nil
}

// TopCoPurchases returns the strongest co-purchase pairs.
func TopCoPurchases(ctx context.Context, dbc *sql.DB, limit int) ([]CoPurchase, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := dbc.QueryContext(ctx, `
		SELECT p.item_a, p.item_b, a.title, b.title, a.category, b.category,
		       p.count, p.b2b_count, p.b2c_count
		FROM co_purchase_pairs p
		JOIN lk_items a ON a.item_id = p.item_a
		JOIN lk_items b ON b.item_id = p.item_b
		ORDER BY p.count DESC, p.item_a, p.item_b
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("top co-purchases: %w", err)
	}
	defer rows.Close()
	var out []CoPurchase
	for rows.Next() {
		var p CoPurchase
		if err := rows.Scan(&p.ItemA, &p.ItemB, &p.TitleA, &p.TitleB,
			&p.CategoryA, &p.CategoryB, &p.Count, &p.B2B, &p.B2C); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// CreateCoPurchaseCandidates turns strong co-purchase pairs into reviewable relations.
func CreateCoPurchaseCandidates(ctx context.Context, dbc *sql.DB, limit, minCount int) (int, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	if minCount < 1 {
		minCount = 2
	}
	res, err := dbc.ExecContext(ctx, `
		INSERT INTO lk_relation_candidates
			(from_item_id, to_item_id, type, confidence, status, reason, evidence, source)
		SELECT p.item_a, p.item_b, 'adjacent_task',
		       LEAST(0.90, 0.55 + p.count * 0.03),
		       'quarantine',
		       'куплено вместе ' || p.count || ' раз',
		       jsonb_build_array(jsonb_build_object('source', 'co_purchase', 'value', 'count=' || p.count)),
		       'co_purchase'
		FROM co_purchase_pairs p
		WHERE p.count >= $2
		ORDER BY p.count DESC, p.item_a, p.item_b
		LIMIT $1
		ON CONFLICT (from_item_id, to_item_id, type) DO NOTHING`, limit, minCount)
	if err != nil {
		return 0, fmt.Errorf("create co-purchase candidates: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// DatasetItem is one item from the real dataset.
type DatasetItem struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Category    string `json:"category"`
	Subcategory string `json:"subcategory"`
}

// DatasetItems returns item cards by id.
func DatasetItems(ctx context.Context, dbc *sql.DB, ids []string) (map[string]DatasetItem, error) {
	if len(ids) == 0 {
		return map[string]DatasetItem{}, nil
	}
	rows, err := dbc.QueryContext(ctx, `
		SELECT item_id, title, category, subcategory
		FROM lk_items WHERE item_id = ANY($1)`, pq.Array(ids))
	if err != nil {
		return nil, fmt.Errorf("dataset items: %w", err)
	}
	defer rows.Close()
	out := map[string]DatasetItem{}
	for rows.Next() {
		var x DatasetItem
		if err := rows.Scan(&x.ID, &x.Title, &x.Category, &x.Subcategory); err != nil {
			return nil, err
		}
		out[x.ID] = x
	}
	return out, rows.Err()
}

// DatasetCandidate is a relation suggestion built from the real dataset.
type DatasetCandidate struct {
	ID         int64   `json:"id"`
	FromID     string  `json:"from_item_id"`
	ToID       string  `json:"to_item_id"`
	TitleA     string  `json:"title_a"`
	TitleB     string  `json:"title_b"`
	Type       string  `json:"type"`
	Confidence float64 `json:"confidence"`
	Status     string  `json:"status"`
	Reason     string  `json:"reason"`
	Evidence   string  `json:"evidence"`
	Source     string  `json:"source"`
	ReviewNote string  `json:"review_note"`
}

// DatasetCandidates lists relation proposals for the real dataset.
func DatasetCandidates(ctx context.Context, dbc *sql.DB, limit int) ([]DatasetCandidate, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := dbc.QueryContext(ctx, `
		SELECT c.id, c.from_item_id, c.to_item_id, a.title, b.title,
		       c.type, c.confidence::float8, c.status, c.reason,
		       c.evidence::text, c.source, c.review_note
		FROM lk_relation_candidates c
		JOIN lk_items a ON a.item_id = c.from_item_id
		JOIN lk_items b ON b.item_id = c.to_item_id
		ORDER BY c.confidence DESC, c.id
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("dataset candidates: %w", err)
	}
	defer rows.Close()
	var out []DatasetCandidate
	for rows.Next() {
		var c DatasetCandidate
		if err := rows.Scan(&c.ID, &c.FromID, &c.ToID, &c.TitleA, &c.TitleB,
			&c.Type, &c.Confidence, &c.Status, &c.Reason, &c.Evidence,
			&c.Source, &c.ReviewNote); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// InsertDatasetCandidate stores one proposal.
func InsertDatasetCandidate(ctx context.Context, dbc *sql.DB, from, to, typ string, confidence float64, reason, evidence, source string) (bool, error) {
	res, err := dbc.ExecContext(ctx, `
		INSERT INTO lk_relation_candidates
			(from_item_id, to_item_id, type, confidence, status, reason, evidence, source)
		VALUES ($1,$2,$3,$4,'quarantine',$5,$6,$7)
		ON CONFLICT (from_item_id, to_item_id, type) DO NOTHING`,
		from, to, typ, confidence, reason, evidence, source)
	if err != nil {
		return false, fmt.Errorf("insert dataset candidate: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ReviewDatasetCandidate confirms or rejects one proposal.
func ReviewDatasetCandidate(ctx context.Context, dbc *sql.DB, id int64, status, note string) (bool, error) {
	res, err := dbc.ExecContext(ctx, `
		UPDATE lk_relation_candidates
		SET status=$2, reviewed_by='admin', reviewed_at=now(), review_note=$3
		WHERE id=$1`, id, status, note)
	if err != nil {
		return false, fmt.Errorf("review dataset candidate: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
