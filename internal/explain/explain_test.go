package explain_test

import (
	"context"
	"strings"
	"testing"

	"github.com/basili4-1982/lk-advisor/internal/explain"
	"github.com/basili4-1982/lk-advisor/internal/model"
)

func TestRenderRenewal(t *testing.T) {
	rec := model.Recommendation{Title: "Antivirus", Type: "renewal"}
	ev := []model.Evidence{{Source: "license", Value: "lic-0001, end 2026-12-09"}}
	got, err := explain.Render(context.Background(), rec, ev)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.HasPrefix(got, "Продлите Antivirus") || !strings.Contains(got, "2026-12-09") {
		t.Fatalf("answer = %q", got)
	}
}

func TestRenderRelationFact(t *testing.T) {
	rec := model.Recommendation{Title: "Microsoft Visio Professional 2024", Type: "cross_sell"}
	ev := []model.Evidence{{Source: "relation", Value: "Microsoft Visio Standard 2024 → Microsoft Visio Professional 2024, adjacent_task"}}
	got, err := explain.Render(context.Background(), rec, ev)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(got, "дополняет Microsoft Visio Standard 2024") {
		t.Fatalf("answer = %q", got)
	}
}

func TestRenderNoEvidence(t *testing.T) {
	rec := model.Recommendation{Title: "Backup", Type: "cross_sell"}
	got, err := explain.Render(context.Background(), rec, nil)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if got != "Добавьте Backup." {
		t.Fatalf("answer = %q", got)
	}
}

func TestValidateNumbersRejectsUnknown(t *testing.T) {
	ev := []model.Evidence{{Source: "promo", Value: "10% until 2026-12-30"}}
	if err := explain.ValidateNumbers("Скидка 40%.", ev); err == nil {
		t.Fatal("expected an error for 40")
	}
}

func TestValidateNumbersAllowsEvidence(t *testing.T) {
	ev := []model.Evidence{{Source: "license", Value: "lic-0001, end 2026-12-09"}}
	if err := explain.ValidateNumbers("Лицензия истекает 2026-12-09.", ev); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
