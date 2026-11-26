package recommend

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/basili4-1982/lk-advisor/internal/model"
)

// ApplyRules removes forbidden candidates and adds constraints.
func ApplyRules(in Input, cands []model.Candidate) ([]model.Candidate, []model.Rejection) {
	owned := ownedIDs(in.Profile)
	allowed := make([]model.Candidate, 0, len(cands))
	var rejected []model.Rejection

	ownedSecurity := false
	for id := range owned {
		if p, ok := in.Products[id]; ok && p.Category == "security" {
			ownedSecurity = true
			break
		}
	}

	audience := clientAudience(in, owned)
	for _, c := range cands {
		if owned[c.Product.ID] && c.Type != "renewal" && c.Type != "upsell" {
			rejected = append(rejected, rejection(c, "already_owned", "уже куплено"))
			continue
		}
		if c.Type != "renewal" && audience != "any" && c.Product.Audience != "" &&
			c.Product.Audience != "any" && c.Product.Audience != audience {
			rejected = append(rejected, rejection(c, "audience_mismatch", "не подходит сегменту клиента"))
			continue
		}
		if c.Type == "cross_sell" && ownedSecurity && c.Product.Category == "security" {
			rejected = append(rejected, rejection(c, "already_have_security", "уже есть средство защиты"))
			continue
		}

		blocked := false
		for _, r := range in.Rules {
			switch r.Type {
			case "excludes":
				from, to := parsePair(r.Condition)
				if from != "" && owned[from] && to == c.Product.ID {
					rejected = append(rejected, rejection(c, r.Type, "несовместимо с купленным продуктом"))
					blocked = true
				}
			case "export_control":
				if parseProduct(r.Condition) == c.Product.ID {
					rejected = append(rejected, rejection(c, r.Type, "экспортный контроль"))
					blocked = true
				}
			case "out_of_stock":
				if parseProduct(r.Condition) == c.Product.ID {
					rejected = append(rejected, rejection(c, r.Type, "нет в наличии"))
					blocked = true
				}
			case "manager_only":
				if parseProduct(r.Condition) == c.Product.ID {
					c.Constraints = appendUnique(c.Constraints, "manager_approval")
				}
			case "max_discount":
				if c.Promo != nil {
					limit := parseDiscount(r.Condition)
					if limit > 0 && c.Promo.Discount > limit {
						promo := *c.Promo
						promo.Discount = limit
						c.Promo = &promo
						for i := range c.Evidence {
							if c.Evidence[i].Source == "promo" {
								c.Evidence[i].Value = fmt.Sprintf("скидка %.0f%% до %s",
									promo.Discount, promo.ValidTo.Format("2006-01-02"))
							}
						}
					}
				}
			}
			if blocked {
				break
			}
		}
		if blocked {
			continue
		}

		if !compatible(c.Product.Requirements, in.Profile.Environment) {
			rejected = append(rejected, rejection(c, "compatibility", "не подходит окружение клиента"))
			continue
		}

		allowed = append(allowed, c)
	}

	return allowed, rejected
}

func compatible(req model.Requirements, env model.Environment) bool {
	if env.RAMGB > 0 && req.RAMGB > env.RAMGB {
		return false
	}
	if req.OS != "" && env.OS != "" &&
		strings.Contains(req.OS, "Windows") && !strings.Contains(env.OS, "Windows") {
		return false
	}
	if req.GPU == "dedicated" && env.GPU != "" && env.GPU != "dedicated" {
		return false
	}
	return true
}

func clientAudience(in Input, owned map[string]bool) string {
	consumer, business := false, false
	for id := range owned {
		p, ok := in.Products[id]
		if !ok {
			continue
		}
		switch p.Audience {
		case "consumer":
			consumer = true
		case "business":
			business = true
		}
	}
	switch {
	case consumer && !business:
		return "consumer"
	case business && !consumer:
		return "business"
	default:
		return "any"
	}
}

func rejection(c model.Candidate, rule, reason string) model.Rejection {
	return model.Rejection{ProductID: c.Product.ID, Rule: rule, Reason: reason}
}

func parsePair(cond string) (string, string) {
	s := strings.TrimPrefix(cond, "pair=")
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func parseProduct(cond string) string {
	return strings.TrimPrefix(cond, "product=")
}

func parseDiscount(cond string) float64 {
	s := strings.TrimPrefix(cond, "discount>")
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

func appendUnique(xs []string, v string) []string {
	for _, x := range xs {
		if x == v {
			return xs
		}
	}
	return append(xs, v)
}
