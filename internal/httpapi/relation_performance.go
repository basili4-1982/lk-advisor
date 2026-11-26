package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/basili4-1982/lk-advisor/internal/catalog"
)

func (h *handler) adminRelationPerformance(w http.ResponseWriter, r *http.Request) {
	items, err := catalog.LoadRelationPerformance(r.Context(), h.db, 200)
	if err != nil {
		h.logger.Error("relation performance", "err", err)
		writeError(w, http.StatusInternalServerError, "relation performance error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *handler) adminAutoReviewRelations(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MinShown int     `json:"min_shown"`
		MinCTR   float64 `json:"min_ctr"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.MinShown <= 0 {
		req.MinShown = 20
	}
	if req.MinCTR <= 0 {
		req.MinCTR = 0.03
	}
	promoted, rejected, err := catalog.AutoReviewRelations(r.Context(), h.db, req.MinShown, req.MinCTR)
	if err != nil {
		h.logger.Error("auto review relations", "err", err)
		writeError(w, http.StatusInternalServerError, "auto review error")
		return
	}
	_ = catalog.LogAdminChange(r.Context(), h.db, "bandit", "relations.auto_review",
		map[string]any{"promoted": promoted, "rejected": rejected, "min_shown": req.MinShown, "min_ctr": req.MinCTR})
	writeJSON(w, http.StatusOK, map[string]int{"promoted": promoted, "rejected": rejected})
}
