package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/basili4-1982/lk-advisor/internal/catalog"
	"github.com/basili4-1982/lk-advisor/internal/model"
	"github.com/basili4-1982/lk-advisor/internal/relationtype"
)

// adminClassifyCoPurchases sends the strongest co-purchase pairs through the
// relation constructor (active templates + LLM), not a hardcoded type.
func (h *handler) adminClassifyCoPurchases(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Limit int `json:"limit"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Limit <= 0 || req.Limit > 50 {
		req.Limit = 10
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
	pairs, err := catalog.TopCoPurchases(r.Context(), h.db, req.Limit, "b2b")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "co-purchases error")
		return
	}
	created, skipped := 0, 0
	for _, pair := range pairs {
		typ, confidence, reason, evidence, source := h.classifyPair(r.Context(), pair, active)
		if typ == "" {
			skipped++
			continue
		}
		ok, err := catalog.InsertDatasetCandidate(r.Context(), h.db, pair.ItemA, pair.ItemB,
			typ, confidence, reason, evidence, source)
		if err != nil {
			h.logger.Error("dataset classify insert", "err", err)
			continue
		}
		if ok {
			created++
		} else {
			skipped++
		}
	}
	_ = catalog.LogAdminChange(r.Context(), h.db, "relation-agent", "dataset.classify",
		map[string]int{"processed": len(pairs), "created": created, "skipped": skipped})
	writeJSON(w, http.StatusOK, map[string]any{
		"processed": len(pairs), "created": created, "skipped": skipped,
	})
}

func (h *handler) classifyPair(ctx context.Context, pair catalog.CoPurchase, types []model.RelationType) (string, float64, string, string, string) {
	if h.llm != nil {
		system := `Ты классифицируешь связь между товарами по совместным покупкам. Верни ТОЛЬКО JSON:
{"type":"код из реестра","confidence":0.0,"reason":"кратко по-русски","evidence":["факт"]}.
Используй только коды из реестра. Если связи нет — type=none.`
		var b strings.Builder
		b.WriteString(relationtype.Prompt(types))
		fmt.Fprintf(&b, "\nТовар A: %s | id=%s | категория=%s\nТовар B: %s | id=%s | категория=%s\nКуплены вместе: %d раз (B2B %d, B2C %d).\n",
			pair.TitleA, pair.ItemA, pair.CategoryA,
			pair.TitleB, pair.ItemB, pair.CategoryB,
			pair.Count, pair.B2B, pair.B2C)
		raw, err := h.llm.Complete(ctx, system, b.String())
		if err == nil {
			var out struct {
				Type       string   `json:"type"`
				Confidence float64  `json:"confidence"`
				Reason     string   `json:"reason"`
				Evidence   []string `json:"evidence"`
			}
			if err := decodeJSONObject(raw, &out); err == nil {
				out.Type = strings.ToLower(strings.TrimSpace(out.Type))
				if out.Type == "none" {
					return "", 0, "", "", ""
				}
				if rt, ok := findRelationType(types, out.Type); ok {
					evidence := []model.Evidence{{Source: "co_purchase", Value: fmt.Sprintf("куплено вместе %d раз", pair.Count)}}
					for _, e := range out.Evidence {
						if strings.TrimSpace(e) != "" {
							evidence = append(evidence, model.Evidence{Source: "agent", Value: strings.TrimSpace(e)})
						}
					}
					rawEv, _ := json.Marshal(evidence)
					return rt.Code, clampConfidence(out.Confidence), nonEmpty(out.Reason, "связь по совместным покупкам"), string(rawEv), "co_purchase_agent"
				}
			}
		}
	}
	// Fallback: use the constructor's types plus simple title heuristics.
	code := "adjacent_task"
	la, lb := strings.ToLower(pair.TitleA), strings.ToLower(pair.TitleB)
	tier := func(s string) bool {
		return strings.Contains(s, "starter") || strings.Contains(s, "basic") ||
			strings.Contains(s, "standard") || strings.Contains(s, "business") ||
			strings.Contains(s, "pro") || strings.Contains(s, "enterprise")
	}
	if tier(la) && tier(lb) && (strings.Contains(la, "starter") || strings.Contains(lb, "starter")) {
		if _, ok := findRelationType(types, "upgrade"); ok {
			code = "upgrade"
		}
	}
	if strings.Contains(la, "дистрибутив") && strings.Contains(lb, "лицензия") ||
		strings.Contains(lb, "дистрибутив") && strings.Contains(la, "лицензия") {
		if _, ok := findRelationType(types, "accessory"); ok {
			code = "accessory"
		}
	}
	if _, ok := findRelationType(types, code); !ok {
		for _, rt := range types {
			if rt.Behavior == "cross_sell" {
				code = rt.Code
				break
			}
		}
	}
	evidence, _ := json.Marshal([]model.Evidence{{Source: "co_purchase", Value: fmt.Sprintf("куплено вместе %d раз", pair.Count)}})
	return code, clampConfidence(0.55 + float64(pair.Count)*0.02), "связь по совместным покупкам, автоматическая классификация недоступна", string(evidence), "co_purchase_fallback"
}

func (h *handler) adminDatasetCandidates(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			limit = v
		}
	}
	audience := r.URL.Query().Get("audience")
	items, err := catalog.DatasetCandidates(r.Context(), h.db, limit, audience)
	if err != nil {
		h.logger.Error("dataset candidates", "err", err)
		writeError(w, http.StatusInternalServerError, "dataset candidates error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *handler) adminReviewDatasetCandidate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID       int64  `json:"id"`
		Decision string `json:"decision"`
		Note     string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	status := "confirmed"
	if req.Decision == "reject" {
		status = "rejected"
	} else if req.Decision != "confirm" {
		writeError(w, http.StatusBadRequest, "decision must be confirm or reject")
		return
	}
	ok, err := catalog.ReviewDatasetCandidate(r.Context(), h.db, req.ID, status, strings.TrimSpace(req.Note))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "review error")
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "candidate not found")
		return
	}
	_ = catalog.LogAdminChange(r.Context(), h.db, "admin", "dataset.review", req)
	writeJSON(w, http.StatusOK, map[string]bool{"reviewed": true})
}
