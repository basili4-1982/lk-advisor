package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/basili4-1982/lk-advisor/internal/model"
)

// LoadWeights returns the configured scoring weights or the defaults.
func LoadWeights(ctx context.Context, dbc *sql.DB) (model.Weights, error) {
	var raw []byte
	err := dbc.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = 'scoring_weights'`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return model.DefaultWeights(), nil
	}
	if err != nil {
		return model.Weights{}, fmt.Errorf("settings: %w", err)
	}
	var w model.Weights
	if err := json.Unmarshal(raw, &w); err != nil {
		return model.DefaultWeights(), nil
	}
	return w.Normalize(), nil
}

// SaveWeights stores normalized scoring weights.
func SaveWeights(ctx context.Context, dbc *sql.DB, w model.Weights) error {
	raw, err := json.Marshal(w.Normalize())
	if err != nil {
		return fmt.Errorf("weights encode: %w", err)
	}
	_, err = dbc.ExecContext(ctx, `
		INSERT INTO settings (key, value, updated_at)
		VALUES ('scoring_weights', $1, now())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`, raw)
	if err != nil {
		return fmt.Errorf("weights save: %w", err)
	}
	return nil
}

// LoadWeightsForAudience returns weights for a segment, falling back to the default.
func LoadWeightsForAudience(ctx context.Context, dbc *sql.DB, audience string) (model.Weights, error) {
	if audience != "" {
		var raw []byte
		err := dbc.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = $1`,
			"scoring_weights_"+audience).Scan(&raw)
		if err == nil {
			var w model.Weights
			if json.Unmarshal(raw, &w) == nil {
				return w.Normalize(), nil
			}
		} else if !errors.Is(err, sql.ErrNoRows) {
			return model.Weights{}, fmt.Errorf("weights %s: %w", audience, err)
		}
	}
	return LoadWeights(ctx, dbc)
}

// SaveWeightsForAudience stores weights for one segment.
func SaveWeightsForAudience(ctx context.Context, dbc *sql.DB, audience string, w model.Weights) error {
	key := "scoring_weights"
	if audience != "" {
		key = "scoring_weights_" + audience
	}
	raw, err := json.Marshal(w.Normalize())
	if err != nil {
		return fmt.Errorf("weights encode: %w", err)
	}
	_, err = dbc.ExecContext(ctx, `
		INSERT INTO settings (key, value, updated_at) VALUES ($1, $2, now())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`, key, raw)
	if err != nil {
		return fmt.Errorf("weights save %s: %w", audience, err)
	}
	return nil
}
