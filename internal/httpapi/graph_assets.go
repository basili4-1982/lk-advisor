package httpapi

import (
	_ "embed"
	"net/http"
)

//go:embed static/3d-force-graph.min.js
var graph3dJS []byte

func (h *handler) graph3dAsset(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(graph3dJS)
}
