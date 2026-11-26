package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func doJSON(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func TestProducts(t *testing.T) {
	h, _ := setupAPI(t)
	rec := doJSON(t, h, http.MethodGet, "/v1/products", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var list []productItem
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list) != 200 {
		t.Fatalf("products = %d, want 200", len(list))
	}
}

func TestAddRemoveItem(t *testing.T) {
	h, _ := setupAPI(t)

	rec := doJSON(t, h, http.MethodPost, "/v1/demo/clients/smb-001/items", `{"product_id":"prod-backup"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("add status = %d, body = %s", rec.Code, rec.Body.String())
	}

	profile := doJSON(t, h, http.MethodGet, "/v1/clients/smb-001/profile", "")
	var p profileResponse
	if err := json.Unmarshal(profile.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode profile: %v", err)
	}
	found := false
	for _, l := range p.Licenses {
		if l.ProductID == "prod-backup" {
			found = true
		}
	}
	if !found {
		t.Fatalf("backup license not in profile: %+v", p.Licenses)
	}

	dup := doJSON(t, h, http.MethodPost, "/v1/demo/clients/smb-001/items", `{"product_id":"prod-backup"}`)
	if dup.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d, want 409", dup.Code)
	}

	del := doJSON(t, h, http.MethodDelete, "/v1/demo/clients/smb-001/items/prod-backup", "")
	if del.Code != http.StatusOK {
		t.Fatalf("remove status = %d", del.Code)
	}

	profile = doJSON(t, h, http.MethodGet, "/v1/clients/smb-001/profile", "")
	p = profileResponse{}
	if err := json.Unmarshal(profile.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode profile: %v", err)
	}
	for _, l := range p.Licenses {
		if l.ProductID == "prod-backup" {
			t.Fatalf("backup still in profile")
		}
	}
}

func TestAddUnknownProduct(t *testing.T) {
	h, _ := setupAPI(t)
	rec := doJSON(t, h, http.MethodPost, "/v1/demo/clients/smb-001/items", `{"product_id":"prod-nope"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
