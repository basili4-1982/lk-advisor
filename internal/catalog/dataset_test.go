package catalog_test

import (
	"context"
	"testing"

	"github.com/basili4-1982/lk-advisor/internal/catalog"
	"github.com/basili4-1982/lk-advisor/internal/store"
)

func TestDatasetCoverageDeltaAndCandidateReview(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	if err := store.Migrate(ctx, dbc); err != nil {
		t.Fatal(err)
	}
	before, err := catalog.LoadDatasetCoverage(ctx, dbc)
	if err != nil {
		t.Fatal(err)
	}

	const a, b = "test-item-a", "test-item-b"
	if _, err := dbc.ExecContext(ctx, `
		INSERT INTO lk_items (item_id, title, category) VALUES
		($1, 'Test A', 'devtools'), ($2, 'Test B', 'security')
		ON CONFLICT DO NOTHING`, a, b); err != nil {
		t.Fatal(err)
	}
	defer dbc.ExecContext(ctx, `DELETE FROM lk_items WHERE item_id IN ($1,$2)`, a, b)
	if _, err := dbc.ExecContext(ctx, `
		INSERT INTO co_purchase_pairs (item_a, item_b, count, b2b_count, b2c_count)
		VALUES ($1,$2,3,3,0)
		ON CONFLICT (item_a,item_b) DO UPDATE SET count=3, b2b_count=3, b2c_count=0`, a, b); err != nil {
		t.Fatal(err)
	}
	defer dbc.ExecContext(ctx, `DELETE FROM co_purchase_pairs WHERE item_a=$1 AND item_b=$2`, a, b)

	orderIDs := "'test-order-multi','test-order-cross','test-order-a3','test-order-b3'"
	dbc.ExecContext(ctx, `DELETE FROM lk_orders WHERE order_id IN (`+orderIDs+`)`)
	if _, err := dbc.ExecContext(ctx, `
		INSERT INTO lk_orders (order_id, client_id, order_week, item_id) VALUES
		('test-order-multi', 'test-client-1', '2026-08-01', $1),
		('test-order-multi', 'test-client-2', '2026-08-08', $1),
		('test-order-a3', 'test-client-3', '2026-08-15', $1),
		('test-order-cross', 'test-client-1', '2026-08-31', $2),
		('test-order-cross', 'test-client-2', '2026-09-07', $2),
		('test-order-b3', 'test-client-3', '2026-09-14', $2)`, a, b); err != nil {
		t.Fatal(err)
	}
	defer dbc.ExecContext(ctx, `DELETE FROM lk_orders WHERE order_id IN (`+orderIDs+`)`)

	mid, err := catalog.LoadDatasetCoverage(ctx, dbc)
	if err != nil {
		t.Fatal(err)
	}
	if mid.Items != before.Items+2 || mid.Pairs != before.Pairs+1 {
		t.Fatalf("before=%+v mid=%+v", before, mid)
	}
	if mid.Orders != before.Orders+4 || mid.OrderInstances != before.OrderInstances+6 {
		t.Fatalf("orders before=%+v mid=%+v", before, mid)
	}
	if mid.MultiWeekOrders != before.MultiWeekOrders+2 {
		t.Fatalf("multi-week before=%+v mid=%+v", before, mid)
	}
	if mid.CrossCutOrders != before.CrossCutOrders+1 {
		t.Fatalf("cross-cut before=%+v mid=%+v", before, mid)
	}
	if mid.PublicItems != before.PublicItems+2 || mid.SuppressedItems != before.SuppressedItems {
		t.Fatalf("privacy before=%+v mid=%+v", before, mid)
	}

	ok, err := catalog.InsertDatasetCandidate(ctx, dbc, a, b, "adjacent_task", 0.8,
		"test", `[{"source":"co_purchase","value":"count=3"}]`, "test")
	if err != nil || !ok {
		t.Fatalf("insert ok=%v err=%v", ok, err)
	}
	ok, err = catalog.InsertDatasetCandidate(ctx, dbc, a, b, "adjacent_task", 0.8,
		"test", `[{"source":"co_purchase","value":"count=3"}]`, "test")
	if err != nil || ok {
		t.Fatalf("duplicate insert ok=%v err=%v", ok, err)
	}
	list, err := catalog.DatasetCandidates(ctx, dbc, 500, "b2b")
	if err != nil {
		t.Fatal(err)
	}
	var id int64
	for _, c := range list {
		if c.FromID == a && c.ToID == b {
			id = c.ID
		}
	}
	if id == 0 {
		t.Fatal("candidate not found")
	}
	ok, err = catalog.ReviewDatasetCandidate(ctx, dbc, id, "active", "ok")
	if err != nil || !ok {
		t.Fatalf("review ok=%v err=%v", ok, err)
	}
	defer dbc.ExecContext(ctx, `DELETE FROM relations WHERE id=$1`, id)
}
