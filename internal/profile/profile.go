// Package profile reads the sharded materialized profile views.
package profile

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/basili4-1982/lk-advisor/internal/model"
)

// ShardCount is the number of profile shards.
const ShardCount = 8

// ErrNotFound means the client has no profile row.
var ErrNotFound = errors.New("profile not found")

// Shard returns the stable shard for a client id.
func Shard(clientID string, n int) int { return model.Shard(clientID, n) }

// Enqueue marks the client's shard for refresh.
func Enqueue(ctx context.Context, dbc *sql.DB, clientID string) error {
	_, err := dbc.ExecContext(ctx, `
		INSERT INTO profile_refresh_queue (shard_id)
		SELECT shard FROM clients WHERE id = $1
		ON CONFLICT (shard_id) DO NOTHING`, clientID)
	if err != nil {
		return fmt.Errorf("enqueue %s: %w", clientID, err)
	}
	return nil
}

// RefreshShard rebuilds one materialized view and clears its queue row.
func RefreshShard(ctx context.Context, dbc *sql.DB, shard int) error {
	if shard < 0 || shard >= ShardCount {
		return fmt.Errorf("shard %d out of range", shard)
	}
	if _, err := dbc.ExecContext(ctx,
		fmt.Sprintf("REFRESH MATERIALIZED VIEW CONCURRENTLY client_profile_s%d", shard)); err != nil {
		return fmt.Errorf("refresh shard %d: %w", shard, err)
	}
	if _, err := dbc.ExecContext(ctx,
		`DELETE FROM profile_refresh_queue WHERE shard_id = $1`, shard); err != nil {
		return fmt.Errorf("clear queue %d: %w", shard, err)
	}
	return nil
}

// RefreshAll rebuilds every shard.
func RefreshAll(ctx context.Context, dbc *sql.DB) error {
	for i := 0; i < ShardCount; i++ {
		if err := RefreshShard(ctx, dbc, i); err != nil {
			return err
		}
	}
	return nil
}

// PendingShards returns shards waiting for refresh.
func PendingShards(ctx context.Context, dbc *sql.DB) ([]int, error) {
	rows, err := dbc.QueryContext(ctx, `SELECT shard_id FROM profile_refresh_queue ORDER BY shard_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []int
	for rows.Next() {
		var shard int
		if err := rows.Scan(&shard); err != nil {
			return nil, err
		}
		out = append(out, shard)
	}
	return out, rows.Err()
}

type facts struct {
	Segment     string            `json:"segment"`
	Industry    string            `json:"industry"`
	Size        string            `json:"size"`
	Licenses    []model.License   `json:"licenses"`
	Purchases   []model.Purchase  `json:"purchases"`
	Environment model.Environment `json:"environment"`
}

// Get reads one client profile in a single indexed query.
func Get(ctx context.Context, dbc *sql.DB, clientID string) (model.Profile, error) {
	shard := Shard(clientID, ShardCount)
	var raw []byte
	err := dbc.QueryRowContext(ctx,
		fmt.Sprintf("SELECT facts FROM client_profile_s%d WHERE client_id = $1", shard),
		clientID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Profile{}, ErrNotFound
	}
	if err != nil {
		return model.Profile{}, fmt.Errorf("get profile %s: %w", clientID, err)
	}

	var f facts
	if err := json.Unmarshal(raw, &f); err != nil {
		return model.Profile{}, fmt.Errorf("decode profile %s: %w", clientID, err)
	}
	var audience string
	_ = dbc.QueryRowContext(ctx, `SELECT audience FROM clients WHERE id = $1`, clientID).Scan(&audience)
	return model.Profile{
		ClientID:    clientID,
		Segment:     f.Segment,
		Industry:    f.Industry,
		Size:        f.Size,
		Audience:    audience,
		Licenses:    f.Licenses,
		Purchases:   f.Purchases,
		Environment: f.Environment,
	}, nil
}
