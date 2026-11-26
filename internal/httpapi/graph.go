package httpapi

import (
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/basili4-1982/lk-advisor/internal/catalog"
)

func (h *handler) demoGraph(w http.ResponseWriter, r *http.Request) {
	if os.Getenv("DEMO_ALLOW_REAL_DATA") != "1" {
		writeError(w, http.StatusForbidden, "real dataset demo is disabled; set DEMO_ALLOW_REAL_DATA=1 for the closed hackathon demo")
		return
	}
	audience := r.URL.Query().Get("audience")
	soloLimit := 800
	if raw := r.URL.Query().Get("solo_limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			soloLimit = v
		}
	}
	period := r.URL.Query().Get("period")
	graph, err := catalog.LoadDemoGraphAt(r.Context(), h.db, audience, period, soloLimit)
	if err != nil {
		h.logger.Error("demo graph", "err", err)
		writeError(w, http.StatusInternalServerError, "demo graph error")
		return
	}
	writeJSON(w, http.StatusOK, graph)
}

func (h *handler) demoGraphPage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, graphPageHTML)
}
