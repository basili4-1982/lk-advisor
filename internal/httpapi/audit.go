package httpapi

import (
	"context"
	"net/http"
	"time"
)

type auditEntry struct {
	At        time.Time `json:"at"`
	ProductID string    `json:"product_id"`
	Title     string    `json:"title"`
	Action    string    `json:"action"`
}

func (h *handler) audit(ctx context.Context, clientID, productID, title, action string) {
	if h.db == nil {
		return
	}
	if _, err := h.db.ExecContext(ctx, `
		INSERT INTO demo_audit (client_id, product_id, title, action)
		VALUES ($1,$2,$3,$4)`, clientID, productID, title, action); err != nil {
		h.logger.Error("demo audit", "err", err)
	}
	h.logger.Info("demo audit", "client_id", clientID, "product_id", productID, "action", action)
}

func (h *handler) clientAudit(w http.ResponseWriter, r *http.Request) {
	clientID := r.PathValue("id")
	if clientID == "" || h.db == nil {
		writeError(w, http.StatusBadRequest, "client id is required")
		return
	}

	rows, err := h.db.QueryContext(r.Context(), `
		SELECT at, product_id, title, action
		FROM demo_audit WHERE client_id = $1
		ORDER BY at DESC, id DESC LIMIT 20`, clientID)
	if err != nil {
		h.logger.Error("audit query", "err", err)
		writeError(w, http.StatusInternalServerError, "audit error")
		return
	}
	defer rows.Close()

	out := make([]auditEntry, 0, 20)
	for rows.Next() {
		var e auditEntry
		if err := rows.Scan(&e.At, &e.ProductID, &e.Title, &e.Action); err != nil {
			writeError(w, http.StatusInternalServerError, "audit scan")
			return
		}
		out = append(out, e)
	}
	writeJSON(w, http.StatusOK, out)
}
