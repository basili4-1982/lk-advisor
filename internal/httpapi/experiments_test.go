package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestExperimentEndpoints(t *testing.T) {
	h, dbc := setupAPI(t)
	t.Cleanup(func() {
		dbc.Exec(`DELETE FROM experiments WHERE id IN ('exp-pdf-b2b','exp-coreldraw-bad') OR id LIKE 'exp-catalog-%'`)
	})
	dbc.Exec(`DELETE FROM experiments WHERE id IN ('exp-pdf-b2b','exp-coreldraw-bad') OR id LIKE 'exp-catalog-%'`)

	rec := doJSON(t, h, http.MethodPost, "/v1/admin/experiments/demo", `{}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("seed: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/admin/experiments", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	var list struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) == 0 || list.Items[0].ID != "exp-pdf-b2b" {
		t.Fatalf("items = %+v", list.Items)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/admin/experiments/exp-pdf-b2b/decision",
		`{"decision":"reject","note":"test"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("decision: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/admin/experiments/knowledge?decision=reject", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("knowledge: %d %s", rec.Code, rec.Body.String())
	}
	var knowledge struct {
		Items []struct {
			ID       string `json:"id"`
			Decision string `json:"decision"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &knowledge); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range knowledge.Items {
		if item.ID == "exp-pdf-b2b" && item.Decision == "reject" {
			found = true
		}
	}
	if !found {
		t.Fatalf("knowledge = %+v", knowledge.Items)
	}
}

func TestCreateExperimentFromCatalogRelation(t *testing.T) {
	h, dbc := setupAPI(t)
	ctx := context.Background()
	dbc.ExecContext(ctx, `DELETE FROM lk_items WHERE item_id LIKE 'exp-cat-%'`)
	dbc.ExecContext(ctx, `DELETE FROM experiments WHERE from_id LIKE 'exp-cat-%'`)
	defer func() {
		dbc.ExecContext(ctx, `DELETE FROM experiments WHERE from_id LIKE 'exp-cat-%'`)
		dbc.ExecContext(ctx, `DELETE FROM lk_items WHERE item_id LIKE 'exp-cat-%'`)
	}()
	for _, id := range []string{"exp-cat-a", "exp-cat-b"} {
		dbc.ExecContext(ctx, `INSERT INTO lk_items (item_id,title,category,product_group)
			VALUES ($1,$1,'software','EXP-CAT')`, id)
	}
	rec := doJSON(t, h, http.MethodPost, "/v1/admin/experiments",
		`{"from_id":"exp-cat-a","to_id":"exp-cat-b","relation_type":"adjacent_task","audience":"b2b"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("create experiment: %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	json.Unmarshal(rec.Body.Bytes(), &created)
	if created.ID == "" {
		t.Fatal("no experiment id")
	}
	var exposures int
	if err := dbc.QueryRow(`SELECT count(*) FROM experiment_events
		WHERE experiment_id=$1 AND event='exposed'`, created.ID).Scan(&exposures); err != nil {
		t.Fatal(err)
	}
	if exposures != 300 {
		t.Fatalf("exposures = %d, want 300", exposures)
	}
}
