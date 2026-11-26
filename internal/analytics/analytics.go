// Package analytics stores recommendation impressions and returns aggregates.
package analytics

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrTraceNotFound means the trace id is unknown.
var ErrTraceNotFound = errors.New("trace not found")

// ErrClientMismatch means the event belongs to another client.
var ErrClientMismatch = errors.New("client mismatch")

// Item is one shown or skipped recommendation.
type Item struct {
	ProductID string
	Type      string
	Reason    string
	Detail    string
	Source    string
	Rank      int
	Score     float64
}

// ProductStat aggregates one product over a period.
type ProductStat struct {
	ProductID string  `json:"product_id"`
	Shown     int     `json:"shown"`
	Clicks    int     `json:"clicks"`
	Purchases int     `json:"purchases"`
	CTR       float64 `json:"ctr"`
}

// ReasonStat aggregates skipped recommendations by reason.
type ReasonStat struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
	Sample string `json:"sample"`
}

// Summary is the analytics dashboard payload.
type Summary struct {
	Days             int           `json:"days"`
	Requests         int           `json:"requests"`
	FallbackRequests int           `json:"fallback_requests"`
	Shown            int           `json:"shown"`
	Skipped          int           `json:"skipped"`
	Clicks           int           `json:"clicks"`
	Purchases        int           `json:"purchases"`
	CTR              float64       `json:"ctr"`
	Conversion       float64       `json:"conversion"`
	TopProducts      []ProductStat `json:"top_products"`
	SkipReasons      []ReasonStat  `json:"skip_reasons"`
}

// Log writes one recommendation request and its shown/skipped items.
func Log(ctx context.Context, dbc *sql.DB, traceID, clientID, intent string, fallback bool, shown, skipped []Item) error {
	tx, err := dbc.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("analytics begin: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO recommendation_requests
			(trace_id, client_id, intent, fallback, shown_count, skipped_count)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (trace_id) DO NOTHING`,
		traceID, clientID, intent, fallback, len(shown), len(skipped)); err != nil {
		return fmt.Errorf("analytics request: %w", err)
	}
	for _, item := range shown {
		if err := insertEvent(ctx, tx, traceID, clientID, item.ProductID, "shown", item); err != nil {
			return err
		}
	}
	for _, item := range skipped {
		if err := insertEvent(ctx, tx, traceID, clientID, item.ProductID, "skipped", item); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("analytics commit: %w", err)
	}
	return nil
}

func insertEvent(ctx context.Context, tx *sql.Tx, traceID, clientID, productID, event string, item Item) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO recommendation_events
			(trace_id, client_id, product_id, event, reason, detail, rank, score, type, source)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		traceID, clientID, productID, event, item.Reason, item.Detail,
		item.Rank, item.Score, item.Type, item.Source)
	if err != nil {
		return fmt.Errorf("analytics event: %w", err)
	}
	return nil
}

// Record stores a user action (click or purchase) for a shown item.
func Record(ctx context.Context, dbc *sql.DB, traceID, clientID, productID, event string) error {
	var owner string
	err := dbc.QueryRowContext(ctx,
		`SELECT client_id FROM recommendation_requests WHERE trace_id = $1`, traceID).Scan(&owner)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrTraceNotFound
	}
	if err != nil {
		return fmt.Errorf("analytics trace: %w", err)
	}
	if owner != clientID {
		return ErrClientMismatch
	}
	_, err = dbc.ExecContext(ctx, `
		INSERT INTO recommendation_events
			(trace_id, client_id, product_id, event, reason, source)
		VALUES ($1, $2, $3, $4, 'user', 'client')`,
		traceID, clientID, productID, event)
	if err != nil {
		return fmt.Errorf("analytics record: %w", err)
	}
	return nil
}

// Build returns aggregates for the last N days.
func Build(ctx context.Context, dbc *sql.DB, days int, now time.Time) (Summary, error) {
	if days < 1 {
		days = 1
	}
	if days > 90 {
		days = 90
	}
	since := now.AddDate(0, 0, -days)
	out := Summary{Days: days, TopProducts: []ProductStat{}, SkipReasons: []ReasonStat{}}

	if err := dbc.QueryRowContext(ctx, `
		SELECT count(*), count(*) FILTER (WHERE fallback)
		FROM recommendation_requests WHERE created_at >= $1`, since).
		Scan(&out.Requests, &out.FallbackRequests); err != nil {
		return Summary{}, fmt.Errorf("analytics requests: %w", err)
	}
	if err := dbc.QueryRowContext(ctx, `
		SELECT
			count(*) FILTER (WHERE event = 'shown'),
			count(*) FILTER (WHERE event = 'skipped'),
			count(*) FILTER (WHERE event = 'clicked'),
			count(*) FILTER (WHERE event = 'purchased')
		FROM recommendation_events WHERE created_at >= $1`, since).
		Scan(&out.Shown, &out.Skipped, &out.Clicks, &out.Purchases); err != nil {
		return Summary{}, fmt.Errorf("analytics events: %w", err)
	}
	if out.Shown > 0 {
		out.CTR = float64(out.Clicks) / float64(out.Shown)
		out.Conversion = float64(out.Purchases) / float64(out.Shown)
	}

	rows, err := dbc.QueryContext(ctx, `
		SELECT product_id,
		       count(*) FILTER (WHERE event = 'shown') AS shown,
		       count(*) FILTER (WHERE event = 'clicked') AS clicks,
		       count(*) FILTER (WHERE event = 'purchased') AS purchases
		FROM recommendation_events
		WHERE created_at >= $1
		GROUP BY product_id
		HAVING count(*) FILTER (WHERE event = 'shown') > 0
		ORDER BY shown DESC, clicks DESC, product_id
		LIMIT 10`, since)
	if err != nil {
		return Summary{}, fmt.Errorf("analytics products: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var p ProductStat
		if err := rows.Scan(&p.ProductID, &p.Shown, &p.Clicks, &p.Purchases); err != nil {
			return Summary{}, err
		}
		if p.Shown > 0 {
			p.CTR = float64(p.Clicks) / float64(p.Shown)
		}
		out.TopProducts = append(out.TopProducts, p)
	}
	if err := rows.Err(); err != nil {
		return Summary{}, err
	}

	rows, err = dbc.QueryContext(ctx, `
		SELECT reason, count(*), min(detail)
		FROM recommendation_events
		WHERE created_at >= $1 AND event = 'skipped'
		GROUP BY reason
		ORDER BY count(*) DESC, reason
		LIMIT 10`, since)
	if err != nil {
		return Summary{}, fmt.Errorf("analytics reasons: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var r ReasonStat
		if err := rows.Scan(&r.Reason, &r.Count, &r.Sample); err != nil {
			return Summary{}, err
		}
		out.SkipReasons = append(out.SkipReasons, r)
	}
	return out, rows.Err()
}
