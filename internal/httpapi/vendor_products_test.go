package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVendorProductsImportAndLink(t *testing.T) {
	h, dbc := setupAPI(t)
	ctx := context.Background()
	cleanup := func() {
		dbc.ExecContext(ctx, `DELETE FROM vendor_products WHERE vendor = 'TEST-VENDOR'`)
		dbc.ExecContext(ctx, `DELETE FROM lk_items WHERE item_id LIKE 'TEST-VP-%'`)
	}
	cleanup()
	t.Cleanup(cleanup)

	if _, err := dbc.ExecContext(ctx, `
		INSERT INTO lk_items (item_id, title, category)
		VALUES ('TEST-VP-1', 'Тестовый Продукт Лицензия Старт', 'office')`); err != nil {
		t.Fatal(err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "vendors.csv")
	if err != nil {
		t.Fatal(err)
	}
	part.Write([]byte("vendor,product,family,edition,platforms,registry_number,source_url,aliases\n" +
		"TEST-VENDOR,Тестовый Продукт,Тестовая линейка,Старт,Windows|Linux,,https://example.test,Тестовый Продукт Лицензия\n"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/vendor-products/import", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("import: %d %s", rec.Code, rec.Body.String())
	}
	var imported struct {
		Added int `json:"added"`
	}
	json.Unmarshal(rec.Body.Bytes(), &imported)
	if imported.Added != 1 {
		t.Fatalf("import = %+v", imported)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/admin/vendor-products/link", `{}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("link: %d %s", rec.Code, rec.Body.String())
	}
	var linked struct {
		Mappings int `json:"mappings"`
		Items    int `json:"items"`
	}
	json.Unmarshal(rec.Body.Bytes(), &linked)
	if linked.Mappings < 1 || linked.Items < 1 {
		t.Fatalf("link = %+v", linked)
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/admin/vendor-products", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	var list struct {
		Stats struct {
			Products     int `json:"products"`
			CoveredItems int `json:"covered_items"`
		} `json:"stats"`
	}
	json.Unmarshal(rec.Body.Bytes(), &list)
	if list.Stats.Products < 1 || list.Stats.CoveredItems < 1 {
		t.Fatalf("stats = %+v", list.Stats)
	}
}
