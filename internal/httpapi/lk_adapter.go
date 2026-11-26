package httpapi

import (
	"bytes"
	"encoding/json"
	"math"
	"net/http"
	"net/url"

	"github.com/basili4-1982/lk-advisor/internal/catalog"
	"github.com/basili4-1982/lk-advisor/internal/model"
)

type lkCrossSellRequest struct {
	ClientID string `json:"client_id"`
	Audience string `json:"audience"`
	Industry string `json:"industry"`
	Limit    int    `json:"limit"`
}

type lkItem struct {
	Link              string           `json:"link"`
	Image             string           `json:"image"`
	Name              string           `json:"name"`
	VendorName        string           `json:"vendorName"`
	VendorLink        string           `json:"vendorLink"`
	Price             float64          `json:"price"`
	PriceWithDiscount float64          `json:"priceWithDiscount"`
	ProductID         string           `json:"product_id"`
	Type              string           `json:"type"`
	Reason            string           `json:"reason"`
	Evidence          []model.Evidence `json:"evidence"`
	Score             float64          `json:"score"`
}

type lkCrossSellResponse struct {
	Status string   `json:"status"`
	Data   []lkItem `json:"data"`
}

type captureWriter struct {
	header http.Header
	code   int
	body   bytes.Buffer
}

func newCaptureWriter() *captureWriter {
	return &captureWriter{header: make(http.Header)}
}

func (c *captureWriter) Header() http.Header { return c.header }
func (c *captureWriter) Write(p []byte) (int, error) {
	return c.body.Write(p)
}
func (c *captureWriter) WriteHeader(code int) { c.code = code }

// lkCrossSell is a drop-in adapter for the personal cabinet's cross_sell/list.
// It reuses the normal recommendation handler, then maps the answer to the LK shape.
func (h *handler) lkCrossSell(w http.ResponseWriter, r *http.Request) {
	var req lkCrossSellRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	limit := req.Limit
	if limit < 1 || limit > 3 {
		limit = 3
	}
	body, _ := json.Marshal(recommendationRequest{
		ClientID: req.ClientID, Limit: &limit,
		Audience: req.Audience, Industry: req.Industry,
	})
	inner, _ := http.NewRequestWithContext(r.Context(), http.MethodPost,
		"/v1/recommendations", bytes.NewReader(body))
	inner.Header.Set("Content-Type", "application/json")
	cw := newCaptureWriter()
	h.recommendations(cw, inner)
	if cw.code != http.StatusOK {
		status := cw.code
		if status == 0 {
			status = http.StatusInternalServerError
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(status)
		_, _ = w.Write(cw.body.Bytes())
		return
	}

	var resp recommendationResponse
	if err := json.Unmarshal(cw.body.Bytes(), &resp); err != nil {
		writeError(w, http.StatusInternalServerError, "adapter decode error")
		return
	}
	products, err := catalog.AllProducts(r.Context(), h.db)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "catalog error")
		return
	}
	out := lkCrossSellResponse{Status: "hidden", Data: make([]lkItem, 0, len(resp.Recommendations))}
	for _, rec := range resp.Recommendations {
		p := products[rec.ProductID]
		item := lkItem{
			Link:       "https://store.softline.ru/search/?text=" + url.QueryEscape(rec.Title),
			Image:      "",
			Name:       rec.Title,
			VendorName: p.Vendor,
			VendorLink: "https://store.softline.ru/search/?text=" + url.QueryEscape(p.Vendor),
			Price:      float64(rec.Price.Amount),
			ProductID:  rec.ProductID,
			Type:       rec.Type,
			Reason:     rec.Reason,
			Evidence:   rec.Evidence,
			Score:      rec.Score,
		}
		item.PriceWithDiscount = item.Price
		if rec.Promo != nil {
			item.PriceWithDiscount = math.Round(item.Price*(1-rec.Promo.Discount/100)*100) / 100
		}
		out.Data = append(out.Data, item)
	}
	if len(out.Data) > 0 {
		out.Status = "shown"
	}
	writeJSON(w, http.StatusOK, out)
}
