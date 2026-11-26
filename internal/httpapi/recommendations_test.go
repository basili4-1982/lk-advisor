package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/basili4-1982/lk-advisor/internal/profile"
	"github.com/basili4-1982/lk-advisor/internal/store"
)

var apiNow = time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)

func setupAPI(t *testing.T) (http.Handler, *sql.DB) {
	t.Helper()
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	if err := store.Migrate(ctx, dbc); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	store.Truncate(t, dbc)
	ds, err := store.Generate(42)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if err := store.Load(ctx, dbc, ds); err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := profile.RefreshAll(ctx, dbc); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewWithClock(logger, dbc, func() time.Time { return apiNow }), dbc
}

func post(t *testing.T, h http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/recommendations", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) recommendationResponse {
	t.Helper()
	var out recommendationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode %s: %v", rec.Body.String(), err)
	}
	return out
}

func TestRecommendationsRenewal(t *testing.T) {
	h, _ := setupAPI(t)
	rec := post(t, h, `{"client_id":"smb-001"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	out := decode(t, rec)
	if out.Intent != "renewal" {
		t.Fatalf("intent = %q", out.Intent)
	}
	if len(out.Recommendations) == 0 {
		t.Fatal("no recommendations")
	}
	first := out.Recommendations[0]
	if first.ProductID != "prod-antivirus" {
		t.Fatalf("first = %+v", first)
	}
	if !strings.HasPrefix(first.Reason, "Продлите") {
		t.Fatalf("reason = %q", first.Reason)
	}
	if first.Price.Currency != "RUB" || len(first.Evidence) == 0 {
		t.Fatalf("bad first: %+v", first)
	}
}

func TestRecommendationsUpsellWithPromo(t *testing.T) {
	h, _ := setupAPI(t)
	rec := post(t, h, `{"client_id":"smb-002"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	out := decode(t, rec)
	if out.Intent != "upsell" {
		t.Fatalf("intent = %q", out.Intent)
	}
	first := out.Recommendations[0]
	if first.ProductID != "prod-vector" {
		t.Fatalf("first = %+v", first)
	}
	if first.Promo == nil {
		t.Fatal("expected an active promo on prod-vector")
	}
}

func TestRecommendationsUpsell(t *testing.T) {
	h, _ := setupAPI(t)
	rec := post(t, h, `{"client_id":"smb-003"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	out := decode(t, rec)
	if out.Intent != "upsell" {
		t.Fatalf("intent = %q", out.Intent)
	}
	if out.Recommendations[0].ProductID != "prod-office-pack" {
		t.Fatalf("first = %+v", out.Recommendations[0])
	}
}

func TestRecommendationsManagerApproval(t *testing.T) {
	h, _ := setupAPI(t)
	rec := post(t, h, `{"client_id":"smb-005"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	out := decode(t, rec)
	if len(out.Recommendations) == 0 || out.Recommendations[0].ProductID != "prod-vector-pro" {
		t.Fatalf("recommendations = %+v", out.Recommendations)
	}
	found := false
	for _, c := range out.Recommendations[0].Constraints {
		if c == "manager_approval" {
			found = true
		}
	}
	if !found {
		t.Fatalf("constraints = %v", out.Recommendations[0].Constraints)
	}
}

func TestRecommendationsDemandColdStart(t *testing.T) {
	h, _ := setupAPI(t)
	rec := post(t, h, `{"client_id":"smb-006"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	out := decode(t, rec)
	if out.Fallback || len(out.Recommendations) == 0 {
		t.Fatalf("out = %+v", out)
	}
	found := false
	for _, e := range out.Recommendations[0].Evidence {
		if e.Source == "demand" {
			found = true
		}
	}
	if !found {
		t.Fatalf("cold-start recommendation has no demand evidence: %+v", out.Recommendations[0])
	}
}

func TestRecommendationsValidation(t *testing.T) {
	h, _ := setupAPI(t)
	cases := []struct {
		name string
		body string
		want int
	}{
		{"no client", `{}`, http.StatusBadRequest},
		{"limit zero", `{"client_id":"smb-001","limit":0}`, http.StatusBadRequest},
		{"limit six", `{"client_id":"smb-001","limit":6}`, http.StatusBadRequest},
		{"unknown event", `{"client_id":"smb-001","event":"nope"}`, http.StatusBadRequest},
		{"unknown client", `{"client_id":"smb-999"}`, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := post(t, h, tc.body)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d, body = %s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

func TestRecommendationsNoPII(t *testing.T) {
	h, _ := setupAPI(t)
	rec := post(t, h, `{"client_id":"smb-001"}`)
	if strings.Contains(rec.Body.String(), "Клиент 01") ||
		strings.Contains(rec.Body.String(), "@") {
		t.Fatalf("response contains PII: %s", rec.Body.String())
	}
}

func TestRecommendationsDeterministic(t *testing.T) {
	h, _ := setupAPI(t)
	a := decode(t, post(t, h, `{"client_id":"smb-001"}`))
	b := decode(t, post(t, h, `{"client_id":"smb-001"}`))

	ja, _ := json.Marshal(a.Recommendations)
	jb, _ := json.Marshal(b.Recommendations)
	if !bytes.Equal(ja, jb) {
		t.Fatalf("recommendations differ:\n%s\n%s", ja, jb)
	}
}
