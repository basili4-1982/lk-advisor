package agent

import (
	"fmt"
	"time"

	"github.com/basili4-1982/lk-advisor/internal/explain"
	"github.com/basili4-1982/lk-advisor/internal/model"
)

func answerWhy(cands []model.Candidate, ids []string, products map[string]model.Product) (string, []model.Evidence) {
	if len(cands) == 0 {
		return "Недостаточно данных.", nil
	}
	for _, id := range ids {
		for _, c := range cands {
			if c.Product.ID == id {
				if len(c.Evidence) == 0 {
					return fmt.Sprintf("Предлагаем %s.", c.Product.Title), nil
				}
				return fmt.Sprintf("Предлагаем %s, потому что %s.", c.Product.Title, explain.FactText(c.Evidence[0])), c.Evidence
			}
		}
	}

	top := cands[0]
	fact := "подходит по профилю"
	if len(top.Evidence) > 0 {
		fact = explain.FactText(top.Evidence[0])
	}
	if len(ids) > 0 {
		return fmt.Sprintf("%s сейчас нет в выдаче. Рекомендуем %s: %s.",
			titleOf(products, ids[0]), top.Product.Title, fact), top.Evidence
	}
	return fmt.Sprintf("Предлагаем %s, потому что %s.", top.Product.Title, fact), top.Evidence
}

func pick(cands []model.Candidate, ids []string) (model.Candidate, bool) {
	for _, id := range ids {
		for _, c := range cands {
			if c.Product.ID == id {
				return c, true
			}
		}
	}
	if len(cands) > 0 {
		return cands[0], true
	}
	return model.Candidate{}, false
}

func answerAlternatives(products map[string]model.Product, rels []model.Relation, rejected []model.Rejection, ids []string) (string, []model.Evidence) {
	for _, id := range ids {
		for _, r := range rejected {
			if r.ProductID == id {
				return fmt.Sprintf("%s исключён: %s.", titleOf(products, id), r.Reason),
					[]model.Evidence{{Source: "rule", Value: "правило: " + r.Reason}}
			}
		}
		for _, rel := range rels {
			if rel.ToID == id && (rel.Type == "substitute" || rel.Type == "same_task") {
				return fmt.Sprintf("%s — это замена, её как дополнение не предлагаем; при необходимости покажем как бюджетную альтернативу.", titleOf(products, id)),
					[]model.Evidence{{Source: "relation", Value: "замена " + titleOf(products, rel.FromID)}}
			}
		}
	}
	return "Недостаточно данных.", nil
}

func answerWhatIf(p model.Profile, ids []string, now time.Time) (string, []model.Evidence) {
	for _, l := range p.Licenses {
		if l.Status != "active" {
			continue
		}
		if len(ids) == 0 || contains(ids, l.ProductID) {
			left := int(l.End.Sub(time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())).Hours() / 24)
			return fmt.Sprintf("Лицензия истекает %s. После этой даты продукт перестанет работать.", l.End.Format("2006-01-02")),
				[]model.Evidence{
					{Source: "license", Value: fmt.Sprintf("действует до %s, %d мест, осталось %d дней", l.End.Format("2006-01-02"), l.Seats, left)},
					{Source: "rule", Value: "после даты окончания продукт перестаёт работать"},
				}
		}
	}
	return "Данных о лицензии нет.", nil
}

func fallbackAnswer(cands []model.Candidate) string {
	if len(cands) == 0 {
		return "Недостаточно данных."
	}
	return fmt.Sprintf("Рекомендуем %s.", cands[0].Product.Title)
}

func topEvidence(cands []model.Candidate) []model.Evidence {
	if len(cands) == 0 {
		return []model.Evidence{}
	}
	return cands[0].Evidence
}

func titleOf(products map[string]model.Product, id string) string {
	if p, ok := products[id]; ok {
		return p.Title
	}
	return id
}

func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
