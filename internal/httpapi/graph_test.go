package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestDemoGraphEndpoints(t *testing.T) {
	h, _ := setupAPI(t)
	rec := doJSON(t, h, http.MethodGet, "/v1/demo/graph?audience=b2b", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("graph: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Audience string `json:"audience"`
		Nodes    []any  `json:"nodes"`
		Edges    []any  `json:"edges"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Audience != "b2b" {
		t.Fatalf("audience = %q", out.Audience)
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/demo/graph?audience=b2b&period=2026-08", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("period graph: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodGet, "/demo/graph", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("page: %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "graph") {
		t.Fatal("graph page does not look like the demo page")
	}
}
