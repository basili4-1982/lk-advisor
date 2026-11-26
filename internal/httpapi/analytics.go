package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/basili4-1982/lk-advisor/internal/analytics"
)

type eventRequest struct {
	TraceID   string `json:"trace_id"`
	ClientID  string `json:"client_id"`
	ProductID string `json:"product_id"`
	Event     string `json:"event"`
}

// recordEvent stores a click or purchase for an item shown in a recommendation.
func (h *handler) recordEvent(w http.ResponseWriter, r *http.Request) {
	var req eventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.TraceID = strings.TrimSpace(req.TraceID)
	req.ClientID = strings.TrimSpace(req.ClientID)
	req.ProductID = strings.TrimSpace(req.ProductID)
	req.Event = strings.TrimSpace(req.Event)
	if req.TraceID == "" || req.ClientID == "" || req.ProductID == "" {
		writeError(w, http.StatusBadRequest, "trace_id, client_id и product_id обязательны")
		return
	}
	if req.Event != "clicked" && req.Event != "purchased" {
		writeError(w, http.StatusBadRequest, "event должен быть clicked или purchased")
		return
	}
	if h.db == nil {
		writeError(w, http.StatusInternalServerError, "database is not configured")
		return
	}

	err := analytics.Record(r.Context(), h.db, req.TraceID, req.ClientID, req.ProductID, req.Event)
	switch {
	case errors.Is(err, analytics.ErrTraceNotFound):
		writeError(w, http.StatusNotFound, "trace_id не найден")
		return
	case errors.Is(err, analytics.ErrClientMismatch):
		writeError(w, http.StatusForbidden, "trace_id принадлежит другому клиенту")
		return
	case err != nil:
		h.logger.Error("analytics event", "err", err)
		writeError(w, http.StatusInternalServerError, "event save error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"recorded": true})
}

func (h *handler) adminAnalytics(w http.ResponseWriter, r *http.Request) {
	days := 7
	if raw := r.URL.Query().Get("days"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			days = v
		}
	}
	report, err := analytics.Build(r.Context(), h.db, days, h.now())
	if err != nil {
		h.logger.Error("analytics summary", "err", err)
		writeError(w, http.StatusInternalServerError, "analytics error")
		return
	}
	writeJSON(w, http.StatusOK, report)
}
