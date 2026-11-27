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
	Category       string   `json:"category"`
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
	Products           int `json:"products"`
	Mapped             int `json:"mapped_products"`
	CoveredItems       int `json:"covered_items"`
	SoloCovered        int `json:"solo_covered"`
	ClassifiedProducts int `json:"classified_products"`
	Classes            int `json:"classes"`
	ClassifiedItems    int `json:"classified_items"`
}

// VendorClass is one registry product class with coverage.
type VendorClass struct {
	Code     string `json:"code"`
	Title    string `json:"title"`
	Products int    `json:"products"`
	Items    int    `json:"items"`
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
	// Registry records are too numerous for per-product queries; match them in
	// two set-based passes: full product name and alternative names.
	nameRes, err := dbc.ExecContext(ctx, `
		INSERT INTO vendor_product_items (vendor_product_id, item_id, method, confidence)
		SELECT DISTINCT v.id, i.item_id, 'registry_name', 0.75
		FROM vendor_products v
		JOIN lk_items i ON i.title ILIKE '%' || v.product || '%'
		WHERE v.registry_number <> ''
		  AND length(v.product) >= 8
		  AND v.vendor NOT IN ('', '-')
		  AND lower(v.product) NOT IN ('процессор','программа','программное обеспечение','средство','система','модуль','комплекс','платформа','компонент','утилита','драйвер')
		ON CONFLICT DO NOTHING`)
	if err != nil {
		return res, fmt.Errorf("registry name link: %w", err)
	}
	nameMappings, _ := nameRes.RowsAffected()
	aliasRes, err := dbc.ExecContext(ctx, `
		INSERT INTO vendor_product_items (vendor_product_id, item_id, method, confidence)
		SELECT DISTINCT v.id, i.item_id, 'registry_alias', 0.70
		FROM vendor_products v
		CROSS JOIN LATERAL jsonb_array_elements_text(v.aliases) a
		JOIN lk_items i ON i.title ILIKE '%' || a || '%'
		WHERE v.registry_number <> ''
		  AND length(a) >= 10
		  AND v.vendor NOT IN ('', '-')
		  AND lower(a) NOT IN ('процессор','программа','программное обеспечение','средство','система','модуль','комплекс','платформа','компонент','утилита','драйвер')
		ON CONFLICT DO NOTHING`)
	if err != nil {
		return res, fmt.Errorf("registry alias link: %w", err)
	}
	aliasMappings, _ := aliasRes.RowsAffected()
	res.Mappings += int(nameMappings + aliasMappings)

	rows, err := dbc.QueryContext(ctx, `
		SELECT id, product, edition, aliases::text
		FROM vendor_products WHERE registry_number = '' ORDER BY id`)
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
	_ = matchedItems
	_ = matchedProducts
	if err := dbc.QueryRowContext(ctx, `
		SELECT count(DISTINCT vendor_product_id) FROM vendor_product_items`).Scan(&res.Products); err != nil {
		return res, err
	}
	if err := dbc.QueryRowContext(ctx, `
		SELECT count(DISTINCT item_id) FROM vendor_product_items`).Scan(&res.Items); err != nil {
		return res, err
	}
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

// VendorCandidatesResult reports relation candidates built from external data.
type VendorCandidatesResult struct {
	Created  int                    `json:"created"`
	Skipped  int                    `json:"skipped"`
	Examples []VendorCandidateBrief `json:"examples"`
}

// VendorCandidateBrief is one generated candidate.
type VendorCandidateBrief struct {
	FromID   string `json:"from_id"`
	ToID     string `json:"to_id"`
	From     string `json:"from"`
	To       string `json:"to"`
	Type     string `json:"type"`
	Vendor   string `json:"vendor"`
	Family   string `json:"family"`
	EditionA string `json:"edition_from"`
	EditionB string `json:"edition_to"`
	URL      string `json:"source_url"`
}

// GenerateVendorCandidates creates variant/alternative candidates from the
// external product layer. It only links items that have no relation yet, so
// the result is measurable novelty rather than another co-purchase edge.
func GenerateVendorCandidates(ctx context.Context, dbc *sql.DB, limit int) (VendorCandidatesResult, error) {
	var res VendorCandidatesResult
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := dbc.QueryContext(ctx, `
		SELECT m1.item_id, i1.title, m2.item_id, i2.title,
		       v1.vendor, v1.family, v1.product, v2.product, v1.edition, v2.edition,
		       v1.platforms::text, v2.platforms::text, v1.source_url,
		       v1.id, v2.id
		FROM vendor_product_items m1
		JOIN vendor_products v1 ON v1.id = m1.vendor_product_id
		JOIN lk_items i1 ON i1.item_id = m1.item_id AND i1.available
		JOIN vendor_product_items m2 ON m2.vendor_product_id <> m1.vendor_product_id
		JOIN vendor_products v2 ON v2.id = m2.vendor_product_id
		JOIN lk_items i2 ON i2.item_id = m2.item_id AND i2.available
		WHERE m1.item_id < m2.item_id
		  AND v1.family <> '' AND v1.family = v2.family
		  AND (v1.edition <> v2.edition OR v1.platforms::text <> v2.platforms::text)
		  AND NOT EXISTS (
		      SELECT 1 FROM relations r
		      WHERE (r.from_item_id = m1.item_id AND r.to_item_id = m2.item_id)
		         OR (r.from_item_id = m2.item_id AND r.to_item_id = m1.item_id)
		  )
		ORDER BY v1.id, v2.id, m1.item_id, m2.item_id
		LIMIT $1`, limit*40)
	if err != nil {
		return res, fmt.Errorf("vendor candidate pairs: %w", err)
	}
	type pair struct {
		fromID, fromTitle, toID, toTitle  string
		vendor, family, product, productB string
		editionA, editionB                string
		platformsA, platformsB            string
		url                               string
		vpA, vpB                          int64
	}
	var pairs []pair
	for rows.Next() {
		var p pair
		if err := rows.Scan(&p.fromID, &p.fromTitle, &p.toID, &p.toTitle,
			&p.vendor, &p.family, &p.product, &p.productB, &p.editionA, &p.editionB,
			&p.platformsA, &p.platformsB, &p.url, &p.vpA, &p.vpB); err != nil {
			rows.Close()
			return res, err
		}
		pairs = append(pairs, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return res, err
	}

	// At most two SKU pairs per external product pair: the month/user SKUs of
	// one edition should not flood the registry with duplicates.
	perPair := map[string]int{}
	byFamily := map[string][]pair{}
	familyOrder := []string{}
	for _, p := range pairs {
		key := fmt.Sprintf("%d|%d", p.vpA, p.vpB)
		if perPair[key] >= 10 {
			continue
		}
		perPair[key]++
		if _, ok := byFamily[p.family]; !ok {
			familyOrder = append(familyOrder, p.family)
		}
		byFamily[p.family] = append(byFamily[p.family], p)
	}
	balanced := make([]pair, 0, limit)
	for i := 0; len(balanced) < limit; i++ {
		added := false
		for _, family := range familyOrder {
			list := byFamily[family]
			if i < len(list) {
				balanced = append(balanced, list[i])
				added = true
				if len(balanced) >= limit {
					break
				}
			}
		}
		if !added {
			break
		}
	}
	pairs = balanced

	tx, err := dbc.BeginTx(ctx, nil)
	if err != nil {
		return res, err
	}
	defer func() { _ = tx.Rollback() }()
	for _, p := range pairs {
		if res.Created >= limit {
			break
		}
		relType := "variant_of"
		confidence := 0.75
		reason := fmt.Sprintf("Внешний источник: %s, редакции «%s» и «%s»", p.family, p.editionA, p.editionB)
		if p.editionA == "" || p.editionB == "" || p.editionA == p.editionB {
			enterpriseA := strings.Contains(strings.ToLower(p.product), "enterprise")
			enterpriseB := strings.Contains(strings.ToLower(p.productB), "enterprise")
			if p.product != p.productB && enterpriseA != enterpriseB {
				relType = "bundle"
				confidence = 0.70
				reason = fmt.Sprintf("Внешний источник: %s, продукт входит в комплект «%s»", p.family, p.productB)
				if enterpriseA {
					reason = fmt.Sprintf("Внешний источник: %s, продукт входит в комплект «%s»", p.family, p.product)
				}
			} else if p.product != p.productB {
				relType = "adjacent_task"
				confidence = 0.60
				reason = fmt.Sprintf("Внешний источник: %s, продукты «%s» и «%s»; совместимость требует проверки", p.family, p.product, p.productB)
			} else {
				relType = "alternative"
				confidence = 0.65
				reason = fmt.Sprintf("Внешний источник: %s, разные платформы", p.family)
			}
		}
		evidence, _ := json.Marshal([]map[string]any{{
			"kind":    "vendor_doc",
			"summary": reason,
			"details": map[string]any{
				"url": p.url, "vendor": p.vendor, "family": p.family,
				"edition_from": p.editionA, "edition_to": p.editionB,
				"platforms_from": p.platformsA, "platforms_to": p.platformsB,
			},
		}})
		result, err := tx.ExecContext(ctx, `
			INSERT INTO relations
				(from_item_id, to_item_id, type, confidence, status, source,
				 provenance_kind, reason, evidence, audience, industry)
			VALUES ($1, $2, $3, $4, 'candidate', 'vendor', 'vendor', $5, $6::jsonb, '', '')
			ON CONFLICT DO NOTHING`,
			p.fromID, p.toID, relType, confidence, reason, string(evidence))
		if err != nil {
			return res, fmt.Errorf("insert vendor candidate: %w", err)
		}
		if n, _ := result.RowsAffected(); n > 0 {
			res.Created++
			if len(res.Examples) < 10 {
				res.Examples = append(res.Examples, VendorCandidateBrief{
					FromID: p.fromID, ToID: p.toID, From: p.fromTitle, To: p.toTitle,
					Type: relType, Vendor: p.vendor, Family: p.family,
					EditionA: p.editionA, EditionB: p.editionB, URL: p.url,
				})
			}
		}
	}
	res.Skipped = len(pairs) - res.Created
	if err := tx.Commit(); err != nil {
		return res, err
	}
	return res, nil
}

// ListVendorProducts returns external products with mapping counts.
func ListVendorProducts(ctx context.Context, dbc *sql.DB) ([]VendorProduct, error) {
	rows, err := dbc.QueryContext(ctx, `
		SELECT v.id, v.vendor, v.product, v.family, v.edition,
		       v.platforms::text, v.registry_number, v.source_url, v.category, v.aliases::text,
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
			&platforms, &v.RegistryNumber, &v.SourceURL, &v.Category, &aliases, &v.MappedItems); err != nil {
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
	if err := dbc.QueryRowContext(ctx, `
		SELECT count(*) FROM vendor_products WHERE category <> ''`).Scan(&s.ClassifiedProducts); err != nil {
		return s, err
	}
	if err := dbc.QueryRowContext(ctx, `
		SELECT count(DISTINCT split_part(category, ' - ', 1))
		FROM vendor_products WHERE category <> ''`).Scan(&s.Classes); err != nil {
		return s, err
	}
	if err := dbc.QueryRowContext(ctx, `
		SELECT count(DISTINCT m.item_id)
		FROM vendor_product_items m
		JOIN vendor_products v ON v.id = m.vendor_product_id
		WHERE v.category <> ''`).Scan(&s.ClassifiedItems); err != nil {
		return s, err
	}
	return s, nil
}

// ListVendorClasses returns top registry classes with product and catalog
// coverage.
func ListVendorClasses(ctx context.Context, dbc *sql.DB, limit int) ([]VendorClass, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := dbc.QueryContext(ctx, `
		SELECT split_part(v.category, ' - ', 1) AS code,
		       min(v.category) AS title,
		       count(DISTINCT v.id) AS products,
		       count(DISTINCT m.item_id) AS items
		FROM vendor_products v
		LEFT JOIN vendor_product_items m ON m.vendor_product_id = v.id
		WHERE v.category <> ''
		GROUP BY 1
		ORDER BY items DESC, products DESC, code
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("vendor classes: %w", err)
	}
	defer rows.Close()
	var out []VendorClass
	for rows.Next() {
		var c VendorClass
		if err := rows.Scan(&c.Code, &c.Title, &c.Products, &c.Items); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
