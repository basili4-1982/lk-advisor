package catalog_test

import (
	"context"
	"testing"

	"github.com/basili4-1982/lk-advisor/internal/catalog"
	"github.com/basili4-1982/lk-advisor/internal/store"
)

func TestDemoGraphUsesPublicLayer(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	if err := store.Migrate(ctx, dbc); err != nil {
		t.Fatal(err)
	}
	items := []string{"graph-pub-a", "graph-pub-b", "graph-pub-solo", "graph-small"}
	clients := []string{"graph-c1", "graph-c2", "graph-c3"}
	clean := func() {
		dbc.ExecContext(ctx, `DELETE FROM co_purchase_pairs WHERE item_a LIKE 'graph-%' OR item_b LIKE 'graph-%'`)
		dbc.ExecContext(ctx, `DELETE FROM lk_orders WHERE item_id LIKE 'graph-%'`)
		dbc.ExecContext(ctx, `DELETE FROM lk_clients WHERE client_id LIKE 'graph-%'`)
		dbc.ExecContext(ctx, `DELETE FROM lk_items WHERE item_id LIKE 'graph-%'`)
	}
	clean()
	defer clean()

	for _, item := range items {
		if _, err := dbc.ExecContext(ctx, `
			INSERT INTO lk_items (item_id, title, category) VALUES ($1, $1, 'security')`, item); err != nil {
			t.Fatal(err)
		}
	}
	for _, cid := range clients {
		if _, err := dbc.ExecContext(ctx, `
			INSERT INTO lk_clients (client_id, client_group) VALUES ($1, 'B2B')`, cid); err != nil {
			t.Fatal(err)
		}
	}
	// Three public buyers for a, b and solo. One buyer for the suppressed item.
	for _, item := range []string{"graph-pub-a", "graph-pub-b", "graph-pub-solo"} {
		for i, cid := range clients {
			oid := item + "-o" + cid
			if _, err := dbc.ExecContext(ctx, `
				INSERT INTO lk_orders (order_id, client_id, order_week, item_id)
				VALUES ($1, $2, '2026-08-01', $3)`, oid, cid, item); err != nil {
				t.Fatal(err)
			}
			_ = i
		}
	}
	if _, err := dbc.ExecContext(ctx, `
		INSERT INTO lk_orders (order_id, client_id, order_week, item_id)
		VALUES ('graph-small-o1', 'graph-c1', '2026-08-01', 'graph-small')`); err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][3]any{
		{"graph-pub-a", "graph-pub-b", 10},
		{"graph-pub-a", "graph-small", 10},
	} {
		if _, err := dbc.ExecContext(ctx, `
			INSERT INTO co_purchase_pairs (item_a, item_b, count, b2b_count, b2c_count)
			VALUES ($1, $2, $3, $3, 0)`, pair[0], pair[1], pair[2]); err != nil {
			t.Fatal(err)
		}
	}

	graph, err := catalog.LoadDemoGraph(ctx, dbc, "b2b", 1000)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, n := range graph.Nodes {
		ids[n.ID] = true
	}
	if !ids["graph-pub-a"] || !ids["graph-pub-b"] || !ids["graph-pub-solo"] {
		t.Fatalf("public nodes missing: %+v", graph.Nodes)
	}
	if ids["graph-small"] {
		t.Fatalf("suppressed item leaked into graph: %+v", graph.Nodes)
	}
	foundAB := false
	for _, e := range graph.Edges {
		if e.Source == "graph-small" || e.Target == "graph-small" {
			t.Fatalf("suppressed edge leaked: %+v", e)
		}
		if e.Source == "graph-pub-a" && e.Target == "graph-pub-b" {
			foundAB = true
		}
	}
	if !foundAB {
		t.Fatalf("public edge a-b missing")
	}
	if graph.Connected < 2 || graph.PublicSolo < 1 {
		t.Fatalf("connected=%d solo=%d", graph.Connected, graph.PublicSolo)
	}
	if _, err := catalog.CreateManualDatasetRelation(ctx, dbc,
		"graph-pub-a", "graph-pub-b", "adjacent_task", "quarantine", 0.8, "manual test"); err != nil {
		t.Fatal(err)
	}
	defer dbc.ExecContext(ctx, `DELETE FROM lk_relation_candidates WHERE source='manual' AND reason='manual test'`)
	manualGraph, err := catalog.LoadDemoGraph(ctx, dbc, "b2b", 1000)
	if err != nil {
		t.Fatal(err)
	}
	foundManual := false
	for _, e := range manualGraph.Edges {
		if e.Manual && e.Source == "graph-pub-a" && e.Target == "graph-pub-b" {
			foundManual = true
		}
	}
	if !foundManual {
		t.Fatalf("manual edge missing: %+v", manualGraph.Edges)
	}

	month, err := catalog.LoadDemoGraphAt(ctx, dbc, "b2b", "2026-08", 1000)
	if err != nil {
		t.Fatal(err)
	}
	if month.Period != "2026-08" || month.PeriodPublicItems < 2 {
		t.Fatalf("period graph = %+v", month)
	}
	foundDemand := false
	for _, n := range month.Nodes {
		if n.ID == "graph-pub-a" && n.PeriodBuyers >= 3 {
			foundDemand = true
		}
	}
	if !foundDemand {
		t.Fatalf("period demand missing: %+v", month.Nodes)
	}
	for _, n := range graph.Nodes {
		if n.ID == "graph-pub-solo" && !n.Solo {
			t.Fatalf("graph-pub-solo should be solo: %+v", n)
		}
	}
}

func TestDemoGraphSegmentFilter(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	if err := store.Migrate(ctx, dbc); err != nil {
		t.Fatal(err)
	}
	clean := func() {
		dbc.ExecContext(ctx, `DELETE FROM lk_orders WHERE item_id LIKE 'graph-seg-%'`)
		dbc.ExecContext(ctx, `DELETE FROM lk_clients WHERE client_id LIKE 'graph-seg-%'`)
		dbc.ExecContext(ctx, `DELETE FROM lk_items WHERE item_id LIKE 'graph-seg-%'`)
	}
	clean()
	defer clean()

	for _, item := range []string{"graph-seg-it", "graph-seg-it-solo", "graph-seg-mfg"} {
		if _, err := dbc.ExecContext(ctx, `
			INSERT INTO lk_items (item_id, title, category) VALUES ($1, $1, 'security')`, item); err != nil {
			t.Fatal(err)
		}
	}
	for i := 1; i <= 3; i++ {
		it := "graph-seg-it-client-" + string(rune('0'+i))
		mfg := "graph-seg-mfg-client-" + string(rune('0'+i))
		for _, c := range []struct{ id, industry string }{{it, "TEST-IT"}, {mfg, "TEST-MFG"}} {
			if _, err := dbc.ExecContext(ctx, `
				INSERT INTO lk_clients (client_id, client_group, industry, segment)
				VALUES ($1, 'B2B', $2, 'small')`, c.id, c.industry); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := dbc.ExecContext(ctx, `
			INSERT INTO lk_orders (order_id, client_id, order_week, item_id) VALUES
			($1, $2, '2026-08-01', 'graph-seg-it'),
			($1, $2, '2026-08-01', 'graph-seg-it-solo')`, "graph-seg-it-order-"+string(rune('0'+i)), it); err != nil {
			t.Fatal(err)
		}
		if _, err := dbc.ExecContext(ctx, `
			INSERT INTO lk_orders (order_id, client_id, order_week, item_id) VALUES
			($1, $2, '2026-08-01', 'graph-seg-mfg')`, "graph-seg-mfg-order-"+string(rune('0'+i)), mfg); err != nil {
			t.Fatal(err)
		}
	}

	graph, err := catalog.LoadDemoGraphFiltered(ctx, dbc, "b2b", "TEST-IT", "small", "", 1000)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, n := range graph.Nodes {
		ids[n.ID] = true
	}
	if !ids["graph-seg-it"] || !ids["graph-seg-it-solo"] {
		t.Fatalf("IT nodes missing: %+v", graph.Nodes)
	}
	if ids["graph-seg-mfg"] {
		t.Fatalf("manufacturing item leaked into IT segment: %+v", graph.Nodes)
	}
	if len(graph.Edges) != 1 || graph.Edges[0].Source != "graph-seg-it" || graph.Edges[0].Target != "graph-seg-it-solo" {
		t.Fatalf("segment edges = %+v", graph.Edges)
	}
	if graph.FilterLabel != "B2B · TEST-IT · small" {
		t.Fatalf("filter label = %q", graph.FilterLabel)
	}
}

func TestDemoGraphPaginationExternalLinks(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	if err := store.Migrate(ctx, dbc); err != nil {
		t.Fatal(err)
	}
	clean := func() {
		dbc.ExecContext(ctx, `DELETE FROM lk_orders WHERE item_id LIKE 'graph-page-%'`)
		dbc.ExecContext(ctx, `DELETE FROM lk_clients WHERE client_id LIKE 'graph-page-%'`)
		dbc.ExecContext(ctx, `DELETE FROM lk_items WHERE item_id LIKE 'graph-page-%'`)
	}
	clean()
	defer clean()
	for _, item := range []string{"graph-page-a", "graph-page-b"} {
		dbc.ExecContext(ctx, `INSERT INTO lk_items (item_id, title, category) VALUES ($1, $1, 'security')`, item)
	}
	for i := 1; i <= 3; i++ {
		cid := "graph-page-client-" + string(rune('0'+i))
		dbc.ExecContext(ctx, `INSERT INTO lk_clients (client_id, client_group) VALUES ($1, 'B2B')`, cid)
		dbc.ExecContext(ctx, `
			INSERT INTO lk_orders (order_id, client_id, order_week, item_id) VALUES
			($1, $2, '2026-08-01', 'graph-page-a'),
			($1, $2, '2026-08-01', 'graph-page-b')`,
			"graph-page-order-"+string(rune('0'+i)), cid)
	}

	graph, err := catalog.LoadDemoGraphPage(ctx, dbc, "b2b", "", "", "", 1, 1, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if graph.TotalPages < 2 || len(graph.Nodes) != 1 {
		t.Fatalf("pagination = page %d/%d nodes=%d", graph.Page, graph.TotalPages, len(graph.Nodes))
	}
	if len(graph.ExternalEdges) == 0 {
		t.Fatalf("expected external edge, got none")
	}
	if graph.Nodes[0].ExternalCount == 0 {
		t.Fatalf("external count not set: %+v", graph.Nodes[0])
	}
}
