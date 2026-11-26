package httpapi

import (
	"errors"
	"net/http"
	"sort"

	"github.com/basili4-1982/lk-advisor/internal/catalog"
	"github.com/basili4-1982/lk-advisor/internal/model"
	"github.com/basili4-1982/lk-advisor/internal/persona"
	"github.com/basili4-1982/lk-advisor/internal/profile"
)

type profileLicense struct {
	ProductID string `json:"product_id"`
	Title     string `json:"title"`
	Seats     int    `json:"seats"`
	End       string `json:"end"`
	Status    string `json:"status"`
}

type profilePurchase struct {
	ProductID string `json:"product_id"`
	Title     string `json:"title"`
	Date      string `json:"date"`
	Amount    int64  `json:"amount"`
	Quantity  int    `json:"quantity"`
}

type profileResponse struct {
	ClientID      string            `json:"client_id"`
	Segment       string            `json:"segment"`
	Industry      string            `json:"industry"`
	Size          string            `json:"size"`
	Environment   model.Environment `json:"environment"`
	Persona       string            `json:"persona"`
	PersonaReason string            `json:"persona_reason"`
	Personas      []persona.Score   `json:"personas"`
	Licenses      []profileLicense  `json:"licenses"`
	Purchases     []profilePurchase `json:"purchases"`
}

func (h *handler) clientProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "client id is required")
		return
	}
	if h.db == nil {
		writeError(w, http.StatusInternalServerError, "database is not configured")
		return
	}

	ctx := r.Context()
	p, err := profile.Get(ctx, h.db, id)
	if errors.Is(err, profile.ErrNotFound) {
		writeError(w, http.StatusNotFound, "client not found")
		return
	}
	if err != nil {
		h.logger.Error("profile", "err", err)
		writeError(w, http.StatusInternalServerError, "profile error")
		return
	}

	products, err := catalog.AllProducts(ctx, h.db)
	if err != nil {
		h.logger.Error("products", "err", err)
		writeError(w, http.StatusInternalServerError, "catalog error")
		return
	}

	personaTop, personaReason, personaScores := personaFor(p, products)
	resp := profileResponse{
		ClientID:      p.ClientID,
		Segment:       p.Segment,
		Industry:      p.Industry,
		Size:          p.Size,
		Environment:   p.Environment,
		Persona:       personaTop,
		PersonaReason: personaReason,
		Personas:      personaScores,
	}
	for _, l := range p.Licenses {
		resp.Licenses = append(resp.Licenses, profileLicense{
			ProductID: l.ProductID,
			Title:     products[l.ProductID].Title,
			Seats:     l.Seats,
			End:       l.End.Format("2006-01-02"),
			Status:    l.Status,
		})
	}
	for _, x := range p.Purchases {
		resp.Purchases = append(resp.Purchases, profilePurchase{
			ProductID: x.ProductID,
			Title:     products[x.ProductID].Title,
			Date:      x.Date.Format("2006-01-02"),
			Amount:    x.Amount,
			Quantity:  x.Quantity,
		})
	}
	sort.Slice(resp.Purchases, func(i, j int) bool {
		return resp.Purchases[i].Date > resp.Purchases[j].Date
	})
	if len(resp.Purchases) > 8 {
		resp.Purchases = resp.Purchases[:8]
	}
	if resp.Licenses == nil {
		resp.Licenses = []profileLicense{}
	}
	if resp.Purchases == nil {
		resp.Purchases = []profilePurchase{}
	}

	writeJSON(w, http.StatusOK, resp)
}
