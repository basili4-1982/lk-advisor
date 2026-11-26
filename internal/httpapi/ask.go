package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/basili4-1982/lk-advisor/internal/agent"
	"github.com/basili4-1982/lk-advisor/internal/profile"
)

func (h *handler) ask(w http.ResponseWriter, r *http.Request) {
	var req agent.AskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if h.db == nil {
		writeError(w, http.StatusInternalServerError, "database is not configured")
		return
	}

	a := agent.NewWithDeps(h.db, h.now, h.llm)
	a.Logger = h.logger
	resp, err := a.Ask(r.Context(), req)
	switch {
	case errors.Is(err, agent.ErrInvalid):
		writeError(w, http.StatusBadRequest, "client_id and question are required")
		return
	case errors.Is(err, agent.ErrClientMismatch):
		writeError(w, http.StatusForbidden, "session belongs to another client")
		return
	case errors.Is(err, profile.ErrNotFound):
		writeError(w, http.StatusNotFound, "client not found")
		return
	case err != nil:
		h.logger.Error("ask", "err", err)
		writeError(w, http.StatusInternalServerError, "agent error")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}
