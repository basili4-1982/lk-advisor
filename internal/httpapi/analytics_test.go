package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestRecommendationEventsAndAnalytics(t *testing.T) {
	h, _ := setupAPI(t)
	rec := post(t, h, `{"client_id":"smb-001"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("recommendations: %d %s", rec.Code, rec.Body.String())
	}
	var rr recommendationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &rr); err != nil {
		t.Fatal(err)
	}
	if rr.TraceID == "" || len(rr.Recommendations) == 0 {
		t.Fatalf("response = %+v", rr)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/events",
		`{"trace_id":"`+rr.TraceID+`","client_id":"smb-001","product_id":"`+rr.Recommendations[0].ProductID+`","event":"clicked"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("event: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/admin/analytics?days=7", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("analytics: %d %s", rec.Code, rec.Body.String())
	}
	var report analyticsReport
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Requests < 1 || report.Shown < 3 || report.Clicks < 1 {
		t.Fatalf("report = %+v", report)
	}
	if len(report.TopProducts) == 0 || len(report.SkipReasons) == 0 {
		t.Fatalf("report = %+v", report)
	}
	if len(report.ByStrategy) == 0 || report.ByStrategy[0].Strategy == "" {
		t.Fatalf("by strategy = %+v", report.ByStrategy)
	}
}

// analyticsReport mirrors the JSON payload without importing the package.
type analyticsReport struct {
	Requests    int `json:"requests"`
	Shown       int `json:"shown"`
	Skipped     int `json:"skipped"`
	Clicks      int `json:"clicks"`
	Purchases   int `json:"purchases"`
	TopProducts []struct {
		ProductID string `json:"product_id"`
	} `json:"top_products"`
	SkipReasons []struct {
		Reason string `json:"reason"`
	} `json:"skip_reasons"`
	ByStrategy []struct {
		Strategy string `json:"strategy"`
		Audience string `json:"audience"`
	} `json:"by_strategy"`
}

func TestEventValidation(t *testing.T) {
	h, _ := setupAPI(t)
	rec := doJSON(t, h, http.MethodPost, "/v1/events",
		`{"trace_id":"missing","client_id":"smb-001","product_id":"prod-a","event":"clicked"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/events",
		`{"trace_id":"x","client_id":"smb-001","product_id":"prod-a","event":"shown"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
}
