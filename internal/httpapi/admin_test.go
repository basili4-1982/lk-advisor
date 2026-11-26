package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestAdminState(t *testing.T) {
	h, _ := setupAPI(t)
	rec := doJSON(t, h, http.MethodGet, "/v1/admin/state", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var st adminState
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(st.Products) != 200 || len(st.Relations) == 0 || len(st.Rules) == 0 {
		t.Fatalf("state = products %d relations %d rules %d", len(st.Products), len(st.Relations), len(st.Rules))
	}
	if len(st.Clients) == 0 {
		t.Fatal("no clients")
	}
}

func TestAdminRelationCRUD(t *testing.T) {
	h, dbc := setupAPI(t)
	body := `{"from_id":"prod-pdf","to_id":"prod-backup","type":"accessory","status":"confirmed","confidence":0.7}`
	rec := doJSON(t, h, http.MethodPost, "/v1/admin/relations", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	defer func() {
		doJSON(t, h, http.MethodDelete, "/v1/admin/relations",
			`{"from_id":"prod-pdf","to_id":"prod-backup","type":"accessory"}`)
	}()

	var exists bool
	if err := dbc.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM product_relations WHERE from_id='prod-pdf' AND to_id='prod-backup' AND type='accessory')`).
		Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("relation was not created")
	}

	rec = doJSON(t, h, http.MethodDelete, "/v1/admin/relations",
		`{"from_id":"prod-pdf","to_id":"prod-backup","type":"accessory"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
}

func TestAdminRuleCRUD(t *testing.T) {
	h, _ := setupAPI(t)
	body := `{"type":"max_discount","discount":7}`
	rec := doJSON(t, h, http.MethodPost, "/v1/admin/rules", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var rule adminRule
	if err := json.Unmarshal(rec.Body.Bytes(), &rule); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if rule.ID == "" || rule.Summary == "" {
		t.Fatalf("rule = %+v", rule)
	}
	defer doJSON(t, h, http.MethodDelete, "/v1/admin/rules/"+rule.ID, "")

	rec = doJSON(t, h, http.MethodDelete, "/v1/admin/rules/"+rule.ID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
}

func TestAdminWeightsRoundTrip(t *testing.T) {
	h, _ := setupAPI(t)
	body := `{"relevance":0.4,"urgency":0.2,"affinity":0.2,"margin":0.1,"confidence":0.1}`
	rec := doJSON(t, h, http.MethodPut, "/v1/admin/weights", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	defer doJSON(t, h, http.MethodPut, "/v1/admin/weights",
		`{"relevance":0.35,"urgency":0.25,"affinity":0.2,"margin":0.1,"confidence":0.1}`)

	rec = doJSON(t, h, http.MethodGet, "/v1/admin/state", "")
	var st adminState
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if st.Weights.Relevance != 0.4 {
		t.Fatalf("weights = %+v", st.Weights)
	}
}

func TestAdminPage(t *testing.T) {
	h, _ := setupAPI(t)
	rec := doJSON(t, h, http.MethodGet, "/admin", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, needle := range []string{"Реестр связей", "Каталог", "Типы связей", "Данные", "Гипотезы"} {
		if !contains(body, needle) {
			t.Fatalf("unexpected page: missing %q", needle)
		}
	}
	for _, legacy := range []string{`data-tab="relations"`, `data-tab="validation"`, `data-tab="rules"`, `data-tab="weights"`, `data-tab="analytics"`, `data-tab="assistant"`} {
		if contains(body, legacy) {
			t.Fatalf("legacy tab button still present: %s", legacy)
		}
	}
}

func TestAdminAgentApplyRelation(t *testing.T) {
	h, _ := setupAPI(t)
	body := `{"operations":[{"action":"add_relation","from_id":"prod-sl-86","to_id":"prod-backup","type":"adjacent_task","confidence":0.9,"status":"confirmed"}]}`
	rec := doJSON(t, h, http.MethodPost, "/v1/admin/agent/apply", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("apply: %d %s", rec.Code, rec.Body.String())
	}
	defer doJSON(t, h, http.MethodDelete, "/v1/admin/relations",
		`{"from_id":"prod-sl-86","to_id":"prod-backup","type":"adjacent_task"}`)
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestRelationTypeCRUDAndPrompt(t *testing.T) {
	h, _ := setupAPI(t)
	body := `{"code":"industry_addon","title":"Отраслевое дополнение","description":"Отраслевой модуль к платформе: закрывает отраслевую задачу и не заменяет платформу.","examples":"1С → отраслевой модуль; CAD → библиотека ГОСТ.","behavior":"cross_sell","direction":"forward","default_confidence":0.85,"status":"active"}`
	rec := doJSON(t, h, http.MethodPost, "/v1/admin/relation-types", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	defer doJSON(t, h, http.MethodDelete, "/v1/admin/relation-types/industry_addon", "")

	rec = doJSON(t, h, http.MethodGet, "/v1/admin/state", "")
	var st adminState
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, rt := range st.RelationTypes {
		if rt.Code == "industry_addon" {
			found = true
			if rt.Description == "" || rt.Behavior != "cross_sell" {
				t.Fatalf("type = %+v", rt)
			}
		}
	}
	if !found {
		t.Fatal("custom type not in state")
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/admin/relation-types/prompt", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("prompt: %d %s", rec.Code, rec.Body.String())
	}
	var prompt struct {
		Prompt string `json:"prompt"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &prompt); err != nil {
		t.Fatal(err)
	}
	if !contains(prompt.Prompt, "industry_addon") || !contains(prompt.Prompt, "Отраслевой модуль") {
		t.Fatalf("prompt = %s", prompt.Prompt)
	}
}

func TestBuiltinRelationTypeCannotBeDeleted(t *testing.T) {
	h, _ := setupAPI(t)
	rec := doJSON(t, h, http.MethodDelete, "/v1/admin/relation-types/upgrade", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestSymmetricRelationCreatesReverse(t *testing.T) {
	h, dbc := setupAPI(t)
	body := `{"from_id":"prod-pdf","to_id":"prod-backup","type":"compatibility","status":"confirmed","confidence":0.8}`
	rec := doJSON(t, h, http.MethodPost, "/v1/admin/relations", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	defer func() {
		doJSON(t, h, http.MethodDelete, "/v1/admin/relations",
			`{"from_id":"prod-pdf","to_id":"prod-backup","type":"compatibility"}`)
		doJSON(t, h, http.MethodDelete, "/v1/admin/relations",
			`{"from_id":"prod-backup","to_id":"prod-pdf","type":"compatibility"}`)
	}()
	var n int
	if err := dbc.QueryRow(
		`SELECT count(*) FROM product_relations WHERE type='compatibility' AND ((from_id='prod-pdf' AND to_id='prod-backup') OR (from_id='prod-backup' AND to_id='prod-pdf'))`).
		Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("relations = %d, want 2", n)
	}
}

func TestReviewQueueConfirm(t *testing.T) {
	h, dbc := setupAPI(t)
	body := `{"from_id":"prod-pdf","to_id":"prod-backup","type":"adjacent_task","status":"llm_suggested","confidence":0.62,"reason":"тестовая причина","evidence":[{"source":"agent","value":"тестовый факт"}]}`
	rec := doJSON(t, h, http.MethodPost, "/v1/admin/relations", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	defer doJSON(t, h, http.MethodDelete, "/v1/admin/relations",
		`{"from_id":"prod-pdf","to_id":"prod-backup","type":"adjacent_task"}`)

	rec = doJSON(t, h, http.MethodGet, "/v1/admin/relations/review?status=llm_suggested", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("queue: %d %s", rec.Code, rec.Body.String())
	}
	var queue struct {
		Items  []reviewItem   `json:"items"`
		Counts map[string]int `json:"counts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &queue); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range queue.Items {
		if item.FromID == "prod-pdf" && item.ToID == "prod-backup" {
			found = true
			if item.Reason != "тестовая причина" || len(item.Evidence) != 1 || item.FromTitle == "" {
				t.Fatalf("item = %+v", item)
			}
		}
	}
	if !found {
		t.Fatalf("relation not in queue: %+v", queue)
	}
	if queue.Counts["llm_suggested"] < 1 {
		t.Fatalf("counts = %+v", queue.Counts)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/admin/relations/review",
		`{"decision":"confirm","note":"проверено","items":[{"from_id":"prod-pdf","to_id":"prod-backup","type":"adjacent_task"}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("decision: %d %s", rec.Code, rec.Body.String())
	}
	var status, reviewedBy, note string
	if err := dbc.QueryRow(
		`SELECT status, reviewed_by, review_note FROM product_relations WHERE from_id='prod-pdf' AND to_id='prod-backup' AND type='adjacent_task'`).
		Scan(&status, &reviewedBy, &note); err != nil {
		t.Fatal(err)
	}
	if status != "confirmed" || reviewedBy != "admin" || note != "проверено" {
		t.Fatalf("status=%s reviewedBy=%s note=%s", status, reviewedBy, note)
	}
}

func TestReviewQueueRejectBulk(t *testing.T) {
	h, dbc := setupAPI(t)
	for _, id := range []string{"prod-pdf", "prod-bi"} {
		body := `{"from_id":"` + id + `","to_id":"prod-backup","type":"adjacent_task","status":"quarantine","confidence":0.4}`
		if rec := doJSON(t, h, http.MethodPost, "/v1/admin/relations", body); rec.Code != http.StatusOK {
			t.Fatalf("create %s: %d %s", id, rec.Code, rec.Body.String())
		}
	}
	defer func() {
		for _, id := range []string{"prod-pdf", "prod-bi"} {
			doJSON(t, h, http.MethodDelete, "/v1/admin/relations", `{"from_id":"`+id+`","to_id":"prod-backup","type":"adjacent_task"}`)
		}
	}()

	rec := doJSON(t, h, http.MethodPost, "/v1/admin/relations/review",
		`{"decision":"reject","note":"не подходит","items":[{"from_id":"prod-pdf","to_id":"prod-backup","type":"adjacent_task"},{"from_id":"prod-bi","to_id":"prod-backup","type":"adjacent_task"}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("decision: %d %s", rec.Code, rec.Body.String())
	}
	var n int
	if err := dbc.QueryRow(
		`SELECT count(*) FROM product_relations WHERE to_id='prod-backup' AND type='adjacent_task' AND status='rejected' AND review_note='не подходит'`).
		Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("rejected = %d, want 2", n)
	}
}

func TestAdminPageHasRegistryWorkbench(t *testing.T) {
	h, _ := setupAPI(t)
	rec := doJSON(t, h, http.MethodGet, "/admin", "")
	body := rec.Body.String()
	for _, needle := range []string{
		`data-tab="registry"`, `id="tab-registry"`, `id="registry-metrics"`,
		`id="registry-detail"`, `id="registry-generate"`, `id="registry-import"`, `id="registry-import-modal"`, `id="registry-reset"`, `id="registry-reset-modal"`, `function loadRegistry`,
	} {
		if !strings.Contains(body, needle) {
			t.Errorf("admin page missing %s", needle)
		}
	}
}
