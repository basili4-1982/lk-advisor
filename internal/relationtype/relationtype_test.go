package relationtype_test

import (
	"strings"
	"testing"

	"github.com/basili4-1982/lk-advisor/internal/model"
	"github.com/basili4-1982/lk-advisor/internal/relationtype"
)

func TestPromptIncludesActiveDescriptionAndExamples(t *testing.T) {
	types := relationtype.Builtins()
	types = append(types, model.RelationType{
		Code: "industry_addon", Title: "Отраслевое дополнение", Behavior: "cross_sell",
		Direction: "forward", Status: "active",
		Description: "Отраслевой модуль к платформе.", Examples: "1С → модуль.",
	})
	types = append(types, model.RelationType{
		Code: "draft_type", Title: "Черновик", Behavior: "info",
		Direction: "forward", Status: "draft", Description: "Не должен попасть.",
	})
	p := relationtype.Prompt(types)
	if !strings.Contains(p, "industry_addon") || !strings.Contains(p, "Отраслевой модуль") {
		t.Fatalf("prompt = %s", p)
	}
	if strings.Contains(p, "draft_type") {
		t.Fatalf("draft type leaked into prompt: %s", p)
	}
}

func TestValidate(t *testing.T) {
	ok := model.RelationType{
		Code: "industry_addon", Title: "Отраслевое дополнение", Behavior: "cross_sell",
		Direction: "forward", DefaultConfidence: 0.8, Status: "active",
		Description: "Отраслевой модуль.",
	}
	if err := relationtype.Validate(ok); err != nil {
		t.Fatalf("validate: %v", err)
	}
	bad := ok
	bad.Description = ""
	if err := relationtype.Validate(bad); err == nil {
		t.Fatal("empty description must fail")
	}
}
