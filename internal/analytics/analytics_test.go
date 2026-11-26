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
	if err := analytics.Log(ctx, dbc, "tr-test-1", "smb-001", "upsell", false, shown, skipped); err != nil {
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
	if err := analytics.Log(ctx, dbc, "tr-owner", "smb-001", "none", false, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := analytics.Record(ctx, dbc, "tr-owner", "smb-002", "prod-a", "clicked"); err != analytics.ErrClientMismatch {
		t.Fatalf("err = %v", err)
	}
}
