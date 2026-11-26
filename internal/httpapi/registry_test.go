package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestRegistryAPIFlow(t *testing.T) {
	h, dbc := setupAPI(t)
	ctx := context.Background()
	clean := func() {
		dbc.ExecContext(ctx, `DELETE FROM relations WHERE from_item_id LIKE 'reg-%'`)
		dbc.ExecContext(ctx, `DELETE FROM lk_items WHERE item_id LIKE 'reg-%'`)
	}
	clean()
	defer clean()
	for _, id := range []string{"reg-a", "reg-b"} {
		dbc.ExecContext(ctx, `INSERT INTO lk_items (item_id,title,category,product_group)
			VALUES ($1,$1,'security','REG-G1')`, id)
	}
	dbc.ExecContext(ctx, `INSERT INTO relations
		(from_item_id,to_item_id,type,confidence,status,provenance_kind,source)
		VALUES ('reg-a','reg-b','accessory',0.8,'candidate','manual','manual')`)

	rec := doJSON(t, h, http.MethodGet, "/v1/admin/registry/relations?status=candidate&q=reg-a", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}

	var page struct {
		Total int `json:"total"`
	}
	json.Unmarshal(rec.Body.Bytes(), &page)
	if page.Total != 1 {
		t.Fatalf("total = %d", page.Total)
	}

	var id int64
	dbc.QueryRow(`SELECT id FROM relations WHERE from_item_id='reg-a'`).Scan(&id)
	rec = doJSON(t, h, http.MethodPost,
		fmt.Sprintf("/v1/admin/registry/relations/%d/review", id),
		`{"decision":"reject","note":"","expected_version":1}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty note = %d", rec.Code)
	}

	rec = doJSON(t, h, http.MethodPost,
		fmt.Sprintf("/v1/admin/registry/relations/%d/review", id),
		`{"decision":"confirm","note":"ok","expected_version":1}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("confirm = %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodPost,
		fmt.Sprintf("/v1/admin/registry/relations/%d/review", id),
		`{"decision":"confirm","note":"ok","expected_version":1}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("stale = %d", rec.Code)
	}
}

func TestRegistryBulkReviewByIDs(t *testing.T) {
	h, dbc := setupAPI(t)
	ctx := context.Background()
	clean := func() {
		dbc.ExecContext(ctx, `DELETE FROM relations WHERE from_item_id LIKE 'reg-%'`)
		dbc.ExecContext(ctx, `DELETE FROM lk_items WHERE item_id LIKE 'reg-%'`)
	}
	clean()
	defer clean()
	for _, id := range []string{"reg-a", "reg-b", "reg-c"} {
		dbc.ExecContext(ctx, `INSERT INTO lk_items (item_id,title,category,product_group)
			VALUES ($1,$1,'security','REG-G1')`, id)
	}
	for _, pair := range [][2]string{{"reg-a", "reg-b"}, {"reg-a", "reg-c"}} {
		dbc.ExecContext(ctx, `INSERT INTO relations
			(from_item_id,to_item_id,type,confidence,status,provenance_kind,source)
			VALUES ($1,$2,'accessory',0.8,'candidate','manual','manual')`, pair[0], pair[1])
	}
	var id int64
	dbc.QueryRow(`SELECT id FROM relations WHERE to_item_id='reg-b'`).Scan(&id)

	rec := doJSON(t, h, http.MethodPost, "/v1/admin/registry/relations/bulk-review",
		fmt.Sprintf(`{"ids":[%d],"decision":"confirm","note":"пакет"}`, id))
	if rec.Code != http.StatusOK {
		t.Fatalf("bulk: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Reviewed int `json:"reviewed"`
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Reviewed != 1 {
		t.Fatalf("reviewed = %d, want 1", out.Reviewed)
	}
	var active int
	dbc.QueryRow(`SELECT count(*) FROM relations
		WHERE from_item_id LIKE 'reg-%' AND status='active'`).Scan(&active)
	if active != 1 {
		t.Fatalf("active = %d, want 1 (selection must not touch the other row)", active)
	}
}

func TestRegistryBulkRequiresCriterion(t *testing.T) {
	h, _ := setupAPI(t)
	rec := doJSON(t, h, http.MethodPost, "/v1/admin/registry/relations/bulk-review",
		`{"filter":{},"decision":"confirm","note":""}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("no criterion = %d, want 400", rec.Code)
	}
}

func TestRegistryMetricsEndpoint(t *testing.T) {
	h, _ := setupAPI(t)
	rec := doJSON(t, h, http.MethodGet, "/v1/admin/registry/metrics", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics = %d %s", rec.Code, rec.Body.String())
	}
}

func TestRegistrySweepEndpoint(t *testing.T) {
	h, dbc := setupAPI(t)
	ctx := context.Background()
	clean := func() {
		dbc.ExecContext(ctx, `DELETE FROM relations WHERE from_item_id LIKE 'reg-%'`)
		dbc.ExecContext(ctx, `DELETE FROM lk_items WHERE item_id LIKE 'reg-%'`)
	}
	clean()
	defer clean()
	for _, id := range []string{"reg-a", "reg-b"} {
		dbc.ExecContext(ctx, `INSERT INTO lk_items (item_id,title,category,product_group)
			VALUES ($1,$1,'security','REG-G1')`, id)
	}
	dbc.ExecContext(ctx, `INSERT INTO relations
		(from_item_id,to_item_id,type,confidence,status,provenance_kind,source,valid_until)
		VALUES ('reg-a','reg-b','accessory',0.8,'active','manual','manual', now()-interval '1 day')`)
	dbc.ExecContext(ctx, `INSERT INTO relations
		(from_item_id,to_item_id,type,confidence,status,provenance_kind,source,revalidate_after)
		VALUES ('reg-a','reg-b','upgrade',0.7,'candidate','manual','manual', now()-interval '1 day')`)

	rec := doJSON(t, h, http.MethodPost, "/v1/admin/registry/sweep", `{}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("sweep: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Stale   int `json:"stale"`
		Expired int `json:"expired"`
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Stale != 1 || out.Expired != 1 {
		t.Fatalf("sweep = %+v, want stale=1 expired=1", out)
	}
}
