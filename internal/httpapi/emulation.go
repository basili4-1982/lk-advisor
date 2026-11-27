package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/basili4-1982/lk-advisor/internal/catalog"
	"github.com/basili4-1982/lk-advisor/internal/emulator"
)

// adminClientProfiles lists saved client profiles and the industry vocabulary.
func (h *handler) adminClientProfiles(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, name, audience, industry, description
		FROM client_profiles ORDER BY id`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "profiles error")
		return
	}
	defer rows.Close()
	items := []emulator.Profile{}
	for rows.Next() {
		var p emulator.Profile
		if err := rows.Scan(&p.ID, &p.Name, &p.Audience, &p.Industry, &p.Description); err != nil {
			writeError(w, http.StatusInternalServerError, "profiles scan error")
			return
		}
		items = append(items, p)
	}
	industries := []string{}
	indRows, err := h.db.QueryContext(r.Context(), `
		SELECT DISTINCT industry FROM lk_clients
		WHERE industry <> '' ORDER BY industry`)
	if err == nil {
		defer indRows.Close()
		for indRows.Next() {
			var name string
			if indRows.Scan(&name) == nil {
				industries = append(industries, name)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "industries": industries})
}

// adminCreateClientProfile adds a described client profile.
func (h *handler) adminCreateClientProfile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Audience    string `json:"audience"`
		Industry    string `json:"industry"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Audience = strings.ToLower(strings.TrimSpace(req.Audience))
	req.Industry = strings.TrimSpace(req.Industry)
	req.Description = strings.TrimSpace(req.Description)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "название профиля обязательно")
		return
	}
	if req.Audience != "b2b" && req.Audience != "b2c" {
		writeError(w, http.StatusBadRequest, "аудитория должна быть b2b или b2c")
		return
	}
	if req.Description == "" {
		writeError(w, http.StatusBadRequest, "опишите клиента словами")
		return
	}
	id := "prof-" + time.Now().UTC().Format("20060102150405")
	if _, err := h.db.ExecContext(r.Context(), `
		INSERT INTO client_profiles (id, name, audience, industry, description)
		VALUES ($1, $2, $3, $4, $5)`,
		id, req.Name, req.Audience, req.Industry, req.Description); err != nil {
		h.logger.Error("create client profile", "err", err)
		writeError(w, http.StatusInternalServerError, "profile create error")
		return
	}
	_ = catalog.LogAdminChange(r.Context(), h.db, "admin", "client_profile.create",
		map[string]string{"id": id, "name": req.Name})
	writeJSON(w, http.StatusOK, emulator.Profile{
		ID: id, Name: req.Name, Audience: req.Audience,
		Industry: req.Industry, Description: req.Description,
	})
}

// adminCatalogClients lists real catalog clients with purchase history for a
// live demo run.
func (h *handler) adminCatalogClients(w http.ResponseWriter, r *http.Request) {
	audience := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("audience")))
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 500 {
			limit = v
		}
	}
	query := `
		SELECT c.client_id, c.client_type, COALESCE(c.industry, ''), count(o.item_id) AS orders
		FROM lk_clients c
		JOIN lk_orders o ON o.client_id = c.client_id
		WHERE 1 = 1`
	args := []any{}
	if audience == "b2b" {
		query += " AND c.client_type IN ('ЮЛ', 'ИП')"
	} else if audience == "b2c" {
		query += " AND c.client_type = 'ФЛ'"
	}
	if exclude := strings.TrimSpace(r.URL.Query().Get("exclude_item")); exclude != "" {
		query += ` AND NOT EXISTS (
			SELECT 1 FROM lk_orders x
			WHERE x.client_id = c.client_id AND x.item_id = $1)`
		args = append(args, exclude)
	}
	query += fmt.Sprintf(" GROUP BY 1, 2, 3 ORDER BY orders DESC, c.client_id LIMIT $%d", len(args)+1)
	args = append(args, limit)
	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "catalog clients error")
		return
	}
	defer rows.Close()
	type client struct {
		ClientID   string `json:"client_id"`
		ClientType string `json:"client_type"`
		Industry   string `json:"industry"`
		Orders     int    `json:"orders"`
	}
	out := []client{}
	for rows.Next() {
		var c client
		if err := rows.Scan(&c.ClientID, &c.ClientType, &c.Industry, &c.Orders); err != nil {
			writeError(w, http.StatusInternalServerError, "catalog clients scan error")
			return
		}
		out = append(out, c)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

// adminResetExperimentEvents clears simulated and live events of an experiment.
func (h *handler) adminResetExperimentEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	res, err := h.db.ExecContext(r.Context(), `
		DELETE FROM experiment_events WHERE experiment_id = $1`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "reset events error")
		return
	}
	deleted, _ := res.RowsAffected()
	_ = catalog.LogAdminChange(r.Context(), h.db, "admin", "experiment.events_reset",
		map[string]string{"experiment_id": id})
	writeJSON(w, http.StatusOK, map[string]any{"deleted": deleted})
}

// adminEmulateExperiment replays a hypothesis for one client profile.
func (h *handler) adminEmulateExperiment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProfileID   string `json:"profile_id"`
		Impressions int    `json:"impressions"`
		Seed        int64  `json:"seed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	summary, err := emulator.Run(r.Context(), h.db, emulator.Options{
		ExperimentID: r.PathValue("id"),
		ProfileID:    req.ProfileID,
		Impressions:  req.Impressions,
		Seed:         req.Seed,
	})
	switch {
	case errors.Is(err, emulator.ErrNotFound):
		writeError(w, http.StatusNotFound, "гипотеза или профиль не найдены")
		return
	case err != nil:
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = catalog.LogAdminChange(r.Context(), h.db, "admin", "experiment.emulate", map[string]string{
		"experiment_id": summary.ExperimentID,
		"profile_id":    summary.ProfileID,
		"impressions":   strconv.Itoa(summary.Impressions),
	})
	writeJSON(w, http.StatusOK, summary)
}

// adminExperimentEmulations returns the saved runs for one experiment.
func (h *handler) adminExperimentEmulations(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT r.id, r.profile_id, p.name, r.impressions, r.seed, r.fit::float8,
		       r.summary::text, r.created_at
		FROM emulation_runs r
		JOIN client_profiles p ON p.id = r.profile_id
		WHERE r.experiment_id = $1
		ORDER BY r.created_at DESC`, r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "emulations error")
		return
	}
	defer rows.Close()
	out := []json.RawMessage{}
	for rows.Next() {
		var id, profileID, profileName, summary string
		var impressions int
		var seed int64
		var fit float64
		var created time.Time
		if err := rows.Scan(&id, &profileID, &profileName, &impressions, &seed, &fit, &summary, &created); err != nil {
			writeError(w, http.StatusInternalServerError, "emulations scan error")
			return
		}
		out = append(out, json.RawMessage(summary))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}
