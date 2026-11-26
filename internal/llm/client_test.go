package llm_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/basili4-1982/lk-advisor/internal/llm"
)

func TestComplete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("auth = %s", r.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "hello") {
			t.Errorf("body = %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"  ответ  "}}]}`))
	}))
	defer srv.Close()

	c := llm.New(srv.URL, "test-key", "model")
	got, err := c.Complete(context.Background(), "system", "hello")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if got != "ответ" {
		t.Fatalf("got = %q", got)
	}
}

func TestNoKey(t *testing.T) {
	c := llm.New("http://example.invalid", "", "model")
	if _, err := c.Complete(context.Background(), "s", "u"); err != llm.ErrNoKey {
		t.Fatalf("err = %v, want ErrNoKey", err)
	}
}

func TestAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"bad key"}`))
	}))
	defer srv.Close()

	c := llm.New(srv.URL, "key", "model")
	if _, err := c.Complete(context.Background(), "s", "u"); err == nil {
		t.Fatal("expected an error")
	}
}
