// Package recommend turns a client profile into ranked recommendations.
package recommend

import (
	"math"
	"time"

	"github.com/basili4-1982/lk-advisor/internal/model"
	"github.com/basili4-1982/lk-advisor/internal/relationtype"
)

// Input is everything the engine needs for one request.
type Input struct {
	Profile       model.Profile
	Products      map[string]model.Product
	Relations     []model.Relation
	Promotions    []model.Promotion
	Rules         []model.Rule
	RelationTypes map[string]model.RelationType
	Weights       model.Weights
	// Popularity is decayed recent demand by product id. It is an optional
	// supporting slice: nil means the caller did not load it.
	Popularity map[string]float64
	Now        time.Time
}

// RelationType returns the editable template or a built-in fallback.
func (in Input) RelationType(code string) model.RelationType {
	if in.RelationTypes != nil {
		if rt, ok := in.RelationTypes[code]; ok {
			return rt
		}
	}
	return relationtype.Builtin(code)
}

// RelationBehavior returns how the engine should use a relation.
func (in Input) RelationBehavior(code string) string {
	rt := in.RelationType(code)
	if rt.Status != "" && rt.Status != "active" {
		return "info"
	}
	if rt.Behavior == "" {
		return "info"
	}
	return rt.Behavior
}

// Product returns a catalog item by id.
func (in Input) Product(id string) (model.Product, bool) {
	p, ok := in.Products[id]
	return p, ok
}

func ownedIDs(p model.Profile) map[string]bool {
	out := make(map[string]bool, len(p.Purchases)+len(p.Licenses))
	for _, x := range p.Purchases {
		out[x.ProductID] = true
	}
	for _, x := range p.Licenses {
		out[x.ProductID] = true
	}
	return out
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func daysUntil(t, now time.Time) int {
	return int(t.Sub(startOfDay(now)).Hours() / 24)
}

func daysSince(t, now time.Time) int {
	return int(startOfDay(now).Sub(startOfDay(t)).Hours() / 24)
}

// PromotionFor returns the first active promotion for a product.
func (in Input) PromotionFor(productID string) *model.Promotion {
	today := startOfDay(in.Now)
	for i := range in.Promotions {
		p := &in.Promotions[i]
		if p.ProductID != productID {
			continue
		}
		if p.ValidFrom.After(in.Now) || p.ValidTo.Before(today) {
			continue
		}
		return p
	}
	return nil
}

func normalizeMargin(price int64) float64 {
	if price <= 0 {
		return 0
	}
	return math.Min(1, float64(price)/100000)
}
