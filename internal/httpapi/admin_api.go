package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/basili4-1982/lk-advisor/internal/catalog"
	"github.com/basili4-1982/lk-advisor/internal/model"
	"github.com/basili4-1982/lk-advisor/internal/relationtype"
)

var relationStatuses = map[string]bool{
	"confirmed": true, "llm_suggested": true, "quarantine": true, "rejected": true,
}

var ruleTypes = map[string]bool{
	"excludes": true, "export_control": true, "out_of_stock": true,
	"manager_only": true, "max_discount": true,
}

type adminProduct struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Category    string `json:"category"`
	Subcategory string `json:"subcategory"`
	Audience    string `json:"audience"`
	Status      string `json:"status"`
	Price       int64  `json:"price"`
}

type adminRule struct {
	model.Rule
	Summary        string  `json:"summary"`
	ProductID      string  `json:"product_id,omitempty"`
	OtherProductID string  `json:"other_product_id,omitempty"`
	Discount       float64 `json:"discount,omitempty"`
}

type adminState struct {
	Products      []adminProduct            `json:"products"`
	Relations     []model.Relation          `json:"relations"`
	Rules         []adminRule               `json:"rules"`
	Weights       model.Weights             `json:"weights"`
	Clients       []model.Client            `json:"clients"`
	Audit         []catalog.AdminAuditEntry `json:"audit"`
	RelationTypes []model.RelationType      `json:"relation_types"`
}

func (h *handler) adminState(w http.ResponseWriter, r *http.Request) {
	st, err := h.loadAdminState(r.Context())
	if err != nil {
		h.logger.Error("admin state", "err", err)
		writeError(w, http.StatusInternalServerError, "admin state error")
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (h *handler) loadAdminState(ctx context.Context) (adminState, error) {
	products, err := catalog.AllProducts(ctx, h.db)
	if err != nil {
		return adminState{}, err
	}
	relations, err := catalog.AllRelations(ctx, h.db)
	if err != nil {
		return adminState{}, err
	}
	rules, err := catalog.Rules(ctx, h.db)
	if err != nil {
		return adminState{}, err
	}
	weights, err := catalog.LoadWeights(ctx, h.db)
	if err != nil {
		return adminState{}, err
	}
	clients, err := catalog.AllClients(ctx, h.db)
	if err != nil {
		return adminState{}, err
	}
	audit, err := catalog.AdminAudit(ctx, h.db, 20)
	if err != nil {
		return adminState{}, err
	}
	relationTypes, err := catalog.RelationTypes(ctx, h.db)
	if err != nil {
		return adminState{}, err
	}

	compact := make([]adminProduct, 0, len(products))
	list := make([]model.Product, 0, len(products))
	for _, p := range products {
		list = append(list, p)
		compact = append(compact, adminProduct{
			ID: p.ID, Title: p.Title, Category: p.Category, Subcategory: p.Subcategory,
			Audience: p.Audience, Status: p.Status, Price: p.Price,
		})
	}
	byID := map[string]model.Product{}
	for _, p := range list {
		byID[p.ID] = p
	}
	enriched := make([]adminRule, 0, len(rules))
	for _, rule := range rules {
		enriched = append(enriched, enrichRule(rule, byID))
	}
	return adminState{
		Products: compact, Relations: relations, Rules: enriched,
		Weights: weights, Clients: clients, Audit: audit, RelationTypes: relationTypes,
	}, nil
}

func enrichRule(rule model.Rule, products map[string]model.Product) adminRule {
	out := adminRule{Rule: rule}
	title := func(id string) string {
		if p, ok := products[id]; ok {
			return p.Title
		}
		return id
	}
	switch rule.Type {
	case "excludes":
		from, to := splitPair(rule.Condition)
		out.ProductID, out.OtherProductID = from, to
		out.Summary = fmt.Sprintf("Несовместимо: %s + %s", title(from), title(to))
	case "export_control":
		out.ProductID = strings.TrimPrefix(rule.Condition, "product=")
		out.Summary = fmt.Sprintf("Экспортный контроль: %s", title(out.ProductID))
	case "out_of_stock":
		out.ProductID = strings.TrimPrefix(rule.Condition, "product=")
		out.Summary = fmt.Sprintf("Нет в наличии: %s", title(out.ProductID))
	case "manager_only":
		out.ProductID = strings.TrimPrefix(rule.Condition, "product=")
		out.Summary = fmt.Sprintf("Нужно согласование менеджера: %s", title(out.ProductID))
	case "max_discount":
		out.Discount = parseDiscountValue(rule.Condition)
		out.Summary = fmt.Sprintf("Ограничение скидки: не больше %.0f%%", out.Discount)
	default:
		out.Summary = rule.Type + ": " + rule.Condition
	}
	return out
}

func splitPair(cond string) (string, string) {
	s := strings.TrimPrefix(cond, "pair=")
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func parseDiscountValue(cond string) float64 {
	s := strings.TrimPrefix(cond, "discount>")
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

type relationRequest struct {
	FromID     string           `json:"from_id"`
	ToID       string           `json:"to_id"`
	Type       string           `json:"type"`
	Direction  string           `json:"direction"`
	Confidence float64          `json:"confidence"`
	Status     string           `json:"status"`
	Reason     string           `json:"reason"`
	Evidence   []model.Evidence `json:"evidence"`
	Source     string           `json:"source"`
}

func (h *handler) adminUpsertRelation(w http.ResponseWriter, r *http.Request) {
	var req relationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.FromID = strings.TrimSpace(req.FromID)
	req.ToID = strings.TrimSpace(req.ToID)
	if req.FromID == "" || req.ToID == "" || req.FromID == req.ToID {
		writeError(w, http.StatusBadRequest, "нужны разные товары from_id и to_id")
		return
	}
	rt, err := h.relationTypeFor(r.Context(), req.Type)
	if err != nil || rt.Status != "active" {
		writeError(w, http.StatusBadRequest, "неизвестный или отключённый тип связи")
		return
	}
	if req.Status == "" {
		req.Status = "confirmed"
	}
	if !relationStatuses[req.Status] {
		writeError(w, http.StatusBadRequest, "неизвестный статус связи")
		return
	}
	if req.Direction == "" {
		req.Direction = "forward"
	}
	if req.Confidence == 0 {
		req.Confidence = 0.8
	}
	if req.Confidence < 0 || req.Confidence > 1 {
		writeError(w, http.StatusBadRequest, "confidence должен быть от 0 до 1")
		return
	}
	exists, err := h.productsExist(r.Context(), req.FromID, req.ToID)
	if err != nil || !exists {
		writeError(w, http.StatusBadRequest, "товар не найден")
		return
	}

	rel := model.Relation{
		FromID: req.FromID, ToID: req.ToID, Type: req.Type,
		Direction: req.Direction, Confidence: req.Confidence,
		Status: req.Status, Source: strings.TrimSpace(req.Source),
		Reason: strings.TrimSpace(req.Reason), Evidence: req.Evidence,
	}
	if err := catalog.UpsertRelation(r.Context(), h.db, rel); err != nil {
		h.logger.Error("upsert relation", "err", err)
		writeError(w, http.StatusInternalServerError, "relation save error")
		return
	}
	if rt.Direction == "symmetric" {
		reverse := rel
		reverse.FromID, reverse.ToID = rel.ToID, rel.FromID
		reverse.Direction = "forward"
		if err := catalog.UpsertRelation(r.Context(), h.db, reverse); err != nil {
			h.logger.Error("upsert reverse relation", "err", err)
		}
	}
	_ = catalog.LogAdminChange(r.Context(), h.db, "admin", "relation.upsert", rel)
	writeJSON(w, http.StatusOK, rel)
}

func (h *handler) adminDeleteRelation(w http.ResponseWriter, r *http.Request) {
	var req relationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	ok, err := catalog.DeleteRelation(r.Context(), h.db, req.FromID, req.ToID, req.Type)
	if err != nil {
		h.logger.Error("delete relation", "err", err)
		writeError(w, http.StatusInternalServerError, "relation delete error")
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "связь не найдена")
		return
	}
	_ = catalog.LogAdminChange(r.Context(), h.db, "admin", "relation.delete", req)
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (h *handler) productsExist(ctx context.Context, ids ...string) (bool, error) {
	for _, id := range ids {
		var one int
		err := h.db.QueryRowContext(ctx, `SELECT 1 FROM products WHERE id = $1`, id).Scan(&one)
		if errors.Is(err, context.Canceled) {
			return false, err
		}
		if err != nil {
			return false, nil
		}
	}
	return true, nil
}

type ruleRequest struct {
	Type           string  `json:"type"`
	ProductID      string  `json:"product_id"`
	OtherProductID string  `json:"other_product_id"`
	Discount       float64 `json:"discount"`
}

func (h *handler) adminCreateRule(w http.ResponseWriter, r *http.Request) {
	var req ruleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if !ruleTypes[req.Type] {
		writeError(w, http.StatusBadRequest, "неизвестный тип правила")
		return
	}

	rule := model.Rule{ID: newAdminID("rule-adm"), Type: req.Type}
	switch req.Type {
	case "excludes":
		if req.ProductID == "" || req.OtherProductID == "" || req.ProductID == req.OtherProductID {
			writeError(w, http.StatusBadRequest, "для исключения нужны два разных товара")
			return
		}
		rule.Condition = "pair=" + req.ProductID + ":" + req.OtherProductID
		rule.Action = "exclude"
	case "max_discount":
		if req.Discount <= 0 || req.Discount > 100 {
			writeError(w, http.StatusBadRequest, "скидка должна быть от 0 до 100")
			return
		}
		rule.Condition = fmt.Sprintf("discount>%.0f", req.Discount)
		rule.Action = "cap"
	default:
		if req.ProductID == "" {
			writeError(w, http.StatusBadRequest, "выберите товар")
			return
		}
		rule.Condition = "product=" + req.ProductID
		if req.Type == "manager_only" {
			rule.Action = "approval"
		} else {
			rule.Action = "exclude"
		}
	}
	var ids []string
	if req.ProductID != "" {
		ids = append(ids, req.ProductID)
	}
	if req.OtherProductID != "" {
		ids = append(ids, req.OtherProductID)
	}
	if len(ids) > 0 {
		exists, err := h.productsExist(r.Context(), ids...)
		if err != nil || !exists {
			writeError(w, http.StatusBadRequest, "товар не найден")
			return
		}
	}

	existing, err := catalog.Rules(r.Context(), h.db)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "rules error")
		return
	}
	for _, old := range existing {
		if old.Type == rule.Type && old.Condition == rule.Condition {
			writeError(w, http.StatusConflict, "такое правило уже есть")
			return
		}
	}

	if err := catalog.InsertRule(r.Context(), h.db, rule); err != nil {
		h.logger.Error("insert rule", "err", err)
		writeError(w, http.StatusInternalServerError, "rule save error")
		return
	}
	_ = catalog.LogAdminChange(r.Context(), h.db, "admin", "rule.create", rule)
	products, _ := catalog.AllProducts(r.Context(), h.db)
	byID := map[string]model.Product{}
	for _, p := range products {
		byID[p.ID] = p
	}
	writeJSON(w, http.StatusOK, enrichRule(rule, byID))
}

func (h *handler) adminDeleteRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ok, err := catalog.DeleteRule(r.Context(), h.db, id)
	if err != nil {
		h.logger.Error("delete rule", "err", err)
		writeError(w, http.StatusInternalServerError, "rule delete error")
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "правило не найдено")
		return
	}
	_ = catalog.LogAdminChange(r.Context(), h.db, "admin", "rule.delete", map[string]string{"id": id})
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (h *handler) adminSaveWeights(w http.ResponseWriter, r *http.Request) {
	var weights model.Weights
	if err := json.NewDecoder(r.Body).Decode(&weights); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if weights.IsZero() {
		writeError(w, http.StatusBadRequest, "веса не могут быть нулевыми")
		return
	}
	weights = weights.Normalize()
	if err := catalog.SaveWeights(r.Context(), h.db, weights); err != nil {
		h.logger.Error("save weights", "err", err)
		writeError(w, http.StatusInternalServerError, "weights save error")
		return
	}
	_ = catalog.LogAdminChange(r.Context(), h.db, "admin", "weights.update", weights)
	writeJSON(w, http.StatusOK, weights)
}

func (h *handler) relationTypeFor(ctx context.Context, code string) (model.RelationType, error) {
	rt, err := catalog.RelationType(ctx, h.db, code)
	if err == nil {
		return rt, nil
	}
	for _, b := range relationtype.Builtins() {
		if b.Code == code {
			return b, nil
		}
	}
	return model.RelationType{}, err
}

func (h *handler) adminUpsertRelationType(w http.ResponseWriter, r *http.Request) {
	var rt model.RelationType
	if err := json.NewDecoder(r.Body).Decode(&rt); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	rt.Code = strings.TrimSpace(rt.Code)
	rt.Title = strings.TrimSpace(rt.Title)
	rt.Description = strings.TrimSpace(rt.Description)
	rt.Examples = strings.TrimSpace(rt.Examples)
	if rt.Direction == "" {
		rt.Direction = "forward"
	}
	if rt.Status == "" {
		rt.Status = "active"
	}
	if rt.DefaultConfidence == 0 {
		rt.DefaultConfidence = 0.8
	}

	existing, err := catalog.RelationType(r.Context(), h.db, rt.Code)
	if err == nil {
		rt.Builtin = existing.Builtin
		if existing.Builtin {
			rt.Behavior = existing.Behavior
			rt.Direction = existing.Direction
			rt.DefaultConfidence = existing.DefaultConfidence
		}
	}
	if err := relationtype.Validate(rt); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := catalog.UpsertRelationType(r.Context(), h.db, rt); err != nil {
		h.logger.Error("upsert relation type", "err", err)
		writeError(w, http.StatusInternalServerError, "relation type save error")
		return
	}
	_ = catalog.LogAdminChange(r.Context(), h.db, "admin", "relation_type.upsert", rt)
	writeJSON(w, http.StatusOK, rt)
}

func (h *handler) adminDeleteRelationType(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	ok, err := catalog.DeleteRelationType(r.Context(), h.db, code)
	if err != nil {
		h.logger.Error("delete relation type", "err", err)
		writeError(w, http.StatusInternalServerError, "relation type delete error")
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "пользовательский тип не найден")
		return
	}
	_ = catalog.LogAdminChange(r.Context(), h.db, "admin", "relation_type.delete", map[string]string{"code": code})
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (h *handler) adminRelationPrompt(w http.ResponseWriter, r *http.Request) {
	types, err := catalog.RelationTypes(r.Context(), h.db)
	if err != nil {
		h.logger.Error("relation types", "err", err)
		writeError(w, http.StatusInternalServerError, "relation types error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"prompt": relationtype.Prompt(types),
	})
}

type reviewItem struct {
	model.Relation
	FromTitle string `json:"from_title"`
	ToTitle   string `json:"to_title"`
	TypeTitle string `json:"type_title"`
}

func (h *handler) adminReviewQueue(w http.ResponseWriter, r *http.Request) {
	statuses := []string{"quarantine", "llm_suggested"}
	if raw := strings.TrimSpace(r.URL.Query().Get("status")); raw != "" {
		statuses = nil
		for _, s := range strings.Split(raw, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				statuses = append(statuses, s)
			}
		}
	}
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			limit = v
		}
	}
	relations, err := catalog.ReviewRelations(r.Context(), h.db, statuses, limit)
	if err != nil {
		h.logger.Error("review queue", "err", err)
		writeError(w, http.StatusInternalServerError, "review queue error")
		return
	}
	counts, err := catalog.ReviewCounts(r.Context(), h.db)
	if err != nil {
		h.logger.Error("review counts", "err", err)
		writeError(w, http.StatusInternalServerError, "review counts error")
		return
	}
	products, _ := catalog.AllProducts(r.Context(), h.db)
	titles := make(map[string]string, len(products))
	for _, p := range products {
		titles[p.ID] = p.Title
	}
	types, _ := catalog.RelationTypes(r.Context(), h.db)
	typeTitles := make(map[string]string, len(types))
	for _, rt := range types {
		typeTitles[rt.Code] = rt.Title
	}
	items := make([]reviewItem, 0, len(relations))
	for _, rel := range relations {
		title := typeTitles[rel.Type]
		if title == "" {
			title = rel.Type
		}
		items = append(items, reviewItem{
			Relation: rel, FromTitle: titles[rel.FromID], ToTitle: titles[rel.ToID], TypeTitle: title,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "counts": counts})
}

type reviewDecisionRequest struct {
	Items    []relationRequest `json:"items"`
	Decision string            `json:"decision"`
	Note     string            `json:"note"`
}

func (h *handler) adminReviewDecision(w http.ResponseWriter, r *http.Request) {
	var req reviewDecisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if len(req.Items) == 0 {
		writeError(w, http.StatusBadRequest, "не выбрано ни одной связи")
		return
	}
	status := ""
	switch req.Decision {
	case "confirm":
		status = "confirmed"
	case "reject":
		status = "rejected"
	default:
		writeError(w, http.StatusBadRequest, "decision должен быть confirm или reject")
		return
	}
	req.Note = strings.TrimSpace(req.Note)
	if len([]rune(req.Note)) > 500 {
		writeError(w, http.StatusBadRequest, "заметка слишком длинная")
		return
	}
	reviewed := 0
	for _, item := range req.Items {
		ok, err := catalog.ReviewRelation(r.Context(), h.db, item.FromID, item.ToID, item.Type, status, req.Note)
		if err != nil {
			h.logger.Error("review decision", "err", err)
			writeError(w, http.StatusInternalServerError, "review error")
			return
		}
		if ok {
			reviewed++
		}
	}
	_ = catalog.LogAdminChange(r.Context(), h.db, "admin", "relation.review", req)
	writeJSON(w, http.StatusOK, map[string]any{"reviewed": reviewed})
}

func (h *handler) adminSoloProducts(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			limit = v
		}
	}
	items, total, err := catalog.SoloProducts(r.Context(), h.db, limit)
	if err != nil {
		h.logger.Error("solo products", "err", err)
		writeError(w, http.StatusInternalServerError, "solo products error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"total": total, "items": items})
}

func newAdminID(prefix string) string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return prefix + "-" + hex.EncodeToString(b[:])
}
