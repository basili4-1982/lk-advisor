package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/basili4-1982/lk-advisor/internal/llm"
)

func TestDemoAudit(t *testing.T) {
	h, dbc := setupAPI(t)
	if _, err := dbc.Exec(`DELETE FROM demo_audit WHERE client_id = 'smb-001'`); err != nil {
		t.Fatalf("clear audit: %v", err)
	}

	add := doJSON(t, h, http.MethodPost, "/v1/demo/clients/smb-001/items", `{"product_id":"prod-backup"}`)
	if add.Code != http.StatusOK {
		t.Fatalf("add status = %d", add.Code)
	}
	del := doJSON(t, h, http.MethodDelete, "/v1/demo/clients/smb-001/items/prod-backup", "")
	if del.Code != http.StatusOK {
		t.Fatalf("remove status = %d", del.Code)
	}

	rec := doJSON(t, h, http.MethodGet, "/v1/demo/clients/smb-001/audit", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("audit status = %d", rec.Code)
	}
	var list []auditEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("audit entries = %d, want 2: %+v", len(list), list)
	}
	if list[0].Action != "remove" || list[1].Action != "add" {
		t.Fatalf("actions = %q, %q", list[0].Action, list[1].Action)
	}
	if list[1].Title == "" {
		t.Fatal("empty product title in audit")
	}
}

func TestAgentCallLogged(t *testing.T) {
	h, _ := setupAPI(t)

	rec := postTo(t, h, "/v1/ask", `{"client_id":"smb-002","question":"Почему Microsoft Visio Professional 2024?"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("ask status = %d, body = %s", rec.Code, rec.Body.String())
	}

	calls := doJSON(t, h, http.MethodGet, "/v1/demo/clients/smb-002/agent-calls", "")
	if calls.Code != http.StatusOK {
		t.Fatalf("agent calls status = %d", calls.Code)
	}
	var list []agentCallEntry
	if err := json.Unmarshal(calls.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list) == 0 {
		t.Fatal("no agent calls logged")
	}
	c := list[0]
	if c.Intent != "why" || c.FinalAnswer == "" || len(c.Report) == 0 {
		t.Fatalf("bad call: %+v", c)
	}
}

func TestAgentCallGuardLogged(t *testing.T) {
	_, dbc := setupAPI(t)
	srv := fakeLLM(t, "Скидка 40%.")
	h := handlerWithLLM(dbc, llm.New(srv.URL, "test-key", "test-model"))

	rec := postTo(t, h, "/v1/ask", `{"client_id":"smb-002","question":"Почему Microsoft Visio Professional 2024?"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("ask status = %d", rec.Code)
	}

	calls := doJSON(t, h, http.MethodGet, "/v1/demo/clients/smb-002/agent-calls", "")
	var list []agentCallEntry
	if err := json.Unmarshal(calls.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list) == 0 {
		t.Fatal("no agent calls logged")
	}
	if list[0].GuardError == "" || !list[0].Fallback {
		t.Fatalf("guard not logged: %+v", list[0])
	}
}
