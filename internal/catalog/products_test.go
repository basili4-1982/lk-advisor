package catalog_test

import (
	"context"
	"testing"
	"time"

	"github.com/basili4-1982/lk-advisor/internal/catalog"
	"github.com/basili4-1982/lk-advisor/internal/store"
)

func TestPopularityScoresDecay(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	if err := store.Migrate(ctx, dbc); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	clean := func() {
		dbc.ExecContext(ctx, `DELETE FROM products WHERE id IN ('test-pop-recent','test-pop-old')`)
	}
	clean()
	defer clean()
	if _, err := dbc.ExecContext(ctx, `
		INSERT INTO products (id, sku, title, category, type, price, status, audience) VALUES
		('test-pop-recent', 'test-pop-recent', 'Recent', 'security', 'license', 1000, 'active', 'any'),
		('test-pop-old', 'test-pop-old', 'Old', 'security', 'license', 1000, 'active', 'any')`); err != nil {
		t.Fatal(err)
	}
	if _, err := dbc.ExecContext(ctx, `
		INSERT INTO purchases (id, client_id, product_id, date, amount) VALUES
		('test-pop-r1', 'smb-001', 'test-pop-recent', $1, 1000),
		('test-pop-o1', 'smb-001', 'test-pop-old', $2, 1000),
		('test-pop-o2', 'smb-001', 'test-pop-old', $3, 1000),
		('test-pop-o3', 'smb-001', 'test-pop-old', $4, 1000),
		('test-pop-o4', 'smb-001', 'test-pop-old', $5, 1000),
		('test-pop-o5', 'smb-001', 'test-pop-old', $6, 1000)`,
		now.AddDate(0, 0, -7), now.AddDate(0, 0, -280), now.AddDate(0, 0, -281),
		now.AddDate(0, 0, -282), now.AddDate(0, 0, -283), now.AddDate(0, 0, -284)); err != nil {
		t.Fatal(err)
	}

	scores, err := catalog.PopularityScores(ctx, dbc, "", now, 8)
	if err != nil {
		t.Fatal(err)
	}
	recent, old := scores["test-pop-recent"], scores["test-pop-old"]
	if recent <= old {
		t.Fatalf("recent score %.3f must beat old score %.3f", recent, old)
	}
}
