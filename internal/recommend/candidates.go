package recommend

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/basili4-1982/lk-advisor/internal/model"
	"github.com/basili4-1982/lk-advisor/internal/persona"
)

// Candidates builds the offer pool: renewal, upsell, cross-sell and analog.
// The primary intent only boosts ranking; it does not filter the pool.
func Candidates(in Input) []model.Candidate {
	owned := ownedIDs(in.Profile)
	scores := persona.Scores(in.Profile, in.Products)
	businessWeight, consumerWeight := 0.0, 0.0
	for _, s := range scores {
		switch s.Type {
		case "business":
			businessWeight = s.Score
		case "consumer":
			consumerWeight = s.Score
		}
	}
	businessDominant := businessWeight >= 0.6 && businessWeight > consumerWeight
	consumerDominant := consumerWeight >= 0.6 && consumerWeight > businessWeight
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

	add := func(productID, typ string, relevance, urgency, affinity, confidence float64,
		sourceID, relationType string, evidence ...model.Evidence) {
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
		candidateID := ""
		if sourceID != "" && relationType != "" {
			candidateID = sourceID + "|" + relationType + "|" + productID
		}
		out = append(out, model.Candidate{
			Product:         p,
			Type:            typ,
			Relevance:       relevance,
			Urgency:         urgency,
			Affinity:        affinity,
			Margin:          normalizeMargin(p.Price),
			Confidence:      confidence,
			Promo:           promo,
			Evidence:        evidence,
			SourceProductID: sourceID,
			RelationType:    relationType,
			CandidateID:     candidateID,
		})
	}

	// Renewal: active licenses expiring within 90 days, plus recently expired
	// licenses whose product can still be sold.
	hasActiveRenewal := false
	for _, l := range in.Profile.Licenses {
		if l.Status == "active" {
			if d := daysUntil(l.End, in.Now); d >= 0 && d <= 90 {
				hasActiveRenewal = true
				break
			}
		}
	}
	for _, l := range in.Profile.Licenses {
		if l.Status != "active" && l.Status != "expired" {
			continue
		}
		d := daysUntil(l.End, in.Now)
		if l.Status == "active" {
			if d < 0 || d > 90 {
				continue
			}
			add(l.ProductID, "renewal", 1.0, urgencyFromDays(d), 0.9, 0.9, l.ProductID, "renewal", licenseEvidence(l, in.Now))
		} else {
			if hasActiveRenewal || d >= 0 || d < -30 {
				continue
			}
			if p, ok := in.Product(l.ProductID); !ok || p.Status == "discontinued" {
				continue
			}
			add(l.ProductID, "renewal", 1.0, 1.0, 0.9, 0.9, l.ProductID, "renewal", expiredEvidence(l, in.Now))
		}
		for _, r := range in.Relations {
			behavior := in.RelationBehavior(r.Type)
			if r.FromID == l.ProductID && (behavior == "bundle" || behavior == "upsell") {
				urgency := urgencyFromDays(d)
				if l.Status == "expired" {
					urgency = 1.0
				}
				add(r.ToID, "renewal", 0.8, urgency, r.Confidence, r.Confidence, r.FromID, r.Type, relationEvidence(r, in))
			}
		}
	}

	// Upsell: upgrades and bundles from owned products.
	for _, r := range in.Relations {
		if !owned[r.FromID] {
			continue
		}
		weight := audienceWeight(in, r.FromID, r.ToID, businessDominant, consumerDominant)
		if weight == 0 {
			continue
		}
		switch in.RelationBehavior(r.Type) {
		case "upsell":
			add(r.ToID, "upsell", 1.0*weight, 0, r.Confidence*weight, r.Confidence, r.FromID, r.Type, relationEvidence(r, in))
		case "bundle":
			add(r.ToID, "upsell", 0.8*weight, 0, r.Confidence*weight, r.Confidence, r.FromID, r.Type, relationEvidence(r, in))
		}
	}

	// Cross-sell: complementary products from owned products.
	for _, r := range in.Relations {
		behavior := in.RelationBehavior(r.Type)
		if !owned[r.FromID] || (behavior != "cross_sell" && behavior != "bundle") || substituteOf[r.ToID] != "" {
			continue
		}
		weight := audienceWeight(in, r.FromID, r.ToID, businessDominant, consumerDominant)
		if weight == 0 {
			continue
		}
		add(r.ToID, "cross_sell", relationRelevance(behavior)*weight, 0, r.Confidence*weight, r.Confidence, r.FromID, r.Type, relationEvidence(r, in))
	}

	// Alternative: a similar product for the same job, not a full replacement.
	for _, r := range in.Relations {
		if !owned[r.FromID] || in.RelationBehavior(r.Type) != "alternative" {
			continue
		}
		weight := audienceWeight(in, r.FromID, r.ToID, businessDominant, consumerDominant)
		if weight == 0 {
			continue
		}
		add(r.ToID, "alternative", 0.6*weight, 0, r.Confidence*0.8, r.Confidence, r.FromID, r.Type, relationEvidence(r, in))
	}

	// Analog: replacements for discontinued products.
	for _, r := range in.Relations {
		if owned[r.FromID] && in.RelationBehavior(r.Type) == "analog" {
			weight := audienceWeight(in, r.FromID, r.ToID, businessDominant, consumerDominant)
			if weight == 0 {
				continue
			}
			add(r.ToID, "analog", 1.0*weight, 0, r.Confidence*weight, r.Confidence, r.FromID, r.Type, relationEvidence(r, in))
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
		weight := audienceWeight(in, fromID, toID, businessDominant, consumerDominant)
		if weight == 0 {
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
		add(toID, "alternative", 0.5*weight, 0, 0.7*weight, 0.8, fromID, "substitute", ev)
	}

	// Demand: decayed recent popularity is a supporting slice, not a
	// replacement for relations. It only fills gaps when relations are sparse.
	if len(out) == 0 && len(in.Popularity) > 0 {
		ids := make([]string, 0, len(in.Popularity))
		maxScore := 0.0
		for id, score := range in.Popularity {
			ids = append(ids, id)
			if score > maxScore {
				maxScore = score
			}
		}
		sort.Slice(ids, func(i, j int) bool {
			if in.Popularity[ids[i]] != in.Popularity[ids[j]] {
				return in.Popularity[ids[i]] > in.Popularity[ids[j]]
			}
			return ids[i] < ids[j]
		})
		for _, id := range ids {
			if len(out) >= 3 {
				break
			}
			if owned[id] {
				continue
			}
			p, ok := in.Product(id)
			if !ok || p.Status == "discontinued" {
				continue
			}
			weight := audienceWeight(in, "", id, businessDominant, consumerDominant)
			if weight == 0 {
				continue
			}
			normalized := 0.0
			if maxScore > 0 {
				normalized = in.Popularity[id] / maxScore
			}
			add(id, "cross_sell", (0.2+0.2*normalized)*weight, 0, 0.3*weight, 0.45,
				"segment", "popular",
				model.Evidence{Source: "demand", Value: "растёт спрос за последние 8 недель"})
		}
	}

	return out
}

// audienceWeight downweights a consumer source/product in a B2B-dominant profile
// (and the reverse). A business upgrade from a personal product is allowed.
func audienceWeight(in Input, sourceID, targetID string, businessDominant, consumerDominant bool) float64 {
	if !businessDominant && !consumerDominant {
		return 1
	}
	src, hasSrc := in.Products[sourceID]
	tgt, hasTgt := in.Products[targetID]
	if !hasTgt {
		return 1
	}
	if businessDominant {
		if tgt.Audience == "consumer" {
			return 0.4
		}
		if hasSrc && src.Audience == "consumer" && tgt.Audience != "business" {
			return 0.35
		}
	}
	if consumerDominant {
		if tgt.Audience == "business" {
			return 0
		}
		if hasSrc && src.Audience == "business" && tgt.Audience != "consumer" {
			return 0.35
		}
	}
	return 1
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
	case "alternative":
		phrase = "альтернатива " + from
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

func licenseEvidence(l model.License, now time.Time) model.Evidence {
	d := daysUntil(l.End, now)
	return model.Evidence{
		Source: "license",
		Value:  fmt.Sprintf("лицензия действует до %s, продление через %d дней", l.End.Format("2006-01-02"), d),
	}
}

func expiredEvidence(l model.License, now time.Time) model.Evidence {
	d := daysSince(l.End, now)
	return model.Evidence{
		Source: "license",
		Value:  fmt.Sprintf("лицензия истекла %s, просрочено %d дней", l.End.Format("2006-01-02"), d),
	}
}

// FillPopular appends decayed-demand candidates until the list reaches limit.
// It is the final safety net: relations and rules win, popularity only fills.
func FillPopular(in Input, picked []model.Candidate, limit int) []model.Candidate {
	if len(picked) >= limit || len(in.Popularity) == 0 {
		return picked
	}
	seen := make(map[string]bool, len(picked))
	for _, c := range picked {
		seen[c.Product.ID] = true
	}
	owned := ownedIDs(in.Profile)
	ids := make([]string, 0, len(in.Popularity))
	maxScore := 0.0
	for id, score := range in.Popularity {
		ids = append(ids, id)
		if score > maxScore {
			maxScore = score
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		if in.Popularity[ids[i]] != in.Popularity[ids[j]] {
			return in.Popularity[ids[i]] > in.Popularity[ids[j]]
		}
		return ids[i] < ids[j]
	})
	for _, id := range ids {
		if len(picked) >= limit {
			break
		}
		if seen[id] || owned[id] {
			continue
		}
		p, ok := in.Product(id)
		if !ok || p.Status == "discontinued" {
			continue
		}
		normalized := 0.0
		if maxScore > 0 {
			normalized = in.Popularity[id] / maxScore
		}
		c := model.Candidate{
			Product:         p,
			Type:            "cross_sell",
			Relevance:       0.2 + 0.2*normalized,
			Affinity:        0.3,
			Margin:          normalizeMargin(p.Price),
			Confidence:      0.45,
			Evidence:        []model.Evidence{{Source: "demand", Value: "растёт спрос за последние 8 недель"}},
			SourceProductID: "segment",
			RelationType:    "popular",
			CandidateID:     "segment|popular|" + id,
		}
		c = ScoreWith(c, in.Now, in.Weights)
		picked = append(picked, c)
		seen[id] = true
	}
	return picked
}
