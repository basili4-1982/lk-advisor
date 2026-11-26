package store_test

import (
	"context"
	"testing"

	"github.com/basili4-1982/lk-advisor/internal/store"
)

func TestLoadCounts(t *testing.T) {
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

	counts := []struct {
		table string
		want  int
	}{
		{"clients", 50},
		{"products", 200},
		{"client_environment", 50},
		{"purchases", 500},
		{"licenses", 100},
		{"orders", 200},
		{"events", 1000},
		{"rules", 20},
		{"promotions", 40},
	}
	for _, c := range counts {
		if got := store.Count(t, dbc, c.table); got != c.want {
			t.Errorf("%s = %d, want %d", c.table, got, c.want)
		}
	}
	if got := store.Count(t, dbc, "product_relations"); got < 300 {
		t.Errorf("product_relations = %d, want at least 300", got)
	}

	var covered int
	if err := dbc.QueryRow(`SELECT count(DISTINCT client_id) FROM ground_truth`).Scan(&covered); err != nil {
		t.Fatalf("ground truth clients: %v", err)
	}
	if covered != 50 {
		t.Errorf("ground truth covers %d clients, want 50", covered)
	}

	// Loading twice must not duplicate rows.
	if err := store.Load(ctx, dbc, ds); err != nil {
		t.Fatalf("second load: %v", err)
	}
	if got := store.Count(t, dbc, "products"); got != 200 {
		t.Errorf("products after second load = %d, want 200", got)
	}
}
