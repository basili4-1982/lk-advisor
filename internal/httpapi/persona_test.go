package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/basili4-1982/lk-advisor/internal/model"
	"github.com/basili4-1982/lk-advisor/internal/persona"
)

func TestPersonaStudent(t *testing.T) {
	p := model.Profile{Purchases: []model.Purchase{{ProductID: "prod-office-suite"}}}
	products := map[string]model.Product{
		"prod-office-suite": {
			ID: "prod-office-suite", Title: "Microsoft Office 2024 для дома и учёбы", Audience: "consumer",
		},
	}
	persona, reason, _ := personaFor(p, products)
	if persona != "student" || reason == "" {
		t.Fatalf("persona = %q, reason = %q", persona, reason)
	}
}

func TestPersonaBusiness(t *testing.T) {
	p := model.Profile{Purchases: []model.Purchase{{ProductID: "prod-antivirus"}}}
	products := map[string]model.Product{
		"prod-antivirus": {
			ID: "prod-antivirus", Title: "Kaspersky Endpoint Security Cloud", Audience: "business",
		},
	}
	persona, _, _ := personaFor(p, products)
	if persona != "business" {
		t.Fatalf("persona = %q", persona)
	}
}

func TestClientProfilePersonaStudent(t *testing.T) {
	h, _ := setupAPI(t)

	if rec := doJSON(t, h, http.MethodPost, "/v1/demo/clients/smb-006/items", `{"product_id":"prod-office-suite"}`); rec.Code != http.StatusOK {
		t.Fatalf("add status = %d", rec.Code)
	}
	rec := doJSON(t, h, http.MethodGet, "/v1/clients/smb-006/profile", "")
	var p profileResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if p.Persona != "student" {
		t.Fatalf("persona = %q, want student", p.Persona)
	}

	_ = doJSON(t, h, http.MethodDelete, "/v1/demo/clients/smb-006/items/prod-office-suite", "")
}

func TestPersonaBusinessHistoryBeatsStudent(t *testing.T) {
	p := model.Profile{Purchases: []model.Purchase{
		{ProductID: "prod-office-suite"},
		{ProductID: "prod-office-business"},
	}}
	products := map[string]model.Product{
		"prod-office-suite": {
			ID: "prod-office-suite", Title: "Microsoft Office 2024 для дома и учёбы", Audience: "consumer",
		},
		"prod-office-business": {
			ID: "prod-office-business", Title: "Microsoft 365 бизнес стандарт", Audience: "business",
		},
	}
	persona, _, _ := personaFor(p, products)
	if persona != "business" {
		t.Fatalf("persona = %q, want business", persona)
	}
}

func TestPersonaScoresDistribution(t *testing.T) {
	p := model.Profile{Purchases: []model.Purchase{
		{ProductID: "b1"}, {ProductID: "b2"}, {ProductID: "s1"},
	}}
	products := map[string]model.Product{
		"b1": {ID: "b1", Title: "Kaspersky Endpoint Security", Audience: "business"},
		"b2": {ID: "b2", Title: "BI Studio", Audience: "business"},
		"s1": {ID: "s1", Title: "Microsoft Office 2024 для дома и учёбы", Audience: "consumer"},
	}
	scores := persona.Scores(p, products)
	if len(scores) != 2 {
		t.Fatalf("scores = %+v", scores)
	}
	if scores[0].Type != "business" || scores[0].Score < 0.66 {
		t.Fatalf("top = %+v", scores[0])
	}
	if scores[1].Type != "student" || scores[1].Score < 0.33 {
		t.Fatalf("second = %+v", scores[1])
	}
}

func TestPersonaLimitedEdition(t *testing.T) {
	p := model.Profile{Purchases: []model.Purchase{{ProductID: "ess"}}}
	products := map[string]model.Product{
		"ess": {ID: "ess", Title: "CorelDRAW Essentials 2024", Audience: "consumer"},
	}
	scores := persona.Scores(p, products)
	if len(scores) != 2 {
		t.Fatalf("scores = %+v", scores)
	}
	if scores[0].Type != "student" || scores[0].Score != 0.5 {
		t.Fatalf("top = %+v", scores[0])
	}
	if scores[1].Type != "consumer" || scores[1].Score != 0.5 {
		t.Fatalf("second = %+v", scores[1])
	}
}
