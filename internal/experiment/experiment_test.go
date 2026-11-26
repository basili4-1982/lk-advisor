package experiment_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/basili4-1982/lk-advisor/internal/experiment"
	"github.com/basili4-1982/lk-advisor/internal/store"
)

func TestExperimentLifecycle(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	if err := store.Migrate(ctx, dbc); err != nil {
		t.Fatal(err)
	}
	if _, err := dbc.ExecContext(ctx, `
		INSERT INTO products (id, sku, title, category, type, price, status)
		VALUES ('prod-pdf', 'exp-pdf', 'PDF Toolkit', 'office', 'license', 10000, 'active'),
		       ('prod-office-suite', 'exp-office', 'Office', 'office', 'license', 16000, 'active')
		ON CONFLICT (id) DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	dbc.ExecContext(ctx, `DELETE FROM experiments WHERE id IN ('exp-pdf-b2b','exp-coreldraw-bad')`)
	if err := experiment.SeedDemo(ctx, dbc); err != nil {
		t.Fatal(err)
	}
	defer dbc.ExecContext(ctx, `DELETE FROM experiments WHERE id IN ('exp-pdf-b2b','exp-coreldraw-bad')`)

	list, err := experiment.List(ctx, dbc)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != "exp-pdf-b2b" {
		t.Fatalf("list = %+v", list)
	}
	if list[0].Strategy != "hybrid" {
		t.Fatalf("strategy = %q", list[0].Strategy)
	}
	runningPopular, err := experiment.LoadRunning(ctx, dbc, "b2b", "", "popular")
	if err != nil {
		t.Fatal(err)
	}
	if len(runningPopular) != 0 {
		t.Fatalf("popular must not match hybrid experiment: %+v", runningPopular)
	}
	runningHybrid, err := experiment.LoadRunning(ctx, dbc, "b2b", "", "hybrid")
	if err != nil {
		t.Fatal(err)
	}
	if len(runningHybrid) != 1 {
		t.Fatalf("hybrid experiment missing: %+v", runningHybrid)
	}

	var treatment experiment.Assignment
	for i := 0; i < 100; i++ {
		a, err := experiment.Assign(ctx, dbc, list[0], fmt.Sprintf("exp-client-%d", i))
		if err != nil {
			t.Fatal(err)
		}
		if a.Treatment {
			treatment = a
			break
		}
	}
	if treatment.ExperimentID == "" {
		t.Fatal("no treatment assignment")
	}
	again, err := experiment.Assign(ctx, dbc, list[0], treatment.ClientID)
	if err != nil {
		t.Fatal(err)
	}
	if again.Variant != treatment.Variant {
		t.Fatalf("assignment changed: %s -> %s", treatment.Variant, again.Variant)
	}

	if err := experiment.RecordExposure(ctx, dbc, treatment, "tr-exp", "prod-pdf", 1, 0, false); err != nil {
		t.Fatal(err)
	}
	if _, err := dbc.ExecContext(ctx, `UPDATE products SET cost = 5000 WHERE id = 'prod-pdf'`); err != nil {
		t.Fatal(err)
	}
	if err := experiment.RecordEvent(ctx, dbc, "tr-exp", "prod-pdf", "clicked"); err != nil {
		t.Fatal(err)
	}
	if err := experiment.RecordEvent(ctx, dbc, "tr-exp", "prod-pdf", "purchased"); err != nil {
		t.Fatal(err)
	}

	results, err := experiment.Results(ctx, dbc, "exp-pdf-b2b")
	if err != nil {
		t.Fatal(err)
	}
	var got experiment.Result
	for _, r := range results {
		if r.Variant == "treatment" {
			got = r
		}
	}
	if got.Exposures != 1 || got.Clicks != 1 || got.Purchases != 1 || got.Conversion != 1 {
		t.Fatalf("results = %+v", results)
	}
	if got.RenewalShown != 1 || got.RenewalRate != 1 {
		t.Fatalf("guardrails = %+v", got)
	}
	if !got.MarginAvailable || got.Margin != 5000 || got.Revenue != 10000 {
		t.Fatalf("margin = %+v", got)
	}

	outcome, err := experiment.Decide(ctx, dbc, "exp-pdf-b2b", "promote", "test")
	if err != nil {
		t.Fatal(err)
	}
	if !outcome.Auto || outcome.Decision != "extend" {
		t.Fatalf("expected guardrail extend, got %+v", outcome)
	}
	if _, err := experiment.Decide(ctx, dbc, "exp-pdf-b2b", "reject", "test"); err != nil {
		t.Fatal(err)
	}
	knowledge, err := experiment.SearchKnowledge(ctx, dbc, "PDF", "reject")
	if err != nil {
		t.Fatal(err)
	}
	if len(knowledge) != 1 || knowledge[0].ID != "exp-pdf-b2b" {
		t.Fatalf("knowledge = %+v", knowledge)
	}
}

func TestGuardrailCheck(t *testing.T) {
	ok := experiment.CheckGuardrails([]experiment.Result{
		{Variant: "control", Exposures: 200, RenewalRate: 0.80, UpsellRate: 0.20},
		{Variant: "treatment", Exposures: 200, RenewalRate: 0.78, UpsellRate: 0.20},
	})
	if ok.Blocked {
		t.Fatalf("healthy guardrails blocked: %+v", ok)
	}
	bad := experiment.CheckGuardrails([]experiment.Result{
		{Variant: "control", Exposures: 200, RenewalRate: 0.80, UpsellRate: 0.20},
		{Variant: "treatment", Exposures: 200, RenewalRate: 0.65, UpsellRate: 0.20},
	})
	if !bad.Blocked || bad.Suggested != "reject" {
		t.Fatalf("severe renewal drop should reject: %+v", bad)
	}
	futility := experiment.CheckGuardrails([]experiment.Result{
		{Variant: "control", Exposures: 200, Conversion: 0.30, RenewalRate: 0.80},
		{Variant: "treatment", Exposures: 200, Conversion: 0.20, RenewalRate: 0.80},
	})
	if !futility.Blocked || futility.Suggested != "reject" || !futility.Futility {
		t.Fatalf("futility guardrail = %+v", futility)
	}
	margin := experiment.CheckGuardrails([]experiment.Result{
		{Variant: "control", Exposures: 200, RenewalRate: 0.80, MarginAvailable: true, MarginPerExposure: 100},
		{Variant: "treatment", Exposures: 200, RenewalRate: 0.80, MarginAvailable: true, MarginPerExposure: 70},
	})
	if !margin.Blocked || margin.Suggested != "reject" || margin.MarginDrop < 0.29 {
		t.Fatalf("margin guardrail = %+v", margin)
	}
}
