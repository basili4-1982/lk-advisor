package emulator_test

import (
	"context"
	"testing"

	"github.com/basili4-1982/lk-advisor/internal/emulator"
	"github.com/basili4-1982/lk-advisor/internal/store"
)

func TestRunEmulationScoresProfileAndWritesEvents(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	if err := store.Migrate(ctx, dbc); err != nil {
		t.Fatal(err)
	}
	cleanup := func() {
		dbc.ExecContext(ctx, `DELETE FROM experiments WHERE id LIKE 'emu-test-%'`)
		dbc.ExecContext(ctx, `DELETE FROM client_profiles WHERE id LIKE 'emu-test-%'`)
		dbc.ExecContext(ctx, `DELETE FROM lk_orders WHERE item_id LIKE 'emu-test-%'`)
		dbc.ExecContext(ctx, `DELETE FROM lk_items WHERE item_id LIKE 'emu-test-%'`)
		dbc.ExecContext(ctx, `DELETE FROM lk_clients WHERE client_id LIKE 'emu-test-%'`)
	}
	cleanup()
	t.Cleanup(cleanup)

	if _, err := dbc.ExecContext(ctx, `
		INSERT INTO lk_items (item_id, title, category, subcategory, product_group) VALUES
		('emu-test-src', 'Офисный пакет Office', 'office', '', 'TEST-OFFICE'),
		('emu-test-target', 'Корпоративная почта Workspace Starter', 'office', '', 'TEST-WS'),
		('emu-test-control', 'Курс по продажам', 'office', '', 'TEST-COURSE')`); err != nil {
		t.Fatal(err)
	}
	if _, err := dbc.ExecContext(ctx, `
		INSERT INTO client_profiles (id, name, audience, industry, description)
		VALUES ('emu-test-prof', 'Клиника', 'b2b', 'Медицинские организации',
		        'Клинике нужны корпоративная почта, офисные программы и защита данных')`); err != nil {
		t.Fatal(err)
	}
	if _, err := dbc.ExecContext(ctx, `
		INSERT INTO lk_clients (client_id, industry) VALUES
		('emu-test-c1', 'Медицинские организации'),
		('emu-test-c2', 'Медицинские организации'),
		('emu-test-c3', 'ИТ (разработчики)')`); err != nil {
		t.Fatal(err)
	}
	if _, err := dbc.ExecContext(ctx, `
		INSERT INTO lk_orders (order_id, client_id, order_week, item_id) VALUES
		('emu-test-o1', 'emu-test-c1', '2026-01-01', 'emu-test-control'),
		('emu-test-o2', 'emu-test-c2', '2026-01-01', 'emu-test-control')`); err != nil {
		t.Fatal(err)
	}
	if _, err := dbc.ExecContext(ctx, `
		INSERT INTO experiments
			(id, title, hypothesis, status, audience, industry, strategy, metric, guardrails,
			 variants, traffic_split, from_id, to_id, relation_type, treatment, started_at)
		VALUES ('emu-test-exp', 'Тест', 'Тест', 'running', 'b2b', '', 'hybrid', 'conversion',
		        'renewal, upsell, fallback',
		        '[{"name":"control","weight":0.5},{"name":"treatment","weight":0.5}]'::jsonb,
		        0.5, 'emu-test-src', 'emu-test-target', 'upgrade', 'force_relation', now())`); err != nil {
		t.Fatal(err)
	}

	summary, err := emulator.Run(ctx, dbc, emulator.Options{
		ExperimentID: "emu-test-exp",
		ProfileID:    "emu-test-prof",
		Impressions:  400,
		Seed:         42,
	})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Treatment.Fit <= summary.Control.Fit {
		t.Fatalf("treatment fit %.2f should beat control %.2f",
			summary.Treatment.Fit, summary.Control.Fit)
	}
	if summary.Treatment.Conversion <= summary.Control.Conversion {
		t.Fatalf("treatment conversion %.3f should beat control %.3f",
			summary.Treatment.Conversion, summary.Control.Conversion)
	}
	if len(summary.Matched) == 0 || len(summary.Sessions) == 0 {
		t.Fatalf("no explanation: matched=%v sessions=%d", summary.Matched, len(summary.Sessions))
	}
	var exposures int
	if err := dbc.QueryRowContext(ctx, `
		SELECT count(*) FROM experiment_events
		WHERE experiment_id = 'emu-test-exp' AND event = 'exposed'`).Scan(&exposures); err != nil {
		t.Fatal(err)
	}
	if exposures != 400 {
		t.Fatalf("exposures = %d, want 400", exposures)
	}

	// A second run replaces the emulated events instead of stacking them.
	if _, err := emulator.Run(ctx, dbc, emulator.Options{
		ExperimentID: "emu-test-exp",
		ProfileID:    "emu-test-prof",
		Impressions:  200,
		Seed:         7,
	}); err != nil {
		t.Fatal(err)
	}
	if err := dbc.QueryRowContext(ctx, `
		SELECT count(*) FROM experiment_events
		WHERE experiment_id = 'emu-test-exp' AND event = 'exposed'`).Scan(&exposures); err != nil {
		t.Fatal(err)
	}
	if exposures != 200 {
		t.Fatalf("after rerun exposures = %d, want 200", exposures)
	}
	var runs int
	if err := dbc.QueryRowContext(ctx, `
		SELECT count(*) FROM emulation_runs WHERE experiment_id = 'emu-test-exp'`).Scan(&runs); err != nil {
		t.Fatal(err)
	}
	if runs != 1 {
		t.Fatalf("runs = %d, want 1", runs)
	}
}
