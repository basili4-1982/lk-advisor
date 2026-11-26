package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/basili4-1982/lk-advisor/internal/catalog"
)

func (h *handler) adminRelationScopes(w http.ResponseWriter, r *http.Request) {
	limit := 200
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			limit = v
		}
	}
	items, err := catalog.LoadRelationScopes(r.Context(), h.db, limit)
	if err != nil {
		h.logger.Error("relation scopes", "err", err)
		writeError(w, http.StatusInternalServerError, "relation scopes error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// adminReviewRelationScope applies a human decision to one audience/industry scope.
func (h *handler) adminReviewRelationScope(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FromID   string `json:"from_id"`
		ToID     string `json:"to_id"`
		Type     string `json:"type"`
		Audience string `json:"audience"`
		Industry string `json:"industry"`
		Decision string `json:"decision"`
		Note     string `json:"note"`
		TTLDays  int    `json:"ttl_days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.FromID = strings.TrimSpace(req.FromID)
	req.ToID = strings.TrimSpace(req.ToID)
	req.Type = strings.TrimSpace(req.Type)
	req.Audience = strings.ToLower(strings.TrimSpace(req.Audience))
	req.Industry = strings.ToLower(strings.TrimSpace(req.Industry))
	if req.FromID == "" || req.ToID == "" || req.Type == "" {
		writeError(w, http.StatusBadRequest, "from_id, to_id and type are required")
		return
	}
	status := ""
	switch req.Decision {
	case "confirm":
		status = "confirmed"
	case "reject":
		status = "rejected"
	case "quarantine":
		status = "quarantine"
	case "reopen":
		status = "testing"
	default:
		writeError(w, http.StatusBadRequest, "decision must be confirm, reject, quarantine or reopen")
		return
	}
	ttl := time.Duration(req.TTLDays) * 24 * time.Hour
	ok, err := catalog.ReviewRelationScope(r.Context(), h.db, req.FromID, req.ToID, req.Type,
		req.Audience, req.Industry, status, strings.TrimSpace(req.Note), ttl)
	if err != nil {
		h.logger.Error("review relation scope", "err", err)
		writeError(w, http.StatusInternalServerError, "review error")
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "relation scope not found")
		return
	}
	_ = catalog.LogAdminChange(r.Context(), h.db, "admin", "relation_scope.review", req)
	writeJSON(w, http.StatusOK, map[string]bool{"reviewed": true})
}
