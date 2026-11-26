package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/basili4-1982/lk-advisor/internal/strategy"
)

// StrategyStats returns arms with their priors and observed outcomes.
func StrategyStats(ctx context.Context, dbc *sql.DB) ([]strategy.Stat, error) {
	rows, err := dbc.QueryContext(ctx, `
		SELECT s.audience, s.strategy, s.enabled, s.alpha::float8, s.beta::float8,
		       count(e.id) FILTER (WHERE e.event = 'shown'),
		       count(e.id) FILTER (WHERE e.event = 'clicked'),
		       count(e.id) FILTER (WHERE e.event = 'purchased')
		FROM strategy_state s
		LEFT JOIN recommendation_events e
		  ON e.audience = s.audience AND e.strategy = s.strategy
		GROUP BY s.audience, s.strategy, s.enabled, s.alpha, s.beta
		ORDER BY s.audience, s.strategy`)
	if err != nil {
		return nil, fmt.Errorf("strategy stats: %w", err)
	}
	defer rows.Close()
	var out []strategy.Stat
	for rows.Next() {
		var s strategy.Stat
		if err := rows.Scan(&s.Audience, &s.Strategy, &s.Enabled, &s.Alpha, &s.Beta,
			&s.Shown, &s.Clicks, &s.Purchases); err != nil {
			return nil, err
		}
		if s.Shown > 0 {
			s.CTR = float64(s.Clicks) / float64(s.Shown)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// AutoStrategy reports whether the strategy bandit is enabled.
func AutoStrategy(ctx context.Context, dbc *sql.DB) (bool, error) {
	var raw []byte
	err := dbc.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = 'auto_strategy'`).Scan(&raw)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("auto strategy: %w", err)
	}
	var enabled bool
	if err := json.Unmarshal(raw, &enabled); err != nil {
		return false, nil
	}
	return enabled, nil
}

// SetAutoStrategy switches the strategy bandit on or off.
func SetAutoStrategy(ctx context.Context, dbc *sql.DB, enabled bool) error {
	raw, _ := json.Marshal(enabled)
	_, err := dbc.ExecContext(ctx, `
		INSERT INTO settings (key, value, updated_at) VALUES ('auto_strategy', $1, now())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`, raw)
	if err != nil {
		return fmt.Errorf("set auto strategy: %w", err)
	}
	return nil
}
