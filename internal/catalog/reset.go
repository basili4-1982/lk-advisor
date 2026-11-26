package catalog

import (
	"context"
	"database/sql"
	"fmt"
)

// ResetResult reports a catalog reset to the clean v5 baseline.
type ResetResult struct {
	ItemsKept    int `json:"items_kept"`
	ItemsRemoved int `json:"items_removed"`
	Relations    int `json:"relations"`
	CoPurchase   int `json:"co_purchase"`
	Variant      int `json:"variant"`
}

// ResetCatalog removes every relation, event, alias and imported position that
// is not part of the v5 dataset, then regenerates candidates from v5.
func ResetCatalog(ctx context.Context, dbc *sql.DB) (ResetResult, error) {
	var res ResetResult
	var before int
	if err := dbc.QueryRowContext(ctx, `SELECT count(*) FROM lk_items`).Scan(&before); err != nil {
		return res, fmt.Errorf("reset catalog: count: %w", err)
	}
	tx, err := dbc.BeginTx(ctx, nil)
	if err != nil {
		return res, fmt.Errorf("reset catalog: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, query := range []string{
		`DELETE FROM relations`,
		`DELETE FROM relation_events`,
		`DELETE FROM item_aliases`,
		`DELETE FROM lk_items WHERE source <> 'dataset_v5'`,
	} {
		if _, err := tx.ExecContext(ctx, query); err != nil {
			return res, fmt.Errorf("reset catalog: %w", err)
		}
	}
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM lk_items`).Scan(&res.ItemsKept); err != nil {
		return res, fmt.Errorf("reset catalog: kept: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return res, fmt.Errorf("reset catalog: commit: %w", err)
	}
	res.ItemsRemoved = before - res.ItemsKept

	gen, err := GenerateRelations(ctx, dbc, GenerateOptions{
		MinCoPurchase: 3, Limit: 500, IncludeVariants: true,
	})
	if err != nil {
		return res, fmt.Errorf("reset catalog: regenerate: %w", err)
	}
	res.CoPurchase = gen.CoPurchase
	res.Variant = gen.Variant
	res.Relations = gen.CoPurchase + gen.Variant
	return res, nil
}
