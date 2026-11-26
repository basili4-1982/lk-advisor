package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestExperimentEndpoints(t *testing.T) {
	h, dbc := setupAPI(t)
	t.Cleanup(func() { dbc.Exec(`DELETE FROM experiments WHERE id = 'exp-pdf-b2b'`) })

	rec := doJSON(t, h, http.MethodPost, "/v1/admin/experiments/demo", `{}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("seed: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/admin/experiments", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	var list struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) == 0 || list.Items[0].ID != "exp-pdf-b2b" {
		t.Fatalf("items = %+v", list.Items)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/admin/experiments/exp-pdf-b2b/decision",
		`{"decision":"promote","note":"test"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("decision: %d %s", rec.Code, rec.Body.String())
	}
}
