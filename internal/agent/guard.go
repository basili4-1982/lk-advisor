package agent

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/basili4-1982/lk-advisor/internal/explain"
	"github.com/basili4-1982/lk-advisor/internal/model"
)

var phoneRe = regexp.MustCompile(`(\+7|8)[\s\-()]*\d{3}[\s\-()]*\d{3}[\s\-()]*\d{2}[\s\-()]*\d{2}`)

// Guard enforces length, PII, prompt leakage and number grounding.
func Guard(answer string, ev []model.Evidence, allow ...string) error {
	if strings.TrimSpace(answer) == "" || len(answer) > 1200 {
		return fmt.Errorf("bad answer length")
	}
	lower := strings.ToLower(answer)
	for _, bad := range []string{"системный промпт", "llm_model", "ignore previous", "api key"} {
		if strings.Contains(lower, bad) {
			return fmt.Errorf("prompt leakage")
		}
	}
	if phoneRe.MatchString(answer) || strings.Contains(answer, "@") {
		return fmt.Errorf("pii in answer")
	}
	return explain.ValidateNumbers(answer, ev, allow...)
}
