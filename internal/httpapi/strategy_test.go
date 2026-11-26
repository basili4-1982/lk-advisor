package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/basili4-1982/lk-advisor/internal/analytics"
)

func TestAdminStrategiesAndAutoToggle(t *testing.T) {
	h, _ := setupAPI(t)
	doJSON(t, h, http.MethodPut, "/v1/admin/strategies/auto", `{"enabled":false}`)
	defer doJSON(t, h, http.MethodPut, "/v1/admin/strategies/auto", `{"enabled":false}`)

	rec := doJSON(t, h, http.MethodGet, "/v1/admin/strategies", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Auto  bool `json:"auto"`
		Stats []struct {
			Audience string `json:"audience"`
			Strategy string `json:"strategy"`
		} `json:"stats"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Auto || len(out.Stats) < 2 {
		t.Fatalf("out = %+v", out)
	}
	rec = doJSON(t, h, http.MethodPut, "/v1/admin/strategies/auto", `{"enabled":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("auto: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/admin/strategies", "")
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if !out.Auto {
		t.Fatal("auto strategy did not turn on")
	}
}

func TestAdminWeightsByAudience(t *testing.T) {
	h, _ := setupAPI(t)
	body := `{"audience":"b2b","weights":{"relevance":0.4,"urgency":0.2,"affinity":0.2,"margin":0.1,"confidence":0.1}}`
	rec := doJSON(t, h, http.MethodPut, "/v1/admin/weights", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	defer doJSON(t, h, http.MethodPut, "/v1/admin/weights",
		`{"audience":"b2b","weights":{"relevance":0.35,"urgency":0.25,"affinity":0.2,"margin":0.1,"confidence":0.1}}`)

	rec = doJSON(t, h, http.MethodGet, "/v1/admin/state", "")
	var st adminState
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if st.WeightsByAudience["b2b"].Relevance != 0.4 {
		t.Fatalf("weights = %+v", st.WeightsByAudience)
	}
}

func TestRecommendationUsesAudienceStrategy(t *testing.T) {
	h, _ := setupAPI(t)
	doJSON(t, h, http.MethodPut, "/v1/admin/strategies/auto", `{"enabled":false}`)
	defer doJSON(t, h, http.MethodPut, "/v1/admin/strategies/auto", `{"enabled":false}`)

	rec := post(t, h, `{"client_id":"smb-002","audience":"b2c"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out recommendationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Strategy != "popular" {
		t.Fatalf("strategy = %q, want popular", out.Strategy)
	}
	rec = post(t, h, `{"client_id":"smb-002"}`)
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Strategy != "hybrid" {
		t.Fatalf("strategy = %q, want hybrid", out.Strategy)
	}
}

func TestAnalyticsHealthEndpoint(t *testing.T) {
	h, dbc := setupAPI(t)
	shown := make([]analytics.Item, 0, 60)
	for i := 0; i < 60; i++ {
		shown = append(shown, analytics.Item{ProductID: "prod-dead", Type: "cross_sell", Source: "engine"})
	}
	if err := analytics.Log(context.Background(), dbc, "tr-health-http", "smb-001", "cross_sell",
		analytics.Meta{Strategy: "hybrid", Audience: "b2b", WeightsVersion: "v1"}, false, shown, nil); err != nil {
		t.Fatal(err)
	}
	rec := doJSON(t, h, http.MethodGet, "/v1/admin/analytics/health?days=7&min_shown=50", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out analytics.Health
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.LowClick) == 0 || out.LowClick[0].ProductID != "prod-dead" {
		t.Fatalf("health = %+v", out)
	}
}

func TestAutoReviewMarksRelations(t *testing.T) {
	h, dbc := setupAPI(t)
	shown := make([]analytics.Item, 0, 60)
	for i := 0; i < 60; i++ {
		shown = append(shown, analytics.Item{ProductID: "prod-vector", Type: "upsell", Source: "engine"})
	}
	if err := analytics.Log(context.Background(), dbc, "tr-auto-review", "smb-001", "upsell",
		analytics.Meta{Strategy: "hybrid", Audience: "b2b", WeightsVersion: "v1"}, false, shown, nil); err != nil {
		t.Fatal(err)
	}
	rec := doJSON(t, h, http.MethodPost, "/v1/admin/analytics/auto-review", `{}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var note string
	if err := dbc.QueryRow(`SELECT max(review_note) FROM product_relations WHERE to_id='prod-vector' AND status='confirmed'`).Scan(&note); err != nil {
		t.Fatal(err)
	}
	if note == "" {
		t.Fatal("relation was not flagged")
	}
}
