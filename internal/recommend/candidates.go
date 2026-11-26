package recommend

import (
	"fmt"
	"sort"
	"strings"

	"github.com/basili4-1982/lk-advisor/internal/model"
)

// Candidates builds the offer pool: renewal, upsell, cross-sell and analog.
// The primary intent only boosts ranking; it does not filter the pool.
func Candidates(in Input) []model.Candidate {
	owned := ownedIDs(in.Profile)
	out := make([]model.Candidate, 0, 12)
	seen := map[string]bool{}
	substituteOf := map[string]string{}
	lowerThanOwned := map[string]bool{}
	for _, r := range in.Relations {
		behavior := in.RelationBehavior(r.Type)
		if owned[r.FromID] && behavior == "substitute" {
			substituteOf[r.ToID] = r.FromID
		}
		if behavior == "upsell" && owned[r.ToID] {
			lowerThanOwned[r.FromID] = true
		}
	}

	add := func(productID, typ string, relevance, urgency, affinity, confidence float64, evidence ...model.Evidence) {
		if productID == "" || seen[productID] || lowerThanOwned[productID] {
			return
		}
		p, ok := in.Product(productID)
		if !ok {
			return
		}
		seen[productID] = true
		promo := in.PromotionFor(productID)
		if promo != nil && !hasEvidence(evidence, "promo") {
			evidence = append(evidence, model.Evidence{
				Source: "promo",
				Value:  fmt.Sprintf("скидка %.0f%% до %s", promo.Discount, promo.ValidTo.Format("2006-01-02")),
			})
		}
		out = append(out, model.Candidate{
			Product:    p,
			Type:       typ,
			Relevance:  relevance,
			Urgency:    urgency,
			Affinity:   affinity,
			Margin:     normalizeMargin(p.Price),
			Confidence: confidence,
			Promo:      promo,
			Evidence:   evidence,
		})
	}

	// Renewal: active licenses expiring within 90 days, plus bundles/upgrades.
	for _, l := range in.Profile.Licenses {
		if l.Status != "active" {
			continue
		}
		d := daysUntil(l.End, in.Now)
		if d < 0 || d > 90 {
			continue
		}
		add(l.ProductID, "renewal", 1.0, urgencyFromDays(d), 0.9, 0.9, licenseEvidence(l))
		for _, r := range in.Relations {
			behavior := in.RelationBehavior(r.Type)
			if r.FromID == l.ProductID && (behavior == "bundle" || behavior == "upsell") {
				add(r.ToID, "renewal", 0.8, urgencyFromDays(d), r.Confidence, r.Confidence, relationEvidence(r, in))
			}
		}
	}

	// Upsell: upgrades and bundles from owned products.
	for _, r := range in.Relations {
		if !owned[r.FromID] {
			continue
		}
		switch in.RelationBehavior(r.Type) {
		case "upsell":
			add(r.ToID, "upsell", 1.0, 0, r.Confidence, r.Confidence, relationEvidence(r, in))
		case "bundle":
			add(r.ToID, "upsell", 0.8, 0, r.Confidence, r.Confidence, relationEvidence(r, in))
		}
	}

	// Cross-sell: complementary products from owned products.
	for _, r := range in.Relations {
		behavior := in.RelationBehavior(r.Type)
		if !owned[r.FromID] || (behavior != "cross_sell" && behavior != "bundle") || substituteOf[r.ToID] != "" {
			continue
		}
		add(r.ToID, "cross_sell", relationRelevance(behavior), 0, r.Confidence, r.Confidence, relationEvidence(r, in))
	}

	// Analog: replacements for discontinued products.
	for _, r := range in.Relations {
		if owned[r.FromID] && in.RelationBehavior(r.Type) == "analog" {
			add(r.ToID, "analog", 1.0, 0, r.Confidence, r.Confidence, relationEvidence(r, in))
		}
	}

	// Alternatives: competitors only when cheaper on a subscription or on promo.
	keys := make([]string, 0, len(substituteOf))
	for k := range substituteOf {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, toID := range keys {
		fromID := substituteOf[toID]
		ownedProd, ok1 := in.Product(fromID)
		cand, ok2 := in.Product(toID)
		if !ok1 || !ok2 {
			continue
		}
		promo := in.PromotionFor(toID)
		cheaper := cand.Price > 0 && ownedProd.Price > 0 && cand.Price < ownedProd.Price
		subscription := cand.TermMonths > 0 ||
			strings.Contains(strings.ToLower(cand.Title), "подписка") ||
			strings.Contains(strings.ToLower(cand.Title), "subscription")
		if promo == nil && !(subscription && cheaper) {
			continue
		}
		var ev model.Evidence
		if subscription && cheaper {
			ev = model.Evidence{
				Source: "price",
				Value:  fmt.Sprintf("дешевле %s на %d ₽", ownedProd.Title, ownedProd.Price-cand.Price),
			}
		} else {
			ev = model.Evidence{
				Source: "promo",
				Value:  fmt.Sprintf("акция: скидка %.0f%% до %s", promo.Discount, promo.ValidTo.Format("2006-01-02")),
			}
		}
		add(toID, "alternative", 0.5, 0, 0.7, 0.8, ev)
	}

	return out
}

func urgencyFromDays(d int) float64 {
	switch {
	case d <= 7:
		return 1.0
	case d <= 30:
		return 0.7
	case d <= 90:
		return 0.4
	default:
		return 0
	}
}

func relationRelevance(behavior string) float64 {
	switch behavior {
	case "bundle":
		return 0.8
	case "cross_sell":
		return 0.6
	default:
		return 0.2
	}
}

func hasEvidence(ev []model.Evidence, source string) bool {
	for _, e := range ev {
		if e.Source == source {
			return true
		}
	}
	return false
}

func relationEvidence(r model.Relation, in Input) model.Evidence {
	from := r.FromID
	if p, ok := in.Products[r.FromID]; ok && p.Title != "" {
		from = p.Title
	}
	rt := in.RelationType(r.Type)
	var phrase string
	switch rt.Code {
	case "adjacent_task":
		phrase = "дополняет " + from
	case "upgrade":
		phrase = "старший тариф к " + from
	case "bundle":
		phrase = "выгоднее вместе с " + from
	case "analog":
		phrase = "замена " + from
	case "workflow_next_step":
		phrase = "следующий шаг после " + from
	case "accessory":
		phrase = "дополняет " + from
	case "compatibility":
		phrase = "совместимо с " + from
	default:
		title := rt.Title
		if title == "" {
			title = r.Type
		}
		phrase = "связь «" + title + "» с " + from
	}
	return model.Evidence{Source: "relation", Value: phrase}
}

func licenseEvidence(l model.License) model.Evidence {
	return model.Evidence{
		Source: "license",
		Value:  fmt.Sprintf("лицензия действует до %s", l.End.Format("2006-01-02")),
	}
}
