package httpapi

import (
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/basili4-1982/lk-advisor/internal/llm"
)

func fakeLLM(t *testing.T, answer string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": answer}},
			},
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func handlerWithLLM(dbc *sql.DB, client *llm.Client) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewWithLLM(logger, dbc, func() time.Time { return apiNow }, client)
}

func TestAskUsesLLM(t *testing.T) {
	_, dbc := setupAPI(t)
	srv := fakeLLM(t, "Предлагаем Microsoft Visio Professional 2024, потому что это соседняя задача.")

	h := handlerWithLLM(dbc, llm.New(srv.URL, "test-key", "test-model"))
	rec := postTo(t, h, "/v1/ask", `{"client_id":"smb-002","question":"Почему векторный?"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	out := decodeAsk(t, rec)
	if !strings.Contains(out.Answer, "соседняя задача") {
		t.Fatalf("answer = %q", out.Answer)
	}
	if out.Fallback {
		t.Fatal("unexpected fallback")
	}
}

func TestAskLLMGuardFallsBack(t *testing.T) {
	_, dbc := setupAPI(t)
	srv := fakeLLM(t, "Скидка 40%.")

	h := handlerWithLLM(dbc, llm.New(srv.URL, "test-key", "test-model"))
	rec := postTo(t, h, "/v1/ask", `{"client_id":"smb-002","question":"Почему векторный?"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	out := decodeAsk(t, rec)
	if strings.Contains(out.Answer, "40") {
		t.Fatalf("ungrounded number leaked: %q", out.Answer)
	}
	if !strings.Contains(out.Answer, "Microsoft Visio Professional 2024") {
		t.Fatalf("expected the template answer, got %q", out.Answer)
	}
	if !out.Fallback {
		t.Fatal("expected fallback=true")
	}
}
