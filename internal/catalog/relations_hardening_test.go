package catalog_test

import (
	"context"
	"testing"
	"time"

	"github.com/basili4-1982/lk-advisor/internal/catalog"
	"github.com/basili4-1982/lk-advisor/internal/store"
)

func TestDatasetCoverageCountsOnlyCandidates(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	seedRegistry(t, dbc)
	defer cleanRegistry(t, dbc)

	dbc.ExecContext(ctx, `INSERT INTO relations
		(from_item_id,to_item_id,type,confidence,status,provenance_kind,source)
		VALUES ('reg-a','reg-b','upgrade',0.7,'active','manual','manual')`)

	c, err := catalog.LoadDatasetCoverage(ctx, dbc)
	if err != nil {
		t.Fatal(err)
	}
	if c.Candidates != 1 {
		t.Fatalf("coverage candidates = %d, want 1 (only status=candidate)", c.Candidates)
	}
}

func TestGenerateCoPurchaseRevivesAfterCooldown(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	seedRegistry(t, dbc)
	defer cleanRegistry(t, dbc)

	if _, err := dbc.ExecContext(ctx, `INSERT INTO co_purchase_pairs
		(item_a,item_b,count,b2b_count,b2c_count) VALUES ('reg-a','reg-b',5,5,0)
		ON CONFLICT DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	if _, err := dbc.ExecContext(ctx, `INSERT INTO relations
		(from_item_id,to_item_id,type,confidence,status,provenance_kind,source,
		 rejection_reason,rejected_until)
		VALUES ('reg-a','reg-b','adjacent_task',0.7,'rejected','co_purchase','co_purchase',
		        'старое решение', now() - interval '1 day')`); err != nil {
		t.Fatal(err)
	}

	res, err := catalog.GenerateRelations(ctx, dbc, catalog.GenerateOptions{
		MinCoPurchase: 3, Limit: 50, OnlyItems: []string{"reg-a", "reg-b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.CoPurchase != 1 {
		t.Fatalf("co-purchase after cooldown = %d, want 1", res.CoPurchase)
	}
	var status, reason string
	var until *time.Time
	if err := dbc.QueryRowContext(ctx, `SELECT status, rejection_reason, rejected_until
		FROM relations WHERE from_item_id='reg-a' AND to_item_id='reg-b'
		  AND type='adjacent_task'`).Scan(&status, &reason, &until); err != nil {
		t.Fatal(err)
	}
	if status != "candidate" || reason != "" || until != nil {
		t.Fatalf("revived row = status %q reason %q until %v", status, reason, until)
	}
}

func TestLegacyInsertsSetProvenance(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	seedRegistry(t, dbc)
	defer cleanRegistry(t, dbc)

	if ok, err := catalog.CreateManualDatasetRelation(ctx, dbc,
		"reg-a", "reg-c", "bundle", "candidate", 0.8, "ручная"); err != nil || !ok {
		t.Fatalf("manual insert = %v, %v", ok, err)
	}
	var manual string
	dbc.QueryRowContext(ctx, `SELECT provenance_kind FROM relations
		WHERE from_item_id='reg-a' AND to_item_id='reg-c' AND type='bundle'`).Scan(&manual)
	if manual != "manual" {
		t.Fatalf("manual provenance = %q, want manual", manual)
	}

	if ok, err := catalog.InsertDatasetCandidate(ctx, dbc,
		"reg-a", "reg-c", "accessory", 0.6, "из данных", "[]", "co_purchase"); err != nil || !ok {
		t.Fatalf("candidate insert = %v, %v", ok, err)
	}
	var prov string
	dbc.QueryRowContext(ctx, `SELECT provenance_kind FROM relations
		WHERE from_item_id='reg-a' AND to_item_id='reg-c' AND type='accessory'`).Scan(&prov)
	if prov != "co_purchase" {
		t.Fatalf("candidate provenance = %q, want co_purchase", prov)
	}
}

func TestGenerateCoPurchaseSetsRevalidateAfter(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	seedRegistry(t, dbc)
	defer cleanRegistry(t, dbc)

	dbc.ExecContext(ctx, `INSERT INTO co_purchase_pairs
		(item_a,item_b,count,b2b_count,b2c_count) VALUES ('reg-a','reg-b',5,5,0)
		ON CONFLICT DO NOTHING`)
	if _, err := catalog.GenerateRelations(ctx, dbc, catalog.GenerateOptions{
		MinCoPurchase: 3, Limit: 50, OnlyItems: []string{"reg-a", "reg-b"},
	}); err != nil {
		t.Fatal(err)
	}
	var revalidate *time.Time
	if err := dbc.QueryRowContext(ctx, `SELECT revalidate_after FROM relations
		WHERE from_item_id='reg-a' AND to_item_id='reg-b' AND type='adjacent_task'`).
		Scan(&revalidate); err != nil {
		t.Fatal(err)
	}
	if revalidate == nil || !revalidate.After(time.Now()) {
		t.Fatalf("revalidate_after = %v, want a future timestamp", revalidate)
	}
}

func TestGenerateVariantsSupersedesOldRepresentative(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	seedRegistry(t, dbc)
	defer cleanRegistry(t, dbc)

	if _, err := catalog.GenerateRelations(ctx, dbc, catalog.GenerateOptions{
		IncludeVariants: true, OnlyItems: []string{"reg-a", "reg-b"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := dbc.ExecContext(ctx, `INSERT INTO lk_items
		(item_id,title,category,product_group) VALUES ('reg-0','Товар reg-0','software','REG-G1')`); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.GenerateRelations(ctx, dbc, catalog.GenerateOptions{
		IncludeVariants: true, OnlyItems: []string{"reg-0", "reg-a", "reg-b"},
	}); err != nil {
		t.Fatal(err)
	}

	var oldStatus string
	dbc.QueryRowContext(ctx, `SELECT status FROM relations
		WHERE from_item_id='reg-b' AND to_item_id='reg-a' AND type='variant_of'`).Scan(&oldStatus)
	if oldStatus != "superseded" {
		t.Fatalf("old rep edge status = %q, want superseded", oldStatus)
	}
	var active int
	dbc.QueryRowContext(ctx, `SELECT count(*) FROM relations
		WHERE type='variant_of' AND status='active' AND to_item_id='reg-0'`).Scan(&active)
	if active != 2 {
		t.Fatalf("edges to new rep = %d, want 2", active)
	}
}

func TestHubMetricIgnoresVariantStars(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	seedRegistry(t, dbc)
	defer cleanRegistry(t, dbc)

	// Two semantic edges give a 50% top-1% hub share.
	dbc.ExecContext(ctx, `INSERT INTO relations
		(from_item_id,to_item_id,type,confidence,status,provenance_kind,source)
		VALUES ('reg-a','reg-d','accessory',0.8,'active','manual','manual')`)
	// A structural star into reg-a must not turn it into a hub.
	for i := 1; i <= 9; i++ {
		id := "reg-v" + string(rune('0'+i))
		dbc.ExecContext(ctx, `INSERT INTO lk_items (item_id,title,category,product_group)
			VALUES ($1,$1,'software','HARD-V') ON CONFLICT DO NOTHING`, id)
		dbc.ExecContext(ctx, `INSERT INTO relations
			(from_item_id,to_item_id,type,confidence,status,provenance_kind,source)
			VALUES ($1,'reg-a','variant_of',0.99,'active','attribute','attribute')`, id)
	}
	m, err := catalog.RegistryMetrics(ctx, dbc)
	if err != nil {
		t.Fatal(err)
	}
	if m.HubTop1Pct != 50 {
		t.Fatalf("hub top1 = %v, want 50 (variant_of excluded)", m.HubTop1Pct)
	}
	dbc.ExecContext(ctx, `DELETE FROM lk_items WHERE item_id LIKE 'reg-v%'`)
}
