package recommend_test

import (
	"math"
	"testing"

	"github.com/basili4-1982/lk-advisor/internal/model"
	"github.com/basili4-1982/lk-advisor/internal/recommend"
)

func TestScoreWithWeights(t *testing.T) {
	c := model.Candidate{Relevance: 1, Urgency: 0, Affinity: 0, Margin: 0, Confidence: 0}
	got := recommend.ScoreWith(c, at, model.Weights{Relevance: 1})
	if math.Abs(got.Score-1) > 0.001 {
		t.Fatalf("score = %v, want 1", got.Score)
	}
}

func TestScoreFormula(t *testing.T) {
	c := model.Candidate{Relevance: 1, Urgency: 0.7, Affinity: 0.6, Margin: 0.5, Confidence: 0.8}
	got := recommend.Score(c, at)
	want := 0.35 + 0.25*0.7 + 0.20*0.6 + 0.10*0.5 + 0.10*0.8
	if math.Abs(got.Score-want) > 0.006 {
		t.Fatalf("score = %v, want %v", got.Score, want)
	}
}

func TestPromoUrgencyCap(t *testing.T) {
	c := model.Candidate{
		Urgency: 0.95,
		Promo:   &model.Promotion{ValidTo: at.AddDate(0, 0, 5)},
	}
	got := recommend.Score(c, at)
	if got.Urgency != 1.0 {
		t.Fatalf("urgency = %v, want 1.0", got.Urgency)
	}
}

func TestTieBreakPromoFirst(t *testing.T) {
	a := model.Candidate{Product: prod("prod-a", 1000, 4, "active"), Score: 0.8, Urgency: 0.5}
	b := model.Candidate{
		Product: prod("prod-b", 1000, 4, "active"), Score: 0.8, Urgency: 0.5,
		Promo: &model.Promotion{ValidTo: at.AddDate(0, 0, 5)},
	}
	cands := []model.Candidate{a, b}
	recommend.Sort(cands)
	if cands[0].Product.ID != "prod-b" {
		t.Fatalf("first = %s, want prod-b", cands[0].Product.ID)
	}
}

func TestDiversifyOnePerType(t *testing.T) {
	cands := []model.Candidate{
		{Product: prod("p1", 1000, 4, "active"), Type: "cross_sell"},
		{Product: prod("p2", 1000, 4, "active"), Type: "cross_sell"},
		{Product: prod("p3", 1000, 4, "active"), Type: "renewal"},
		{Product: prod("p4", 1000, 4, "active"), Type: "upsell"},
	}
	out := recommend.Diversify(cands, 3)
	if len(out) != 3 {
		t.Fatalf("len = %d, want 3", len(out))
	}
	types := map[string]int{}
	for _, c := range out {
		types[c.Type]++
	}
	if types["cross_sell"] != 1 {
		t.Fatalf("cross_sell count = %d, want 1", types["cross_sell"])
	}
}

func TestRunCrossSell(t *testing.T) {
	in := input(profile(nil, []model.Purchase{purchase("prod-raster", 10)}),
		[]model.Product{prod("prod-raster", 22000, 4, "active"), prod("prod-vector", 20000, 4, "active")},
		[]model.Relation{{FromID: "prod-raster", ToID: "prod-vector", Type: "adjacent_task", Status: "confirmed", Confidence: 0.9}},
		nil, nil)

	got, rejected := recommend.Run(in)
	if len(rejected) != 0 {
		t.Fatalf("rejected = %+v", rejected)
	}
	if len(got) != 1 || got[0].Product.ID != "prod-vector" || got[0].Score <= 0 {
		t.Fatalf("got = %+v", got)
	}
}

func TestDiversifyByTask(t *testing.T) {
	a := prod("p1", 1000, 4, "active")
	a.Category = "graphics"
	a.Subcategory = "diagramming"
	b := prod("p2", 1000, 4, "active")
	b.Category = "graphics"
	b.Subcategory = "project_management"
	c := prod("p3", 1000, 4, "active")
	c.Category = "office"
	c.Subcategory = "office_suite"

	out := recommend.Diversify([]model.Candidate{
		{Product: a, Type: "cross_sell"},
		{Product: b, Type: "cross_sell"},
		{Product: c, Type: "cross_sell"},
	}, 3)
	if len(out) != 3 {
		t.Fatalf("len = %d, want 3", len(out))
	}
}

func TestStudentGetsTraining(t *testing.T) {
	office := prod("prod-office-suite", 16551, 4, "active")
	office.Category = "office"
	office.Subcategory = "office_suite"
	office.Audience = "consumer"
	pdf := prod("prod-pdf", 10000, 4, "active")
	pdf.Category = "office"
	pdf.Subcategory = "pdf"
	pdf.Audience = "any"
	course := prod("prod-admin-course", 8000, 4, "active")
	course.Category = "training"
	course.Subcategory = "training"
	course.Audience = "any"

	p := profile(nil, []model.Purchase{purchase("prod-office-suite", 0)})
	rels := []model.Relation{
		{FromID: "prod-office-suite", ToID: "prod-pdf", Type: "adjacent_task", Status: "confirmed", Confidence: 0.9},
		{FromID: "prod-office-suite", ToID: "prod-admin-course", Type: "adjacent_task", Status: "confirmed", Confidence: 0.9},
	}
	in := input(p, []model.Product{office, pdf, course}, rels, nil, nil)
	got, _ := recommend.Run(in)

	if len(got) == 0 || got[0].Product.ID != "prod-admin-course" {
		t.Fatalf("top = %+v, want training course", got)
	}
}
