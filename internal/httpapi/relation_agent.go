package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/basili4-1982/lk-advisor/internal/catalog"
	"github.com/basili4-1982/lk-advisor/internal/model"
	"github.com/basili4-1982/lk-advisor/internal/relationtype"
)

type relationAgentRequest struct {
	FromID string `json:"from_id"`
	ToID   string `json:"to_id"`
}

type relationAgentProposal struct {
	FromID     string           `json:"from_id"`
	ToID       string           `json:"to_id"`
	Type       string           `json:"type"`
	Title      string           `json:"title"`
	Confidence float64          `json:"confidence"`
	Status     string           `json:"status"`
	Reason     string           `json:"reason"`
	Evidence   []model.Evidence `json:"evidence"`
	Source     string           `json:"source"`
}

func (h *handler) adminProposeRelation(w http.ResponseWriter, r *http.Request) {
	var req relationAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.FromID = strings.TrimSpace(req.FromID)
	req.ToID = strings.TrimSpace(req.ToID)
	if req.FromID == "" || req.ToID == "" || req.FromID == req.ToID {
		writeError(w, http.StatusBadRequest, "нужны два разных товара")
		return
	}

	products, err := catalog.AllProducts(r.Context(), h.db)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "catalog error")
		return
	}
	byID := make(map[string]model.Product, len(products))
	for _, p := range products {
		byID[p.ID] = p
	}
	from, ok := byID[req.FromID]
	if !ok {
		writeError(w, http.StatusBadRequest, "товар from_id не найден")
		return
	}
	to, ok := byID[req.ToID]
	if !ok {
		writeError(w, http.StatusBadRequest, "товар to_id не найден")
		return
	}
	types, err := catalog.RelationTypes(r.Context(), h.db)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "relation types error")
		return
	}
	active := activeRelationTypes(types)
	if len(active) == 0 {
		writeError(w, http.StatusConflict, "нет активных шаблонов связей")
		return
	}

	if h.llm != nil {
		if p, err := h.llmRelationProposal(r.Context(), from, to, active); err == nil {
			writeJSON(w, http.StatusOK, p)
			return
		} else {
			h.logger.Warn("relation agent llm", "err", err)
		}
	}
	writeJSON(w, http.StatusOK, heuristicRelationProposal(from, to, active))
}

// relationSystemPrompt is kept server-side. The full prompt is supplied through
// RELATION_SYSTEM_PROMPT and is not part of the repository.
func relationSystemPrompt() string {
	if v := strings.TrimSpace(os.Getenv("RELATION_SYSTEM_PROMPT")); v != "" {
		return v
	}
	return `Ты агент разметки связей между товарами. Верни только JSON без markdown.
Схема: {"type":"код или none","confidence":0.0,"reason":"кратко","evidence":["факт"]}.`
}

func (h *handler) llmRelationProposal(ctx context.Context, from, to model.Product, types []model.RelationType) (relationAgentProposal, error) {
	system := relationSystemPrompt()

	var b strings.Builder
	b.WriteString(relationtype.Prompt(types))
	b.WriteString("\nКарточки товаров:\n")
	fmt.Fprintf(&b, "Откуда (куплено): %s | id=%s | категория=%s | подкатегория=%s | вендор=%s | аудитория=%s\n",
		from.Title, from.ID, from.Category, from.Subcategory, from.Vendor, from.Audience)
	fmt.Fprintf(&b, "Куда (предлагаем): %s | id=%s | категория=%s | подкатегория=%s | вендор=%s | аудитория=%s\n",
		to.Title, to.ID, to.Category, to.Subcategory, to.Vendor, to.Audience)
	b.WriteString("\nВерни связь «от купленного к предлагаемому».")

	raw, err := h.llm.Complete(ctx, system, b.String())
	if err != nil {
		return relationAgentProposal{}, err
	}
	var out struct {
		Type       string   `json:"type"`
		Confidence float64  `json:"confidence"`
		Reason     string   `json:"reason"`
		Evidence   []string `json:"evidence"`
	}
	if err := decodeJSONObject(raw, &out); err != nil {
		return relationAgentProposal{}, err
	}
	out.Type = strings.ToLower(strings.TrimSpace(out.Type))
	if out.Type == "" || out.Type == "none" {
		return relationAgentProposal{
			FromID: from.ID, ToID: to.ID, Type: "none", Title: "Связь не найдена",
			Confidence: clampConfidence(out.Confidence), Status: "none",
			Reason:   nonEmpty(out.Reason, "Агент не нашёл достаточно оснований для связи."),
			Evidence: stringEvidence(out.Evidence), Source: "llm",
		}, nil
	}

	rt, ok := findRelationType(types, out.Type)
	if !ok {
		return relationAgentProposal{}, fmt.Errorf("llm proposed unknown type %q", out.Type)
	}
	status := h.relationPolicy().Status(out.Confidence)
	evidence := stringEvidence(out.Evidence)
	if len(evidence) == 0 {
		evidence = []model.Evidence{{Source: "agent", Value: nonEmpty(out.Reason, "связь по данным карточек")}}
	}
	return relationAgentProposal{
		FromID: from.ID, ToID: to.ID, Type: rt.Code, Title: rt.Title,
		Confidence: clampConfidence(out.Confidence), Status: status,
		Reason:   nonEmpty(out.Reason, "Связь предложена по данным карточек."),
		Evidence: evidence, Source: "llm",
	}, nil
}

func heuristicRelationProposal(from, to model.Product, types []model.RelationType) relationAgentProposal {
	p := relationAgentProposal{
		FromID: from.ID, ToID: to.ID, Type: "none", Title: "Связь не найдена",
		Confidence: 0.2, Status: "none", Source: "fallback",
		Reason: "Нет общих признаков: разные категории и подкатегории.",
		Evidence: []model.Evidence{{
			Source: "catalog",
			Value:  fmt.Sprintf("категории: %s / %s", from.Category, to.Category),
		}},
	}
	code := ""
	reason := ""
	if from.Subcategory != "" && from.Subcategory == to.Subcategory {
		code = "same_task"
		reason = fmt.Sprintf("Одинаковая подкатегория: %s.", from.Subcategory)
		p.Confidence = 0.55
	} else if from.Category != "" && from.Category == to.Category {
		code = "adjacent_task"
		reason = fmt.Sprintf("Одна категория (%s), разные подкатегории.", from.Category)
		p.Confidence = 0.5
	}
	if code == "" {
		return p
	}
	rt, ok := findRelationType(types, code)
	if !ok {
		return p
	}
	p.Type, p.Title, p.Status = rt.Code, rt.Title, "quarantine"
	p.Reason = reason
	p.Evidence = []model.Evidence{
		{Source: "catalog", Value: fmt.Sprintf("категория: %s; подкатегория: %s", from.Category, from.Subcategory)},
		{Source: "agent", Value: "эвристика без LLM, требует проверки"},
	}
	return p
}

func activeRelationTypes(types []model.RelationType) []model.RelationType {
	out := make([]model.RelationType, 0, len(types))
	for _, rt := range types {
		if rt.Status == "active" {
			out = append(out, rt)
		}
	}
	return out
}

func findRelationType(types []model.RelationType, code string) (model.RelationType, bool) {
	for _, rt := range types {
		if rt.Code == code {
			return rt, true
		}
	}
	return model.RelationType{}, false
}

func clampConfidence(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func stringEvidence(values []string) []model.Evidence {
	out := make([]model.Evidence, 0, len(values))
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			out = append(out, model.Evidence{Source: "agent", Value: strings.TrimSpace(v)})
		}
	}
	return out
}

func nonEmpty(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return strings.TrimSpace(v)
}
