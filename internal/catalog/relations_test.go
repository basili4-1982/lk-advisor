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

	// Extend the shared seed into the "metrics fixture": 4 active, 1 candidate,
	// 1 rejected, 1 stale edge over reg-a..reg-d.
	//   active accessory  a->b  (target b, expiring in 10 days)
	//   active bundle     b->c  (target c)
	//   active compat.    a->c  (target c)
	//   active adjacent   b->d  (target d)
	//   stale  bundle     c->d
	// Semantic edges (active + candidate, variant_of excluded) target degrees:
	// c=3 (b->c, a->c active, a->c candidate), b=1, d=1 -> 3 targets, top 1% =
	// 1 target (c), so HubTop1Pct = 3/5 = 60%.
	// StaleShare = stale/(active+stale) = 1/5 = 20%.
	// Distinct items touched by active edges = reg-a..reg-d = 4.
	extra := []struct {
		from, to, typ, status, provenance string
		confidence                        float64
	}{
		{"reg-b", "reg-c", "bundle", "active", "manual", 0.80},
		{"reg-a", "reg-c", "compatibility", "active", "manual", 0.85},
		{"reg-b", "reg-d", "adjacent_task", "active", "manual", 0.70},
		{"reg-c", "reg-d", "bundle", "stale", "attribute", 0.55},
	}
	for _, e := range extra {
		if _, err := dbc.ExecContext(ctx, `
			INSERT INTO relations
				(from_item_id, to_item_id, type, confidence, status, provenance_kind, source)
			VALUES ($1, $2, $3, $4, $5, $6, $6)`,
			e.from, e.to, e.typ, e.confidence, e.status, e.provenance); err != nil {
			t.Fatalf("metrics fixture edge %s->%s: %v", e.from, e.to, err)
		}
	}
	if _, err := dbc.ExecContext(ctx, `
		UPDATE relations SET valid_until = now() + interval '10 days'
		WHERE from_item_id = 'reg-a' AND to_item_id = 'reg-b'`); err != nil {
		t.Fatalf("metrics fixture expiring edge: %v", err)
	}

	m, err := catalog.RegistryMetrics(ctx, dbc)
	if err != nil {
		t.Fatal(err)
	}

	if m.Total != 7 || m.Active != 4 || m.Candidates != 1 || m.Rejected != 1 || m.Stale != 1 {
		t.Fatalf("status metrics = %+v", m)
	}
	if m.Expiring30 != 1 {
		t.Fatalf("expiring metrics = %d, want 1", m.Expiring30)
	}
	if m.ItemsWithActive != 4 {
		t.Fatalf("items with active = %d, want 4", m.ItemsWithActive)
	}
	// ItemsTotal spans the whole shared lk_items table, so CoveragePct is
	// asserted against its exact definition rather than a fixed number.
	if want := 100 * float64(m.ItemsWithActive) / float64(m.ItemsTotal); m.CoveragePct != want {
		t.Fatalf("coverage = %v, want %v (items %d/%d)", m.CoveragePct, want, m.ItemsWithActive, m.ItemsTotal)
	}
	if m.StaleShare != 20 {
		t.Fatalf("stale share = %v, want 20", m.StaleShare)
	}
	if m.HubTop1Pct != 60 {
		t.Fatalf("hub top1 = %v, want 60", m.HubTop1Pct)
	}

	if m.ByType["accessory"] != 1 || m.ByType["bundle"] != 2 ||
		m.ByType["compatibility"] != 1 || m.ByType["adjacent_task"] != 2 ||
		m.ByType["upgrade"] != 1 {
		t.Fatalf("by type = %+v", m.ByType)
	}
	if m.ByProvenance["co_purchase"] != 1 || m.ByProvenance["manual"] != 4 {
		t.Fatalf("by provenance = %+v", m.ByProvenance)
	}

	// Families is capped at the first 100 distinct groups of the whole shared
	// lk_items table, so the fixture's REG-* groups fall outside that window.
	// Assert the contract exactly: Families is the sorted, distinct prefix of at
	// most 100 groups, and the cap is really applied.
	allRows, err := dbc.QueryContext(ctx, `
		SELECT DISTINCT product_group FROM lk_items
		WHERE product_group <> '' ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	var all []string
	for allRows.Next() {
		var family string
		if err := allRows.Scan(&family); err != nil {
			t.Fatal(err)
		}
		all = append(all, family)
	}
	if err := allRows.Err(); err != nil {
		t.Fatal(err)
	}
	allRows.Close()

	want := all
	if len(want) > 100 {
		want = want[:100]
	}
	if len(m.Families) != len(want) {
		t.Fatalf("families = %d, want %d (distinct groups %d)", len(m.Families), len(want), len(all))
	}
	for i := range want {
		if m.Families[i] != want[i] {
			t.Fatalf("families[%d] = %q, want %q", i, m.Families[i], want[i])
		}
	}
	if len(m.FamilyOptions) == 0 {
		t.Fatal("family options are empty")
	}
	for _, o := range m.FamilyOptions {
		if o.Value == "" || o.Label == "" || o.Count <= 0 {
			t.Fatalf("bad family option %+v", o)
		}
	}
	// The fixture groups exist and are therefore reachable when the table has
	// fewer than 100 groups; on the shared dataset they sort past the cap.
	inWindow := len(all) <= 100
	for _, g := range []string{"REG-G1", "REG-G2"} {
		found := false
		for _, f := range all {
			if f == g {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("fixture group %s missing from lk_items", g)
		}
		if inWindow {
			present := false
			for _, f := range m.Families {
				if f == g {
					present = true
					break
				}
			}
			if !present {
				t.Fatalf("fixture group %s missing from families %v", g, m.Families)
			}
		}
	}
}
