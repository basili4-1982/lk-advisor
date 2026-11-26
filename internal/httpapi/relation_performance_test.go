package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/basili4-1982/lk-advisor/internal/analytics"
)

func TestRelationPerformanceAndAutoReview(t *testing.T) {
	h, dbc := setupAPI(t)
	ctx := context.Background()
	if _, err := dbc.ExecContext(ctx, `
		INSERT INTO product_relations (from_id,to_id,type,direction,confidence,status,source)
		VALUES ('prod-pdf','prod-backup','accessory','forward',0.8,'llm_suggested','test')
		ON CONFLICT (from_id,to_id,type) DO UPDATE SET status='llm_suggested'`); err != nil {
		t.Fatal(err)
	}
	defer dbc.ExecContext(ctx, `DELETE FROM product_relations WHERE from_id='prod-pdf' AND to_id='prod-backup' AND type='accessory'`)

	shown := make([]analytics.Item, 0, 30)
	for i := 0; i < 30; i++ {
		shown = append(shown, analytics.Item{
			ProductID: "prod-backup", Type: "cross_sell", Source: "engine",
			CandidateID:     "prod-pdf|accessory|prod-backup",
			SourceProductID: "prod-pdf", RelationType: "accessory",
		})
	}
	if err := analytics.Log(ctx, dbc, "tr-perf", "smb-001", "cross_sell",
		analytics.Meta{Strategy: "hybrid", Audience: "b2b", WeightsVersion: "v1"}, false, shown, nil); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if err := analytics.Record(ctx, dbc, "tr-perf", "smb-001", "prod-backup", "clicked"); err != nil {
			t.Fatal(err)
		}
	}

	rec := doJSON(t, h, http.MethodGet, "/v1/admin/relations/performance", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("perf: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Items []struct {
			FromID string  `json:"from_id"`
			ToID   string  `json:"to_id"`
			Shown  int     `json:"shown"`
			Clicks int     `json:"clicks"`
			CTR    float64 `json:"ctr"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, x := range out.Items {
		if x.FromID == "prod-pdf" && x.ToID == "prod-backup" {
			found = true
			if x.Shown != 30 || x.Clicks != 5 || x.CTR < 0.16 {
				t.Fatalf("item = %+v", x)
			}
		}
	}
	if !found {
		t.Fatalf("relation not found: %+v", out.Items)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/admin/relations/auto-review", `{"min_shown":20,"min_ctr":0.03}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("auto: %d %s", rec.Code, rec.Body.String())
	}
	var status string
	if err := dbc.QueryRow(`SELECT status FROM product_relations WHERE from_id='prod-pdf' AND to_id='prod-backup' AND type='accessory'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "confirmed" {
		t.Fatalf("status = %s, want confirmed", status)
	}
}
