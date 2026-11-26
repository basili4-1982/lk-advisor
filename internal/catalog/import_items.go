package catalog

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
)

// ImportItemsResult reports one incremental catalog upload.
type ImportItemsResult struct {
	Added   int      `json:"added"`
	Updated int      `json:"updated"`
	Skipped int      `json:"skipped"`
	Errors  []string `json:"errors,omitempty"`
}

// ImportItemsCSV upserts catalog positions by item_id.
// It never deletes relations, relation events or item aliases.
func ImportItemsCSV(ctx context.Context, dbc *sql.DB, src io.Reader) (ImportItemsResult, error) {
	var res ImportItemsResult
	raw, err := io.ReadAll(io.LimitReader(src, 12<<20))
	if err != nil {
		return res, fmt.Errorf("read csv: %w", err)
	}
	if len(raw) == 0 {
		return res, fmt.Errorf("empty file")
	}
	delimiter := ','
	if bytes.Count(raw, []byte(";")) > bytes.Count(raw, []byte(",")) {
		delimiter = ';'
	}
	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))))
	reader.Comma = delimiter
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	records, err := reader.ReadAll()
	if err != nil {
		return res, fmt.Errorf("parse csv: %w", err)
	}
	if len(records) < 2 {
		return res, fmt.Errorf("csv has no data rows")
	}
	index := map[string]int{}
	for i, header := range records[0] {
		key := strings.ToLower(strings.TrimSpace(header))
		switch key {
		case "item_name", "name", "название":
			key = "title"
		case "артикул":
			key = "sku"
		case "доступен":
			key = "available"
		}
		index[key] = i
	}
	for _, required := range []string{"item_id", "title"} {
		if _, ok := index[required]; !ok {
			return res, fmt.Errorf("csv must contain %q column", required)
		}
	}

	type row struct {
		id, sku, title, category, subcategory, group string
		available                                    bool
	}
	seen := map[string]row{}
	for line, record := range records[1:] {
		get := func(key string) string {
			if i, ok := index[key]; ok && i < len(record) {
				return strings.TrimSpace(record[i])
			}
			return ""
		}
		id := get("item_id")
		title := get("title")
		if id == "" || title == "" {
			res.Skipped++
			if len(res.Errors) < 5 {
				res.Errors = append(res.Errors, fmt.Sprintf("строка %d: нужны item_id и title", line+2))
			}
			continue
		}
		available := true
		if raw := strings.ToLower(get("available")); raw != "" {
			available = raw == "true" || raw == "1" || raw == "yes" || raw == "да"
		}
		seen[id] = row{
			id: id, sku: get("sku"), title: title, category: get("category"),
			subcategory: get("subcategory"), group: get("product_group"), available: available,
		}
	}
	if len(seen) == 0 {
		return res, fmt.Errorf("no valid rows")
	}

	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	rows, err := dbc.QueryContext(ctx,
		`SELECT item_id FROM lk_items WHERE item_id = ANY($1)`, ids)
	if err != nil {
		return res, fmt.Errorf("existing items: %w", err)
	}
	existing := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return res, err
		}
		existing[id] = true
	}
	rows.Close()

	tx, err := dbc.BeginTx(ctx, nil)
	if err != nil {
		return res, fmt.Errorf("import items: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for id, r := range seen {
		category := r.category
		if category == "" {
			category = "other"
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO lk_items
				(item_id, sku, title, category, subcategory, product_group, available, source)
			VALUES ($1, $2, $3, $4, $5, $6, $7, 'ui_import')
			ON CONFLICT (item_id) DO UPDATE SET
				sku = EXCLUDED.sku,
				title = EXCLUDED.title,
				category = EXCLUDED.category,
				subcategory = COALESCE(NULLIF(EXCLUDED.subcategory, ''), lk_items.subcategory),
				product_group = EXCLUDED.product_group,
				available = EXCLUDED.available`,
			id, r.sku, r.title, category, r.subcategory, r.group, r.available); err != nil {
			return res, fmt.Errorf("upsert item %s: %w", id, err)
		}
		if existing[id] {
			res.Updated++
		} else {
			res.Added++
		}
	}
	if err := tx.Commit(); err != nil {
		return res, fmt.Errorf("import items: commit: %w", err)
	}
	return res, nil
}
