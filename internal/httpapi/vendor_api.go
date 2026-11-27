package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/basili4-1982/lk-advisor/internal/catalog"
)

// adminVendorProducts lists external products and coverage.
func (h *handler) adminVendorProducts(w http.ResponseWriter, r *http.Request) {
	items, err := catalog.ListVendorProducts(r.Context(), h.db)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "vendor products error")
		return
	}
	stats, err := catalog.VendorKnowledgeStats(r.Context(), h.db)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "vendor stats error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "stats": stats})
}

// adminVendorImport upserts a CSV of external products.
func (h *handler) adminVendorImport(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid form")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()
	res, err := catalog.ImportVendorProductsCSV(r.Context(), h.db, file)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = catalog.LogAdminChange(r.Context(), h.db, "admin", "vendor.import", map[string]string{
		"added":   strconv.Itoa(res.Added),
		"updated": strconv.Itoa(res.Updated),
	})
	writeJSON(w, http.StatusOK, res)
}

// adminVendorCandidates creates relation candidates from external products.
func (h *handler) adminVendorCandidates(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Limit int `json:"limit"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	res, err := catalog.GenerateVendorCandidates(r.Context(), h.db, req.Limit)
	if err != nil {
		h.logger.Error("vendor candidates", "err", err)
		writeError(w, http.StatusInternalServerError, "vendor candidates error")
		return
	}
	_ = catalog.LogAdminChange(r.Context(), h.db, "admin", "vendor.candidates", map[string]string{
		"created": strconv.Itoa(res.Created),
	})
	writeJSON(w, http.StatusOK, res)
}

// adminVendorLink resolves external products to catalog SKUs.
func (h *handler) adminVendorLink(w http.ResponseWriter, r *http.Request) {
	res, err := catalog.LinkVendorProducts(r.Context(), h.db)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "vendor link error")
		return
	}
	_ = catalog.LogAdminChange(r.Context(), h.db, "admin", "vendor.link", map[string]string{
		"mappings": strconv.Itoa(res.Mappings),
		"items":    strconv.Itoa(res.Items),
	})
	writeJSON(w, http.StatusOK, res)
}
