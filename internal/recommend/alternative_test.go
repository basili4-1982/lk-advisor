package recommend_test

import (
	"testing"

	"github.com/basili4-1982/lk-advisor/internal/model"
	"github.com/basili4-1982/lk-advisor/internal/recommend"
)

func TestCheaperSubscriptionAlternative(t *testing.T) {
	owned := prod("prod-owned", 14000, 4, "active")
	owned.TermMonths = 12
	cheaper := prod("prod-cheaper", 2000, 4, "active")
	cheaper.TermMonths = 12

	in := input(profile(nil, []model.Purchase{purchase("prod-owned", 10)}),
		[]model.Product{owned, cheaper},
		[]model.Relation{{FromID: "prod-owned", ToID: "prod-cheaper", Type: "substitute", Status: "confirmed", Confidence: 0.9}},
		nil, nil)

	got, _ := recommend.Run(in)
	if len(got) != 1 {
		t.Fatalf("got = %+v, want one alternative", got)
	}
	if got[0].Type != "alternative" {
		t.Fatalf("type = %s, want alternative", got[0].Type)
	}
	if len(got[0].Evidence) == 0 || got[0].Evidence[0].Source != "price" {
		t.Fatalf("evidence = %+v, want price evidence", got[0].Evidence)
	}
}

func TestExpensiveAlternativeHiddenWithoutPromo(t *testing.T) {
	owned := prod("prod-owned", 2000, 4, "active")
	owned.TermMonths = 12
	pricey := prod("prod-pricey", 14000, 4, "active")
	pricey.TermMonths = 12

	in := input(profile(nil, []model.Purchase{purchase("prod-owned", 10)}),
		[]model.Product{owned, pricey},
		[]model.Relation{{FromID: "prod-owned", ToID: "prod-pricey", Type: "substitute", Status: "confirmed", Confidence: 0.9}},
		nil, nil)

	got, _ := recommend.Run(in)
	if len(got) != 0 {
		t.Fatalf("got = %+v, want no alternative", got)
	}
}

func TestAlternativeShownOnPromo(t *testing.T) {
	owned := prod("prod-owned", 2000, 4, "active")
	owned.TermMonths = 12
	pricey := prod("prod-pricey", 14000, 4, "active")
	pricey.TermMonths = 12

	in := input(profile(nil, []model.Purchase{purchase("prod-owned", 10)}),
		[]model.Product{owned, pricey},
		[]model.Relation{{FromID: "prod-owned", ToID: "prod-pricey", Type: "substitute", Status: "confirmed", Confidence: 0.9}},
		nil,
		[]model.Promotion{{
			ProductID: "prod-pricey", Discount: 20,
			ValidFrom: at.AddDate(0, 0, -1), ValidTo: at.AddDate(0, 0, 5),
		}})

	got, _ := recommend.Run(in)
	if len(got) != 1 || got[0].Type != "alternative" {
		t.Fatalf("got = %+v, want promo alternative", got)
	}
	if got[0].Evidence[0].Source != "promo" {
		t.Fatalf("evidence = %+v, want promo evidence", got[0].Evidence)
	}
}

func TestNoDowngradeToIncludedEdition(t *testing.T) {
	full := prod("prod-full", 20000, 4, "active")
	full.TermMonths = 12
	light := prod("prod-light", 4000, 4, "active")
	light.TermMonths = 12

	in := input(profile(nil, []model.Purchase{purchase("prod-full", 10)}),
		[]model.Product{full, light},
		[]model.Relation{{FromID: "prod-light", ToID: "prod-full", Type: "upgrade", Status: "confirmed", Confidence: 0.95}},
		nil, nil)

	got, _ := recommend.Run(in)
	if len(got) != 0 {
		t.Fatalf("got = %+v, want no downgrade", got)
	}
}

func TestCustomCrossSellBehavior(t *testing.T) {
	owned := prod("prod-owned", 1000, 4, "active")
	owned.TermMonths = 12
	addon := prod("prod-addon", 2000, 4, "active")
	addon.TermMonths = 12

	in := input(profile(nil, []model.Purchase{purchase("prod-owned", 10)}),
		[]model.Product{owned, addon},
		[]model.Relation{{FromID: "prod-owned", ToID: "prod-addon", Type: "industry_addon", Status: "confirmed", Confidence: 0.9}},
		nil, nil)
	in.RelationTypes = map[string]model.RelationType{
		"industry_addon": {Code: "industry_addon", Title: "Отраслевое дополнение", Behavior: "cross_sell", Status: "active"},
	}
	got, _ := recommend.Run(in)
	if len(got) != 1 || got[0].Type != "cross_sell" || got[0].Product.ID != "prod-addon" {
		t.Fatalf("got = %+v", got)
	}
}

func TestCustomSubstituteIsNotCrossSell(t *testing.T) {
	owned := prod("prod-owned", 1000, 4, "active")
	owned.TermMonths = 12
	rival := prod("prod-rival", 2000, 4, "active")
	rival.TermMonths = 12

	in := input(profile(nil, []model.Purchase{purchase("prod-owned", 10)}),
		[]model.Product{owned, rival},
		[]model.Relation{{FromID: "prod-owned", ToID: "prod-rival", Type: "rival", Status: "confirmed", Confidence: 0.9}},
		nil, nil)
	in.RelationTypes = map[string]model.RelationType{
		"rival": {Code: "rival", Title: "Прямой конкурент", Behavior: "substitute", Status: "active"},
	}
	got, _ := recommend.Run(in)
	if len(got) != 0 {
		t.Fatalf("got = %+v, want no cross-sell", got)
	}
}

func TestUnknownPriceIsNotCheaper(t *testing.T) {
	owned := prod("prod-owned", 1000, 4, "active")
	owned.TermMonths = 12
	unknown := prod("prod-unknown", 0, 4, "active")
	unknown.TermMonths = 12

	in := input(profile(nil, []model.Purchase{purchase("prod-owned", 10)}),
		[]model.Product{owned, unknown},
		[]model.Relation{{FromID: "prod-owned", ToID: "prod-unknown", Type: "substitute", Status: "confirmed", Confidence: 0.9}},
		nil, nil)
	got, _ := recommend.Run(in)
	if len(got) != 0 {
		t.Fatalf("got = %+v, want no price-based alternative", got)
	}
}
