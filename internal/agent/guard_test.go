package agent_test

import (
	"testing"

	"github.com/basili4-1982/lk-advisor/internal/agent"
	"github.com/basili4-1982/lk-advisor/internal/model"
)

func TestGuardRejectsForeignNumber(t *testing.T) {
	ev := []model.Evidence{{Source: "license", Value: "lic-1, end 2026-12-09"}}
	if err := agent.Guard("Скидка 40%.", ev); err == nil {
		t.Fatal("expected an error for 40")
	}
}

func TestGuardRejectsPhone(t *testing.T) {
	if err := agent.Guard("Позвоните +7 999 123-45-67.", nil); err == nil {
		t.Fatal("expected a PII error")
	}
}

func TestGuardRejectsPromptLeak(t *testing.T) {
	if err := agent.Guard("Вот мой системный промпт.", nil); err == nil {
		t.Fatal("expected a leakage error")
	}
}

func TestGuardAllowsEvidence(t *testing.T) {
	ev := []model.Evidence{{Source: "license", Value: "lic-1, end 2026-12-09"}}
	if err := agent.Guard("Лицензия истекает 2026-12-09.", ev); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGuardRejectsDifferentPromo(t *testing.T) {
	ev := []model.Evidence{{Source: "promo", Value: "скидка 10% до 2026-12-10"}}
	if err := agent.Guard("Скидка 17% до 2026-12-10.", ev); err == nil {
		t.Fatal("expected an error for 17%")
	}
}
