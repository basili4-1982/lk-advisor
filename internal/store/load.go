package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// Load writes the whole dataset into PostgreSQL in one transaction.
func Load(ctx context.Context, dbc *sql.DB, ds Dataset) error {
	tx, err := dbc.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, c := range ds.Clients {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO clients (id, name, segment, industry, size, region, audience, shard)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT DO NOTHING`,
			c.ID, c.Name, c.Segment, c.Industry, c.Size, c.Region, audienceOr(c.Audience), c.Shard); err != nil {
			return fmt.Errorf("client %s: %w", c.ID, err)
		}
	}

	for _, p := range ds.Products {
		req, err := json.Marshal(p.Requirements)
		if err != nil {
			return fmt.Errorf("requirements %s: %w", p.ID, err)
		}
		audience := p.Audience
		if audience == "" {
			audience = "any"
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO products (id, sku, title, category, subcategory, vendor, type,
				price, term_months, seats, version, status, system_requirements, audience)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) ON CONFLICT DO NOTHING`,
			p.ID, p.SKU, p.Title, p.Category, p.Subcategory, p.Vendor, p.Type,
			p.Price, p.TermMonths, p.Seats, p.Version, p.Status, string(req), audience); err != nil {
			return fmt.Errorf("product %s: %w", p.ID, err)
		}
	}

	for _, e := range ds.Environment {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO client_environment (client_id, os, ram_gb, cpu, gpu, source, confidence)
			VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`,
			e.ClientID, e.OS, e.RAMGB, e.CPU, e.GPU, e.Source, e.Confidence); err != nil {
			return fmt.Errorf("environment %s: %w", e.ClientID, err)
		}
	}

	for _, r := range ds.Relations {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO product_relations (from_id, to_id, type, direction, confidence, status, source)
			VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`,
			r.FromID, r.ToID, r.Type, r.Direction, r.Confidence, r.Status, r.Source); err != nil {
			return fmt.Errorf("relation %s->%s: %w", r.FromID, r.ToID, err)
		}
	}

	for _, p := range ds.Purchases {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO purchases (id, client_id, product_id, date, amount, quantity, seats)
			VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`,
			p.ID, p.ClientID, p.ProductID, p.Date, p.Amount, p.Quantity, p.Seats); err != nil {
			return fmt.Errorf("purchase %s: %w", p.ID, err)
		}
	}

	for _, l := range ds.Licenses {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO licenses (id, client_id, product_id, seats, start_date, end_date, status)
			VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`,
			l.ID, l.ClientID, l.ProductID, l.Seats, l.Start, l.End, l.Status); err != nil {
			return fmt.Errorf("license %s: %w", l.ID, err)
		}
	}

	for _, o := range ds.Orders {
		items, err := json.Marshal(o.Items)
		if err != nil {
			return fmt.Errorf("order items %s: %w", o.ID, err)
		}
		var delivered any
		if o.DeliveredAt != nil {
			delivered = *o.DeliveredAt
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO orders (id, client_id, status, created_at, delivered_at, items)
			VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`,
			o.ID, o.ClientID, o.Status, o.CreatedAt, delivered, string(items)); err != nil {
			return fmt.Errorf("order %s: %w", o.ID, err)
		}
	}

	for _, e := range ds.Events {
		ctxJSON, err := json.Marshal(e.Context)
		if err != nil {
			return fmt.Errorf("event context %s: %w", e.ID, err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO events (id, client_id, type, ts, context)
			VALUES ($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`,
			e.ID, e.ClientID, e.Type, e.TS, string(ctxJSON)); err != nil {
			return fmt.Errorf("event %s: %w", e.ID, err)
		}
	}

	for _, r := range ds.Rules {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO rules (id, type, condition, action)
			VALUES ($1,$2,$3,$4) ON CONFLICT DO NOTHING`,
			r.ID, r.Type, r.Condition, r.Action); err != nil {
			return fmt.Errorf("rule %s: %w", r.ID, err)
		}
	}

	for _, p := range ds.Promotions {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO promotions (id, product_id, discount, valid_from, valid_to, conditions)
			VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`,
			p.ID, p.ProductID, p.Discount, p.ValidFrom, p.ValidTo, p.Conditions); err != nil {
			return fmt.Errorf("promotion %s: %w", p.ID, err)
		}
	}

	for _, g := range ds.GroundTruth {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO ground_truth (client_id, event, expected_product_id, expected_type)
			VALUES ($1,$2,$3,$4) ON CONFLICT DO NOTHING`,
			g.ClientID, g.Event, g.ExpectedProduct, g.ExpectedType); err != nil {
			return fmt.Errorf("ground truth %s: %w", g.ClientID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// Clear removes all business rows. Used by the seed command.
func Clear(ctx context.Context, dbc *sql.DB) error {
	_, err := dbc.ExecContext(ctx, `TRUNCATE clients, products, client_environment, product_relations,
		purchases, licenses, orders, documents, events, rules, promotions,
		ground_truth, agent_sessions, profile_refresh_queue, demo_audit, agent_calls,
		recommendation_requests, recommendation_events RESTART IDENTITY CASCADE`)
	if err != nil {
		return fmt.Errorf("clear: %w", err)
	}
	return nil
}

func audienceOr(a string) string {
	if a == "" {
		return "b2b"
	}
	return a
}
