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
	if err := experiment.SeedDemo(ctx, dbc); err != nil {
		t.Fatal(err)
	}
	defer dbc.ExecContext(ctx, `DELETE FROM experiments WHERE id = 'exp-pdf-b2b'`)

	list, err := experiment.List(ctx, dbc)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != "exp-pdf-b2b" {
		t.Fatalf("list = %+v", list)
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

	ok, err := experiment.Decide(ctx, dbc, "exp-pdf-b2b", "promote", "test")
	if err != nil || !ok {
		t.Fatalf("decide ok=%v err=%v", ok, err)
	}
	list, err = experiment.List(ctx, dbc)
	if err != nil {
		t.Fatal(err)
	}
	if list[0].Status != "completed" || list[0].Decision != "promote" {
		t.Fatalf("experiment = %+v", list[0])
	}
}
