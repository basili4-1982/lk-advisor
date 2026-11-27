package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestAgentDemoReturnsEvidence(t *testing.T) {
	h, dbc := setupAPI(t)
	ctx := context.Background()
	cleanup := func() {
		dbc.ExecContext(ctx, `DELETE FROM relations WHERE from_item_id LIKE 'agent-test-%'`)
		dbc.ExecContext(ctx, `DELETE FROM lk_items WHERE item_id LIKE 'agent-test-%'`)
	}
	cleanup()
	t.Cleanup(cleanup)
	if _, err := dbc.ExecContext(ctx, `
		INSERT INTO lk_items (item_id, title) VALUES
		('agent-test-a', 'Товар A'), ('agent-test-b', 'Товар B')`); err != nil {
		t.Fatal(err)
	}
	if _, err := dbc.ExecContext(ctx, `
		INSERT INTO relations (from_item_id, to_item_id, type, confidence, status, reason, evidence)
		VALUES ('agent-test-a', 'agent-test-b', 'upgrade', 0.88, 'candidate', 'тестовая связь',
		        '[{"kind":"vendor_doc","summary":"Документация","details":{"url":"https://example.test/doc"}}]'::jsonb)`); err != nil {
		t.Fatal(err)
	}
	rec := doJSON(t, h, http.MethodGet, "/v1/agent/demo?from=agent-test-a&to=agent-test-b", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Type     string `json:"type"`
		Evidence []struct {
			Kind string `json:"kind"`
			URL  string `json:"url"`
		} `json:"evidence"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Type != "upgrade" || len(out.Evidence) != 1 || out.Evidence[0].Kind != "vendor_doc" {
		t.Fatalf("demo = %+v", out)
	}
}

func TestAgentDemoUnknownRelation(t *testing.T) {
	h, _ := setupAPI(t)
	rec := doJSON(t, h, http.MethodGet, "/v1/agent/demo?from=nope&to=nope2", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
