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
