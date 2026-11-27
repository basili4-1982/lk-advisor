package catalog_test

import (
	"context"
	"testing"

	"github.com/basili4-1982/lk-advisor/internal/catalog"
	"github.com/basili4-1982/lk-advisor/internal/store"
)

func TestServeUsesConfirmedRelationsAndExcludesOwned(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	if err := store.Migrate(ctx, dbc); err != nil {
		t.Fatal(err)
	}
	cleanup := func() {
		dbc.ExecContext(ctx, `DELETE FROM fstec_certificates WHERE certificate_number = 'TEST-CERT'`)
		dbc.ExecContext(ctx, `DELETE FROM relations WHERE from_item_id LIKE 'serve-test-%'`)
		dbc.ExecContext(ctx, `DELETE FROM lk_orders WHERE order_id LIKE 'serve-test-%'`)
		dbc.ExecContext(ctx, `DELETE FROM lk_items WHERE item_id LIKE 'serve-test-%'`)
		dbc.ExecContext(ctx, `DELETE FROM lk_clients WHERE client_id LIKE 'serve-test-%'`)
	}
	cleanup()
	t.Cleanup(cleanup)

	mustExec := func(q string, args ...any) {
		t.Helper()
		if _, err := dbc.ExecContext(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	mustExec(`INSERT INTO lk_clients (client_id, client_type, industry) VALUES
		('serve-test-c1', 'ЮЛ', 'Медицинские организации'),
		('serve-test-c2', 'ЮЛ', 'Медицинские организации'),
		('serve-test-cold0', 'ЮЛ', 'Медицинские организации')`)
	mustExec(`INSERT INTO lk_items (item_id, title, category, available) VALUES
		('serve-test-src', 'Купленный товар', 'office', true),
		('serve-test-target', 'Целевой товар', 'office', true),
		('serve-test-acc', 'Аксессуар', 'office', true),
		('serve-test-cand', 'Неподтверждённый', 'office', true),
		('serve-test-off', 'Снят с продажи', 'office', false)`)
	mustExec(`INSERT INTO lk_orders (order_id, client_id, order_week, item_id) VALUES
		('serve-test-o1', 'serve-test-c1', '2026-01-01', 'serve-test-src'),
		('serve-test-o2', 'serve-test-c2', '2026-01-01', 'serve-test-target'),
		('serve-test-o3', 'serve-test-c2', '2026-01-01', 'serve-test-acc')`)
	mustExec(`INSERT INTO relations (from_item_id, to_item_id, type, confidence, status) VALUES
		('serve-test-src', 'serve-test-target', 'upgrade', 0.9, 'active'),
		('serve-test-src', 'serve-test-acc', 'accessory', 0.8, 'active'),
		('serve-test-src', 'serve-test-cand', 'adjacent_task', 0.95, 'candidate'),
		('serve-test-src', 'serve-test-off', 'adjacent_task', 0.95, 'active')`)

	res, err := catalog.Serve(ctx, dbc, catalog.ServeOptions{ClientID: "serve-test-c1", Limit: 3})
	if err != nil {
		t.Fatal(err)
	}
	if res.ColdStart {
		t.Fatal("client with an order must not be cold start")
	}
	if res.Items[0].ItemID != "serve-test-target" {
		t.Fatalf("first item = %s, want target", res.Items[0].ItemID)
	}
	mustExec(`INSERT INTO fstec_certificates (certificate_number, product, applicant, valid_until)
		VALUES ('TEST-CERT', 'Целевой товар', 'Тест', current_date + 365)`)
	mustExec(`INSERT INTO fstec_certificate_items (certificate_id, item_id, method, confidence)
		SELECT id, 'serve-test-target', 'test', 1 FROM fstec_certificates WHERE certificate_number='TEST-CERT'`)
	certified, err := catalog.Serve(ctx, dbc, catalog.ServeOptions{
		ClientID: "serve-test-c1", Limit: 3, CertifiedOnly: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(certified.Items) != 1 || certified.Items[0].ItemID != "serve-test-target" || !certified.Items[0].Certified {
		t.Fatalf("certified filter = %+v", certified.Items)
	}
	if certified.Items[0].Score < res.Items[0].Score {
		t.Fatalf("certificate must not lower score: %.3f vs %.3f", certified.Items[0].Score, res.Items[0].Score)
	}

	if res.Items[0].Compatibility != "unverified" {
		t.Fatalf("compatibility = %q, want unverified", res.Items[0].Compatibility)
	}
	mustExec(`UPDATE lk_items SET system_requirements='{"os":"Windows"}'::jsonb
		WHERE item_id='serve-test-target'`)
	res, err = catalog.Serve(ctx, dbc, catalog.ServeOptions{ClientID: "serve-test-c1", Limit: 3})
	if err != nil {
		t.Fatal(err)
	}
	if res.Items[0].Compatibility != "ok" {
		t.Fatalf("compatibility = %q, want ok", res.Items[0].Compatibility)
	}
	for _, it := range res.Items {
		if it.ItemID == "serve-test-cand" {
			t.Fatalf("candidate relation leaked into serving: %+v", res.Items)
		}
		if it.ItemID == "serve-test-off" {
			t.Fatalf("unavailable item leaked into serving: %+v", res.Items)
		}
	}

	// A client that already owns the target must not get it forced.
	mustExec(`INSERT INTO lk_orders (order_id, client_id, order_week, item_id)
		VALUES ('serve-test-o4', 'serve-test-c1', '2026-01-02', 'serve-test-target')`)
	res, err = catalog.Serve(ctx, dbc, catalog.ServeOptions{
		ClientID: "serve-test-c1", Limit: 3, ForceItemID: "serve-test-target", ForceType: "upgrade",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range res.Items {
		if it.ItemID == "serve-test-target" {
			t.Fatalf("owned item was recommended: %+v", res.Items)
		}
	}

	// A client without history gets popular items from the same profile.
	res2, err := catalog.Serve(ctx, dbc, catalog.ServeOptions{ClientID: "serve-test-cold0", Limit: 3})
	if err != nil {
		t.Fatal(err)
	}
	if !res2.ColdStart || len(res2.Items) == 0 {
		t.Fatalf("cold start = %v, items = %+v", res2.ColdStart, res2.Items)
	}
}
