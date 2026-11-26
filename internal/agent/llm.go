package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/basili4-1982/lk-advisor/internal/model"
)

func isRefusal(s string) bool {
	low := strings.ToLower(strings.TrimSpace(s))
	return strings.Contains(low, "недостаточно данных") || strings.HasPrefix(low, "нет данных")
}

const systemPrompt = `Ты — ассистент магазина ПО. Отвечай только по отчёту.
Не добавляй числа, товары и цены, которых нет в отчёте.
Если в отчёте есть кандидат и evidence — объясни причину по evidence.
Если ответа в отчёте нет — напиши: Недостаточно данных.
Не более трёх предложений.`

type report struct {
	QuestionType string            `json:"question_type"`
	Licenses     []string          `json:"licenses"`
	Candidates   []reportCandidate `json:"candidates"`
	Rejected     []model.Rejection `json:"rejected,omitempty"`
	Facts        []model.Evidence  `json:"facts"`
}

type reportCandidate struct {
	Title    string           `json:"title"`
	Type     string           `json:"type"`
	Score    float64          `json:"score"`
	Evidence []model.Evidence `json:"evidence"`
}

type formulation struct {
	Answer     string
	Raw        string
	Report     string
	GuardError string
	Failed     bool
}

func (a *Agent) formulate(ctx context.Context, question string, ex extraction, st state, base string, ev []model.Evidence) formulation {
	f := formulation{Answer: base}

	r := report{QuestionType: ex.Type, Facts: ev}
	for _, l := range st.profile.Licenses {
		r.Licenses = append(r.Licenses, fmt.Sprintf("%s, end %s, seats %d",
			l.ID, l.End.Format("2006-01-02"), l.Seats))
	}
	for _, c := range st.candidates {
		r.Candidates = append(r.Candidates, reportCandidate{
			Title: c.Product.Title, Type: c.Type, Score: c.Score, Evidence: c.Evidence,
		})
	}
	r.Rejected = st.rejected

	raw, err := json.Marshal(r)
	if err == nil {
		f.Report = string(raw)
	}
	if a.LLM == nil {
		return f
	}

	out, err := a.LLM.Complete(ctx, systemPrompt, "Вопрос: "+question+"\nОтчёт: "+string(raw))
	f.Raw = out
	if err != nil || strings.TrimSpace(out) == "" {
		f.Failed = true
		if err != nil {
			f.GuardError = err.Error()
		} else {
			f.GuardError = "empty llm answer"
		}
		return f
	}
	if isRefusal(out) && !isRefusal(base) {
		f.Failed = true
		f.GuardError = "llm refused a grounded answer"
		return f
	}

	allow := make([]string, 0, len(st.products)+len(st.candidates)+1)
	for _, p := range st.products {
		allow = append(allow, p.Title)
	}
	for _, c := range st.candidates {
		allow = append(allow, fmt.Sprintf("%.2f", c.Score))
	}
	if err := Guard(out, ev, allow...); err != nil {
		f.Failed = true
		f.GuardError = err.Error()
		return f
	}
	f.Answer = out
	return f
}
