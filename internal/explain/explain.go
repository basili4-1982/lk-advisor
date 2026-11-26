// Package explain builds the human-readable reason for a recommendation.
package explain

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/basili4-1982/lk-advisor/internal/model"
)

var numberRe = regexp.MustCompile(`\d+(?:[.,]\d+)?`)

// Render builds a template answer from evidence only.
func Render(_ context.Context, rec model.Recommendation, ev []model.Evidence) (string, error) {
	var verb string
	switch rec.Type {
	case "renewal":
		verb = "Продлите"
	case "upsell":
		verb = "Перейдите на"
	case "cross_sell":
		verb = "Добавьте"
	case "analog":
		verb = "Замените снятый продукт на"
	case "alternative":
		verb = "Рассмотрите"
	default:
		verb = "Рекомендуем"
	}

	answer := fmt.Sprintf("%s %s.", verb, rec.Title)
	if len(ev) > 0 {
		answer = fmt.Sprintf("%s %s: %s.", verb, rec.Title, FactText(ev[0]))
	}

	if err := ValidateNumbers(answer, ev, rec.Title); err != nil {
		return "", err
	}
	return answer, nil
}

// FactText turns an evidence item into a human-readable phrase.
func FactText(e model.Evidence) string {
	switch e.Source {
	case "license":
		if strings.HasPrefix(e.Value, "лицензия") {
			return e.Value
		}
		if i := strings.Index(e.Value, "end "); i >= 0 {
			return "лицензия истекает " + strings.TrimSpace(e.Value[i+4:])
		}
	case "relation":
		if !strings.Contains(e.Value, "→") && !strings.Contains(e.Value, "->") {
			return e.Value
		}
		parts := strings.Split(e.Value, ", ")
		if len(parts) >= 2 {
			typ := parts[len(parts)-1]
			from := strings.TrimSpace(strings.SplitN(parts[0], "→", 2)[0])
			switch typ {
			case "adjacent_task":
				return "дополняет " + from
			case "upgrade":
				return "старший тариф к " + from
			case "bundle":
				return "выгоднее вместе с " + from
			case "analog":
				return "замена " + from
			case "workflow_next_step":
				return "следующий шаг после " + from
			case "accessory":
				return "дополняет " + from
			case "compatibility":
				return "совместимо с " + from
			}
		}
	case "promo":
		if strings.HasPrefix(e.Value, "скидка") {
			return e.Value
		}
		return "действует акция " + strings.ReplaceAll(e.Value, "until", "до")
	case "fallback":
		return "популярное в сегменте"
	}
	return e.Value
}

// ValidateNumbers rejects any number that is not present in evidence.
func ValidateNumbers(answer string, ev []model.Evidence, allow ...string) error {
	var hay strings.Builder
	for _, e := range ev {
		hay.WriteString(e.Value)
		hay.WriteByte(' ')
	}
	for _, a := range allow {
		hay.WriteString(a)
		hay.WriteByte(' ')
	}
	haystack := hay.String()

	for _, n := range numberRe.FindAllString(answer, -1) {
		if !strings.Contains(haystack, n) {
			return fmt.Errorf("number %q is not in evidence", n)
		}
	}
	return nil
}
