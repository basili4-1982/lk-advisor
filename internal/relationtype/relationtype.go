// Package relationtype describes editable relation templates and builds the
// prompt block used by relation extraction.
package relationtype

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/basili4-1982/lk-advisor/internal/model"
)

var codeRe = regexp.MustCompile(`^[a-z][a-z0-9_]{1,31}$`)

// Behaviors are the ways the engine uses a relation.
var Behaviors = []string{"cross_sell", "upsell", "bundle", "substitute", "analog", "info"}

// BehaviorTitles are human-readable behavior names.
var BehaviorTitles = map[string]string{
	"cross_sell": "Допродажа",
	"upsell":     "Апгрейд",
	"bundle":     "Комплект (апгрейд + допродажа)",
	"substitute": "Замена",
	"analog":     "Аналог снятого",
	"info":       "Только справка",
}

// Builtins returns the stock templates.
func Builtins() []model.RelationType {
	return []model.RelationType{
		{
			Code: "upgrade", Title: "Апгрейд", Behavior: "upsell", Direction: "forward",
			DefaultConfidence: 0.9, Status: "active", Builtin: true,
			Description: "Старшая версия или старший тариф того же продукта. Покупатель получает больше возможностей, а не другой продукт.",
			Examples:    "Visio Standard → Visio Professional; КОМПАС-График → КОМПАС-3D; Microsoft 365 бизнес стандарт → бизнес премиум.",
		},
		{
			Code: "bundle", Title: "Комплект", Behavior: "bundle", Direction: "forward",
			DefaultConfidence: 0.9, Status: "active", Builtin: true,
			Description: "Два продукта выгоднее или удобнее продавать вместе; один усиливает другой, но не заменяет его.",
			Examples:    "Office + PDF Toolkit; КриптоАРМ + Office; антивирус + резервное копирование.",
		},
		{
			Code: "adjacent_task", Title: "Дополнение", Behavior: "cross_sell", Direction: "forward",
			DefaultConfidence: 0.8, Status: "active", Builtin: true,
			Description: "Продукт закрывает соседнюю задачу в том же рабочем процессе. Это не замена и не старшая версия.",
			Examples:    "Visio → Project; BI Studio → Report Builder; Office → курс по Office.",
		},
		{
			Code: "accessory", Title: "Аксессуар", Behavior: "cross_sell", Direction: "forward",
			DefaultConfidence: 0.8, Status: "active", Builtin: true,
			Description: "Вспомогательный товар, который нужен для работы основного: расходник, модуль, лицензия на рабочее место.",
			Examples:    "ПО → дополнительная лицензия; оборудование → кабель; платформа → модуль.",
		},
		{
			Code: "compatibility", Title: "Совместимость", Behavior: "cross_sell", Direction: "symmetric",
			DefaultConfidence: 0.8, Status: "active", Builtin: true,
			Description: "Продукты явно совместимы по API, формату, разъёму или модели. Связь симметричная: если A совместим с B, то и B с A.",
			Examples:    "1С → модуль обмена; САПР → плагин; кабель → роутер с тем же разъёмом.",
		},
		{
			Code: "workflow_next_step", Title: "Следующий шаг", Behavior: "cross_sell", Direction: "forward",
			DefaultConfidence: 0.8, Status: "active", Builtin: true,
			Description: "После покупки клиенту логично перейти к следующему этапу: внедрение, обучение, миграция, резервное копирование.",
			Examples:    "ПО → курс; внедрение → аудит; миграция → сопровождение.",
		},
		{
			Code: "substitute", Title: "Замена", Behavior: "substitute", Direction: "forward",
			DefaultConfidence: 0.9, Status: "active", Builtin: true,
			Description: "Конкурент или другой продукт для той же задачи. Прямо не допродаётся; показывается только как бюджетная альтернатива при более низкой цене или акции.",
			Examples:    "GStarCAD ↔ КОМПАС-3D; Kaiten ↔ Яндекс Трекер; один антивирус ↔ другой антивирус.",
		},
		{
			Code: "same_task", Title: "Та же задача", Behavior: "substitute", Direction: "forward",
			DefaultConfidence: 0.9, Status: "active", Builtin: true,
			Description: "Другой продукт для той же задачи, но не обязательно прямой конкурент. В допродажу не идёт.",
			Examples:    "два растровых редактора; две CRM; два средства резервного копирования.",
		},
		{
			Code: "analog", Title: "Аналог снятого", Behavior: "analog", Direction: "forward",
			DefaultConfidence: 0.8, Status: "active", Builtin: true,
			Description: "Замена продукта, который снят с продажи или больше не поддерживается.",
			Examples:    "снятый пакет → актуальный пакет; устаревшая версия → поддерживаемая версия.",
		},
		{
			Code: "excludes", Title: "Несовместимость", Behavior: "info", Direction: "symmetric",
			DefaultConfidence: 0.9, Status: "active", Builtin: true,
			Description: "Продукты нельзя использовать или предлагать вместе. Это ограничение, а не рекомендация.",
			Examples:    "два антивируса с резидентными модулями; два продукта с конфликтом драйверов.",
		},
	}
}

// Builtin returns a stock template by code.
func Builtin(code string) model.RelationType {
	for _, rt := range Builtins() {
		if rt.Code == code {
			return rt
		}
	}
	return model.RelationType{Code: code, Title: code, Behavior: "info", Direction: "forward"}
}

// Validate checks one template.
func Validate(rt model.RelationType) error {
	if !codeRe.MatchString(rt.Code) {
		return errors.New("код должен быть латиницей: a-z, цифры и _, от 2 до 32 символов")
	}
	if strings.TrimSpace(rt.Title) == "" {
		return errors.New("название обязательно")
	}
	if strings.TrimSpace(rt.Description) == "" {
		return errors.New("описание обязательно: оно идёт в промпт")
	}
	if !contains(Behaviors, rt.Behavior) {
		return fmt.Errorf("неизвестное поведение: %s", rt.Behavior)
	}
	if rt.Direction != "forward" && rt.Direction != "symmetric" {
		return errors.New("направление должно быть forward или symmetric")
	}
	if rt.DefaultConfidence < 0 || rt.DefaultConfidence > 1 {
		return errors.New("уверенность должна быть от 0 до 1")
	}
	switch rt.Status {
	case "active", "draft", "disabled":
	default:
		return errors.New("статус должен быть active, draft или disabled")
	}
	return nil
}

// Prompt builds the relation-template block for an extraction prompt.
func Prompt(types []model.RelationType) string {
	var b strings.Builder
	b.WriteString("Допустимые типы связей (используй только эти коды):\n")
	n := 0
	for _, rt := range types {
		if rt.Status != "active" {
			continue
		}
		n++
		fmt.Fprintf(&b, "\n%d. %s — %s.\n", n, rt.Code, rt.Title)
		fmt.Fprintf(&b, "   Когда применять: %s\n", strings.TrimSpace(rt.Description))
		if strings.TrimSpace(rt.Examples) != "" {
			fmt.Fprintf(&b, "   Примеры: %s\n", strings.TrimSpace(rt.Examples))
		}
		dir := "от купленного к предлагаемому"
		if rt.Direction == "symmetric" {
			dir = "симметричная: A→B и B→A"
		}
		fmt.Fprintf(&b, "   Направление: %s. Поведение: %s.\n", dir, BehaviorTitles[rt.Behavior])
	}
	if n == 0 {
		b.WriteString("\nНет активных типов связей.\n")
	}
	b.WriteString("\nЕсли ни один тип не подходит, верни none. Не придумывай новые коды.\n")
	return b.String()
}

func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
