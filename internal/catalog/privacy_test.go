package catalog_test

import (
	"context"
	"testing"

	"github.com/basili4-1982/lk-advisor/internal/catalog"
	"github.com/basili4-1982/lk-advisor/internal/store"
)

func TestPublicDatasetHonoursKThresholds(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	if err := store.Migrate(ctx, dbc); err != nil {
		t.Fatal(err)
	}

	items := []string{"priv-pub", "priv-small", "priv-b2c4", "priv-b2c5", "priv-b2cpub"}
	orderIDs := []string{
		"priv-o-pub-1", "priv-o-pub-2", "priv-o-pub-3",
		"priv-o-small",
		"priv-o-b2c4-1", "priv-o-b2c4-2", "priv-o-b2c4-3", "priv-o-b2c4-4",
		"priv-o-b2c5-1", "priv-o-b2c5-2", "priv-o-b2c5-3", "priv-o-b2c5-4", "priv-o-b2c5-5",
		"priv-o-b2cpub-1", "priv-o-b2cpub-2", "priv-o-b2cpub-3", "priv-o-b2cpub-4", "priv-o-b2cpub-5",
	}
	clean := func() {
		dbc.ExecContext(ctx, `DELETE FROM co_purchase_pairs WHERE item_a LIKE 'priv-%' OR item_b LIKE 'priv-%'`)
		dbc.ExecContext(ctx, `DELETE FROM lk_orders WHERE item_id LIKE 'priv-%'`)
		dbc.ExecContext(ctx, `DELETE FROM lk_clients WHERE client_id LIKE 'priv-%'`)
		dbc.ExecContext(ctx, `DELETE FROM lk_items WHERE item_id LIKE 'priv-%'`)
	}
	clean()
	defer clean()

	for _, item := range items {
		if _, err := dbc.ExecContext(ctx, `
			INSERT INTO lk_items (item_id, title, category) VALUES ($1, $1, 'security')`, item); err != nil {
			t.Fatal(err)
		}
	}

	// 3 B2B buyers for the public anchor.
	for i, oid := range orderIDs[:3] {
		cid := "priv-b2b-" + string(rune('1'+i))
		if _, err := dbc.ExecContext(ctx, `
			INSERT INTO lk_clients (client_id, client_group) VALUES ($1, 'B2B')
			ON CONFLICT DO NOTHING`, cid); err != nil {
			t.Fatal(err)
		}
		if _, err := dbc.ExecContext(ctx, `
			INSERT INTO lk_orders (order_id, client_id, order_week, item_id)
			VALUES ($1, $2, '2026-08-01', 'priv-pub')`, oid, cid); err != nil {
			t.Fatal(err)
		}
	}
	// 1 buyer: below the k=3 line.
	if _, err := dbc.ExecContext(ctx, `
		INSERT INTO lk_clients (client_id, client_group) VALUES ('priv-b2b-small', 'B2B')
		ON CONFLICT DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	if _, err := dbc.ExecContext(ctx, `
		INSERT INTO lk_orders (order_id, client_id, order_week, item_id)
		VALUES ('priv-o-small', 'priv-b2b-small', '2026-08-01', 'priv-small')`); err != nil {
		t.Fatal(err)
	}

	b2c := map[string]int{"priv-b2c4": 4, "priv-b2c5": 5, "priv-b2cpub": 5}
	for item, n := range b2c {
		for i := 0; i < n; i++ {
			cid := "priv-b2c-" + item + "-" + string(rune('1'+i))
			if _, err := dbc.ExecContext(ctx, `
				INSERT INTO lk_clients (client_id, client_group) VALUES ($1, 'B2C')
				ON CONFLICT DO NOTHING`, cid); err != nil {
				t.Fatal(err)
			}
			oid := "priv-o-" + item + "-" + string(rune('1'+i))
			if _, err := dbc.ExecContext(ctx, `
				INSERT INTO lk_orders (order_id, client_id, order_week, item_id)
				VALUES ($1, $2, '2026-08-01', $3)`, oid, cid, item); err != nil {
				t.Fatal(err)
			}
		}
	}

	for _, pair := range [][2]string{
		{"priv-pub", "priv-small"},
		{"priv-b2cpub", "priv-b2c4"},
		{"priv-b2cpub", "priv-b2c5"},
	} {
		if _, err := dbc.ExecContext(ctx, `
			INSERT INTO co_purchase_pairs (item_a, item_b, count, b2b_count, b2c_count)
			VALUES ($1, $2, 10000, 10, 10)`, pair[0], pair[1]); err != nil {
			t.Fatal(err)
		}
	}

	// B2B view hides the <3-buyer item.
	b2b, err := catalog.TopCoPurchases(ctx, dbc, 10, "b2b")
	if err != nil {
		t.Fatal(err)
	}
	if hasPair(b2b, "priv-pub", "priv-small") {
		t.Fatalf("b2b view leaked a <3-buyer item: %+v", b2b)
	}
	if !hasPair(b2b, "priv-b2cpub", "priv-b2c4") {
		t.Fatalf("b2b view should show the 4-buyer B2C item: %+v", b2b)
	}

	// B2C view hides the <5-client B2C item.
	b2cPairs, err := catalog.TopCoPurchases(ctx, dbc, 10, "b2c")
	if err != nil {
		t.Fatal(err)
	}
	if hasPair(b2cPairs, "priv-b2cpub", "priv-b2c4") {
		t.Fatalf("b2c view leaked a <5-client item: %+v", b2cPairs)
	}
	if !hasPair(b2cPairs, "priv-b2cpub", "priv-b2c5") {
		t.Fatalf("b2c view should show the 5-client pair: %+v", b2cPairs)
	}

	cov, err := catalog.LoadDatasetCoverage(ctx, dbc)
	if err != nil {
		t.Fatal(err)
	}
	if cov.SuppressedItems < 1 || cov.SuppressedB2CItems < 1 {
		t.Fatalf("coverage must count suppressed items: %+v", cov)
	}
}

func hasPair(items []catalog.CoPurchase, a, b string) bool {
	for _, x := range items {
		if (x.ItemA == a && x.ItemB == b) || (x.ItemA == b && x.ItemB == a) {
			return true
		}
	}
	return false
}
