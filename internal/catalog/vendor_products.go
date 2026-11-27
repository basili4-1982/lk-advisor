package catalog

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// VendorProduct is an external product/edition from a vendor page or registry.
type VendorProduct struct {
	ID             int64    `json:"id"`
	Vendor         string   `json:"vendor"`
	Product        string   `json:"product"`
	Family         string   `json:"family"`
	Edition        string   `json:"edition"`
	Platforms      []string `json:"platforms"`
	RegistryNumber string   `json:"registry_number"`
	SourceURL      string   `json:"source_url"`
	Aliases        []string `json:"aliases"`
	MappedItems    int      `json:"mapped_items"`
}

// VendorImportResult reports one CSV import.
type VendorImportResult struct {
	Added   int `json:"added"`
	Updated int `json:"updated"`
	Skipped int `json:"skipped"`
}

// VendorLinkResult reports entity resolution.
type VendorLinkResult struct {
	Mappings int `json:"mappings"`
	Products int `json:"products"`
	Items    int `json:"items"`
}

// VendorStats is the external knowledge summary.
type VendorStats struct {
	Products     int `json:"products"`
	Mapped       int `json:"mapped_products"`
	CoveredItems int `json:"covered_items"`
	SoloCovered  int `json:"solo_covered"`
}

// ImportVendorProductsCSV upserts external products.
func ImportVendorProductsCSV(ctx context.Context, dbc *sql.DB, src io.Reader) (VendorImportResult, error) {
	var res VendorImportResult
	reader := csv.NewReader(src)
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		return res, fmt.Errorf("read header: %w", err)
	}
	index := map[string]int{}
	for i, h := range header {
		index[strings.ToLower(strings.TrimSpace(h))] = i
	}
	required := []string{"vendor", "product"}
	for _, name := range required {
		if _, ok := index[name]; !ok {
			return res, fmt.Errorf("csv missing column %q", name)
		}
	}
	get := func(row []string, name string) string {
		i, ok := index[name]
		if !ok || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return res, fmt.Errorf("read csv: %w", err)
		}
		vendor := get(row, "vendor")
		product := get(row, "product")
		if vendor == "" || product == "" {
			res.Skipped++
			continue
		}
		edition := get(row, "edition")
		platforms := splitList(get(row, "platforms"))
		aliases := splitList(get(row, "aliases"))
		platformJSON, _ := json.Marshal(platforms)
		aliasJSON, _ := json.Marshal(aliases)

		var exists bool
		if err := dbc.QueryRowContext(ctx, `
			SELECT EXISTS (SELECT 1 FROM vendor_products
			WHERE vendor = $1 AND product = $2 AND edition = $3)`,
			vendor, product, edition).Scan(&exists); err != nil {
			return res, err
		}
		if _, err := dbc.ExecContext(ctx, `
			INSERT INTO vendor_products
				(vendor, product, family, edition, platforms, registry_number, source_url, aliases)
			VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7, $8::jsonb)
			ON CONFLICT (vendor, product, edition) DO UPDATE SET
				family = EXCLUDED.family,
				platforms = EXCLUDED.platforms,
				registry_number = EXCLUDED.registry_number,
				source_url = EXCLUDED.source_url,
				aliases = EXCLUDED.aliases,
				updated_at = now()`,
			vendor, product, get(row, "family"), edition,
			string(platformJSON), get(row, "registry_number"), get(row, "source_url"),
			string(aliasJSON)); err != nil {
			return res, fmt.Errorf("upsert vendor product: %w", err)
		}
		if exists {
			res.Updated++
		} else {
			res.Added++
		}
	}
	return res, nil
}

func splitList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	parts := strings.Split(raw, "|")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// LinkVendorProducts matches external products to catalog items by aliases.
func LinkVendorProducts(ctx context.Context, dbc *sql.DB) (VendorLinkResult, error) {
	var res VendorLinkResult
	rows, err := dbc.QueryContext(ctx, `
		SELECT id, product, edition, aliases::text
		FROM vendor_products ORDER BY id`)
	if err != nil {
		return res, fmt.Errorf("vendor products: %w", err)
	}
	type product struct {
		id      int64
		product string
		edition string
		aliases []string
	}
	var products []product
	for rows.Next() {
		var p product
		var aliasesRaw string
		if err := rows.Scan(&p.id, &p.product, &p.edition, &aliasesRaw); err != nil {
			rows.Close()
			return res, err
		}
		_ = json.Unmarshal([]byte(aliasesRaw), &p.aliases)
		if len(p.aliases) == 0 {
			p.aliases = []string{strings.TrimSpace(p.product + " " + p.edition)}
		}
		products = append(products, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return res, err
	}

	matchedItems := map[string]bool{}
	matchedProducts := map[int64]bool{}
	for _, p := range products {
		for _, alias := range p.aliases {
			tokens := matchTokens(alias)
			if len(tokens) == 0 {
				continue
			}
			query := "SELECT item_id FROM lk_items WHERE "
			args := make([]any, 0, len(tokens))
			conds := make([]string, 0, len(tokens))
			for i, token := range tokens {
				conds = append(conds, fmt.Sprintf("title ILIKE $%d", i+1))
				args = append(args, "%"+token+"%")
			}
			query += strings.Join(conds, " AND ") + " LIMIT 40"
			itemRows, err := dbc.QueryContext(ctx, query, args...)
			if err != nil {
				return res, fmt.Errorf("match alias: %w", err)
			}
			for itemRows.Next() {
				var itemID string
				if err := itemRows.Scan(&itemID); err != nil {
					itemRows.Close()
					return res, err
				}
				confidence := 0.6 + 0.05*float64(len(tokens))
				if confidence > 0.9 {
					confidence = 0.9
				}
				if _, err := dbc.ExecContext(ctx, `
					INSERT INTO vendor_product_items (vendor_product_id, item_id, method, confidence)
					VALUES ($1, $2, 'alias', $3)
					ON CONFLICT DO NOTHING`, p.id, itemID, confidence); err != nil {
					itemRows.Close()
					return res, err
				}
				res.Mappings++
				matchedItems[itemID] = true
				matchedProducts[p.id] = true
			}
			itemRows.Close()
		}
	}
	res.Items = len(matchedItems)
	res.Products = len(matchedProducts)
	return res, nil
}

func matchTokens(alias string) []string {
	replacer := strings.NewReplacer(
		"\"", " ", "«", " ", "»", " ", ".", " ", ",", " ", ":", " ",
		";", " ", "(", " ", ")", " ", "-", " ", "/", " ",
	)
	fields := strings.Fields(replacer.Replace(strings.ToLower(alias)))
	seen := map[string]bool{}
	out := make([]string, 0, 5)
	for _, f := range fields {
		if len([]rune(f)) < 3 || seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return len(out[i]) > len(out[j]) })
	if len(out) > 5 {
		out = out[:5]
	}
	return out
}

// ListVendorProducts returns external products with mapping counts.
func ListVendorProducts(ctx context.Context, dbc *sql.DB) ([]VendorProduct, error) {
	rows, err := dbc.QueryContext(ctx, `
		SELECT v.id, v.vendor, v.product, v.family, v.edition,
		       v.platforms::text, v.registry_number, v.source_url, v.aliases::text,
		       count(m.item_id)
		FROM vendor_products v
		LEFT JOIN vendor_product_items m ON m.vendor_product_id = v.id
		GROUP BY v.id
		ORDER BY v.vendor, v.product, v.edition`)
	if err != nil {
		return nil, fmt.Errorf("list vendor products: %w", err)
	}
	defer rows.Close()
	var out []VendorProduct
	for rows.Next() {
		var v VendorProduct
		var platforms, aliases string
		if err := rows.Scan(&v.ID, &v.Vendor, &v.Product, &v.Family, &v.Edition,
			&platforms, &v.RegistryNumber, &v.SourceURL, &aliases, &v.MappedItems); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(platforms), &v.Platforms)
		_ = json.Unmarshal([]byte(aliases), &v.Aliases)
		out = append(out, v)
	}
	return out, rows.Err()
}

// VendorKnowledgeStats summarises external coverage.
func VendorKnowledgeStats(ctx context.Context, dbc *sql.DB) (VendorStats, error) {
	var s VendorStats
	if err := dbc.QueryRowContext(ctx, `SELECT count(*) FROM vendor_products`).Scan(&s.Products); err != nil {
		return s, err
	}
	if err := dbc.QueryRowContext(ctx, `
		SELECT count(DISTINCT vendor_product_id) FROM vendor_product_items`).Scan(&s.Mapped); err != nil {
		return s, err
	}
	if err := dbc.QueryRowContext(ctx, `
		SELECT count(DISTINCT item_id) FROM vendor_product_items`).Scan(&s.CoveredItems); err != nil {
		return s, err
	}
	if err := dbc.QueryRowContext(ctx, `
		SELECT count(DISTINCT m.item_id)
		FROM vendor_product_items m
		WHERE NOT EXISTS (
			SELECT 1 FROM relations r
			WHERE r.from_item_id = m.item_id OR r.to_item_id = m.item_id
		)`).Scan(&s.SoloCovered); err != nil {
		return s, err
	}
	return s, nil
}
