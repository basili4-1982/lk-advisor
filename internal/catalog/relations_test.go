package catalog_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/basili4-1982/lk-advisor/internal/catalog"
	"github.com/basili4-1982/lk-advisor/internal/store"
)

// seedRegistry fills the v5 registry with a small deterministic fixture.
// The test database is isolated, so it clears the registry tables first.
func seedRegistry(t *testing.T, dbc *sql.DB) {
	t.Helper()
	ctx := context.Background()
	if _, err := dbc.ExecContext(ctx, `DELETE FROM relation_events`); err != nil {
		t.Fatalf("seed registry: clear events: %v", err)
	}
	if _, err := dbc.ExecContext(ctx, `DELETE FROM relations`); err != nil {
		t.Fatalf("seed registry: clear relations: %v", err)
	}

	items := []struct {
		id    string
		group string
	}{
		{"reg-a", "REG-G1"},
		{"reg-b", "REG-G1"},
		{"reg-c", "REG-G2"},
		{"reg-d", "REG-G2"},
	}
	for _, it := range items {
		if _, err := dbc.ExecContext(ctx, `
			INSERT INTO lk_items (item_id, title, category, product_group)
			VALUES ($1, $2, 'software', $3)
			ON CONFLICT (item_id) DO UPDATE
			SET title = EXCLUDED.title, category = EXCLUDED.category,
			    product_group = EXCLUDED.product_group`,
			it.id, "Товар "+it.id, it.group); err != nil {
			t.Fatalf("seed registry: item %s: %v", it.id, err)
		}
	}

	edges := []struct {
		from, to, typ, status, provenance string
		confidence                        float64
		reason, rejectionReason           string
		rejected                          bool
	}{
		{"reg-a", "reg-b", "accessory", "active", "manual", 0.90, "", "", false},
		{"reg-a", "reg-c", "adjacent_task", "candidate", "co_purchase", 0.60, "куплено вместе 3 раза", "", false},
		{"reg-c", "reg-d", "upgrade", "rejected", "llm", 0.40, "", "не та задача", true},
	}
	for _, e := range edges {
		if _, err := dbc.ExecContext(ctx, `
			INSERT INTO relations
				(from_item_id, to_item_id, type, confidence, status,
				 provenance_kind, source, reason, rejection_reason, rejected_until)
			VALUES ($1, $2, $3, $4, $5, $6, $6, $7, $8,
			        CASE WHEN $9 THEN now() + interval '90 days' ELSE NULL END)`,
			e.from, e.to, e.typ, e.confidence, e.status,
			e.provenance, e.reason, e.rejectionReason, e.rejected); err != nil {
			t.Fatalf("seed registry: edge %s->%s: %v", e.from, e.to, err)
		}
	}
}

// registryID returns the id of the edge from -> to.
func registryID(t *testing.T, dbc *sql.DB, from, to string) int64 {
	t.Helper()
	var id int64
	if err := dbc.QueryRow(`
		SELECT id FROM relations
		WHERE from_item_id = $1 AND to_item_id = $2`, from, to).Scan(&id); err != nil {
		t.Fatalf("registry id %s->%s: %v", from, to, err)
	}
	return id
}

// cleanRegistry removes the registry fixture and its items.
func cleanRegistry(t *testing.T, dbc *sql.DB) {
	t.Helper()
	ctx := context.Background()
	if _, err := dbc.ExecContext(ctx, `DELETE FROM relations`); err != nil {
		t.Fatalf("clean registry: relations: %v", err)
	}
	if _, err := dbc.ExecContext(ctx, `DELETE FROM lk_items WHERE item_id LIKE 'reg-%'`); err != nil {
		t.Fatalf("clean registry: items: %v", err)
	}
}

func TestListRegistryRelationsFilters(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	seedRegistry(t, dbc)
	defer cleanRegistry(t, dbc)

	page, err := catalog.ListRegistryRelations(ctx, dbc, catalog.RelationFilter{
		Statuses: []string{"candidate"}, Limit: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || page.Items[0].FromID != "reg-a" {
		t.Fatalf("candidate filter = %+v", page.Items)
	}

	page, err = catalog.ListRegistryRelations(ctx, dbc, catalog.RelationFilter{
		Query: "reg-b", Limit: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 {
		t.Fatalf("q=reg-b total = %d", page.Total)
	}

	page, err = catalog.ListRegistryRelations(ctx, dbc, catalog.RelationFilter{
		Family: "REG-G1", Limit: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 {
		t.Fatalf("family total = %d", page.Total)
	}

	page, err = catalog.ListRegistryRelations(ctx, dbc, catalog.RelationFilter{
		MinConfidence: 0.7, Limit: 50, Sort: "confidence_desc",
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || page.Items[0].ToID != "reg-b" {
		t.Fatalf("confidence filter = %+v", page.Items)
	}

	// Неизвестная сортировка не должна попадать в SQL и ломать запрос.
	page, err = catalog.ListRegistryRelations(ctx, dbc, catalog.RelationFilter{
		Sort: "confidence_desc; DROP TABLE relations", Limit: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 3 {
		t.Fatalf("injected sort total = %d", page.Total)
	}
}

// Legacy rows store evidence as {"source","value"}; the reader must map it to
// Kind/Summary, and pagination past the last row must keep the real total.
func TestListRegistryRelationsEvidenceAndPaging(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	seedRegistry(t, dbc)
	defer cleanRegistry(t, dbc)

	if _, err := dbc.ExecContext(ctx, `UPDATE relations
		SET evidence='[{"source":"co_purchase","value":"куплено вместе 3 раза"},
		               {"kind":"spec","summary":"вендор","details":{"url":"x"}}]'::jsonb
		WHERE from_item_id='reg-a' AND to_item_id='reg-c'`); err != nil {
		t.Fatal(err)
	}

	page, err := catalog.ListRegistryRelations(ctx, dbc, catalog.RelationFilter{
		Types: []string{"adjacent_task"}, Limit: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("items = %d", len(page.Items))
	}
	rel := page.Items[0]
	ev := rel.Evidence
	if len(ev) != 2 ||
		ev[0].Kind != "co_purchase" || ev[0].Summary != "куплено вместе 3 раза" ||
		ev[1].Kind != "spec" || ev[1].Details["url"] != "x" {
		t.Fatalf("evidence = %+v", ev)
	}
	if rel.FromTitle == "" || rel.FromFamily != "REG-G1" || rel.FromCategory != "software" {
		t.Fatalf("item cards = %+v", rel)
	}

	page, err = catalog.ListRegistryRelations(ctx, dbc, catalog.RelationFilter{
		Limit: 2, Offset: 99,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 0 || page.Total != 3 {
		t.Fatalf("empty page = total %d items %d", page.Total, len(page.Items))
	}
}

func TestRegistryMetrics(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	seedRegistry(t, dbc)
	defer cleanRegistry(t, dbc)

	m, err := catalog.RegistryMetrics(ctx, dbc)
	if err != nil {
		t.Fatal(err)
	}
	if m.Active != 1 || m.Candidates != 1 || m.Rejected != 1 {
		t.Fatalf("status metrics = %+v", m)
	}
	if m.ItemsWithActive < 2 || m.ItemsTotal < 3 {
		t.Fatalf("coverage metrics = %+v", m)
	}
	if m.ByType["accessory"] != 1 || m.ByProvenance["co_purchase"] != 1 {
		t.Fatalf("group metrics = %+v", m)
	}
}
