// Package analytics stores recommendation impressions and returns aggregates.
package analytics

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrTraceNotFound means the trace id is unknown.
var ErrTraceNotFound = errors.New("trace not found")

// ErrClientMismatch means the event belongs to another client.
var ErrClientMismatch = errors.New("client mismatch")

// Meta identifies the strategy and weight version behind one request.
type Meta struct {
	Strategy       string
	Audience       string
	Industry       string
	WeightsVersion string
}

// Item is one shown or skipped recommendation.
type Item struct {
	ProductID       string
	Type            string
	Reason          string
	Detail          string
	Source          string
	Rank            int
	Score           float64
	CandidateID     string
	SourceProductID string
	RelationType    string
}

// ProductStat aggregates one product over a period.
type ProductStat struct {
	ProductID string  `json:"product_id"`
	Shown     int     `json:"shown"`
	Clicks    int     `json:"clicks"`
	Purchases int     `json:"purchases"`
	CTR       float64 `json:"ctr"`
}

// StrategyStat aggregates one strategy and audience.
type StrategyStat struct {
	Audience  string  `json:"audience"`
	Strategy  string  `json:"strategy"`
	Shown     int     `json:"shown"`
	Clicks    int     `json:"clicks"`
	Purchases int     `json:"purchases"`
	CTR       float64 `json:"ctr"`
}

// AudienceStat aggregates one audience.
type AudienceStat struct {
	Audience  string  `json:"audience"`
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
	Days             int            `json:"days"`
	Requests         int            `json:"requests"`
	FallbackRequests int            `json:"fallback_requests"`
	Shown            int            `json:"shown"`
	Skipped          int            `json:"skipped"`
	Clicks           int            `json:"clicks"`
	Purchases        int            `json:"purchases"`
	CTR              float64        `json:"ctr"`
	Conversion       float64        `json:"conversion"`
	TopProducts      []ProductStat  `json:"top_products"`
	SkipReasons      []ReasonStat   `json:"skip_reasons"`
	ByStrategy       []StrategyStat `json:"by_strategy"`
	ByAudience       []AudienceStat `json:"by_audience"`
}

// Log writes one recommendation request and its shown/skipped items.
func Log(ctx context.Context, dbc *sql.DB, traceID, clientID, intent string, meta Meta, fallback bool, shown, skipped []Item) error {
	meta.Audience = strings.ToLower(strings.TrimSpace(meta.Audience))
	meta.Industry = strings.ToLower(strings.TrimSpace(meta.Industry))
	tx, err := dbc.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("analytics begin: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO recommendation_requests
			(trace_id, client_id, intent, fallback, shown_count, skipped_count,
			 strategy, audience, industry, weights_version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (trace_id) DO NOTHING`,
		traceID, clientID, intent, fallback, len(shown), len(skipped),
		meta.Strategy, meta.Audience, meta.Industry, meta.WeightsVersion); err != nil {
		return fmt.Errorf("analytics request: %w", err)
	}
	for _, item := range shown {
		if err := insertEvent(ctx, tx, traceID, clientID, item.ProductID, "shown", meta, item); err != nil {
			return err
		}
	}
	for _, item := range skipped {
		if err := insertEvent(ctx, tx, traceID, clientID, item.ProductID, "skipped", meta, item); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("analytics commit: %w", err)
	}
	return nil
}

func insertEvent(ctx context.Context, tx *sql.Tx, traceID, clientID, productID, event string, meta Meta, item Item) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO recommendation_events
			(trace_id, client_id, product_id, event, reason, detail, rank, score, type, source,
			 strategy, audience, industry, weights_version, candidate_id, source_product_id, relation_type)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)`,
		traceID, clientID, productID, event, item.Reason, item.Detail,
		item.Rank, item.Score, item.Type, item.Source,
		meta.Strategy, meta.Audience, meta.Industry, meta.WeightsVersion,
		item.CandidateID, item.SourceProductID, item.RelationType)
	if err != nil {
		return fmt.Errorf("analytics event: %w", err)
	}
	return nil
}

// Record stores a user action (click or purchase) for a shown item.
func Record(ctx context.Context, dbc *sql.DB, traceID, clientID, productID, event string) error {
	var owner, strategyName, audience, industry, weightsVersion, candidateID, sourceID, relationType string
	var rank int
	err := dbc.QueryRowContext(ctx, `
		SELECT r.client_id, r.strategy, r.audience, r.industry, r.weights_version,
		       COALESCE(e.rank, 0), COALESCE(e.candidate_id, ''),
		       COALESCE(e.source_product_id, ''), COALESCE(e.relation_type, '')
		FROM recommendation_requests r
		LEFT JOIN LATERAL (
			SELECT rank, candidate_id, source_product_id, relation_type
			FROM recommendation_events
			WHERE trace_id = r.trace_id AND product_id = $2 AND event = 'shown'
			ORDER BY id LIMIT 1
		) e ON true
		WHERE r.trace_id = $1`, traceID, productID).
		Scan(&owner, &strategyName, &audience, &industry, &weightsVersion, &rank,
			&candidateID, &sourceID, &relationType)
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
			(trace_id, client_id, product_id, event, reason, source, rank,
			 strategy, audience, industry, weights_version, candidate_id, source_product_id, relation_type)
		VALUES ($1, $2, $3, $4, 'user', 'client', $5, $6, $7, $8, $9, $10, $11, $12)`,
		traceID, clientID, productID, event, rank, strategyName, audience, industry, weightsVersion,
		candidateID, sourceID, relationType)
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
	out := Summary{Days: days, TopProducts: []ProductStat{}, SkipReasons: []ReasonStat{},
		ByStrategy: []StrategyStat{}, ByAudience: []AudienceStat{}}

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
	if out.ByStrategy, err = buildStrategyStats(ctx, dbc, since); err != nil {
		return Summary{}, fmt.Errorf("analytics strategy: %w", err)
	}
	if out.ByAudience, err = buildAudienceStats(ctx, dbc, since); err != nil {
		return Summary{}, fmt.Errorf("analytics audience: %w", err)
	}
	return out, nil
}

func buildStrategyStats(ctx context.Context, dbc *sql.DB, since time.Time) ([]StrategyStat, error) {
	rows, err := dbc.QueryContext(ctx, `
		SELECT strategy, audience,
		       count(*) FILTER (WHERE event = 'shown'),
		       count(*) FILTER (WHERE event = 'clicked'),
		       count(*) FILTER (WHERE event = 'purchased')
		FROM recommendation_events
		WHERE created_at >= $1 AND strategy <> ''
		GROUP BY strategy, audience
		ORDER BY strategy, audience`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StrategyStat
	for rows.Next() {
		var s StrategyStat
		if err := rows.Scan(&s.Strategy, &s.Audience, &s.Shown, &s.Clicks, &s.Purchases); err != nil {
			return nil, err
		}
		if s.Shown > 0 {
			s.CTR = float64(s.Clicks) / float64(s.Shown)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func buildAudienceStats(ctx context.Context, dbc *sql.DB, since time.Time) ([]AudienceStat, error) {
	rows, err := dbc.QueryContext(ctx, `
		SELECT audience,
		       count(*) FILTER (WHERE event = 'shown'),
		       count(*) FILTER (WHERE event = 'clicked'),
		       count(*) FILTER (WHERE event = 'purchased')
		FROM recommendation_events
		WHERE created_at >= $1 AND audience <> ''
		GROUP BY audience
		ORDER BY audience`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AudienceStat
	for rows.Next() {
		var s AudienceStat
		if err := rows.Scan(&s.Audience, &s.Shown, &s.Clicks, &s.Purchases); err != nil {
			return nil, err
		}
		if s.Shown > 0 {
			s.CTR = float64(s.Clicks) / float64(s.Shown)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// TypeStat aggregates recommendation performance by candidate type.
type TypeStat struct {
	Type      string  `json:"type"`
	Shown     int     `json:"shown"`
	Clicks    int     `json:"clicks"`
	Purchases int     `json:"purchases"`
	CTR       float64 `json:"ctr"`
}

// ProblemProduct is a product with many impressions and no clicks.
type ProblemProduct struct {
	ProductID string `json:"product_id"`
	Type      string `json:"type"`
	Shown     int    `json:"shown"`
	Clicks    int    `json:"clicks"`
	Purchases int    `json:"purchases"`
}

// Health is the auto-review signal for T19.
type Health struct {
	Days     int              `json:"days"`
	Types    []TypeStat       `json:"types"`
	LowClick []ProblemProduct `json:"low_click"`
}

// BuildHealth returns type performance and low-click candidates.
func BuildHealth(ctx context.Context, dbc *sql.DB, days, minShown int, now time.Time) (Health, error) {
	if days < 1 {
		days = 1
	}
	if days > 90 {
		days = 90
	}
	if minShown < 1 {
		minShown = 50
	}
	since := now.AddDate(0, 0, -days)
	out := Health{Days: days, Types: []TypeStat{}, LowClick: []ProblemProduct{}}

	rows, err := dbc.QueryContext(ctx, `
		SELECT type,
		       count(*) FILTER (WHERE event = 'shown'),
		       count(*) FILTER (WHERE event = 'clicked'),
		       count(*) FILTER (WHERE event = 'purchased')
		FROM recommendation_events
		WHERE created_at >= $1 AND type <> ''
		GROUP BY type
		ORDER BY type`, since)
	if err != nil {
		return Health{}, fmt.Errorf("health types: %w", err)
	}
	for rows.Next() {
		var t TypeStat
		if err := rows.Scan(&t.Type, &t.Shown, &t.Clicks, &t.Purchases); err != nil {
			rows.Close()
			return Health{}, err
		}
		if t.Shown > 0 {
			t.CTR = float64(t.Clicks) / float64(t.Shown)
		}
		out.Types = append(out.Types, t)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Health{}, err
	}
	rows.Close()

	rows, err = dbc.QueryContext(ctx, `
		SELECT product_id, max(type),
		       count(*) FILTER (WHERE event = 'shown'),
		       count(*) FILTER (WHERE event = 'clicked'),
		       count(*) FILTER (WHERE event = 'purchased')
		FROM recommendation_events
		WHERE created_at >= $1
		GROUP BY product_id
		HAVING count(*) FILTER (WHERE event = 'shown') >= $2
		   AND count(*) FILTER (WHERE event = 'clicked') = 0
		ORDER BY count(*) FILTER (WHERE event = 'shown') DESC, product_id
		LIMIT 20`, since, minShown)
	if err != nil {
		return Health{}, fmt.Errorf("health low click: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var p ProblemProduct
		if err := rows.Scan(&p.ProductID, &p.Type, &p.Shown, &p.Clicks, &p.Purchases); err != nil {
			return Health{}, err
		}
		out.LowClick = append(out.LowClick, p)
	}
	return out, rows.Err()
}
