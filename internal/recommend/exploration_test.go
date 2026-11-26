package recommend_test

import (
	"testing"

	"github.com/basili4-1982/lk-advisor/internal/model"
	"github.com/basili4-1982/lk-advisor/internal/recommend"
)

func TestExplorationSlot(t *testing.T) {
	a := model.Candidate{Product: model.Product{ID: "a"}}
	b := model.Candidate{Product: model.Product{ID: "b"}}
	c := model.Candidate{Product: model.Product{ID: "c"}}
	d := model.Candidate{Product: model.Product{ID: "d"}}
	got, ok := recommend.ExplorationSlot([]model.Candidate{a, b, c, d}, []model.Candidate{a, b, c}, 0.05)
	if !ok || got.Product.ID != "d" || !got.Exploration {
		t.Fatalf("got=%+v ok=%v", got, ok)
	}
	if _, ok := recommend.ExplorationSlot([]model.Candidate{a, b, c, d}, []model.Candidate{a, b, c}, 0.5); ok {
		t.Fatal("exploration fired outside budget")
	}
}
