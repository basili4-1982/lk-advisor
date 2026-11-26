package httpapi

import (
	"net/http"
	"sort"

	"github.com/basili4-1982/lk-advisor/internal/catalog"
)

type productItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Category string `json:"category"`
	Price    int64  `json:"price"`
}

func (h *handler) products(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		writeError(w, http.StatusInternalServerError, "database is not configured")
		return
	}
	products, err := catalog.AllProducts(r.Context(), h.db)
	if err != nil {
		h.logger.Error("products", "err", err)
		writeError(w, http.StatusInternalServerError, "catalog error")
		return
	}

	out := make([]productItem, 0, len(products))
	for _, p := range products {
		out = append(out, productItem{ID: p.ID, Title: p.Title, Category: p.Category, Price: p.Price})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Title < out[j].Title })
	writeJSON(w, http.StatusOK, out)
}
