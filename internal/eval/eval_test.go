package eval_test

import (
	"context"
	"testing"
	"time"

	"github.com/basili4-1982/lk-advisor/internal/eval"
	"github.com/basili4-1982/lk-advisor/internal/profile"
	"github.com/basili4-1982/lk-advisor/internal/store"
)

func TestEvalOnDataset(t *testing.T) {
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

	cases, err := eval.LoadGroundTruth(ctx, dbc)
	if err != nil {
		t.Fatalf("ground truth: %v", err)
	}
	now := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	report, err := eval.Run(ctx, dbc, cases, now)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}

	if report.Total != 50 {
		t.Fatalf("total = %d, want 50", report.Total)
	}
	if !report.Passed {
		t.Fatalf("eval did not pass: %+v", report)
	}
	if report.Violations != 0 {
		t.Fatalf("violations = %d", report.Violations)
	}
}
