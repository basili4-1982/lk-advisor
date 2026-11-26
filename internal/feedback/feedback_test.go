package feedback_test

import (
	"context"
	"testing"
	"time"

	"github.com/basili4-1982/lk-advisor/internal/analytics"
	"github.com/basili4-1982/lk-advisor/internal/feedback"
	"github.com/basili4-1982/lk-advisor/internal/model"
	"github.com/basili4-1982/lk-advisor/internal/store"
)

func TestFeedbackBoostAndPenalty(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	if err := store.Migrate(ctx, dbc); err != nil {
		t.Fatal(err)
	}
	store.Truncate(t, dbc)

	shown := make([]analytics.Item, 0, 40)
	for i := 0; i < 20; i++ {
		shown = append(shown, analytics.Item{ProductID: "prod-good", Type: "cross_sell", Source: "engine", Rank: 1})
		shown = append(shown, analytics.Item{ProductID: "prod-bad", Type: "cross_sell", Source: "engine", Rank: 2})
	}
	if err := analytics.Log(ctx, dbc, "tr-feedback", "smb-001", "cross_sell",
		analytics.Meta{Strategy: "hybrid", Audience: "b2b", WeightsVersion: "v1"}, false, shown, nil); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if err := analytics.Record(ctx, dbc, "tr-feedback", "smb-001", "prod-good", "clicked"); err != nil {
			t.Fatal(err)
		}
	}
	stats, err := feedback.Load(ctx, dbc, []string{"prod-good", "prod-bad"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if stats["prod-good"].Multiplier <= 1 {
		t.Fatalf("good multiplier = %v", stats["prod-good"].Multiplier)
	}
	if stats["prod-bad"].Multiplier >= 1 {
		t.Fatalf("bad multiplier = %v", stats["prod-bad"].Multiplier)
	}
	cands := []model.Candidate{
		{Product: model.Product{ID: "prod-good"}, Score: 0.5},
		{Product: model.Product{ID: "prod-bad"}, Score: 0.5},
	}
	feedback.Apply(cands, stats)
	if cands[0].Score <= cands[1].Score {
		t.Fatalf("feedback did not reorder: %+v", cands)
	}
}
