package httpapi

import (
	"net/http"
	"strings"
	"testing"
)

func TestLandingPageRoutesToDemoScreens(t *testing.T) {
	h, _ := setupAPI(t)
	rec := doJSON(t, h, http.MethodGet, "/", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, needle := range []string{`href="/lk"`, `href="/admin"`, `href="/demo/graph"`} {
		if !strings.Contains(body, needle) {
			t.Errorf("landing page missing %s", needle)
		}
	}
}

func TestGraphPageHasConfirmationLegend(t *testing.T) {
	h, _ := setupAPI(t)
	rec := doJSON(t, h, http.MethodGet, "/demo/graph", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, needle := range []string{"только подтверждённые", "требует проверки", "подтверждено"} {
		if !strings.Contains(body, needle) {
			t.Errorf("graph page missing %q", needle)
		}
	}
}
