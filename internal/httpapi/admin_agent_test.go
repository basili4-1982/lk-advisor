package httpapi

import (
	"testing"

	"github.com/basili4-1982/lk-advisor/internal/model"
)

func demoState() adminState {
	return adminState{
		Products: []adminProduct{
			{ID: "prod-sl-86", Title: "Kaiten"},
			{ID: "prod-backup", Title: "Кибер Бэкап"},
			{ID: "prod-sl-74", Title: "GStarCAD"},
			{ID: "prod-sl-20", Title: "КОМПАС-3D V25"},
		},
		Rules: []adminRule{
			{Rule: model.Rule{ID: "rule-01", Type: "max_discount", Condition: "discount>10"}, Summary: "Ограничение скидки: не больше 10%"},
		},
		Weights: model.DefaultWeights(),
		RelationTypes: []model.RelationType{
			{Code: "industry_addon", Title: "Отраслевое дополнение", Behavior: "cross_sell", Direction: "forward", DefaultConfidence: 0.85, Status: "active"},
		},
	}
}

func TestParseProposalRelation(t *testing.T) {
	p := parseProposal("Сделай Кибер Бэкап дополнением к Kaiten", demoState())
	if len(p.Operations) != 1 {
		t.Fatalf("ops = %+v", p.Operations)
	}
	op := p.Operations[0]
	if op.Action != "add_relation" || op.FromID != "prod-sl-86" || op.ToID != "prod-backup" || op.Type != "adjacent_task" {
		t.Fatalf("op = %+v", op)
	}
}

func TestParseProposalSubstitute(t *testing.T) {
	p := parseProposal("Сделай Kaiten заменой GStarCAD", demoState())
	if len(p.Operations) != 1 {
		t.Fatalf("ops = %+v", p.Operations)
	}
	op := p.Operations[0]
	if op.FromID != "prod-sl-74" || op.ToID != "prod-sl-86" || op.Type != "substitute" {
		t.Fatalf("op = %+v", op)
	}
}

func TestParseProposalHideProduct(t *testing.T) {
	p := parseProposal("Убери GStarCAD из рекомендаций", demoState())
	if len(p.Operations) != 1 || p.Operations[0].Action != "add_rule" ||
		p.Operations[0].RuleType != "out_of_stock" || p.Operations[0].ProductID != "prod-sl-74" {
		t.Fatalf("ops = %+v", p.Operations)
	}
}

func TestParseProposalDiscount(t *testing.T) {
	p := parseProposal("Ограничь скидку 5%", demoState())
	if len(p.Operations) != 1 || p.Operations[0].Discount != 5 {
		t.Fatalf("ops = %+v", p.Operations)
	}
}

func TestParseProposalWeights(t *testing.T) {
	p := parseProposal("Релевантность 40, срочность 20, аффинность 20, маржа 10, уверенность 10", demoState())
	if len(p.Operations) != 1 || p.Operations[0].Weights == nil {
		t.Fatalf("ops = %+v", p.Operations)
	}
	w := p.Operations[0].Weights
	if w.Relevance < 0.39 || w.Relevance > 0.41 || w.Urgency < 0.19 || w.Urgency > 0.21 {
		t.Fatalf("weights = %+v", w)
	}
}

func TestUnrequestedDeleteIsDropped(t *testing.T) {
	ops := withoutUnrequestedDeletes("Ограничь скидку 5%", []adminOperation{
		{Action: "add_rule", RuleType: "max_discount", Discount: 5},
		{Action: "delete_rule", RuleID: "rule-01"},
	})
	if len(ops) != 1 || ops[0].Action != "add_rule" {
		t.Fatalf("ops = %+v", ops)
	}
}

func TestParseProposalCustomType(t *testing.T) {
	p := parseProposal("Сделай Кибер Бэкап отраслевым дополнением к Kaiten", demoState())
	if len(p.Operations) != 1 {
		t.Fatalf("ops = %+v", p.Operations)
	}
	op := p.Operations[0]
	if op.Action != "add_relation" || op.Type != "industry_addon" || op.FromID != "prod-sl-86" || op.ToID != "prod-backup" {
		t.Fatalf("op = %+v", op)
	}
}
