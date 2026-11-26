package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/basili4-1982/lk-advisor/internal/catalog"
	"github.com/basili4-1982/lk-advisor/internal/model"
	"github.com/basili4-1982/lk-advisor/internal/relationtype"
)

type adminOperation struct {
	Action         string         `json:"action"`
	FromID         string         `json:"from_id,omitempty"`
	ToID           string         `json:"to_id,omitempty"`
	Type           string         `json:"type,omitempty"`
	Status         string         `json:"status,omitempty"`
	Confidence     float64        `json:"confidence,omitempty"`
	RuleType       string         `json:"rule_type,omitempty"`
	ProductID      string         `json:"product_id,omitempty"`
	OtherProductID string         `json:"other_product_id,omitempty"`
	Discount       float64        `json:"discount,omitempty"`
	RuleID         string         `json:"rule_id,omitempty"`
	Weights        *model.Weights `json:"weights,omitempty"`
	Summary        string         `json:"summary,omitempty"`
}

type adminProposal struct {
	Explanation string           `json:"explanation"`
	Operations  []adminOperation `json:"operations"`
	Source      string           `json:"source"`
}

func (h *handler) adminPropose(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Text = strings.TrimSpace(req.Text)
	if req.Text == "" {
		writeError(w, http.StatusBadRequest, "опишите изменение")
		return
	}
	st, err := h.loadAdminState(r.Context())
	if err != nil {
		h.logger.Error("admin state", "err", err)
		writeError(w, http.StatusInternalServerError, "admin state error")
		return
	}

	props := h.askAssistant(r.Context(), req.Text, st)
	props.Operations = h.enrichOperations(st, props.Operations)
	writeJSON(w, http.StatusOK, props)
}

func (h *handler) adminApply(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Operations []adminOperation `json:"operations"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if len(req.Operations) == 0 {
		writeError(w, http.StatusBadRequest, "нет изменений")
		return
	}
	st, err := h.loadAdminState(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "admin state error")
		return
	}
	ops := h.enrichOperations(st, req.Operations)
	if err := h.validateOperations(st, ops); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	applied := 0
	for _, op := range ops {
		if err := h.applyOperation(r.Context(), st, op); err != nil {
			h.logger.Error("apply operation", "action", op.Action, "err", err)
			writeError(w, http.StatusInternalServerError, "не удалось применить: "+err.Error())
			return
		}
		applied++
	}
	_ = catalog.LogAdminChange(r.Context(), h.db, "assistant", "agent.apply", ops)
	writeJSON(w, http.StatusOK, map[string]any{"applied": applied})
}

func (h *handler) askAssistant(ctx context.Context, text string, st adminState) adminProposal {
	if h.llm != nil {
		if out, err := h.llmProposal(ctx, text, st); err == nil {
			out.Operations = h.enrichOperations(st, out.Operations)
			if err := h.validateOperations(st, out.Operations); err == nil {
				return out
			} else {
				h.logger.Warn("admin llm validation", "err", err)
			}
		} else {
			h.logger.Warn("admin llm", "err", err)
		}
	}
	return parseProposal(text, st)
}

func (h *handler) llmProposal(ctx context.Context, text string, st adminState) (adminProposal, error) {
	system := `Ты редактор правил рекомендаций в магазине ПО. Верни ТОЛЬКО JSON без markdown.
Схема: {"explanation":"кратко по-русски","operations":[...]}.
Допустимые операции:
{"action":"add_relation","from_id":"id","to_id":"id","type":"код из реестра типов","confidence":0.9,"status":"confirmed"}
{"action":"delete_relation","from_id":"id","to_id":"id","type":"тип или пусто"}
{"action":"add_rule","rule_type":"out_of_stock|manager_only|excludes|export_control","product_id":"id","other_product_id":"id для excludes"}
{"action":"add_rule","rule_type":"max_discount","discount":10}
{"action":"delete_rule","rule_id":"rule-id"}
{"action":"set_weights","weights":{"relevance":0.35,"urgency":0.25,"affinity":0.2,"margin":0.1,"confidence":0.1}}
Используй только id из каталога. "X замена Y" => from_id=Y, to_id=X, type=substitute. "X дополняет Y" => from_id=Y, to_id=X, type=adjacent_task. "X апгрейд Y" => from_id=Y, to_id=X, type=upgrade. "не предлагать X" => add_rule out_of_stock product_id=X. Ничего не выдумывай. Никогда не удаляй существующие правила или связи, если пользователь явно не написал «удали» или «убери». Если изменение невозможно, верни пустой operations и объясни почему.`

	var b strings.Builder
	b.WriteString("Каталог (id | название | категория | подкатегория):\n")
	for _, p := range st.Products {
		fmt.Fprintf(&b, "%s | %s | %s | %s\n", p.ID, p.Title, p.Category, p.Subcategory)
	}
	b.WriteString("\nШаблоны связей:\n")
	b.WriteString(relationtype.Prompt(st.RelationTypes))
	b.WriteString("\nПравила:\n")
	for _, r := range st.Rules {
		fmt.Fprintf(&b, "%s | %s | %s\n", r.ID, r.Summary, r.Condition)
	}
	w := st.Weights
	fmt.Fprintf(&b, "\nВеса: relevance=%.2f urgency=%.2f affinity=%.2f margin=%.2f confidence=%.2f\n",
		w.Relevance, w.Urgency, w.Affinity, w.Margin, w.Confidence)
	b.WriteString("\nЗапрос администратора: ")
	b.WriteString(text)

	raw, err := h.llm.Complete(ctx, system, b.String())
	if err != nil {
		return adminProposal{}, err
	}
	var out adminProposal
	if err := decodeJSONObject(raw, &out); err != nil {
		return adminProposal{}, err
	}
	out.Operations = withoutUnrequestedDeletes(text, out.Operations)
	if len(out.Operations) == 0 {
		return adminProposal{}, fmt.Errorf("llm returned no operations")
	}
	out.Source = "llm"
	return out, nil
}

func withoutUnrequestedDeletes(text string, ops []adminOperation) []adminOperation {
	lower := strings.ToLower(text)
	askedDelete := strings.Contains(lower, "удали") || strings.Contains(lower, "удалить") ||
		strings.Contains(lower, "убери") || strings.Contains(lower, "убрать")
	if askedDelete {
		return ops
	}
	out := make([]adminOperation, 0, len(ops))
	for _, op := range ops {
		if op.Action == "delete_rule" || op.Action == "delete_relation" {
			continue
		}
		out = append(out, op)
	}
	return out
}

func decodeJSONObject(raw string, dst any) error {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return fmt.Errorf("no json object in answer")
	}
	return json.Unmarshal([]byte(raw[start:end+1]), dst)
}

var numberRe = regexp.MustCompile(`(\d+(?:[.,]\d+)?)`)

func parseProposal(text string, st adminState) adminProposal {
	lower := strings.ToLower(text)
	ids := mentionOrder(text, st.Products)
	byID := productIndex(st)
	title := func(id string) string {
		if p, ok := byID[id]; ok {
			return p.Title
		}
		return id
	}
	out := adminProposal{Source: "fallback"}

	add := func(op adminOperation) {
		out.Operations = append(out.Operations, op)
	}

	switch {
	case strings.Contains(lower, "вес") || strings.Contains(lower, "релевант") ||
		strings.Contains(lower, "срочн") || strings.Contains(lower, "аффин") ||
		strings.Contains(lower, "марж"):
		return parseWeights(text, st)
	case strings.Contains(lower, "скидк"):
		if m := numberRe.FindString(lower); m != "" {
			v, _ := strconv.ParseFloat(strings.ReplaceAll(m, ",", "."), 64)
			if v > 0 && v <= 100 {
				add(adminOperation{Action: "add_rule", RuleType: "max_discount", Discount: v})
				out.Explanation = fmt.Sprintf("Ограничу скидку значением %.0f%%.", v)
				return out
			}
		}
	case (strings.Contains(lower, "согласован") || strings.Contains(lower, "менеджер")) && len(ids) > 0:
		add(adminOperation{Action: "add_rule", RuleType: "manager_only", ProductID: ids[0]})
		out.Explanation = fmt.Sprintf("Добавлю обязательное согласование для %s.", title(ids[0]))
		return out
	case (strings.Contains(lower, "не предлага") || strings.Contains(lower, "нет в наличии") ||
		strings.Contains(lower, "скрой") ||
		((strings.Contains(lower, "убери") || strings.Contains(lower, "исключи")) && strings.Contains(lower, "рекоменд"))) && len(ids) > 0:
		add(adminOperation{Action: "add_rule", RuleType: "out_of_stock", ProductID: ids[0]})
		out.Explanation = fmt.Sprintf("Уберу %s из рекомендаций как недоступный.", title(ids[0]))
		return out
	case (strings.Contains(lower, "удали") || strings.Contains(lower, "убери")) &&
		strings.Contains(lower, "связ") && len(ids) >= 2:
		add(adminOperation{Action: "delete_relation", FromID: ids[1], ToID: ids[0]})
		out.Explanation = fmt.Sprintf("Удалю связь между %s и %s.", title(ids[0]), title(ids[1]))
		return out
	}

	if len(ids) < 2 {
		out.Explanation = "Не понял, какие два товара связаны. Укажите названия из каталога и тип связи: замена, дополнение или апгрейд."
		return out
	}

	typ := ""
	confidence := 0.9
	if code, conf, ok := matchCustomType(lower, st.RelationTypes); ok {
		typ, confidence = code, conf
	} else {
		switch {
		case strings.Contains(lower, "замен") || strings.Contains(lower, "конкурент") || strings.Contains(lower, "вместо"):
			typ = "substitute"
		case strings.Contains(lower, "апгрейд") || strings.Contains(lower, "старш"):
			typ = "upgrade"
		case strings.Contains(lower, "дополн"):
			typ = "adjacent_task"
		}
	}
	if typ == "" {
		out.Explanation = "Не понял тип связи. Напишите: «X — замена Y», «X дополняет Y», «X — апгрейд Y» или название своего шаблона."
		return out
	}
	add(adminOperation{
		Action: "add_relation", FromID: ids[1], ToID: ids[0],
		Type: typ, Status: "confirmed", Confidence: confidence,
	})
	out.Explanation = fmt.Sprintf("Свяжу %s → %s как «%s».", title(ids[1]), title(ids[0]), relationTypeRu(typ))
	return out
}

func matchCustomType(lower string, types []model.RelationType) (string, float64, bool) {
	for _, rt := range types {
		if rt.Status != "active" || rt.Builtin || strings.TrimSpace(rt.Title) == "" {
			continue
		}
		if titleMatches(lower, rt.Title) {
			conf := rt.DefaultConfidence
			if conf <= 0 {
				conf = 0.8
			}
			return rt.Code, conf, true
		}
	}
	return "", 0, false
}

func titleMatches(text, title string) bool {
	t := strings.ToLower(title)
	if strings.Contains(text, t) {
		return true
	}
	words := strings.Fields(t)
	significant, matched := 0, 0
	for _, w := range words {
		runes := []rune(w)
		if len(runes) < 5 {
			continue
		}
		significant++
		if strings.Contains(text, string(runes[:5])) {
			matched++
		}
	}
	return significant > 0 && matched == significant
}

func parseWeights(text string, st adminState) adminProposal {
	lower := strings.ToLower(text)
	w := st.Weights
	set := func(key string, value float64) {
		if value > 1 {
			value /= 100
		}
		switch key {
		case "релевант":
			w.Relevance = value
		case "срочн":
			w.Urgency = value
		case "аффин":
			w.Affinity = value
		case "марж":
			w.Margin = value
		case "уверен":
			w.Confidence = value
		}
	}
	found := false
	for _, part := range strings.FieldsFunc(lower, func(r rune) bool { return r == ',' || r == ';' || r == '\n' }) {
		m := numberRe.FindString(part)
		if m == "" {
			continue
		}
		v, _ := strconv.ParseFloat(m, 64)
		switch {
		case strings.Contains(part, "релевант"):
			set("релевант", v)
			found = true
		case strings.Contains(part, "срочн"):
			set("срочн", v)
			found = true
		case strings.Contains(part, "аффин"):
			set("аффин", v)
			found = true
		case strings.Contains(part, "марж"):
			set("марж", v)
			found = true
		case strings.Contains(part, "уверен"):
			set("уверен", v)
			found = true
		}
	}
	if !found {
		return adminProposal{Source: "fallback", Explanation: "Не понял новые веса. Например: «релевантность 40, срочность 20, аффинность 20, маржа 10, уверенность 10»."}
	}
	w = w.Normalize()
	return adminProposal{
		Source:      "fallback",
		Explanation: "Обновлю веса и пересчитаю рекомендации.",
		Operations:  []adminOperation{{Action: "set_weights", Weights: &w}},
	}
}

func mentionOrder(text string, products []adminProduct) []string {
	lower := strings.ToLower(text)
	type hit struct {
		id  string
		pos int
	}
	var hits []hit
	for _, p := range products {
		title := strings.ToLower(p.Title)
		if len(title) < 3 {
			continue
		}
		if pos := strings.Index(lower, title); pos >= 0 {
			hits = append(hits, hit{id: p.ID, pos: pos})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].pos != hits[j].pos {
			return hits[i].pos < hits[j].pos
		}
		return hits[i].id < hits[j].id
	})
	out := make([]string, 0, len(hits))
	seen := map[string]bool{}
	for _, h := range hits {
		if seen[h.id] {
			continue
		}
		seen[h.id] = true
		out = append(out, h.id)
	}
	return out
}

func productIndex(st adminState) map[string]adminProduct {
	out := make(map[string]adminProduct, len(st.Products))
	for _, p := range st.Products {
		out[p.ID] = p
	}
	return out
}

func (h *handler) enrichOperations(st adminState, ops []adminOperation) []adminOperation {
	byID := productIndex(st)
	title := func(id string) string {
		if p, ok := byID[id]; ok {
			return p.Title
		}
		return id
	}
	out := make([]adminOperation, len(ops))
	copy(out, ops)
	for i := range out {
		op := &out[i]
		switch op.Action {
		case "add_relation":
			if op.Status == "" {
				op.Status = "confirmed"
			}
			if op.Confidence == 0 {
				op.Confidence = 0.8
			}
			op.Summary = fmt.Sprintf("Добавить связь: %s → %s · %s · %.0f%% · %s",
				title(op.FromID), title(op.ToID), relationTypeRu(op.Type), op.Confidence*100, relationStatusRu(op.Status))
		case "delete_relation":
			kind := relationTypeRu(op.Type)
			if op.Type == "" {
				kind = "все типы"
			}
			op.Summary = fmt.Sprintf("Удалить связь: %s → %s · %s", title(op.FromID), title(op.ToID), kind)
		case "add_rule":
			switch op.RuleType {
			case "excludes":
				op.Summary = fmt.Sprintf("Запретить совместную продажу: %s + %s", title(op.ProductID), title(op.OtherProductID))
			case "max_discount":
				op.Summary = fmt.Sprintf("Ограничить скидку: не больше %.0f%%", op.Discount)
			case "manager_only":
				op.Summary = fmt.Sprintf("Требовать согласование менеджера: %s", title(op.ProductID))
			case "out_of_stock":
				op.Summary = fmt.Sprintf("Убрать из выдачи: %s", title(op.ProductID))
			case "export_control":
				op.Summary = fmt.Sprintf("Экспортный контроль: %s", title(op.ProductID))
			default:
				op.Summary = "Добавить правило: " + op.RuleType
			}
		case "delete_rule":
			for _, r := range st.Rules {
				if r.ID == op.RuleID {
					op.Summary = "Удалить правило: " + r.Summary
					break
				}
			}
			if op.Summary == "" {
				op.Summary = "Удалить правило: " + op.RuleID
			}
		case "set_weights":
			if op.Weights != nil {
				w := op.Weights.Normalize()
				op.Weights = &w
				op.Summary = fmt.Sprintf("Веса: релевантность %.0f%%, срочность %.0f%%, аффинность %.0f%%, маржа %.0f%%, уверенность %.0f%%",
					w.Relevance*100, w.Urgency*100, w.Affinity*100, w.Margin*100, w.Confidence*100)
			}
		}
	}
	return out
}

func (st adminState) knownType(code string) (model.RelationType, bool) {
	for _, rt := range st.RelationTypes {
		if rt.Code == code {
			return rt, true
		}
	}
	for _, rt := range relationtype.Builtins() {
		if rt.Code == code {
			return rt, true
		}
	}
	return model.RelationType{}, false
}

func (h *handler) validateOperations(st adminState, ops []adminOperation) error {
	byID := productIndex(st)
	required := func(id, what string) error {
		if id == "" {
			return fmt.Errorf("не указан %s", what)
		}
		if _, ok := byID[id]; !ok {
			return fmt.Errorf("товар %q не найден", id)
		}
		return nil
	}
	for _, op := range ops {
		switch op.Action {
		case "add_relation":
			if err := required(op.FromID, "from_id"); err != nil {
				return err
			}
			if err := required(op.ToID, "to_id"); err != nil {
				return err
			}
			if op.FromID == op.ToID {
				return fmt.Errorf("нельзя связать товар с самим собой")
			}
			rt, ok := st.knownType(op.Type)
			if !ok || rt.Status != "active" {
				return fmt.Errorf("неизвестный или отключённый тип связи: %s", op.Type)
			}
			if !relationStatuses[op.Status] {
				return fmt.Errorf("неизвестный статус связи: %s", op.Status)
			}
			if op.Confidence < 0 || op.Confidence > 1 {
				return fmt.Errorf("confidence должен быть от 0 до 1")
			}
		case "delete_relation":
			if err := required(op.FromID, "from_id"); err != nil {
				return err
			}
			if err := required(op.ToID, "to_id"); err != nil {
				return err
			}
			if op.Type != "" {
				if _, ok := st.knownType(op.Type); !ok {
					return fmt.Errorf("неизвестный тип связи: %s", op.Type)
				}
			}
		case "add_rule":
			switch op.RuleType {
			case "excludes":
				if err := required(op.ProductID, "product_id"); err != nil {
					return err
				}
				if err := required(op.OtherProductID, "other_product_id"); err != nil {
					return err
				}
				if op.ProductID == op.OtherProductID {
					return fmt.Errorf("для исключения нужны два разных товара")
				}
			case "max_discount":
				if op.Discount <= 0 || op.Discount > 100 {
					return fmt.Errorf("скидка должна быть от 0 до 100")
				}
			case "out_of_stock", "manager_only", "export_control":
				if err := required(op.ProductID, "product_id"); err != nil {
					return err
				}
			default:
				return fmt.Errorf("неизвестный тип правила: %s", op.RuleType)
			}
		case "delete_rule":
			if op.RuleID == "" {
				return fmt.Errorf("не указан rule_id")
			}
			found := false
			for _, r := range st.Rules {
				if r.ID == op.RuleID {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("правило %q не найдено", op.RuleID)
			}
		case "set_weights":
			if op.Weights == nil || op.Weights.IsZero() {
				return fmt.Errorf("веса не могут быть нулевыми")
			}
		default:
			return fmt.Errorf("неизвестная операция: %s", op.Action)
		}
	}
	return nil
}

func (h *handler) applyOperation(ctx context.Context, st adminState, op adminOperation) error {
	switch op.Action {
	case "add_relation":
		rel := model.Relation{
			FromID: op.FromID, ToID: op.ToID, Type: op.Type,
			Direction: "forward", Confidence: op.Confidence,
			Status: op.Status, Source: "agent",
		}
		if err := catalog.UpsertRelation(ctx, h.db, rel); err != nil {
			return err
		}
		if rt, ok := st.knownType(op.Type); ok && rt.Direction == "symmetric" {
			reverse := rel
			reverse.FromID, reverse.ToID = rel.ToID, rel.FromID
			if err := catalog.UpsertRelation(ctx, h.db, reverse); err != nil {
				return err
			}
		}
		return nil
	case "delete_relation":
		if op.Type == "" {
			_, err := h.db.ExecContext(ctx,
				`DELETE FROM product_relations WHERE from_id = $1 AND to_id = $2`, op.FromID, op.ToID)
			return err
		}
		ok, err := catalog.DeleteRelation(ctx, h.db, op.FromID, op.ToID, op.Type)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("связь не найдена")
		}
		return nil
	case "add_rule":
		rule, err := ruleFromOperation(op)
		if err != nil {
			return err
		}
		for _, old := range st.Rules {
			if old.Type == rule.Type && old.Condition == rule.Condition {
				return fmt.Errorf("такое правило уже есть")
			}
		}
		return catalog.InsertRule(ctx, h.db, rule)
	case "delete_rule":
		ok, err := catalog.DeleteRule(ctx, h.db, op.RuleID)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("правило не найдено")
		}
		return nil
	case "set_weights":
		return catalog.SaveWeights(ctx, h.db, *op.Weights)
	}
	return fmt.Errorf("неизвестная операция: %s", op.Action)
}

func ruleFromOperation(op adminOperation) (model.Rule, error) {
	rule := model.Rule{ID: newAdminID("rule-adm"), Type: op.RuleType}
	switch op.RuleType {
	case "excludes":
		rule.Condition = "pair=" + op.ProductID + ":" + op.OtherProductID
		rule.Action = "exclude"
	case "max_discount":
		rule.Condition = fmt.Sprintf("discount>%.0f", op.Discount)
		rule.Action = "cap"
	case "manager_only":
		rule.Condition = "product=" + op.ProductID
		rule.Action = "approval"
	case "out_of_stock", "export_control":
		rule.Condition = "product=" + op.ProductID
		rule.Action = "exclude"
	default:
		return model.Rule{}, fmt.Errorf("неизвестный тип правила: %s", op.RuleType)
	}
	return rule, nil
}

func relationTypeRu(typ string) string {
	switch typ {
	case "upgrade":
		return "апгрейд"
	case "adjacent_task":
		return "дополнение"
	case "substitute", "same_task":
		return "замена"
	case "bundle":
		return "комплект"
	case "accessory":
		return "аксессуар"
	case "workflow_next_step":
		return "следующий шаг"
	case "analog":
		return "аналог"
	case "excludes":
		return "несовместимость"
	default:
		return typ
	}
}

func relationStatusRu(status string) string {
	switch status {
	case "confirmed":
		return "включена"
	case "llm_suggested":
		return "предложена ИИ"
	case "quarantine":
		return "на проверке"
	case "rejected":
		return "отключена"
	default:
		return status
	}
}
