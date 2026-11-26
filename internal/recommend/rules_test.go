package recommend_test

import (
	"testing"

	"github.com/basili4-1982/lk-advisor/internal/model"
	"github.com/basili4-1982/lk-advisor/internal/recommend"
)

func TestRulesExcludes(t *testing.T) {
	p := profile(nil, []model.Purchase{purchase("prod-antivirus", 30)})
	products := []model.Product{prod("prod-antivirus", 15000, 4, "active"), prod("prod-edr", 30000, 4, "active")}
	rels := []model.Relation{{FromID: "prod-antivirus", ToID: "prod-edr", Type: "adjacent_task", Status: "confirmed", Confidence: 0.8}}
	rules := []model.Rule{{ID: "r1", Type: "excludes", Condition: "pair=prod-antivirus:prod-edr", Action: "exclude"}}

	in := input(p, products, rels, rules, nil)
	cands := recommend.Candidates(in)
	allowed, rejected := recommend.ApplyRules(in, cands)

	if len(allowed) != 0 {
		t.Fatalf("allowed = %+v, want empty", allowed)
	}
	if len(rejected) != 1 || rejected[0].Rule != "excludes" {
		t.Fatalf("rejected = %+v", rejected)
	}
}

func TestRulesManagerOnly(t *testing.T) {
	p := profile(nil, []model.Purchase{purchase("prod-vector", 20)})
	products := []model.Product{prod("prod-vector", 20000, 4, "active"), prod("prod-vector-pro", 32000, 4, "active")}
	rels := []model.Relation{{FromID: "prod-vector", ToID: "prod-vector-pro", Type: "upgrade", Status: "confirmed", Confidence: 0.9}}
	rules := []model.Rule{{ID: "r1", Type: "manager_only", Condition: "product=prod-vector-pro", Action: "approval"}}

	in := input(p, products, rels, rules, nil)
	allowed, _ := recommend.ApplyRules(in, recommend.Candidates(in))

	if len(allowed) != 1 || len(allowed[0].Constraints) != 1 || allowed[0].Constraints[0] != "manager_approval" {
		t.Fatalf("allowed = %+v", allowed)
	}
}

func TestRulesAlreadyOwned(t *testing.T) {
	p := profile(nil, []model.Purchase{purchase("prod-antivirus", 10)})
	products := []model.Product{prod("prod-antivirus", 15000, 4, "active")}
	in := input(p, products, nil, nil, nil)

	cand := model.Candidate{Product: products[0], Type: "cross_sell"}
	allowed, rejected := recommend.ApplyRules(in, []model.Candidate{cand})

	if len(allowed) != 0 || len(rejected) != 1 || rejected[0].Rule != "already_owned" {
		t.Fatalf("allowed=%+v rejected=%+v", allowed, rejected)
	}
}

func TestRulesCompatibility(t *testing.T) {
	p := profile(nil, nil)
	p.Environment.RAMGB = 8
	products := []model.Product{prod("prod-heavy", 50000, 16, "active")}
	in := input(p, products, nil, nil, nil)

	cand := model.Candidate{Product: products[0], Type: "cross_sell"}
	allowed, rejected := recommend.ApplyRules(in, []model.Candidate{cand})

	if len(allowed) != 0 || len(rejected) != 1 || rejected[0].Rule != "compatibility" {
		t.Fatalf("allowed=%+v rejected=%+v", allowed, rejected)
	}
}

func TestRulesMaxDiscount(t *testing.T) {
	p := profile(nil, nil)
	products := []model.Product{prod("prod-x", 10000, 4, "active")}
	rules := []model.Rule{{ID: "r1", Type: "max_discount", Condition: "discount>10", Action: "cap"}}
	in := input(p, products, nil, rules, nil)

	cand := model.Candidate{
		Product: products[0], Type: "cross_sell",
		Promo: &model.Promotion{ID: "promo", ProductID: "prod-x", Discount: 20, ValidTo: at.AddDate(0, 0, 5)},
	}
	allowed, _ := recommend.ApplyRules(in, []model.Candidate{cand})

	if len(allowed) != 1 || allowed[0].Promo == nil || allowed[0].Promo.Discount != 10 {
		t.Fatalf("allowed = %+v", allowed)
	}
}

func TestRulesNoSecondSecurity(t *testing.T) {
	antivirus := prod("prod-antivirus", 15000, 4, "active")
	antivirus.Category = "security"
	edr := prod("prod-edr", 30000, 4, "active")
	edr.Category = "security"
	backup := prod("prod-backup", 18000, 4, "active")
	backup.Category = "backup"

	p := profile(nil, []model.Purchase{purchase("prod-antivirus", 10)})
	in := input(p, []model.Product{antivirus, edr, backup}, nil, nil, nil)

	cands := []model.Candidate{
		{Product: edr, Type: "cross_sell"},
		{Product: backup, Type: "cross_sell"},
	}
	allowed, rejected := recommend.ApplyRules(in, cands)

	if len(allowed) != 1 || allowed[0].Product.ID != "prod-backup" {
		t.Fatalf("allowed = %+v", allowed)
	}
	if len(rejected) != 1 || rejected[0].Rule != "already_have_security" {
		t.Fatalf("rejected = %+v", rejected)
	}
}

func TestRulesAudience(t *testing.T) {
	consumerOffice := prod("prod-office-suite", 16551, 4, "active")
	consumerOffice.Category = "office"
	consumerOffice.Audience = "consumer"
	pack := prod("prod-office-pack", 50000, 4, "active")
	pack.Category = "office"
	pack.Audience = "business"
	pdf := prod("prod-pdf", 10000, 4, "active")
	pdf.Category = "office"
	pdf.Audience = "any"

	p := profile(nil, []model.Purchase{purchase("prod-office-suite", 10)})
	in := input(p, []model.Product{consumerOffice, pack, pdf}, nil, nil, nil)
	cands := []model.Candidate{
		{Product: pack, Type: "cross_sell"},
		{Product: pdf, Type: "cross_sell"},
	}
	allowed, rejected := recommend.ApplyRules(in, cands)

	if len(allowed) != 1 || allowed[0].Product.ID != "prod-pdf" {
		t.Fatalf("allowed = %+v", allowed)
	}
	if len(rejected) != 1 || rejected[0].Rule != "audience_mismatch" {
		t.Fatalf("rejected = %+v", rejected)
	}
}
