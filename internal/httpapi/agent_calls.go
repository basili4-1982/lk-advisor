package httpapi

import (
	"encoding/json"
	"net/http"
	"time"
)

type agentCallEntry struct {
	At          time.Time       `json:"at"`
	SessionID   string          `json:"session_id"`
	Question    string          `json:"question"`
	Intent      string          `json:"intent"`
	ToolCalls   json.RawMessage `json:"tool_calls"`
	Report      json.RawMessage `json:"report"`
	BaseAnswer  string          `json:"base_answer"`
	LLMAnswer   string          `json:"llm_answer"`
	FinalAnswer string          `json:"final_answer"`
	GuardError  string          `json:"guard_error"`
	Fallback    bool            `json:"fallback"`
	LatencyMS   int             `json:"latency_ms"`
}

func (h *handler) agentCalls(w http.ResponseWriter, r *http.Request) {
	clientID := r.PathValue("id")
	if clientID == "" || h.db == nil {
		writeError(w, http.StatusBadRequest, "client id is required")
		return
	}

	rows, err := h.db.QueryContext(r.Context(), `
		SELECT at, session_id, question, intent, tool_calls, report,
		       base_answer, llm_answer, final_answer, guard_error, fallback, latency_ms
		FROM agent_calls WHERE client_id = $1
		ORDER BY at DESC, id DESC LIMIT 20`, clientID)
	if err != nil {
		h.logger.Error("agent calls", "err", err)
		writeError(w, http.StatusInternalServerError, "agent calls error")
		return
	}
	defer rows.Close()

	out := make([]agentCallEntry, 0, 20)
	for rows.Next() {
		var e agentCallEntry
		var tools, report []byte
		if err := rows.Scan(&e.At, &e.SessionID, &e.Question, &e.Intent, &tools, &report,
			&e.BaseAnswer, &e.LLMAnswer, &e.FinalAnswer, &e.GuardError, &e.Fallback, &e.LatencyMS); err != nil {
			writeError(w, http.StatusInternalServerError, "agent calls scan")
			return
		}
		e.ToolCalls = json.RawMessage(tools)
		e.Report = json.RawMessage(report)
		out = append(out, e)
	}
	writeJSON(w, http.StatusOK, out)
}
