package profile_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/basili4-1982/lk-advisor/internal/profile"
	"github.com/basili4-1982/lk-advisor/internal/store"
)

func setup(t *testing.T) (*sql.DB, context.Context) {
	t.Helper()
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	if err := store.Migrate(ctx, dbc); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	store.Truncate(t, dbc)
	ds, err := store.Generate(42)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if err := store.Load(ctx, dbc, ds); err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := profile.RefreshAll(ctx, dbc); err != nil {
		t.Fatalf("refresh all: %v", err)
	}
	return dbc, ctx
}

func TestShardStable(t *testing.T) {
	a := profile.Shard("smb-001", profile.ShardCount)
	b := profile.Shard("smb-001", profile.ShardCount)
	if a != b {
		t.Fatalf("shard is not stable: %d != %d", a, b)
	}
	if a < 0 || a >= profile.ShardCount {
		t.Fatalf("shard %d out of range", a)
	}
}

func TestEnqueueCreatesQueueRow(t *testing.T) {
	dbc, ctx := setup(t)
	if _, err := dbc.ExecContext(ctx, `DELETE FROM profile_refresh_queue`); err != nil {
		t.Fatalf("clear queue: %v", err)
	}
	if err := profile.Enqueue(ctx, dbc, "smb-002"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	pending, err := profile.PendingShards(ctx, dbc)
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	want := profile.Shard("smb-002", profile.ShardCount)
	if len(pending) != 1 || pending[0] != want {
		t.Fatalf("pending = %v, want [%d]", pending, want)
	}
}

func TestRefreshSeesNewPurchase(t *testing.T) {
	dbc, ctx := setup(t)

	before, err := profile.Get(ctx, dbc, "smb-001")
	if err != nil {
		t.Fatalf("get before: %v", err)
	}

	if _, err := dbc.ExecContext(ctx, `
		INSERT INTO purchases (id, client_id, product_id, date, amount, quantity, seats)
		VALUES ('pur-test-refresh', 'smb-001', 'prod-backup', now(), 100, 1, 1)`); err != nil {
		t.Fatalf("insert purchase: %v", err)
	}

	stale, err := profile.Get(ctx, dbc, "smb-001")
	if err != nil {
		t.Fatalf("get stale: %v", err)
	}
	if len(stale.Purchases) != len(before.Purchases) {
		t.Fatalf("profile updated before refresh: %d -> %d", len(before.Purchases), len(stale.Purchases))
	}

	if err := profile.RefreshShard(ctx, dbc, profile.Shard("smb-001", profile.ShardCount)); err != nil {
		t.Fatalf("refresh shard: %v", err)
	}

	after, err := profile.Get(ctx, dbc, "smb-001")
	if err != nil {
		t.Fatalf("get after: %v", err)
	}
	if len(after.Purchases) != len(before.Purchases)+1 {
		t.Fatalf("purchases = %d, want %d", len(after.Purchases), len(before.Purchases)+1)
	}
}

func TestGetUnknownClient(t *testing.T) {
	dbc, ctx := setup(t)
	if _, err := profile.Get(ctx, dbc, "smb-999"); err != profile.ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
