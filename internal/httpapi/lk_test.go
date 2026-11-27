package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestStorefrontShowsColdStartClients(t *testing.T) {
	h, dbc := setupAPI(t)
	ctx := context.Background()
	cleanup := func() {
		dbc.ExecContext(ctx, `DELETE FROM lk_orders WHERE order_id LIKE 'lk-cold-%'`)
		dbc.ExecContext(ctx, `DELETE FROM lk_items WHERE item_id LIKE 'lk-cold-%'`)
		dbc.ExecContext(ctx, `DELETE FROM lk_clients WHERE client_id LIKE 'lk-cold-%'`)
	}
	cleanup()
	t.Cleanup(cleanup)

	mustExec := func(q string, args ...any) {
		t.Helper()
		if _, err := dbc.ExecContext(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	mustExec(`INSERT INTO lk_clients (client_id, client_type, industry) VALUES
		('lk-cold-none', 'ЮЛ', 'Тестовая отрасль'),
		('lk-cold-warm', 'ЮЛ', 'Тестовая отрасль')`)
	mustExec(`INSERT INTO lk_items (item_id, title, category, available)
		VALUES ('lk-cold-item', 'Популярный товар', 'office', true)`)
	mustExec(`INSERT INTO lk_orders (order_id, client_id, order_week, item_id)
		VALUES ('lk-cold-o1', 'lk-cold-warm', '2026-01-01', 'lk-cold-item')`)

	rec := doJSON(t, h, http.MethodGet, "/v1/lk/clients", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("clients: %d %s", rec.Code, rec.Body.String())
	}
	var list struct {
		Items []struct {
			ClientID  string `json:"client_id"`
			Orders    int    `json:"orders"`
			ColdStart bool   `json:"cold_start"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range list.Items {
		if c.ClientID == "lk-cold-none" {
			found = true
			if !c.ColdStart || c.Orders != 0 {
				t.Fatalf("cold client = %+v", c)
			}
		}
	}
	if !found {
		t.Fatalf("cold-start client missing: %+v", list.Items)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/recommendations",
		`{"client_id":"lk-cold-none","limit":3}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("recommendations: %d %s", rec.Code, rec.Body.String())
	}
	var block struct {
		ColdStart       bool `json:"cold_start"`
		Recommendations []struct {
			ProductID string `json:"product_id"`
		} `json:"recommendations"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &block); err != nil {
		t.Fatal(err)
	}
	if !block.ColdStart || len(block.Recommendations) == 0 {
		t.Fatalf("cold-start block = %+v", block)
	}
}
