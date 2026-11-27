package httpapi

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/basili4-1982/lk-advisor/internal/analytics"
	"github.com/basili4-1982/lk-advisor/internal/catalog"
	"github.com/basili4-1982/lk-advisor/internal/experiment"
	"github.com/basili4-1982/lk-advisor/internal/model"

	"github.com/lib/pq"
)

// recommendCatalog serves recommendations for a real catalog client (lk_clients).
// It uses the reviewed relations and applies a running hypothesis to treatment.
func (h *handler) recommendCatalog(w http.ResponseWriter, r *http.Request, clientID string, limit int, certifiedOnly *bool) {
	ctx := r.Context()
	audience, industry, ok, err := catalog.ClientAudience(ctx, h.db, clientID)
	if err != nil || !ok {
		writeError(w, http.StatusInternalServerError, "client lookup error")
		return
	}

	certOnly := false
	policy := false
	if certifiedOnly != nil {
		certOnly = *certifiedOnly
	} else if catalog.IsRegulatedIndustry(industry) {
		certOnly = true
		policy = true
	}

	var running *experiment.Experiment
	var assignment experiment.Assignment
	if list, xerr := experiment.LoadRunning(ctx, h.db, audience, industry, ""); xerr == nil && len(list) > 0 {
		e := list[0]
		if a, aerr := experiment.Assign(ctx, h.db, e, clientID); aerr == nil {
			running = &e
			assignment = a
		}
	}

	forceID, forceType := "", ""
	if running != nil && assignment.Treatment {
		forceID = running.ToID
		forceType = running.RelationType
	}
	res, err := catalog.ServeHybrid(ctx, h.db, catalog.ServeOptions{
		ClientID: clientID, Limit: limit,
		ForceItemID: forceID, ForceType: forceType,
		CertifiedOnly: certOnly,
	})
	if err != nil {
		h.logger.Error("catalog recommendations", "err", err)
		writeError(w, http.StatusInternalServerError, "recommendation error")
		return
	}

	traceID := newTraceID()
	certs := h.fstecForItems(ctx, res.Items)
	recs := make([]model.Recommendation, 0, len(res.Items))
	shown := make([]analytics.Item, 0, len(res.Items))
	for i, it := range res.Items {
		recs = append(recs, model.Recommendation{
			ProductID:       it.ItemID,
			Title:           it.Title,
			Type:            it.Type,
			Score:           it.Score,
			SourceProductID: it.SourceItemID,
			RelationType:    it.RelationType,
			Rank:            i + 1,
			Reason:          it.Reason,
			Compatibility:   it.Compatibility,
			Certified:       it.Certified,
			CertExpired:     it.CertExpired,
			Fstec:           certs[it.ItemID],
			Evidence:        []model.Evidence{},
			Constraints:     []string{},
		})
		shown = append(shown, analytics.Item{
			ProductID: it.ItemID, Type: it.Type, Reason: "top_3",
			Detail: it.Reason, Source: "catalog", Rank: i + 1, Score: it.Score,
			SourceProductID: it.SourceItemID, RelationType: it.RelationType,
		})
	}
	if err := analytics.Log(ctx, h.db, traceID, clientID, "catalog",
		analytics.Meta{Strategy: "catalog", Audience: audience, Industry: industry},
		res.ColdStart, shown, nil); err != nil {
		h.logger.Warn("catalog analytics", "err", err)
	}

	if running != nil {
		exposedTarget := ""
		renewalShown, upsellShown := 0, 0
		for _, it := range res.Items {
			if it.ItemID == running.ToID {
				exposedTarget = running.ToID
			}
			switch it.Type {
			case "renewal":
				renewalShown++
			case "upsell":
				upsellShown++
			}
		}
		if err := experiment.RecordExposure(ctx, h.db, assignment, traceID, exposedTarget,
			renewalShown, upsellShown, res.ColdStart && len(res.Items) == 0); err != nil {
			h.logger.Warn("catalog exposure", "err", err)
		}
	}

	resp := recommendationResponse{
		TraceID:         traceID,
		ClientID:        clientID,
		Strategy:        "catalog",
		Intent:          "catalog",
		Weights:         model.Weights{},
		Recommendations: recs,
		Fallback:        false,
		ColdStart:       res.ColdStart,
		CertifiedOnly:   certOnly,
		CertifiedPolicy: policy,
		GeneratedAt:     h.now(),
	}
	if running != nil {
		resp.ExperimentID = running.ID
		resp.Variant = assignment.Variant
	}
	writeJSON(w, http.StatusOK, resp)
}

// fstecForItems loads up to three FSTEC certificates per recommended item.
func (h *handler) fstecForItems(ctx context.Context, items []catalog.ServeItem) map[string][]model.FstecCert {
	out := map[string][]model.FstecCert{}
	if len(items) == 0 {
		return out
	}
	ids := make([]string, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.ItemID)
	}
	rows, err := h.db.QueryContext(ctx, `
		SELECT m.item_id, c.certificate_number, c.valid_until
		FROM fstec_certificate_items m
		JOIN fstec_certificates c ON c.id = m.certificate_id
		WHERE m.item_id = ANY($1)
		ORDER BY c.valid_until DESC NULLS LAST`, pq.Array(ids))
	if err != nil {
		return out
	}
	defer rows.Close()
	now := h.now()
	for rows.Next() {
		var itemID, number string
		var valid sql.NullTime
		if err := rows.Scan(&itemID, &number, &valid); err != nil {
			continue
		}
		list := out[itemID]
		if len(list) >= 3 {
			continue
		}
		cert := model.FstecCert{Number: number}
		if valid.Valid {
			cert.ValidUntil = valid.Time.Format("2006-01-02")
			cert.Expired = valid.Time.Before(now)
		}
		out[itemID] = append(list, cert)
	}
	return out
}

// isCatalogClient reports whether the id belongs to the real catalog dataset.
func (h *handler) isCatalogClient(r *http.Request, clientID string) bool {
	_, _, ok, err := catalog.ClientAudience(r.Context(), h.db, clientID)
	return err == nil && ok
}
