package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/basili4-1982/lk-advisor/internal/profile"
	"github.com/basili4-1982/lk-advisor/internal/store"
)

func TestLKCrossSellAdapter(t *testing.T) {
	h, dbc := setupAPI(t)
	if err := store.SeedLKCase(context.Background(), dbc); err != nil {
		t.Fatal(err)
	}
	if err := profile.RefreshAll(context.Background(), dbc); err != nil {
		t.Fatal(err)
	}
	rec := doJSON(t, h, http.MethodPost, "/api/cross_sell/list", `{"client_id":"lk-36612"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out lkCrossSellResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Status != "shown" || len(out.Data) == 0 || len(out.Data) > 3 {
		t.Fatalf("response = %+v", out)
	}
	first := out.Data[0]
	if first.Name == "" || first.VendorName == "" || first.Reason == "" || first.ProductID == "" {
		t.Fatalf("item = %+v", first)
	}
	if first.Price < 0 || first.PriceWithDiscount < 0 {
		t.Fatalf("item prices = %+v", first)
	}
}

func TestLKCrossSellUnknownClient(t *testing.T) {
	h, _ := setupAPI(t)
	rec := doJSON(t, h, http.MethodPost, "/api/cross_sell/list", `{"client_id":"missing"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
}
