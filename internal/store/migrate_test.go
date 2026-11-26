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

func TestRelationRegistrySchema(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	if err := store.Migrate(ctx, dbc); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, name := range []string{"relations", "relation_events"} {
		if !store.ObjectExists(t, dbc, name) {
			t.Errorf("missing table %s", name)
		}
	}
	// Повторный migrate не должен воскрешать старое имя через 013.
	if err := store.Migrate(ctx, dbc); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if store.ObjectExists(t, dbc, "lk_relation_candidates") {
		t.Error("old table lk_relation_candidates was recreated")
	}
	for _, col := range []string{"provenance_kind", "version", "valid_until",
		"revalidate_after", "rejection_reason", "audience", "org_type"} {
		var ok bool
		if err := dbc.QueryRow(`SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name='relations' AND column_name=$1)`, col).Scan(&ok); err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Errorf("missing relations.%s", col)
		}
	}
	var transitive bool
	if err := dbc.QueryRow(`SELECT is_transitive FROM relation_types WHERE code='alternative'`).Scan(&transitive); err != nil {
		t.Fatal(err)
	}
	if transitive {
		t.Error("alternative must not be transitive")
	}
}
