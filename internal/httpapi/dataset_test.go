package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/basili4-1982/lk-advisor/internal/catalog"
)

func TestDatasetEndpoints(t *testing.T) {
	h, dbc := setupAPI(t)
	ctx := context.Background()
	if _, err := dbc.ExecContext(ctx, `
		INSERT INTO lk_items (item_id, title, category) VALUES
		('tmp-a', 'Tmp A', 'devtools'), ('tmp-b', 'Tmp B', 'security')
		ON CONFLICT DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	defer dbc.ExecContext(ctx, `DELETE FROM lk_items WHERE item_id IN ('tmp-a','tmp-b')`)
	if _, err := dbc.ExecContext(ctx, `
		INSERT INTO co_purchase_pairs (item_a, item_b, count) VALUES ('tmp-a','tmp-b',7)
		ON CONFLICT (item_a,item_b) DO UPDATE SET count=7`); err != nil {
		t.Fatal(err)
	}
	defer dbc.ExecContext(ctx, `DELETE FROM co_purchase_pairs WHERE item_a='tmp-a' AND item_b='tmp-b'`)

	rec := doJSON(t, h, http.MethodGet, "/v1/admin/dataset/coverage", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("coverage: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/admin/dataset/co-purchases?limit=10", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("pairs: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/admin/dataset/candidates?limit=10", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("candidates: %d %s", rec.Code, rec.Body.String())
	}
}

func TestClassifyPairFallbackUsesTemplates(t *testing.T) {
	_, dbc := setupAPI(t)
	hh := &handler{db: dbc, logger: slog.New(slog.NewTextHandler(io.Discard, nil)), now: func() time.Time { return apiNow }}
	types, err := catalog.RelationTypes(context.Background(), dbc)
	if err != nil {
		t.Fatal(err)
	}
	active := activeRelationTypes(types)
	typ, conf, reason, evidence, source := hh.classifyPair(context.Background(), catalog.CoPurchase{
		ItemA: "a", ItemB: "b", TitleA: "A", TitleB: "B", Count: 5,
	}, active)
	if typ == "" || conf <= 0 || reason == "" || evidence == "" || source == "" {
		t.Fatalf("typ=%q conf=%v reason=%q ev=%q source=%q", typ, conf, reason, evidence, source)
	}
	if _, ok := findRelationType(active, typ); !ok {
		t.Fatalf("fallback type %q not in active templates", typ)
	}
}
