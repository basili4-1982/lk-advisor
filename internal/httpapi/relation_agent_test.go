package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/basili4-1982/lk-advisor/internal/llm"
)

func TestRelationAgentLLM(t *testing.T) {
	t.Setenv("RELATION_AUTO_CONFIRM", "true")
	t.Setenv("RELATION_AUTO_CONFIRM_THRESHOLD", "0.85")
	_, dbc := setupAPI(t)
	srv := fakeLLM(t, `{"type":"adjacent_task","confidence":0.86,"reason":"Оба продукта закрывают соседние задачи.","evidence":["категории различаются, но задачи рядом"]}`)
	h := handlerWithLLM(dbc, llm.New(srv.URL, "test-key", "test-model"))

	rec := doJSON(t, h, http.MethodPost, "/v1/admin/relations/agent/propose",
		`{"from_id":"prod-pdf","to_id":"prod-backup"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var p relationAgentProposal
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.Type != "adjacent_task" || p.Source != "llm" || p.Status != "confirmed" {
		t.Fatalf("proposal = %+v", p)
	}
	if len(p.Evidence) == 0 {
		t.Fatal("no evidence")
	}
}

func TestRelationAgentLLMBelowAutoConfirmGoesToReview(t *testing.T) {
	t.Setenv("RELATION_AUTO_CONFIRM", "true")
	t.Setenv("RELATION_AUTO_CONFIRM_THRESHOLD", "0.85")
	_, dbc := setupAPI(t)
	srv := fakeLLM(t, `{"type":"adjacent_task","confidence":0.80,"reason":"Соседние задачи.","evidence":["категория"]}`)
	h := handlerWithLLM(dbc, llm.New(srv.URL, "test-key", "test-model"))

	rec := doJSON(t, h, http.MethodPost, "/v1/admin/relations/agent/propose",
		`{"from_id":"prod-pdf","to_id":"prod-backup"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var p relationAgentProposal
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.Status != "llm_suggested" {
		t.Fatalf("proposal = %+v", p)
	}
}

func TestRelationAgentUnknownTypeFallsBack(t *testing.T) {
	_, dbc := setupAPI(t)
	srv := fakeLLM(t, `{"type":"invented_type","confidence":0.9,"reason":"x","evidence":[]}`)
	h := handlerWithLLM(dbc, llm.New(srv.URL, "test-key", "test-model"))

	rec := doJSON(t, h, http.MethodPost, "/v1/admin/relations/agent/propose",
		`{"from_id":"prod-sl-20","to_id":"prod-sl-23"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var p relationAgentProposal
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.Source != "fallback" || p.Type != "same_task" || p.Status != "quarantine" {
		t.Fatalf("proposal = %+v", p)
	}
}

func TestRelationAgentHeuristicWithoutLLM(t *testing.T) {
	h, _ := setupAPI(t)
	rec := doJSON(t, h, http.MethodPost, "/v1/admin/relations/agent/propose",
		`{"from_id":"prod-sl-20","to_id":"prod-sl-23"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var p relationAgentProposal
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.Source != "fallback" || p.Type != "same_task" || p.Confidence <= 0 {
		t.Fatalf("proposal = %+v", p)
	}
}

func TestRelationAgentNone(t *testing.T) {
	_, dbc := setupAPI(t)
	srv := fakeLLM(t, `{"type":"none","confidence":0.2,"reason":"нет оснований","evidence":[]}`)
	h := handlerWithLLM(dbc, llm.New(srv.URL, "test-key", "test-model"))
	rec := doJSON(t, h, http.MethodPost, "/v1/admin/relations/agent/propose",
		`{"from_id":"prod-pdf","to_id":"prod-sl-20"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var p relationAgentProposal
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.Type != "none" || p.Status != "none" {
		t.Fatalf("proposal = %+v", p)
	}
}
