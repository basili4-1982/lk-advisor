package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/basili4-1982/lk-advisor/internal/agent"
)

func postTo(t *testing.T, h http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeAsk(t *testing.T, rec *httptest.ResponseRecorder) agent.AskResponse {
	t.Helper()
	var out agent.AskResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode %s: %v", rec.Body.String(), err)
	}
	return out
}

func TestAskOffTopic(t *testing.T) {
	h, _ := setupAPI(t)
	rec := postTo(t, h, "/v1/ask", `{"client_id":"smb-001","question":"Какая завтра погода?"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	out := decodeAsk(t, rec)
	if !strings.Contains(out.Answer, "Я отвечаю только") {
		t.Fatalf("answer = %q", out.Answer)
	}
	if len(out.ToolCalls) != 0 {
		t.Fatalf("tool calls = %+v", out.ToolCalls)
	}
}

func TestAskWhy(t *testing.T) {
	h, _ := setupAPI(t)
	rec := postTo(t, h, "/v1/ask", `{"client_id":"smb-002","question":"Почему Microsoft Visio Professional 2024?"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	out := decodeAsk(t, rec)
	if out.Intent != "why" {
		t.Fatalf("intent = %q", out.Intent)
	}
	if len(out.ToolCalls) < 2 {
		t.Fatalf("tool calls = %+v", out.ToolCalls)
	}
	if !strings.Contains(out.Answer, "Microsoft Visio Professional 2024") {
		t.Fatalf("answer = %q", out.Answer)
	}
	if len(out.Evidence) == 0 {
		t.Fatal("no evidence")
	}
}

func TestAskAlternatives(t *testing.T) {
	h, _ := setupAPI(t)
	rec := postTo(t, h, "/v1/ask", `{"client_id":"smb-001","question":"Почему не Kaspersky Standard?"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	out := decodeAsk(t, rec)
	if out.Intent != "alternatives" {
		t.Fatalf("intent = %q", out.Intent)
	}
	if !strings.Contains(out.Answer, "замена") && !strings.Contains(out.Answer, "исключён") {
		t.Fatalf("answer = %q", out.Answer)
	}
}

func TestAskWhatIf(t *testing.T) {
	h, _ := setupAPI(t)
	rec := postTo(t, h, "/v1/ask", `{"client_id":"smb-001","question":"Что будет, если не продлить?"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	out := decodeAsk(t, rec)
	if out.Intent != "what_if" {
		t.Fatalf("intent = %q", out.Intent)
	}
	if !strings.Contains(out.Answer, "Лицензия истекает") || len(out.Evidence) == 0 {
		t.Fatalf("answer = %q evidence = %+v", out.Answer, out.Evidence)
	}
}

func TestAskSessionClientMismatch(t *testing.T) {
	h, _ := setupAPI(t)
	first := decodeAsk(t, postTo(t, h, "/v1/ask", `{"client_id":"smb-001","question":"Почему антивирус?"}`))
	if first.SessionID == "" {
		t.Fatal("no session id")
	}
	body := `{"client_id":"smb-002","session_id":"` + first.SessionID + `","question":"Почему векторный?"}`
	rec := postTo(t, h, "/v1/ask", body)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestAskMemory(t *testing.T) {
	h, _ := setupAPI(t)
	first := decodeAsk(t, postTo(t, h, "/v1/ask", `{"client_id":"smb-002","question":"Почему векторный?"}`))
	body := `{"client_id":"smb-002","session_id":"` + first.SessionID + `","question":"А что ещё?"}`
	rec := postTo(t, h, "/v1/ask", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	out := decodeAsk(t, rec)
	if !strings.Contains(out.Answer, "Microsoft Visio Professional 2024") {
		t.Fatalf("answer = %q", out.Answer)
	}
}

func TestAskValidation(t *testing.T) {
	h, _ := setupAPI(t)
	cases := []struct {
		body string
		want int
	}{
		{`{}`, http.StatusBadRequest},
		{`{"client_id":"smb-001"}`, http.StatusBadRequest},
		{`{"client_id":"smb-999","question":"Почему?"}`, http.StatusNotFound},
	}
	for _, tc := range cases {
		rec := postTo(t, h, "/v1/ask", tc.body)
		if rec.Code != tc.want {
			t.Fatalf("body %s: status = %d, want %d", tc.body, rec.Code, tc.want)
		}
	}
}
