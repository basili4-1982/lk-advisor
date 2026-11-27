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
		`{"from_id":"exp-cat-a","to_id":"exp-cat-b","relation_type":"adjacent_task",
		  "title":"Свой заголовок","hypothesis":"Своя формулировка","audience":"b2b",
		  "industry":"образование","metric":"ctr","guardrails":"renewal, fallback","traffic_split":0.7}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("create experiment: %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID      string `json:"id"`
		Created bool   `json:"created"`
	}
	json.Unmarshal(rec.Body.Bytes(), &created)
	if created.ID == "" || !created.Created {
		t.Fatalf("created = %+v", created)
	}
	var title, hypothesis, metric, guardrails, industry string
	var split float64
	if err := dbc.QueryRow(`SELECT title, hypothesis, metric, guardrails, industry, traffic_split::float8
		FROM experiments WHERE id=$1`, created.ID).Scan(
		&title, &hypothesis, &metric, &guardrails, &industry, &split); err != nil {
		t.Fatal(err)
	}
	if title != "Свой заголовок" || hypothesis != "Своя формулировка" {
		t.Fatalf("title/hypothesis = %q / %q", title, hypothesis)
	}
	if metric != "ctr" || guardrails != "renewal, fallback" || industry != "образование" || split != 0.7 {
		t.Fatalf("fields = %q / %q / %q / %v", metric, guardrails, industry, split)
	}
	var exposures int
	if err := dbc.QueryRow(`SELECT count(*) FROM experiment_events
		WHERE experiment_id=$1`, created.ID).Scan(&exposures); err != nil {
		t.Fatal(err)
	}
	if exposures != 0 {
		t.Fatalf("new hypothesis must start without events, got %d", exposures)
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/admin/experiments",
		`{"from_id":"exp-cat-a","to_id":"exp-cat-b","audience":"b2b"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("duplicate: %d %s", rec.Code, rec.Body.String())
	}
	var dup struct {
		ID      string `json:"id"`
		Created bool   `json:"created"`
	}
	json.Unmarshal(rec.Body.Bytes(), &dup)
	if dup.ID != created.ID || dup.Created {
		t.Fatalf("duplicate = %+v", dup)
	}
}

func TestCreateExperimentRequiresAudience(t *testing.T) {
	h, _ := setupAPI(t)
	rec := doJSON(t, h, http.MethodPost, "/v1/admin/experiments",
		`{"from_id":"a","to_id":"b"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
