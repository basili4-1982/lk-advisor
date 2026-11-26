package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/basili4-1982/lk-advisor/internal/catalog"
)

func (h *handler) adminCreateDatasetRelation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FromItemID string  `json:"from_item_id"`
		ToItemID   string  `json:"to_item_id"`
		Type       string  `json:"type"`
		Status     string  `json:"status"`
		Confidence float64 `json:"confidence"`
		Reason     string  `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.FromItemID = strings.TrimSpace(req.FromItemID)
	req.ToItemID = strings.TrimSpace(req.ToItemID)
	req.Type = strings.TrimSpace(req.Type)
	if req.Status == "" {
		req.Status = "candidate"
	}
	ok, err := catalog.CreateManualDatasetRelation(r.Context(), h.db,
		req.FromItemID, req.ToItemID, req.Type, req.Status, req.Confidence, req.Reason)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusConflict, "relation not created")
		return
	}
	_ = catalog.LogAdminChange(r.Context(), h.db, "admin", "dataset.relation.create", req)
	writeJSON(w, http.StatusOK, map[string]bool{"created": true})
}
