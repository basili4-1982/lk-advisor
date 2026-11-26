package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientProfile(t *testing.T) {
	h, _ := setupAPI(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/clients/smb-001/profile", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out profileResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.ClientID != "smb-001" {
		t.Fatalf("client = %q", out.ClientID)
	}
	if len(out.Licenses) == 0 || out.Licenses[0].Title == "" {
		t.Fatalf("licenses = %+v", out.Licenses)
	}
	if len(out.Purchases) == 0 {
		t.Fatal("no purchases")
	}
}

func TestClientProfileNotFound(t *testing.T) {
	h, _ := setupAPI(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/clients/smb-999/profile", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
}
