package httpapi

import (
	"net/http"

	"github.com/basili4-1982/lk-advisor/internal/analytics"
	"github.com/basili4-1982/lk-advisor/internal/catalog"
	"github.com/basili4-1982/lk-advisor/internal/experiment"
	"github.com/basili4-1982/lk-advisor/internal/model"
)

// recommendCatalog serves recommendations for a real catalog client (lk_clients).
// It uses the reviewed relations and applies a running hypothesis to treatment.
func (h *handler) recommendCatalog(w http.ResponseWriter, r *http.Request, clientID string, limit int) {
	ctx := r.Context()
	audience, industry, ok, err := catalog.ClientAudience(ctx, h.db, clientID)
	if err != nil || !ok {
		writeError(w, http.StatusInternalServerError, "client lookup error")
		return
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
	res, err := catalog.Serve(ctx, h.db, catalog.ServeOptions{
		ClientID: clientID, Limit: limit,
		ForceItemID: forceID, ForceType: forceType,
	})
	if err != nil {
		h.logger.Error("catalog recommendations", "err", err)
		writeError(w, http.StatusInternalServerError, "recommendation error")
		return
	}

	traceID := newTraceID()
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

	writeJSON(w, http.StatusOK, recommendationResponse{
		TraceID:         traceID,
		ClientID:        clientID,
		Strategy:        "catalog",
		Intent:          "catalog",
		Weights:         model.Weights{},
		Recommendations: recs,
		Fallback:        false,
		ColdStart:       res.ColdStart,
		GeneratedAt:     h.now(),
	})
}

// isCatalogClient reports whether the id belongs to the real catalog dataset.
func (h *handler) isCatalogClient(r *http.Request, clientID string) bool {
	_, _, ok, err := catalog.ClientAudience(r.Context(), h.db, clientID)
	return err == nil && ok
}
