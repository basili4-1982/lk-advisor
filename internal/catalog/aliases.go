package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ErrItemNotFound means the alias target does not exist in lk_items.
var ErrItemNotFound = errors.New("item not found")

// UpsertItemAlias stores a curated short name for one catalog item.
// An empty shortName removes the alias, falling back to the UI heuristic.
func UpsertItemAlias(ctx context.Context, dbc *sql.DB, itemID, shortName, actor string) error {
	itemID = strings.TrimSpace(itemID)
	shortName = strings.TrimSpace(shortName)
	if itemID == "" {
		return ErrItemNotFound
	}
	if shortName == "" {
		if _, err := dbc.ExecContext(ctx,
			`DELETE FROM item_aliases WHERE item_id = $1`, itemID); err != nil {
			return fmt.Errorf("delete item alias: %w", err)
		}
		return nil
	}
	if len([]rune(shortName)) > 120 {
		return fmt.Errorf("short name is too long")
	}
	var exists bool
	if err := dbc.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM lk_items WHERE item_id = $1)`, itemID).Scan(&exists); err != nil {
		return fmt.Errorf("item alias: check item: %w", err)
	}
	if !exists {
		return ErrItemNotFound
	}
	if _, err := dbc.ExecContext(ctx, `
		INSERT INTO item_aliases (item_id, short_name, updated_by, updated_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (item_id) DO UPDATE
		SET short_name = EXCLUDED.short_name,
		    updated_by = EXCLUDED.updated_by,
		    updated_at = now()`, itemID, shortName, actor); err != nil {
		return fmt.Errorf("upsert item alias: %w", err)
	}
	return nil
}
