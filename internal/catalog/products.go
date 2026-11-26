package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/basili4-1982/lk-advisor/internal/model"
)

const productColumns = `id, sku, title, category, subcategory, vendor, type,
	price, term_months, seats, version, status, system_requirements, audience`

func scanProduct(rows *sql.Rows) (model.Product, error) {
	var p model.Product
	var req []byte
	if err := rows.Scan(&p.ID, &p.SKU, &p.Title, &p.Category, &p.Subcategory, &p.Vendor,
		&p.Type, &p.Price, &p.TermMonths, &p.Seats, &p.Version, &p.Status, &req, &p.Audience); err != nil {
		return model.Product{}, err
	}
	if len(req) > 0 {
		_ = json.Unmarshal(req, &p.Requirements)
	}
	return p, nil
}

// AllProducts loads the catalog into a map by id.
func AllProducts(ctx context.Context, dbc *sql.DB) (map[string]model.Product, error) {
	rows, err := dbc.QueryContext(ctx, `SELECT `+productColumns+` FROM products`)
	if err != nil {
		return nil, fmt.Errorf("products: %w", err)
	}
	defer rows.Close()

	out := map[string]model.Product{}
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, fmt.Errorf("scan product: %w", err)
		}
		out[p.ID] = p
	}
	return out, rows.Err()
}

// Rules loads all business rules.
func Rules(ctx context.Context, dbc *sql.DB) ([]model.Rule, error) {
	rows, err := dbc.QueryContext(ctx, `SELECT id, type, condition, action FROM rules ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("rules: %w", err)
	}
	defer rows.Close()

	var out []model.Rule
	for rows.Next() {
		var r model.Rule
		if err := rows.Scan(&r.ID, &r.Type, &r.Condition, &r.Action); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// AllPromotions loads every promotion; the engine filters by date.
func AllPromotions(ctx context.Context, dbc *sql.DB) ([]model.Promotion, error) {
	rows, err := dbc.QueryContext(ctx, `
		SELECT id, product_id, discount, valid_from, valid_to, conditions
		FROM promotions ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("promotions: %w", err)
	}
	defer rows.Close()

	var out []model.Promotion
	for rows.Next() {
		var p model.Promotion
		if err := rows.Scan(&p.ID, &p.ProductID, &p.Discount, &p.ValidFrom, &p.ValidTo, &p.Conditions); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Popular returns the most purchased products for fallback.
func Popular(ctx context.Context, dbc *sql.DB, limit int) ([]model.Product, error) {
	rows, err := dbc.QueryContext(ctx, `
		SELECT `+productColumns+`
		FROM products
		JOIN (
			SELECT product_id, count(*) AS c
			FROM purchases
			GROUP BY product_id
			ORDER BY c DESC, product_id
			LIMIT $1
		) x ON x.product_id = products.id
		ORDER BY x.c DESC, products.id`, limit)
	if err != nil {
		return nil, fmt.Errorf("popular: %w", err)
	}
	defer rows.Close()

	var out []model.Product
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PopularityScores returns decayed demand per product for an audience.
// The weight halves every halfLifeWeeks: recent purchases matter more.
func PopularityScores(ctx context.Context, dbc *sql.DB, audience string, now time.Time, halfLifeWeeks float64) (map[string]float64, error) {
	if halfLifeWeeks <= 0 {
		halfLifeWeeks = 8
	}
	rows, err := dbc.QueryContext(ctx, `
		SELECT p.product_id,
		       sum(power(0.5, (extract(epoch from ($1::timestamptz - p.date::timestamptz)) / 604800.0) / $2))::float8 AS score
		FROM purchases p
		JOIN products pr ON pr.id = p.product_id
		WHERE pr.status = 'active'
		  AND p.date <= $1::date
		  AND ($3 = '' OR pr.audience = $3 OR pr.audience = 'any')
		GROUP BY p.product_id
		ORDER BY score DESC
		LIMIT 500`, now, halfLifeWeeks, audience)
	if err != nil {
		return nil, fmt.Errorf("popularity scores: %w", err)
	}
	defer rows.Close()
	out := map[string]float64{}
	for rows.Next() {
		var id string
		var score float64
		if err := rows.Scan(&id, &score); err != nil {
			return nil, err
		}
		out[id] = score
	}
	return out, rows.Err()
}

// PopularByAudience returns bestsellers that fit the client's audience.
func PopularByAudience(ctx context.Context, dbc *sql.DB, audience string, limit int) ([]model.Product, error) {
	if audience == "" {
		return Popular(ctx, dbc, limit)
	}
	rows, err := dbc.QueryContext(ctx, `
		SELECT `+productColumns+`
		FROM products p
		JOIN (
			SELECT product_id, count(*) AS c
			FROM purchases
			GROUP BY product_id
			ORDER BY c DESC, product_id
			LIMIT 200
		) x ON x.product_id = p.id
		WHERE p.status = 'active'
		  AND (p.audience = $2 OR p.audience = 'any')
		ORDER BY x.c DESC, p.id
		LIMIT $1`, limit, audience)
	if err != nil {
		return nil, fmt.Errorf("popular audience: %w", err)
	}
	defer rows.Close()
	var out []model.Product
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
