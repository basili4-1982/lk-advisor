package store_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/basili4-1982/lk-advisor/internal/store"
)

func TestMigrateCreatesTables(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()

	if err := store.Migrate(ctx, dbc); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := store.Migrate(ctx, dbc); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	tables := []string{
		"clients", "products", "client_environment", "product_relations",
		"purchases", "licenses", "orders", "documents", "events",
		"rules", "promotions", "ground_truth", "agent_sessions",
		"profile_refresh_queue",
	}
	for _, name := range tables {
		if !store.ObjectExists(t, dbc, name) {
			t.Errorf("missing table %s", name)
		}
	}

	for i := 0; i < 8; i++ {
		name := fmt.Sprintf("client_profile_s%d", i)
		if !store.ObjectExists(t, dbc, name) {
			t.Errorf("missing materialized view %s", name)
		}
	}
}
