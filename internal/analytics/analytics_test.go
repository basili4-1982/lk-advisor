package analytics_test

import (
	"context"
	"testing"
	"time"

	"github.com/basili4-1982/lk-advisor/internal/analytics"
	"github.com/basili4-1982/lk-advisor/internal/store"
)

func TestLogAndSummary(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	if err := store.Migrate(ctx, dbc); err != nil {
		t.Fatal(err)
	}
	store.Truncate(t, dbc)

	shown := []analytics.Item{
		{ProductID: "prod-a", Type: "upsell", Reason: "top_3", Source: "engine", Rank: 1, Score: 0.9},
		{ProductID: "prod-b", Type: "cross_sell", Reason: "top_3", Source: "engine", Rank: 2, Score: 0.8},
	}
	skipped := []analytics.Item{
		{ProductID: "prod-c", Reason: "rule:already_owned", Detail: "уже куплено"},
		{ProductID: "prod-d", Type: "cross_sell", Reason: "below_top_3", Source: "engine", Score: 0.7},
	}
	if err := analytics.Log(ctx, dbc, "tr-test-1", "smb-001", "upsell", analytics.Meta{Strategy: "hybrid", Audience: "b2b", WeightsVersion: "v1"}, false, shown, skipped); err != nil {
		t.Fatal(err)
	}
	if err := analytics.Record(ctx, dbc, "tr-test-1", "smb-001", "prod-a", "clicked"); err != nil {
		t.Fatal(err)
	}
	if err := analytics.Record(ctx, dbc, "tr-test-1", "smb-001", "prod-a", "purchased"); err != nil {
		t.Fatal(err)
	}

	rep, err := analytics.Build(ctx, dbc, 7, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Requests != 1 || rep.Shown != 2 || rep.Skipped != 2 || rep.Clicks != 1 || rep.Purchases != 1 {
		t.Fatalf("report = %+v", rep)
	}
	if rep.CTR != 0.5 || rep.Conversion != 0.5 {
		t.Fatalf("rates = %+v", rep)
	}
	if len(rep.TopProducts) == 0 || rep.TopProducts[0].ProductID != "prod-a" {
		t.Fatalf("products = %+v", rep.TopProducts)
	}
	if len(rep.SkipReasons) == 0 {
		t.Fatalf("reasons = %+v", rep.SkipReasons)
	}
	if len(rep.ByStrategy) == 0 || rep.ByStrategy[0].Strategy != "hybrid" || rep.ByStrategy[0].Audience != "b2b" {
		t.Fatalf("by strategy = %+v", rep.ByStrategy)
	}
	if len(rep.ByAudience) == 0 || rep.ByAudience[0].Audience != "b2b" {
		t.Fatalf("by audience = %+v", rep.ByAudience)
	}
}

func TestRecordValidation(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	if err := store.Migrate(ctx, dbc); err != nil {
		t.Fatal(err)
	}
	store.Truncate(t, dbc)
	if err := analytics.Record(ctx, dbc, "missing", "smb-001", "prod-a", "clicked"); err != analytics.ErrTraceNotFound {
		t.Fatalf("err = %v", err)
	}
	if err := analytics.Log(ctx, dbc, "tr-owner", "smb-001", "none", analytics.Meta{Strategy: "hybrid", Audience: "b2b", WeightsVersion: "v1"}, false, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := analytics.Record(ctx, dbc, "tr-owner", "smb-002", "prod-a", "clicked"); err != analytics.ErrClientMismatch {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildHealth(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	if err := store.Migrate(ctx, dbc); err != nil {
		t.Fatal(err)
	}
	store.Truncate(t, dbc)

	shown := make([]analytics.Item, 0, 60)
	for i := 0; i < 60; i++ {
		shown = append(shown, analytics.Item{ProductID: "prod-dead", Type: "cross_sell", Reason: "top_3", Source: "engine"})
	}
	if err := analytics.Log(ctx, dbc, "tr-health", "smb-001", "cross_sell",
		analytics.Meta{Strategy: "hybrid", Audience: "b2b", WeightsVersion: "v1"}, false, shown, nil); err != nil {
		t.Fatal(err)
	}
	h, err := analytics.BuildHealth(ctx, dbc, 7, 50, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(h.LowClick) != 1 || h.LowClick[0].ProductID != "prod-dead" || h.LowClick[0].Shown != 60 {
		t.Fatalf("health = %+v", h)
	}
	if len(h.Types) != 1 || h.Types[0].Type != "cross_sell" {
		t.Fatalf("types = %+v", h.Types)
	}
}

func TestRecordStoresPosition(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	if err := store.Migrate(ctx, dbc); err != nil {
		t.Fatal(err)
	}
	store.Truncate(t, dbc)
	shown := []analytics.Item{{ProductID: "prod-a", Type: "cross_sell", Source: "engine", Rank: 2}}
	if err := analytics.Log(ctx, dbc, "tr-rank", "smb-001", "cross_sell",
		analytics.Meta{Strategy: "hybrid", Audience: "b2b", WeightsVersion: "v1"}, false, shown, nil); err != nil {
		t.Fatal(err)
	}
	if err := analytics.Record(ctx, dbc, "tr-rank", "smb-001", "prod-a", "clicked"); err != nil {
		t.Fatal(err)
	}
	var rank int
	if err := dbc.QueryRow(`SELECT rank FROM recommendation_events WHERE trace_id='tr-rank' AND event='clicked'`).Scan(&rank); err != nil {
		t.Fatal(err)
	}
	if rank != 2 {
		t.Fatalf("rank = %d, want 2", rank)
	}
}
