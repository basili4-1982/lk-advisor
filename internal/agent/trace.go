package agent

import (
	"context"
	"encoding/json"
	"time"
)

func (a *Agent) recordCall(ctx context.Context, sess session, req AskRequest, ex extraction,
	resp AskResponse, base string, f formulation, d time.Duration) {
	if a.DB == nil {
		return
	}

	tools, _ := json.Marshal(resp.ToolCalls)
	evidence, _ := json.Marshal(resp.Evidence)
	report := f.Report
	if report == "" {
		report = "{}"
	}

	_, err := a.DB.ExecContext(ctx, `
		INSERT INTO agent_calls (session_id, client_id, question, intent, tool_calls, evidence, report,
			base_answer, llm_answer, final_answer, guard_error, fallback, latency_ms)
		VALUES ($1,$2,$3,$4,$5::jsonb,$6::jsonb,$7::jsonb,$8,$9,$10,$11,$12,$13)`,
		sess.ID, req.ClientID, req.Question, ex.Type, string(tools), string(evidence), report,
		base, f.Raw, resp.Answer, f.GuardError, resp.Fallback, d.Milliseconds())
	if err != nil && a.Logger != nil {
		a.Logger.Error("agent call", "err", err)
	}
	if a.Logger != nil {
		a.Logger.Info("agent_answer",
			"client_id", req.ClientID,
			"intent", ex.Type,
			"fallback", resp.Fallback,
			"latency_ms", d.Milliseconds(),
			"guard_error", f.GuardError,
		)
	}
}
