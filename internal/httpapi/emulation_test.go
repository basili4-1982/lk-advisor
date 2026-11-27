package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestEmulationEndpoints(t *testing.T) {
	h, dbc := setupAPI(t)
	ctx := context.Background()
	cleanup := func() {
		dbc.ExecContext(ctx, `DELETE FROM experiments WHERE id LIKE 'emu-http-%'`)
		dbc.ExecContext(ctx, `DELETE FROM client_profiles WHERE id LIKE 'prof-http-%'`)
		dbc.ExecContext(ctx, `DELETE FROM lk_orders WHERE item_id LIKE 'emu-http-%'`)
		dbc.ExecContext(ctx, `DELETE FROM lk_items WHERE item_id LIKE 'emu-http-%'`)
	}
	cleanup()
	t.Cleanup(cleanup)

	mustExec := func(query string, args ...any) {
		t.Helper()
		if _, err := dbc.ExecContext(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	mustExec(`INSERT INTO lk_items (item_id, title, category, product_group) VALUES
		('emu-http-src', 'Офисный пакет', 'office', 'HTTP-OFFICE'),
		('emu-http-target', 'Корпоративная почта Workspace', 'other', 'HTTP-WS'),
		('emu-http-control', 'Курс по продажам', 'training', 'HTTP-COURSE')`)
	mustExec(`INSERT INTO lk_orders (order_id, client_id, order_week, item_id) VALUES
		('emu-http-o1', 'emu-http-c1', '2026-01-01', 'emu-http-control'),
		('emu-http-o2', 'emu-http-c2', '2026-01-01', 'emu-http-control')`)
	mustExec(`INSERT INTO experiments
		(id, title, hypothesis, status, audience, industry, strategy, metric, guardrails,
		 variants, traffic_split, from_id, to_id, relation_type, treatment, started_at)
		VALUES ('emu-http-exp', 'Тест', 'Тест', 'running', 'b2b', '', 'hybrid', 'conversion',
		        'renewal, upsell, fallback',
		        '[{"name":"control","weight":0.5},{"name":"treatment","weight":0.5}]'::jsonb,
		        0.5, 'emu-http-src', 'emu-http-target', 'upgrade', 'force_relation', now())`)

	rec := doJSON(t, h, http.MethodGet, "/v1/admin/client-profiles", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("profiles: %d %s", rec.Code, rec.Body.String())
	}
	var profiles struct {
		Items      []struct{ ID, Name, Audience string } `json:"items"`
		Industries []string                              `json:"industries"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &profiles); err != nil {
		t.Fatal(err)
	}
	foundClinic := false
	for _, p := range profiles.Items {
		if p.ID == "prof-clinic" {
			foundClinic = true
		}
	}
	if !foundClinic || len(profiles.Industries) == 0 {
		t.Fatalf("seed profiles missing: %+v", profiles)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/admin/client-profiles",
		`{"name":"Тестовый профиль","audience":"b2b","industry":"Образование и наука",
		  "description":"Школе нужны офисные программы и корпоративная почта"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("create profile: %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	json.Unmarshal(rec.Body.Bytes(), &created)
	if created.ID == "" {
		t.Fatal("no profile id")
	}
	t.Cleanup(func() { dbc.ExecContext(ctx, `DELETE FROM client_profiles WHERE id = $1`, created.ID) })

	rec = doJSON(t, h, http.MethodPost, "/v1/admin/experiments/emu-http-exp/emulate",
		`{"profile_id":"prof-clinic","impressions":200,"seed":9}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("emulate: %d %s", rec.Code, rec.Body.String())
	}
	var summary struct {
		Fit       float64 `json:"fit"`
		Treatment struct {
			Exposures  int     `json:"exposures"`
			Conversion float64 `json:"conversion"`
		} `json:"treatment"`
		Matched []string `json:"matched"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Fit <= 0 || summary.Treatment.Exposures == 0 || len(summary.Matched) == 0 {
		t.Fatalf("summary = %+v", summary)
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/admin/experiments", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("experiments: %d %s", rec.Code, rec.Body.String())
	}
	var list struct {
		Items []struct {
			ID        string `json:"id"`
			Emulation *struct {
				ProfileName string  `json:"profile_name"`
				Fit         float64 `json:"fit"`
			} `json:"emulation"`
		} `json:"items"`
	}
	json.Unmarshal(rec.Body.Bytes(), &list)
	found := false
	for _, x := range list.Items {
		if x.ID == "emu-http-exp" && x.Emulation != nil && x.Emulation.ProfileName != "" {
			found = true
		}
	}
	if !found {
		t.Fatalf("experiment has no emulation info: %+v", list.Items)
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/admin/experiments/emu-http-exp/emulations", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("emulations: %d %s", rec.Code, rec.Body.String())
	}
	var runs struct {
		Items []json.RawMessage `json:"items"`
	}
	json.Unmarshal(rec.Body.Bytes(), &runs)
	if len(runs.Items) != 1 {
		t.Fatalf("runs = %d, want 1", len(runs.Items))
	}
}

func TestEmulateRejectsBadInput(t *testing.T) {
	h, _ := setupAPI(t)
	rec := doJSON(t, h, http.MethodPost, "/v1/admin/experiments/nope/emulate",
		`{"profile_id":"prof-clinic","impressions":50}`)
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 400/404", rec.Code)
	}
}
