package store_test

import (
	"context"
	"testing"

	"github.com/basili4-1982/lk-advisor/internal/profile"
	"github.com/basili4-1982/lk-advisor/internal/store"
)

func TestSeedLKCase(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	if err := store.Migrate(ctx, dbc); err != nil {
		t.Fatal(err)
	}
	store.Truncate(t, dbc)
	ds, err := store.Generate(42)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Load(ctx, dbc, ds); err != nil {
		t.Fatal(err)
	}
	if err := store.SeedLKCase(ctx, dbc); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := dbc.QueryRow(`SELECT count(*) FROM licenses WHERE client_id = 'lk-36612'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 4 {
		t.Fatalf("lk licenses = %d", n)
	}
	if err := dbc.QueryRow(`SELECT count(*) FROM products WHERE id LIKE 'prod-lk-%'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 4 {
		t.Fatalf("lk products = %d", n)
	}
	if err := profile.RefreshAll(ctx, dbc); err != nil {
		t.Fatal(err)
	}
	p, err := profile.Get(ctx, dbc, "lk-36612")
	if err != nil {
		t.Fatal(err)
	}
	if p.Audience != "b2b" || len(p.Licenses) != 4 || len(p.Purchases) == 0 {
		t.Fatalf("profile = %+v", p)
	}
}
