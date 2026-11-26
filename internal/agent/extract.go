package agent

import (
	"sort"
	"strings"

	"github.com/basili4-1982/lk-advisor/internal/model"
)

type extraction struct {
	Products []string
	Type     string
}

var aliases = []struct{ word, id string }{
	{"второй растров", "prod-raster-2"},
	{"вектор", "prod-vector"},
	{"растров", "prod-raster"},
	{"антивирус", "prod-antivirus"},
	{"бэкап", "prod-backup"},
	{"резервн", "prod-backup"},
	{"офис", "prod-office-suite"},
	{"pdf", "prod-pdf"},
	{"подпис", "prod-esign"},
	{"игр", "prod-casual-game"},
}

var offTopicWords = []string{
	"погода", "политик", "президент", "анекдот", "напиши код",
	"другой клиент", "промпт", "инструкц", "забудь", "курс доллара",
	"дай скидку", "сделай скидку",
}

func extract(question string, products map[string]model.Product) extraction {
	lower := strings.ToLower(question)
	for _, w := range offTopicWords {
		if strings.Contains(lower, w) {
			return extraction{Type: "off_topic"}
		}
	}

	typ := "why"
	switch {
	case strings.Contains(lower, "почему не") ||
		strings.Contains(lower, "альтернатив") ||
		strings.Contains(lower, "вместо"):
		typ = "alternatives"
	case strings.Contains(lower, "что будет") ||
		strings.Contains(lower, "если не") ||
		strings.Contains(lower, "если отказ"):
		typ = "what_if"
	}

	seen := map[string]bool{}
	ids := make([]string, 0, 3)
	add := func(id string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		ids = append(ids, id)
	}

	for _, a := range aliases {
		if strings.Contains(lower, a.word) {
			add(a.id)
		}
	}

	keys := make([]string, 0, len(products))
	for k := range products {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if strings.Contains(lower, strings.ToLower(products[k].Title)) {
			add(k)
		}
	}

	if len(ids) > 3 {
		ids = ids[:3]
	}
	return extraction{Products: ids, Type: typ}
}

func trim(s string) string { return strings.TrimSpace(s) }
