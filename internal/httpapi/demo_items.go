package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/basili4-1982/lk-advisor/internal/catalog"
	"github.com/basili4-1982/lk-advisor/internal/profile"
)

type addItemRequest struct {
	ProductID string `json:"product_id"`
}

// addItem adds a synthetic purchase and license to the demo client.
// This endpoint exists for the jury demo only; the product API is read-only.
func (h *handler) addItem(w http.ResponseWriter, r *http.Request) {
	clientID := r.PathValue("id")
	if clientID == "" || h.db == nil {
		writeError(w, http.StatusBadRequest, "client id is required")
		return
	}

	var req addItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ProductID == "" {
		writeError(w, http.StatusBadRequest, "product_id is required")
		return
	}

	ctx := r.Context()
	if _, err := profile.Get(ctx, h.db, clientID); errors.Is(err, profile.ErrNotFound) {
		writeError(w, http.StatusNotFound, "client not found")
		return
	} else if err != nil {
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
	product, ok := products[req.ProductID]
	if !ok {
		writeError(w, http.StatusNotFound, "product not found")
		return
	}

	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "transaction error")
		return
	}
	defer func() { _ = tx.Rollback() }()

	var owned bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM purchases WHERE client_id = $1 AND product_id = $2
			UNION ALL
			SELECT 1 FROM licenses WHERE client_id = $1 AND product_id = $2
		)`, clientID, product.ID).Scan(&owned); err != nil {
		writeError(w, http.StatusInternalServerError, "check error")
		return
	}
	if owned {
		writeError(w, http.StatusConflict, "product is already owned")
		return
	}

	now := time.Now()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO purchases (id, client_id, product_id, date, amount, quantity, seats)
		VALUES ($1,$2,$3,now(),$4,1,$5)`,
		fmt.Sprintf("demo-pur-%d", now.UnixNano()), clientID, product.ID, product.Price, product.Seats); err != nil {
		h.logger.Error("add purchase", "err", err)
		writeError(w, http.StatusInternalServerError, "add purchase")
		return
	}
	if product.TermMonths > 0 {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO licenses (id, client_id, product_id, seats, start_date, end_date, status)
			VALUES ($1,$2,$3,$4, now()::date, (now() + make_interval(months => $5::int))::date, 'active')`,
			fmt.Sprintf("demo-lic-%d", now.UnixNano()), clientID, product.ID, product.Seats, product.TermMonths); err != nil {
			h.logger.Error("add license", "err", err)
			writeError(w, http.StatusInternalServerError, "add license")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "commit error")
		return
	}
	h.audit(ctx, clientID, product.ID, product.Title, "add")
	h.refreshProfile(ctx, clientID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// removeItem removes a product from the demo client's portfolio.
func (h *handler) removeItem(w http.ResponseWriter, r *http.Request) {
	clientID := r.PathValue("id")
	productID := r.PathValue("product_id")
	if clientID == "" || productID == "" || h.db == nil {
		writeError(w, http.StatusBadRequest, "client id and product id are required")
		return
	}

	ctx := r.Context()
	if _, err := profile.Get(ctx, h.db, clientID); errors.Is(err, profile.ErrNotFound) {
		writeError(w, http.StatusNotFound, "client not found")
		return
	} else if err != nil {
		h.logger.Error("profile", "err", err)
		writeError(w, http.StatusInternalServerError, "profile error")
		return
	}

	if _, err := h.db.ExecContext(ctx, `DELETE FROM purchases WHERE client_id = $1 AND product_id = $2`, clientID, productID); err != nil {
		h.logger.Error("delete purchase", "err", err)
		writeError(w, http.StatusInternalServerError, "delete purchase")
		return
	}
	if _, err := h.db.ExecContext(ctx, `DELETE FROM licenses WHERE client_id = $1 AND product_id = $2`, clientID, productID); err != nil {
		h.logger.Error("delete license", "err", err)
		writeError(w, http.StatusInternalServerError, "delete license")
		return
	}
	title := productID
	if products, err := catalog.AllProducts(ctx, h.db); err == nil {
		if p, ok := products[productID]; ok && p.Title != "" {
			title = p.Title
		}
	}
	h.audit(ctx, clientID, productID, title, "remove")
	h.refreshProfile(ctx, clientID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *handler) refreshProfile(ctx context.Context, clientID string) {
	_ = profile.RefreshShard(ctx, h.db, profile.Shard(clientID, profile.ShardCount))
}

// resetDemoClient clears a demo client's history and analytics events.
func (h *handler) resetDemoClient(w http.ResponseWriter, r *http.Request) {
	clientID := r.PathValue("id")
	if clientID == "" || h.db == nil {
		writeError(w, http.StatusBadRequest, "client id is required")
		return
	}
	ctx := r.Context()
	if _, err := profile.Get(ctx, h.db, clientID); errors.Is(err, profile.ErrNotFound) {
		writeError(w, http.StatusNotFound, "client not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "profile error")
		return
	}
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "transaction error")
		return
	}
	defer func() { _ = tx.Rollback() }()
	for _, query := range []string{
		`DELETE FROM recommendation_requests WHERE client_id = $1`,
		`DELETE FROM purchases WHERE client_id = $1`,
		`DELETE FROM licenses WHERE client_id = $1`,
		`DELETE FROM orders WHERE client_id = $1`,
		`DELETE FROM agent_sessions WHERE client_id = $1`,
		`DELETE FROM demo_audit WHERE client_id = $1`,
	} {
		if _, err := tx.ExecContext(ctx, query, clientID); err != nil {
			h.logger.Error("reset demo client", "err", err)
			writeError(w, http.StatusInternalServerError, "reset error")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "commit error")
		return
	}
	h.refreshProfile(ctx, clientID)
	writeJSON(w, http.StatusOK, map[string]bool{"reset": true})
}
