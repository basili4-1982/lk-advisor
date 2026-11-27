package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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
		VALUES ('reg-a','reg-b','accessory',0.8,'active','manual','manual', $1)`,
		apiNow.Add(-time.Hour))
	dbc.ExecContext(ctx, `INSERT INTO relations
		(from_item_id,to_item_id,type,confidence,status,provenance_kind,source,revalidate_after)
		VALUES ('reg-a','reg-b','upgrade',0.7,'candidate','manual','manual', $1)`,
		apiNow.Add(-time.Hour))

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

func TestRegistryBulkReviewSelectedActive(t *testing.T) {
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
		VALUES ('reg-a','reg-b','accessory',0.8,'active','manual','manual')`)
	var id int64
	dbc.QueryRow(`SELECT id FROM relations WHERE from_item_id='reg-a'`).Scan(&id)

	rec := doJSON(t, h, http.MethodPost, "/v1/admin/registry/relations/bulk-review",
		fmt.Sprintf(`{"ids":[%d],"decision":"reject","note":"не подходит"}`, id))
	if rec.Code != http.StatusOK {
		t.Fatalf("bulk selected active: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Reviewed int `json:"reviewed"`
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Reviewed != 1 {
		t.Fatalf("reviewed = %d, want 1 (explicit selection must win)", out.Reviewed)
	}
}

func TestDatasetRejectRequiresReason(t *testing.T) {
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
	var id int64
	dbc.QueryRow(`SELECT id FROM relations WHERE from_item_id='reg-a'`).Scan(&id)

	rec := doJSON(t, h, http.MethodPost, "/v1/admin/dataset/candidates/review",
		fmt.Sprintf(`{"id":%d,"decision":"reject","note":""}`, id))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("dataset reject without reason = %d, want 400", rec.Code)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/admin/dataset/candidates/review",
		fmt.Sprintf(`{"id":%d,"decision":"reject","note":"не та задача"}`, id))
	if rec.Code != http.StatusOK {
		t.Fatalf("dataset reject with reason = %d %s", rec.Code, rec.Body.String())
	}
	var status, reason string
	var until *string
	dbc.QueryRow(`SELECT status, rejection_reason, rejected_until::text
		FROM relations WHERE id=$1`, id).Scan(&status, &reason, &until)
	if status != "rejected" || reason != "не та задача" || until == nil {
		t.Fatalf("dataset reject = status %q reason %q until %v", status, reason, until)
	}
}

func TestRegistryAliases(t *testing.T) {
	h, dbc := setupAPI(t)
	ctx := context.Background()
	clean := func() {
		dbc.ExecContext(ctx, `DELETE FROM item_aliases WHERE item_id LIKE 'reg-%'`)
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

	rec := doJSON(t, h, http.MethodPut, "/v1/admin/registry/aliases/reg-a",
		`{"short_name":"GWS Bus. Std"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("alias put: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet,
		"/v1/admin/registry/relations?q=GWS%20Bus.%20Std", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("alias search: %d %s", rec.Code, rec.Body.String())
	}
	var page struct {
		Total int `json:"total"`
		Items []struct {
			FromShort string `json:"from_short"`
		} `json:"items"`
	}
	json.Unmarshal(rec.Body.Bytes(), &page)
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].FromShort != "GWS Bus. Std" {
		t.Fatalf("alias search page = %+v", page)
	}

	rec = doJSON(t, h, http.MethodPut, "/v1/admin/registry/aliases/reg-a", `{"short_name":""}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("alias clear: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet,
		"/v1/admin/registry/relations?q=GWS%20Bus.%20Std", "")
	json.Unmarshal(rec.Body.Bytes(), &page)
	if page.Total != 0 {
		t.Fatalf("alias still searchable after clear: %+v", page)
	}
}

func TestRegistryBulkByConfidenceFilter(t *testing.T) {
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
	for _, e := range []struct {
		typ  string
		conf float64
	}{
		{"accessory", 0.90}, {"bundle", 0.70}, {"compatibility", 0.60},
	} {
		dbc.ExecContext(ctx, `INSERT INTO relations
			(from_item_id,to_item_id,type,confidence,status,provenance_kind,source)
			VALUES ('reg-a','reg-b',$1,$2,'candidate','co_purchase','co_purchase')`, e.typ, e.conf)
	}

	rec := doJSON(t, h, http.MethodPost, "/v1/admin/registry/relations/bulk-review",
		`{"filter":{"statuses":["candidate"],"query":"reg-","min_confidence":0.85},"decision":"confirm","note":""}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("bulk by filter: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Reviewed int `json:"reviewed"`
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Reviewed != 1 {
		t.Fatalf("reviewed = %d, want 1", out.Reviewed)
	}
	var active, candidates int
	dbc.QueryRow(`SELECT count(*) FILTER (WHERE status='active'),
		count(*) FILTER (WHERE status='candidate') FROM relations
		WHERE from_item_id='reg-a'`).Scan(&active, &candidates)
	if active != 1 || candidates != 2 {
		t.Fatalf("active=%d candidates=%d, want 1/2", active, candidates)
	}
}

func TestRegistryImportItemsUpload(t *testing.T) {
	h, dbc := setupAPI(t)
	ctx := context.Background()
	clean := func() {
		dbc.ExecContext(ctx, `DELETE FROM lk_items WHERE item_id LIKE 'TEST-IMPORT-%'`)
	}
	clean()
	defer clean()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "positions.csv")
	if err != nil {
		t.Fatal(err)
	}
	part.Write([]byte("item_id,title,sku,product_group,available\nTEST-IMPORT-1,Импортированная позиция,TEST-IMPORT-1,G-IMPORT,true\n"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/registry/import/items", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("import: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Added   int `json:"added"`
		Updated int `json:"updated"`
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Added != 1 || out.Updated != 0 {
		t.Fatalf("import result = %+v", out)
	}
	var title string
	if err := dbc.QueryRow(`SELECT title FROM lk_items WHERE item_id='TEST-IMPORT-1'`).Scan(&title); err != nil {
		t.Fatal(err)
	}
	if title != "Импортированная позиция" {
		t.Fatalf("title = %q", title)
	}
	var feedUpdated sql.NullTime
	if err := dbc.QueryRow(`SELECT feed_updated_at FROM lk_items WHERE item_id='TEST-IMPORT-1'`).Scan(&feedUpdated); err != nil {
		t.Fatal(err)
	}
	if !feedUpdated.Valid {
		t.Fatal("import must set feed_updated_at")
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/admin/registry/feed", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("feed: %d %s", rec.Code, rec.Body.String())
	}
	var feed struct {
		Available int    `json:"available"`
		Total     int    `json:"total"`
		UpdatedAt string `json:"updated_at"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &feed); err != nil {
		t.Fatal(err)
	}
	if feed.Available == 0 || feed.Total == 0 || feed.UpdatedAt == "" {
		t.Fatalf("feed = %+v", feed)
	}
}

func TestRegistryResetRequiresConfirm(t *testing.T) {
	h, _ := setupAPI(t)
	rec := doJSON(t, h, http.MethodPost, "/v1/admin/registry/reset", `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("reset without confirm = %d, want 400", rec.Code)
	}
}
