package catalog_test

import (
	"context"
	"testing"

	"github.com/basili4-1982/lk-advisor/internal/catalog"
	"github.com/basili4-1982/lk-advisor/internal/store"
)

func TestGenerateVariantRelations(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	seedRegistry(t, dbc)
	defer cleanRegistry(t, dbc)

	res, err := catalog.GenerateRelations(ctx, dbc, catalog.GenerateOptions{
		IncludeVariants: true, OnlyItems: []string{"reg-a", "reg-b", "reg-c", "reg-d"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Variant != 2 {
		t.Fatalf("variants = %d, want 2", res.Variant)
	}
	var active int
	dbc.QueryRowContext(ctx, `SELECT count(*) FROM relations
		WHERE type='variant_of' AND status='active' AND provenance_kind='attribute'`).Scan(&active)
	if active != 2 {
		t.Fatalf("active variant_of = %d", active)
	}
	var summary, group string
	if err := dbc.QueryRowContext(ctx, `SELECT evidence->0->>'summary',
		evidence->0->'details'->>'product_group'
		FROM relations WHERE type='variant_of' LIMIT 1`).Scan(&summary, &group); err != nil {
		t.Fatal(err)
	}
	if summary != "общий product_group" || group == "" {
		t.Fatalf("variant evidence = %q / %q", summary, group)
	}
}

func TestGenerateCoPurchaseSkipsRejected(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	seedRegistry(t, dbc)
	defer cleanRegistry(t, dbc)

	_, err := dbc.ExecContext(ctx, `INSERT INTO co_purchase_pairs (item_a,item_b,count,b2b_count,b2c_count)
		VALUES ('reg-a','reg-b',5,5,0) ON CONFLICT DO NOTHING`)
	if err != nil {
		t.Fatal(err)
	}

	res, err := catalog.GenerateRelations(ctx, dbc, catalog.GenerateOptions{
		MinCoPurchase: 3, Limit: 50, OnlyItems: []string{"reg-a", "reg-b", "reg-c"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.CoPurchase != 1 {
		t.Fatalf("co-purchase created = %d", res.CoPurchase)
	}

	// Отклоняем и проверяем, что повторный прогон не воскрешает пару.
	var id int64
	dbc.QueryRowContext(ctx, `SELECT id FROM relations
		WHERE from_item_id='reg-a' AND to_item_id='reg-b' AND type='adjacent_task'`).Scan(&id)
	if _, err := catalog.ReviewRegistryRelation(ctx, dbc, id, "reject", "не связаны", "ivanov", 1); err != nil {
		t.Fatal(err)
	}
	res, err = catalog.GenerateRelations(ctx, dbc, catalog.GenerateOptions{
		MinCoPurchase: 3, Limit: 50, OnlyItems: []string{"reg-a", "reg-b", "reg-c"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.CoPurchase != 0 || res.SkippedRejected != 1 {
		t.Fatalf("second run = %+v", res)
	}
}
