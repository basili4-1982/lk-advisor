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
	mux.HandleFunc("GET /v1/clients/{id}/profile", h.clientProfile)
	mux.HandleFunc("GET /v1/products", h.products)
	mux.HandleFunc("POST /v1/demo/clients/{id}/items", h.addItem)
	mux.HandleFunc("DELETE /v1/demo/clients/{id}/items/{product_id}", h.removeItem)
	mux.HandleFunc("GET /v1/demo/clients/{id}/audit", h.clientAudit)
	mux.HandleFunc("GET /v1/demo/clients/{id}/agent-calls", h.agentCalls)
	mux.HandleFunc("GET /v1/admin/state", h.adminState)
	mux.HandleFunc("GET /v1/admin/analytics", h.adminAnalytics)
	mux.HandleFunc("POST /v1/admin/relations", h.adminUpsertRelation)
	mux.HandleFunc("DELETE /v1/admin/relations", h.adminDeleteRelation)
	mux.HandleFunc("GET /v1/admin/relations/review", h.adminReviewQueue)
	mux.HandleFunc("POST /v1/admin/relations/review", h.adminReviewDecision)
	mux.HandleFunc("GET /v1/admin/solo-products", h.adminSoloProducts)
	mux.HandleFunc("POST /v1/admin/rules", h.adminCreateRule)
	mux.HandleFunc("DELETE /v1/admin/rules/{id}", h.adminDeleteRule)
	mux.HandleFunc("PUT /v1/admin/weights", h.adminSaveWeights)
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
