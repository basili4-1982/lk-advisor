package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/basili4-1982/lk-advisor/internal/catalog"
	"github.com/basili4-1982/lk-advisor/internal/experiment"
)

func (h *handler) adminExperiments(w http.ResponseWriter, r *http.Request) {
	items, err := experiment.List(r.Context(), h.db)
	if err != nil {
		h.logger.Error("experiments", "err", err)
		writeError(w, http.StatusInternalServerError, "experiments error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *handler) adminSeedExperiment(w http.ResponseWriter, r *http.Request) {
	if err := experiment.SeedDemo(r.Context(), h.db); err != nil {
		h.logger.Error("seed experiment", "err", err)
		writeError(w, http.StatusInternalServerError, "seed experiment error")
		return
	}
	_ = catalog.LogAdminChange(r.Context(), h.db, "admin", "experiment.seed_demo", nil)
	writeJSON(w, http.StatusOK, map[string]bool{"seeded": true})
}

func (h *handler) adminExperimentDecision(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Decision string `json:"decision"`
		Note     string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	id := r.PathValue("id")
	outcome, err := experiment.Decide(r.Context(), h.db, id, req.Decision, req.Note)
	switch {
	case errors.Is(err, experiment.ErrNotFound):
		writeError(w, http.StatusNotFound, "experiment not found")
		return
	case err != nil:
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = catalog.LogAdminChange(r.Context(), h.db, "admin", "experiment.decision",
		map[string]string{"experiment_id": id, "decision": outcome.Decision})
	writeJSON(w, http.StatusOK, outcome)
}

func (h *handler) adminExperimentKnowledge(w http.ResponseWriter, r *http.Request) {
	items, err := experiment.SearchKnowledge(r.Context(), h.db,
		r.URL.Query().Get("q"), r.URL.Query().Get("decision"))
	if err != nil {
		h.logger.Error("experiment knowledge", "err", err)
		writeError(w, http.StatusInternalServerError, "knowledge error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
