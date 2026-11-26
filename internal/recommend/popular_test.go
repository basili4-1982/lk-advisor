package recommend_test

import (
	"testing"

	"github.com/basili4-1982/lk-advisor/internal/model"
	"github.com/basili4-1982/lk-advisor/internal/recommend"
)

func TestPopularCandidatesFromDemand(t *testing.T) {
	products := []model.Product{
		prod("prod-a", 1000, 4, "active"),
		prod("prod-b", 2000, 4, "active"),
	}
	in := input(profile(nil, nil), products, nil, nil, nil)
	in.Popularity = map[string]float64{"prod-a": 1.0, "prod-b": 0.5}

	cands := recommend.Candidates(in)
	if len(cands) == 0 || cands[0].Product.ID != "prod-a" {
		t.Fatalf("candidates = %+v, want prod-a first", cands)
	}
	found := false
	for _, e := range cands[0].Evidence {
		if e.Source == "demand" {
			found = true
		}
	}
	if !found {
		t.Fatalf("popular candidate has no demand evidence: %+v", cands[0])
	}
}

func TestPopularCandidatesSkipOwned(t *testing.T) {
	products := []model.Product{
		prod("prod-a", 1000, 4, "active"),
		prod("prod-b", 2000, 4, "active"),
	}
	in := input(profile(nil, []model.Purchase{purchase("prod-a", 10)}), products, nil, nil, nil)
	in.Popularity = map[string]float64{"prod-a": 10.0, "prod-b": 1.0}

	cands := recommend.Candidates(in)
	for _, c := range cands {
		if c.Product.ID == "prod-a" {
			t.Fatalf("owned product must not be recommended: %+v", c)
		}
	}
	if len(cands) == 0 || cands[0].Product.ID != "prod-b" {
		t.Fatalf("candidates = %+v, want prod-b", cands)
	}
}
