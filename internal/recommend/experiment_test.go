package recommend_test

import (
	"testing"

	"github.com/basili4-1982/lk-advisor/internal/model"
	"github.com/basili4-1982/lk-advisor/internal/recommend"
)

func TestExperimentTreatmentForcesTarget(t *testing.T) {
	products := []model.Product{
		prod("prod-a", 1000, 4, "active"),
		prod("prod-b", 2000, 4, "active"),
	}
	in := input(profile(nil, nil), products, nil, nil, nil)
	in.Experiment = &recommend.ExperimentTreatment{
		ID: "exp", Variant: "treatment", TargetProductID: "prod-b",
		FromID: "prod-a", RelationType: "adjacent_task",
	}
	cands, _ := recommend.Run(in)
	found := false
	for _, c := range cands {
		if c.Product.ID == "prod-b" {
			found = true
			if len(c.Evidence) == 0 || c.Evidence[0].Source != "experiment" {
				t.Fatalf("missing experiment evidence: %+v", c)
			}
		}
	}
	if !found {
		t.Fatalf("target missing from %+v", cands)
	}
}
