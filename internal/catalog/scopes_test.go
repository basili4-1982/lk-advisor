package catalog_test

import (
	"context"
	"testing"
	"time"

	"github.com/basili4-1982/lk-advisor/internal/analytics"
	"github.com/basili4-1982/lk-advisor/internal/catalog"
	"github.com/basili4-1982/lk-advisor/internal/model"
	"github.com/basili4-1982/lk-advisor/internal/store"
)

func TestRelationsScopedStatusAndTTL(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	if err := store.Migrate(ctx, dbc); err != nil {
		t.Fatal(err)
	}
	const from, to, typ = "prod-pdf", "prod-backup", "cross_sell"
	clean := func() {
		dbc.ExecContext(ctx, `DELETE FROM relation_scopes WHERE from_id=$1 AND to_id=$2 AND type=$3`, from, to, typ)
		dbc.ExecContext(ctx, `DELETE FROM product_relations WHERE from_id=$1 AND to_id=$2 AND type=$3`, from, to, typ)
		dbc.ExecContext(ctx, `DELETE FROM recommendation_events WHERE candidate_id = $1`, from+"|"+typ+"|"+to)
		dbc.ExecContext(ctx, `DELETE FROM recommendation_requests WHERE trace_id LIKE 'tr-scope-%'`)
	}
	clean()
	defer clean()
	if _, err := dbc.ExecContext(ctx, `
		INSERT INTO product_relations (from_id,to_id,type,direction,confidence,status,source)
		VALUES ($1,$2,$3,'forward',0.7,'llm_suggested','test')`, from, to, typ); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	ids := []string{from}
	find := func(rels []model.Relation) *model.Relation {
		for i := range rels {
			if rels[i].ToID == to && rels[i].Type == typ {
				return &rels[i]
			}
		}
		return nil
	}

	if _, err := catalog.UpsertRelationScope(ctx, dbc, from, to, typ, "b2b", "it",
		"rejected", 0.8, "test", "wrong for IT", 30*24*time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.UpsertRelationScope(ctx, dbc, from, to, typ, "b2c", "",
		"confirmed", 0.9, "test", "works for consumers", 60*24*time.Hour); err != nil {
		t.Fatal(err)
	}

	rels, err := catalog.RelationsScoped(ctx, dbc, ids, "b2b", "it", now)
	if err != nil {
		t.Fatal(err)
	}
	if got := find(rels); got != nil {
		t.Fatalf("b2b/it should be rejected, got %+v", *got)
	}

	rels, err = catalog.RelationsScoped(ctx, dbc, ids, "b2b", "manufacturing", now)
	if err != nil {
		t.Fatal(err)
	}
	got := find(rels)
	if got == nil || got.Status != "llm_suggested" || got.Scoped {
		t.Fatalf("b2b/manufacturing should fall back to global, got %+v", got)
	}

	rels, err = catalog.RelationsScoped(ctx, dbc, ids, "b2c", "retail", now)
	if err != nil {
		t.Fatal(err)
	}
	got = find(rels)
	if got == nil || got.Status != "confirmed" || got.ScopeAudience != "b2c" {
		t.Fatalf("b2c should be confirmed by audience scope, got %+v", got)
	}

	if _, err := dbc.ExecContext(ctx, `
		UPDATE relation_scopes SET valid_until = now() - interval '1 hour'
		WHERE from_id=$1 AND to_id=$2 AND type=$3 AND audience='b2c'`, from, to, typ); err != nil {
		t.Fatal(err)
	}
	rels, err = catalog.RelationsScoped(ctx, dbc, ids, "b2c", "retail", now)
	if err != nil {
		t.Fatal(err)
	}
	if got := find(rels); got != nil {
		t.Fatalf("expired b2c scope should be hidden, got %+v", *got)
	}
}

func TestAutoReviewIsScoped(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	if err := store.Migrate(ctx, dbc); err != nil {
		t.Fatal(err)
	}
	const from, to, typ = "prod-pdf", "prod-backup", "cross_sell"
	clean := func() {
		dbc.ExecContext(ctx, `DELETE FROM relation_scopes WHERE from_id=$1 AND to_id=$2 AND type=$3`, from, to, typ)
		dbc.ExecContext(ctx, `DELETE FROM product_relations WHERE from_id=$1 AND to_id=$2 AND type=$3`, from, to, typ)
		dbc.ExecContext(ctx, `DELETE FROM recommendation_events WHERE candidate_id = $1`, from+"|"+typ+"|"+to)
		dbc.ExecContext(ctx, `DELETE FROM recommendation_requests WHERE trace_id LIKE 'tr-scope-%'`)
	}
	clean()
	defer clean()
	if _, err := dbc.ExecContext(ctx, `
		INSERT INTO product_relations (from_id,to_id,type,direction,confidence,status,source)
		VALUES ($1,$2,$3,'forward',0.7,'llm_suggested','test')`, from, to, typ); err != nil {
		t.Fatal(err)
	}

	logShown := func(trace, audience, industry string, count, clicks int) {
		items := make([]analytics.Item, 0, count)
		for i := 0; i < count; i++ {
			items = append(items, analytics.Item{
				ProductID: to, Type: "cross_sell", Source: "engine",
				CandidateID:     from + "|" + typ + "|" + to,
				SourceProductID: from, RelationType: typ,
			})
		}
		if err := analytics.Log(ctx, dbc, trace, "smb-001", "cross_sell",
			analytics.Meta{Strategy: "hybrid", Audience: audience, Industry: industry, WeightsVersion: "v1"},
			false, items, nil); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < clicks; i++ {
			if err := analytics.Record(ctx, dbc, trace, "smb-001", to, "clicked"); err != nil {
				t.Fatal(err)
			}
		}
	}
	logShown("tr-scope-b2b", "b2b", "it", 30, 6)
	logShown("tr-scope-b2c-1", "b2c", "retail", 10, 0)

	promoted, rejected, err := catalog.AutoReviewRelations(ctx, dbc, 30, 0.03)
	if err != nil {
		t.Fatal(err)
	}
	if promoted != 1 || rejected != 0 {
		t.Fatalf("promoted=%d rejected=%d, want 1/0", promoted, rejected)
	}
	var status string
	var validUntil *time.Time
	if err := dbc.QueryRowContext(ctx, `
		SELECT status, valid_until FROM relation_scopes
		WHERE from_id=$1 AND to_id=$2 AND type=$3 AND audience='b2b' AND industry='it'`,
		from, to, typ).Scan(&status, &validUntil); err != nil {
		t.Fatal(err)
	}
	if status != "confirmed" || validUntil == nil || validUntil.Before(time.Now()) {
		t.Fatalf("status=%s valid_until=%v", status, validUntil)
	}

	logShown("tr-scope-b2c-2", "b2c", "retail", 20, 0)
	promoted, rejected, err = catalog.AutoReviewRelations(ctx, dbc, 30, 0.03)
	if err != nil {
		t.Fatal(err)
	}
	if promoted != 0 || rejected != 1 {
		t.Fatalf("second run promoted=%d rejected=%d, want 0/1", promoted, rejected)
	}
	var b2cStatus string
	if err := dbc.QueryRowContext(ctx, `
		SELECT status FROM relation_scopes
		WHERE from_id=$1 AND to_id=$2 AND type=$3 AND audience='b2c' AND industry='retail'`,
		from, to, typ).Scan(&b2cStatus); err != nil {
		t.Fatal(err)
	}
	if b2cStatus != "rejected" {
		t.Fatalf("b2c status = %s, want rejected", b2cStatus)
	}
}
