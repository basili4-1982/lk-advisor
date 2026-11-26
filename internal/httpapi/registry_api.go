package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/basili4-1982/lk-advisor/internal/catalog"
)

func registryFilterFromRequest(r *http.Request) catalog.RelationFilter {
	q := r.URL.Query()
	f := catalog.RelationFilter{
		Query:           strings.TrimSpace(q.Get("q")),
		Types:           nonEmptyValues(q["type"]),
		Statuses:        nonEmptyValues(q["status"]),
		Provenance:      nonEmptyValues(q["source"]),
		Family:          strings.TrimSpace(q.Get("family")),
		Audience:        strings.TrimSpace(q.Get("audience")),
		OrgType:         strings.TrimSpace(q.Get("org_type")),
		OwnerID:         strings.TrimSpace(q.Get("owner")),
		Sort:            strings.TrimSpace(q.Get("sort")),
		ExcludeVariants: q.Get("exclude_variants") == "1" || q.Get("exclude") == "variants",
	}
	if v, err := strconv.Atoi(q.Get("limit")); err == nil {
		f.Limit = v
	}
	if v, err := strconv.Atoi(q.Get("offset")); err == nil {
		f.Offset = v
	}
	if v, err := strconv.ParseFloat(q.Get("min_conf"), 64); err == nil {
		f.MinConfidence = v
	}
	if v, err := strconv.ParseFloat(q.Get("max_conf"), 64); err == nil {
		f.MaxConfidence = v
	}
	if v, err := strconv.Atoi(q.Get("expiring")); err == nil {
		f.ExpiringDays = v
	}
	return f
}

func nonEmptyValues(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func registryErrorStatus(err error) (int, string) {
	switch {
	case errors.Is(err, catalog.ErrNoteRequired), errors.Is(err, catalog.ErrCriterionRequired):
		return http.StatusBadRequest, err.Error()
	case errors.Is(err, catalog.ErrVersionConflict):
		return http.StatusConflict, err.Error()
	case errors.Is(err, catalog.ErrRelationNotFound):
		return http.StatusNotFound, err.Error()
	default:
		return http.StatusInternalServerError, "registry error"
	}
}

func (h *handler) adminRegistryList(w http.ResponseWriter, r *http.Request) {
	page, err := catalog.ListRegistryRelations(r.Context(), h.db, registryFilterFromRequest(r))
	if err != nil {
		h.logger.Error("registry list", "err", err)
		writeError(w, http.StatusInternalServerError, "registry list error")
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (h *handler) adminRegistryMetrics(w http.ResponseWriter, r *http.Request) {
	stats, err := catalog.RegistryMetrics(r.Context(), h.db)
	if err != nil {
		h.logger.Error("registry metrics", "err", err)
		writeError(w, http.StatusInternalServerError, "registry metrics error")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (h *handler) adminRegistryReview(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid relation id")
		return
	}
	var req struct {
		Decision        string `json:"decision"`
		Note            string `json:"note"`
		ExpectedVersion int    `json:"expected_version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	switch req.Decision {
	case "confirm", "reject", "defer":
	default:
		writeError(w, http.StatusBadRequest, "decision must be confirm, reject or defer")
		return
	}
	rel, err := catalog.ReviewRegistryRelation(r.Context(), h.db, id,
		req.Decision, strings.TrimSpace(req.Note), "admin", req.ExpectedVersion)
	if err != nil {
		if status, message := registryErrorStatus(err); status != http.StatusInternalServerError {
			writeError(w, status, message)
			return
		}
		h.logger.Error("registry review", "err", err)
		writeError(w, http.StatusInternalServerError, "registry review error")
		return
	}
	_ = catalog.LogAdminChange(r.Context(), h.db, "admin", "registry.review", map[string]any{
		"id": id, "decision": req.Decision, "note": req.Note,
	})
	writeJSON(w, http.StatusOK, rel)
}

func (h *handler) adminRegistryBulkReview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Filter   catalog.RelationFilter `json:"filter"`
		IDs      []int64                `json:"ids"`
		Decision string                 `json:"decision"`
		Note     string                 `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Decision != "confirm" && req.Decision != "reject" && req.Decision != "defer" {
		writeError(w, http.StatusBadRequest, "decision must be confirm, reject or defer")
		return
	}
	f := req.Filter
	if len(req.IDs) > 0 {
		f.IDs = req.IDs
	}
	if len(f.Statuses) == 0 && len(f.IDs) == 0 {
		f.Statuses = []string{"candidate"}
	}
	n, err := catalog.BulkReviewRelations(r.Context(), h.db, f, req.Decision,
		strings.TrimSpace(req.Note), "admin")
	if err != nil {
		if status, message := registryErrorStatus(err); status != http.StatusInternalServerError {
			writeError(w, status, message)
			return
		}
		h.logger.Error("registry bulk review", "err", err)
		writeError(w, http.StatusInternalServerError, "registry bulk review error")
		return
	}
	_ = catalog.LogAdminChange(r.Context(), h.db, "admin", "registry.bulk_review", map[string]any{
		"decision": req.Decision, "ids": req.IDs, "filter": f, "reviewed": n,
	})
	writeJSON(w, http.StatusOK, map[string]int{"reviewed": n})
}

func (h *handler) adminRegistryGenerate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MinCoPurchase   int  `json:"min_co_purchase"`
		Limit           int  `json:"limit"`
		IncludeVariants bool `json:"include_variants"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	res, err := catalog.GenerateRelations(r.Context(), h.db, catalog.GenerateOptions{
		MinCoPurchase:   req.MinCoPurchase,
		Limit:           req.Limit,
		IncludeVariants: req.IncludeVariants,
	})
	if err != nil {
		h.logger.Error("registry generate", "err", err)
		writeError(w, http.StatusInternalServerError, "registry generate error")
		return
	}
	_ = catalog.LogAdminChange(r.Context(), h.db, "admin", "registry.generate", res)
	writeJSON(w, http.StatusOK, res)
}

func (h *handler) adminRegistrySweep(w http.ResponseWriter, r *http.Request) {
	stale, expired, err := catalog.SweepRelations(r.Context(), h.db, h.now())
	if err != nil {
		h.logger.Error("registry sweep", "err", err)
		writeError(w, http.StatusInternalServerError, "registry sweep error")
		return
	}
	_ = catalog.LogAdminChange(r.Context(), h.db, "admin", "registry.sweep", map[string]int{
		"stale": stale, "expired": expired,
	})
	writeJSON(w, http.StatusOK, map[string]int{"stale": stale, "expired": expired})
}

func (h *handler) adminRegistryAlias(w http.ResponseWriter, r *http.Request) {
	itemID := strings.TrimSpace(r.PathValue("item_id"))
	var req struct {
		ShortName string `json:"short_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := catalog.UpsertItemAlias(r.Context(), h.db, itemID, req.ShortName, "admin"); err != nil {
		if errors.Is(err, catalog.ErrItemNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = catalog.LogAdminChange(r.Context(), h.db, "admin", "registry.alias", map[string]string{
		"item_id": itemID, "short_name": req.ShortName,
	})
	writeJSON(w, http.StatusOK, map[string]bool{"saved": true})
}
