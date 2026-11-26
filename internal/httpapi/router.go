// Package httpapi exposes the service over HTTP.
package httpapi

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/basili4-1982/lk-advisor/internal/llm"
)

type handler struct {
	logger *slog.Logger
	db     *sql.DB
	now    func() time.Time
	llm    *llm.Client
}

// New returns the service router with the real clock.
func New(logger *slog.Logger, dbc *sql.DB) http.Handler {
	return newHandler(logger, dbc, time.Now, nil)
}

// NewWithClock returns the service router with an injected clock (tests).
func NewWithClock(logger *slog.Logger, dbc *sql.DB, now func() time.Time) http.Handler {
	return newHandler(logger, dbc, now, nil)
}

// NewWithLLM returns the service router with an optional LLM client.
func NewWithLLM(logger *slog.Logger, dbc *sql.DB, now func() time.Time, client *llm.Client) http.Handler {
	return newHandler(logger, dbc, now, client)
}

func newHandler(logger *slog.Logger, dbc *sql.DB, now func() time.Time, client *llm.Client) http.Handler {
	h := &handler{logger: logger, db: dbc, now: now, llm: client}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /{$}", h.demo)
	mux.HandleFunc("GET /demo", h.demo)
	mux.HandleFunc("GET /admin", h.adminPage)
	mux.HandleFunc("GET /admin/{$}", h.adminPage)
	mux.HandleFunc("GET /api", h.apiIndex)
	mux.HandleFunc("POST /v1/recommendations", h.recommendations)
	mux.HandleFunc("POST /v1/ask", h.ask)
	mux.HandleFunc("POST /v1/events", h.recordEvent)
	mux.HandleFunc("POST /v1/lk/cross-sell", h.lkCrossSell)
	mux.HandleFunc("POST /api/cross_sell/list", h.lkCrossSell)
	mux.HandleFunc("GET /v1/clients/{id}/profile", h.clientProfile)
	mux.HandleFunc("GET /v1/products", h.products)
	mux.HandleFunc("POST /v1/demo/clients/{id}/items", h.addItem)
	mux.HandleFunc("DELETE /v1/demo/clients/{id}/items/{product_id}", h.removeItem)
	mux.HandleFunc("POST /v1/demo/clients/{id}/reset", h.resetDemoClient)
	mux.HandleFunc("GET /v1/demo/clients/{id}/audit", h.clientAudit)
	mux.HandleFunc("GET /v1/demo/clients/{id}/agent-calls", h.agentCalls)
	mux.HandleFunc("GET /v1/admin/state", h.adminState)
	mux.HandleFunc("GET /v1/admin/analytics", h.adminAnalytics)
	mux.HandleFunc("GET /v1/admin/analytics/health", h.adminAnalyticsHealth)
	mux.HandleFunc("POST /v1/admin/analytics/auto-review", h.adminAutoReview)
	mux.HandleFunc("POST /v1/admin/relations", h.adminUpsertRelation)
	mux.HandleFunc("DELETE /v1/admin/relations", h.adminDeleteRelation)
	mux.HandleFunc("GET /v1/admin/experiments", h.adminExperiments)
	mux.HandleFunc("GET /v1/admin/experiments/knowledge", h.adminExperimentKnowledge)
	mux.HandleFunc("POST /v1/admin/experiments/demo", h.adminSeedExperiment)
	mux.HandleFunc("POST /v1/admin/experiments/bad-demo", h.adminSeedBadExperiment)
	mux.HandleFunc("POST /v1/admin/experiments/{id}/monitor", h.adminMonitorExperiment)
	mux.HandleFunc("POST /v1/admin/experiments/{id}/decision", h.adminExperimentDecision)
	mux.HandleFunc("GET /v1/admin/relations/performance", h.adminRelationPerformance)
	mux.HandleFunc("GET /v1/admin/relations/scopes", h.adminRelationScopes)
	mux.HandleFunc("POST /v1/admin/relations/scopes/review", h.adminReviewRelationScope)
	mux.HandleFunc("POST /v1/admin/relations/auto-review", h.adminAutoReviewRelations)
	mux.HandleFunc("GET /v1/admin/relations/review", h.adminReviewQueue)
	mux.HandleFunc("POST /v1/admin/relations/review", h.adminReviewDecision)
	mux.HandleFunc("GET /v1/admin/solo-products", h.adminSoloProducts)
	mux.HandleFunc("GET /v1/demo/graph", h.demoGraph)
	mux.HandleFunc("GET /demo/graph", h.demoGraphPage)
	mux.HandleFunc("GET /static/3d-force-graph.min.js", h.graph3dAsset)
	mux.HandleFunc("GET /v1/admin/dataset/coverage", h.adminDatasetCoverage)
	mux.HandleFunc("GET /v1/admin/dataset/co-purchases", h.adminCoPurchases)
	mux.HandleFunc("POST /v1/admin/dataset/co-purchases/suggest", h.adminSuggestCoPurchases)
	mux.HandleFunc("POST /v1/admin/dataset/co-purchases/classify", h.adminClassifyCoPurchases)
	mux.HandleFunc("GET /v1/admin/dataset/candidates", h.adminDatasetCandidates)
	mux.HandleFunc("POST /v1/admin/dataset/candidates/review", h.adminReviewDatasetCandidate)
	mux.HandleFunc("POST /v1/admin/dataset/relations", h.adminCreateDatasetRelation)
	mux.HandleFunc("GET /v1/admin/registry/relations", h.adminRegistryList)
	mux.HandleFunc("POST /v1/admin/registry/relations/bulk-review", h.adminRegistryBulkReview)
	mux.HandleFunc("POST /v1/admin/registry/relations/{id}/review", h.adminRegistryReview)
	mux.HandleFunc("POST /v1/admin/registry/generate", h.adminRegistryGenerate)
	mux.HandleFunc("POST /v1/admin/registry/sweep", h.adminRegistrySweep)
	mux.HandleFunc("GET /v1/admin/registry/metrics", h.adminRegistryMetrics)
	mux.HandleFunc("POST /v1/admin/rules", h.adminCreateRule)
	mux.HandleFunc("DELETE /v1/admin/rules/{id}", h.adminDeleteRule)
	mux.HandleFunc("PUT /v1/admin/weights", h.adminSaveWeights)
	mux.HandleFunc("GET /v1/admin/strategies", h.adminStrategies)
	mux.HandleFunc("PUT /v1/admin/strategies/auto", h.adminSetAutoStrategy)
	mux.HandleFunc("POST /v1/admin/relation-types", h.adminUpsertRelationType)
	mux.HandleFunc("DELETE /v1/admin/relation-types/{code}", h.adminDeleteRelationType)
	mux.HandleFunc("GET /v1/admin/relation-types/prompt", h.adminRelationPrompt)
	mux.HandleFunc("POST /v1/admin/relations/agent/propose", h.adminProposeRelation)
	mux.HandleFunc("POST /v1/admin/agent/propose", h.adminPropose)
	mux.HandleFunc("POST /v1/admin/agent/apply", h.adminApply)

	return logRequests(logger, mux)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
