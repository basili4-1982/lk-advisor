package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/basili4-1982/lk-advisor/internal/catalog"
)

func (h *handler) adminDatasetCoverage(w http.ResponseWriter, r *http.Request) {
	c, err := catalog.LoadDatasetCoverage(r.Context(), h.db)
	if err != nil {
		h.logger.Error("dataset coverage", "err", err)
		writeError(w, http.StatusInternalServerError, "dataset coverage error")
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (h *handler) adminCoPurchases(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			limit = v
		}
	}
	items, err := catalog.TopCoPurchases(r.Context(), h.db, limit)
	if err != nil {
		h.logger.Error("co-purchases", "err", err)
		writeError(w, http.StatusInternalServerError, "co-purchases error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *handler) adminSuggestCoPurchases(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Limit    int `json:"limit"`
		MinCount int `json:"min_count"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Limit <= 0 {
		req.Limit = 100
	}
	if req.MinCount <= 0 {
		req.MinCount = 2
	}
	created, err := catalog.CreateCoPurchaseCandidates(r.Context(), h.db, req.Limit, req.MinCount)
	if err != nil {
		h.logger.Error("suggest co-purchases", "err", err)
		writeError(w, http.StatusInternalServerError, "suggest error")
		return
	}
	_ = catalog.LogAdminChange(r.Context(), h.db, "dataset-agent", "dataset.co_purchase_suggest",
		map[string]int{"limit": req.Limit, "min_count": req.MinCount, "created": created})
	c, _ := catalog.LoadDatasetCoverage(r.Context(), h.db)
	writeJSON(w, http.StatusOK, map[string]any{"created": created, "coverage": c})
}
