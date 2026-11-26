package catalog_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/basili4-1982/lk-advisor/internal/catalog"
	"github.com/basili4-1982/lk-advisor/internal/store"
)

var at = time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)

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
	return dbc, ctx
}

func TestRelationsSkipRejected(t *testing.T) {
	dbc, ctx := setup(t)
	rels, err := catalog.Relations(ctx, dbc, []string{"prod-raster", "prod-raster-2"})
	if err != nil {
		t.Fatalf("relations: %v", err)
	}
	if len(rels) == 0 {
		t.Fatal("no relations returned")
	}
	for _, r := range rels {
		if r.Status != "confirmed" && r.Status != "llm_suggested" {
			t.Fatalf("unexpected status %q for %s -> %s", r.Status, r.FromID, r.ToID)
		}
	}
}

func TestPromotionsOnlyActive(t *testing.T) {
	dbc, ctx := setup(t)

	active, err := catalog.Promotions(ctx, dbc, []string{"prod-vector"}, at)
	if err != nil {
		t.Fatalf("promotions: %v", err)
	}
	if len(active) == 0 {
		t.Fatal("expected an active promotion for prod-vector")
	}

	later, err := catalog.Promotions(ctx, dbc, []string{"prod-vector"}, at.AddDate(0, 0, 100))
	if err != nil {
		t.Fatalf("promotions later: %v", err)
	}
	if len(later) != 0 {
		t.Fatalf("expected no promotions later, got %d", len(later))
	}
}

func TestNoDanglingRelations(t *testing.T) {
	dbc, ctx := setup(t)
	var n int
	err := dbc.QueryRowContext(ctx, `
		SELECT count(*) FROM product_relations r
		LEFT JOIN products p1 ON p1.id = r.from_id
		LEFT JOIN products p2 ON p2.id = r.to_id
		WHERE p1.id IS NULL OR p2.id IS NULL`).Scan(&n)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if n != 0 {
		t.Fatalf("dangling relations: %d", n)
	}
}
