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

func TestVendorCandidatesFromExternalEditions(t *testing.T) {
	h, dbc := setupAPI(t)
	ctx := context.Background()
	cleanup := func() {
		dbc.ExecContext(ctx, `DELETE FROM relations
			WHERE source = 'vendor' AND evidence->0->'details'->>'vendor' = 'TEST-VC'`)
		dbc.ExecContext(ctx, `DELETE FROM vendor_products WHERE vendor = 'TEST-VC'`)
		dbc.ExecContext(ctx, `DELETE FROM lk_items WHERE item_id LIKE '000-VC-%'`)
	}
	cleanup()
	t.Cleanup(cleanup)

	if _, err := dbc.ExecContext(ctx, `
		INSERT INTO lk_items (item_id, title, category) VALUES
		('000-VC-1', 'Тестовый продукт Старт', 'office'),
		('000-VC-2', 'Тестовый продукт Бизнес', 'office')`); err != nil {
		t.Fatal(err)
	}
	if _, err := dbc.ExecContext(ctx, `
		INSERT INTO vendor_products (vendor, product, family, edition, platforms, aliases) VALUES
		('TEST-VC', 'Тестовый продукт', 'Тестовая линейка', 'Старт', '["Web"]', '["Тестовый продукт Старт"]'),
		('TEST-VC', 'Тестовый продукт', 'Тестовая линейка', 'Бизнес', '["Web"]', '["Тестовый продукт Бизнес"]')`); err != nil {
		t.Fatal(err)
	}
	if _, err := dbc.ExecContext(ctx, `
		INSERT INTO vendor_product_items (vendor_product_id, item_id, method, confidence)
		SELECT id, CASE edition WHEN 'Старт' THEN '000-VC-1' ELSE '000-VC-2' END, 'test', 0.9
		FROM vendor_products WHERE vendor = 'TEST-VC'`); err != nil {
		t.Fatal(err)
	}

	rec := doJSON(t, h, http.MethodPost, "/v1/admin/vendor-products/candidates", `{"limit":200}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("candidates: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Created int `json:"created"`
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Created < 1 {
		t.Fatalf("created = %d, want at least 1", out.Created)
	}
	var typ, source string
	if err := dbc.QueryRowContext(ctx, `
		SELECT type, source FROM relations
		WHERE from_item_id = '000-VC-1' AND to_item_id = '000-VC-2'`).Scan(&typ, &source); err != nil {
		t.Fatal(err)
	}
	if typ != "variant_of" || source != "vendor" {
		t.Fatalf("relation = %s / %s", typ, source)
	}
}

func TestRegistryXMLImport(t *testing.T) {
	h, dbc := setupAPI(t)
	ctx := context.Background()
	cleanup := func() {
		dbc.ExecContext(ctx, `DELETE FROM vendor_products WHERE vendor = 'ООО Тест'`)
	}
	cleanup()
	t.Cleanup(cleanup)

	xmlBody := `<?xml version="1.0" encoding="utf-8"?>
<registry>
  <program>
    <НомерРеестровойЗаписи>12345</НомерРеестровойЗаписи>
    <НаименованиеПрограммы>Тестовый Реестровый Продукт</НаименованиеПрограммы>
    <Правообладатель>ООО Тест</Правообладатель>
    <КлассПО>Офисное ПО</КлассПО>
    <Ссылка>https://example.test/registry/12345</Ссылка>
  </program>
</registry>`

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "registry.xml")
	if err != nil {
		t.Fatal(err)
	}
	part.Write([]byte(xmlBody))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/vendor-products/import-registry", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("import: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Added int `json:"added"`
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Added != 1 {
		t.Fatalf("added = %d, want 1", out.Added)
	}
	var number, url string
	if err := dbc.QueryRowContext(ctx, `
		SELECT registry_number, source_url FROM vendor_products
		WHERE vendor = 'ООО Тест' AND product = 'Тестовый Реестровый Продукт'`).Scan(&number, &url); err != nil {
		t.Fatal(err)
	}
	if number != "12345" || url != "https://example.test/registry/12345" {
		t.Fatalf("row = %s / %s", number, url)
	}
}
