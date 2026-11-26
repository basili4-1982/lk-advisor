// Package experiment implements the hypothesis-to-decision loop for the demo.
package experiment

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"strings"
	"time"
)

// Experiment is one testable hypothesis.
type Experiment struct {
	ID           string          `json:"id"`
	Title        string          `json:"title"`
	Hypothesis   string          `json:"hypothesis"`
	Status       string          `json:"status"`
	Audience     string          `json:"audience"`
	Industry     string          `json:"industry"`
	Strategy     string          `json:"strategy"`
	Metric       string          `json:"metric"`
	Guardrails   string          `json:"guardrails"`
	Variants     []Variant       `json:"variants"`
	TrafficSplit float64         `json:"traffic_split"`
	FromID       string          `json:"from_id"`
	ToID         string          `json:"to_id"`
	FromTitle    string          `json:"from_title,omitempty"`
	ToTitle      string          `json:"to_title,omitempty"`
	RelationType string          `json:"relation_type"`
	Treatment    string          `json:"treatment"`
	Owner        string          `json:"owner"`
	CreatedAt    time.Time       `json:"created_at"`
	StartedAt    *time.Time      `json:"started_at,omitempty"`
	EndsAt       *time.Time      `json:"ends_at,omitempty"`
	Decision     string          `json:"decision"`
	DecisionNote string          `json:"decision_note"`
	DecidedAt    *time.Time      `json:"decided_at,omitempty"`
	Results      []Result        `json:"results,omitempty"`
	Guardrail    *GuardrailCheck `json:"guardrail,omitempty"`
}

// Variant is one experiment arm.
type Variant struct {
	Name   string  `json:"name"`
	Weight float64 `json:"weight"`
}

// Assignment is a client's stable arm in an experiment.
type Assignment struct {
	ExperimentID    string
	ClientID        string
	Variant         string
	TargetProductID string
	FromID          string
	RelationType    string
	Treatment       bool
}

// Result is the aggregate for one variant.
type Result struct {
	Variant           string  `json:"variant"`
	Exposures         int     `json:"exposures"`
	Clients           int     `json:"clients"`
	Clicks            int     `json:"clicks"`
	Purchases         int     `json:"purchases"`
	TargetExposures   int     `json:"target_exposures"`
	TargetPurchases   int     `json:"target_purchases"`
	RenewalShown      int     `json:"renewal_shown"`
	UpsellShown       int     `json:"upsell_shown"`
	FallbackShown     int     `json:"fallback_shown"`
	Revenue           int64   `json:"revenue"`
	Margin            int64   `json:"margin"`
	MarginAvailable   bool    `json:"margin_available"`
	CTR               float64 `json:"ctr"`
	Conversion        float64 `json:"conversion"`
	TargetConversion  float64 `json:"target_conversion"`
	RenewalRate       float64 `json:"renewal_rate"`
	UpsellRate        float64 `json:"upsell_rate"`
	FallbackRate      float64 `json:"fallback_rate"`
	MarginPerExposure float64 `json:"margin_per_exposure"`
}

// GuardrailCheck is the hard-stop evaluation before a decision.
type GuardrailCheck struct {
	Blocked         bool     `json:"blocked"`
	Suggested       string   `json:"suggested,omitempty"` // promote / extend / reject
	Reasons         []string `json:"reasons,omitempty"`
	RenewalDrop     float64  `json:"renewal_drop"`
	UpsellDrop      float64  `json:"upsell_drop"`
	FallbackRise    float64  `json:"fallback_rise"`
	MarginDrop      float64  `json:"margin_drop"`
	MarginAvailable bool     `json:"margin_available"`
	ConversionDelta float64  `json:"conversion_delta"`
	Futility        bool     `json:"futility"`
}

// CheckGuardrails blocks promotion when a treatment harms renewal/upsell,
// raises fallback, or destroys margin. Margin is ignored until cost data exists.
func CheckGuardrails(results []Result) GuardrailCheck {
	byVariant := map[string]Result{}
	for _, r := range results {
		byVariant[r.Variant] = r
	}
	control, treatment := byVariant["control"], byVariant["treatment"]
	check := GuardrailCheck{
		Suggested:       "promote",
		RenewalDrop:     control.RenewalRate - treatment.RenewalRate,
		UpsellDrop:      control.UpsellRate - treatment.UpsellRate,
		FallbackRise:    treatment.FallbackRate - control.FallbackRate,
		MarginAvailable: control.MarginAvailable && treatment.MarginAvailable,
		ConversionDelta: treatment.Conversion - control.Conversion,
	}
	if control.Exposures < 100 || treatment.Exposures < 100 {
		check.Blocked = true
		check.Suggested = "extend"
		check.Reasons = append(check.Reasons, "мало данных: меньше 100 exposure на вариант")
		return check
	}
	if check.MarginAvailable && control.MarginPerExposure > 0 {
		check.MarginDrop = (control.MarginPerExposure - treatment.MarginPerExposure) / control.MarginPerExposure
	}
	if check.ConversionDelta <= -0.01 {
		check.Futility = true
		check.Reasons = append(check.Reasons, fmt.Sprintf("primary metric хуже контроля на %.1f п.п.", -check.ConversionDelta*100))
	}
	severe := check.RenewalDrop >= 0.10 || check.FallbackRise >= 0.05 ||
		(check.MarginAvailable && check.MarginDrop >= 0.20) || check.Futility
	if severe {
		check.Blocked = true
		check.Suggested = "reject"
	}
	if check.RenewalDrop >= 0.05 {
		check.Reasons = append(check.Reasons, fmt.Sprintf("renewal падает на %.1f п.п.", check.RenewalDrop*100))
	}
	if check.UpsellDrop >= 0.05 {
		check.Reasons = append(check.Reasons, fmt.Sprintf("upsell падает на %.1f п.п.", check.UpsellDrop*100))
	}
	if check.FallbackRise >= 0.02 {
		check.Reasons = append(check.Reasons, fmt.Sprintf("fallback растёт на %.1f п.п.", check.FallbackRise*100))
	}
	if check.MarginAvailable && check.MarginDrop >= 0.10 {
		check.Reasons = append(check.Reasons, fmt.Sprintf("маржа на exposure падает на %.0f%%", check.MarginDrop*100))
	}
	if check.Blocked && check.Suggested == "reject" {
		return check
	}
	if check.Blocked && check.Suggested != "extend" {
		check.Suggested = "extend"
	}
	if len(check.Reasons) > 0 {
		check.Blocked = true
		check.Suggested = "extend"
	}
	return check
}

// DecisionOutcome is the final decision after guardrails.
type DecisionOutcome struct {
	Decision  string         `json:"decision"`
	Auto      bool           `json:"auto"`
	Reasons   []string       `json:"reasons,omitempty"`
	Guardrail GuardrailCheck `json:"guardrail"`
}

// ErrNotFound means the experiment does not exist.
var ErrNotFound = errors.New("experiment not found")

// LoadRunning returns experiments matching the client's audience/industry.
func LoadRunning(ctx context.Context, dbc *sql.DB, audience, industry, strategy string) ([]Experiment, error) {
	rows, err := dbc.QueryContext(ctx, `
		SELECT e.id, e.title, e.hypothesis, e.status, e.audience, e.industry,
		       e.strategy, e.metric, e.guardrails, e.variants::text, e.traffic_split::float8,
		       e.from_id, e.to_id, COALESCE(pf.title, ''), COALESCE(pt.title, ''),
		       e.relation_type, e.treatment, e.owner, e.created_at,
		       e.started_at, e.ends_at, e.decision, e.decision_note, e.decided_at
		FROM experiments e
		LEFT JOIN products pf ON pf.id = e.from_id
		LEFT JOIN products pt ON pt.id = e.to_id
		WHERE e.status = 'running'
		  AND (e.audience = '' OR e.audience = $1)
		  AND (e.industry = '' OR e.industry = $2)
		  AND (e.strategy = '' OR e.strategy = $3)
		ORDER BY e.created_at DESC`, audience, industry, strategy)
	if err != nil {
		return nil, fmt.Errorf("running experiments: %w", err)
	}
	defer rows.Close()
	var out []Experiment
	for rows.Next() {
		e, err := scanExperiment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// List returns all experiments with their results.
func List(ctx context.Context, dbc *sql.DB) ([]Experiment, error) {
	rows, err := dbc.QueryContext(ctx, `
		SELECT e.id, e.title, e.hypothesis, e.status, e.audience, e.industry,
		       e.strategy, e.metric, e.guardrails, e.variants::text, e.traffic_split::float8,
		       e.from_id, e.to_id, COALESCE(pf.title, ''), COALESCE(pt.title, ''),
		       e.relation_type, e.treatment, e.owner, e.created_at,
		       e.started_at, e.ends_at, e.decision, e.decision_note, e.decided_at
		FROM experiments e
		LEFT JOIN products pf ON pf.id = e.from_id
		LEFT JOIN products pt ON pt.id = e.to_id
		ORDER BY e.created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("experiments: %w", err)
	}
	defer rows.Close()
	var out []Experiment
	for rows.Next() {
		e, err := scanExperiment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		res, err := Results(ctx, dbc, out[i].ID)
		if err != nil {
			return nil, err
		}
		out[i].Results = res
		check := CheckGuardrails(res)
		out[i].Guardrail = &check
	}
	return out, nil
}

func scanExperiment(rows *sql.Rows) (Experiment, error) {
	var e Experiment
	var variants string
	if err := rows.Scan(&e.ID, &e.Title, &e.Hypothesis, &e.Status, &e.Audience, &e.Industry,
		&e.Strategy, &e.Metric, &e.Guardrails, &variants, &e.TrafficSplit,
		&e.FromID, &e.ToID, &e.FromTitle, &e.ToTitle,
		&e.RelationType, &e.Treatment, &e.Owner, &e.CreatedAt,
		&e.StartedAt, &e.EndsAt, &e.Decision, &e.DecisionNote, &e.DecidedAt); err != nil {
		return e, err
	}
	_ = json.Unmarshal([]byte(variants), &e.Variants)
	return e, nil
}

// Assign returns a stable control/treatment arm for a client.
func Assign(ctx context.Context, dbc *sql.DB, e Experiment, clientID string) (Assignment, error) {
	a := Assignment{
		ExperimentID:    e.ID,
		ClientID:        clientID,
		TargetProductID: e.ToID,
		FromID:          e.FromID,
		RelationType:    e.RelationType,
	}
	var variant string
	err := dbc.QueryRowContext(ctx, `
		SELECT variant FROM experiment_assignments
		WHERE experiment_id = $1 AND client_id = $2`, e.ID, clientID).Scan(&variant)
	if err == nil {
		a.Variant = variant
		a.Treatment = variant == "treatment"
		return a, nil
	}
	if err != sql.ErrNoRows {
		return a, fmt.Errorf("experiment assignment: %w", err)
	}

	h := fnv.New32a()
	_, _ = h.Write([]byte(e.ID + "|" + clientID))
	bucket := float64(h.Sum32()%1000) / 1000.0
	variant = "control"
	if bucket < e.TrafficSplit {
		variant = "treatment"
	}
	if _, err := dbc.ExecContext(ctx, `
		INSERT INTO experiment_assignments (experiment_id, client_id, variant)
		VALUES ($1, $2, $3)
		ON CONFLICT (experiment_id, client_id) DO NOTHING`, e.ID, clientID, variant); err != nil {
		return a, fmt.Errorf("save experiment assignment: %w", err)
	}
	// A concurrent request may have inserted a different arm; always read it back.
	if err := dbc.QueryRowContext(ctx, `
		SELECT variant FROM experiment_assignments
		WHERE experiment_id = $1 AND client_id = $2`, e.ID, clientID).Scan(&variant); err != nil {
		return a, fmt.Errorf("read experiment assignment: %w", err)
	}
	a.Variant = variant
	a.Treatment = variant == "treatment"
	return a, nil
}

// RecordExposure writes one experiment exposure. The target product is set
// only when it was actually shown.
func RecordExposure(ctx context.Context, dbc *sql.DB, a Assignment, traceID, productID string,
	renewalShown, upsellShown int, fallback bool) error {
	if a.ExperimentID == "" {
		return nil
	}
	_, err := dbc.ExecContext(ctx, `
		INSERT INTO experiment_events
			(experiment_id, client_id, variant, trace_id, product_id, event,
			 renewal_shown, upsell_shown, fallback)
		VALUES ($1, $2, $3, $4, $5, 'exposed', $6, $7, $8)`,
		a.ExperimentID, a.ClientID, a.Variant, traceID, productID,
		renewalShown, upsellShown, fallback)
	if err != nil {
		return fmt.Errorf("experiment exposure: %w", err)
	}
	return nil
}

// RecordEvent copies a click/purchase for the experiment target from its exposure.
func RecordEvent(ctx context.Context, dbc *sql.DB, traceID, productID, event string) error {
	if traceID == "" || productID == "" || (event != "clicked" && event != "purchased") {
		return nil
	}
	_, err := dbc.ExecContext(ctx, `
		INSERT INTO experiment_events
			(experiment_id, client_id, variant, trace_id, product_id, event,
			 amount, margin, margin_available)
		SELECT e.experiment_id, e.client_id, e.variant, e.trace_id, $2, $3,
		       p.price,
		       CASE WHEN p.cost > 0 THEN p.price - p.cost ELSE 0 END,
		       (p.cost > 0)
		FROM experiment_events e
		JOIN products p ON p.id = $2
		WHERE e.trace_id = $1 AND e.event = 'exposed'
		LIMIT 1`,
		traceID, productID, event)
	if err != nil {
		return fmt.Errorf("experiment event: %w", err)
	}
	return nil
}

// Results aggregates exposures, clicks and purchases by variant.
func Results(ctx context.Context, dbc *sql.DB, experimentID string) ([]Result, error) {
	rows, err := dbc.QueryContext(ctx, `
		SELECT variant,
		       count(*) FILTER (WHERE event = 'exposed') AS exposures,
		       count(DISTINCT client_id) FILTER (WHERE event = 'exposed') AS clients,
		       count(*) FILTER (WHERE event = 'clicked') AS clicks,
		       count(*) FILTER (WHERE event = 'purchased') AS purchases,
		       count(*) FILTER (WHERE event = 'exposed' AND product_id = (SELECT to_id FROM experiments WHERE id = $1)) AS target_exposures,
		       count(*) FILTER (WHERE event = 'purchased' AND product_id = (SELECT to_id FROM experiments WHERE id = $1)) AS target_purchases,
		       count(*) FILTER (WHERE event = 'exposed' AND renewal_shown > 0) AS renewal_shown,
		       count(*) FILTER (WHERE event = 'exposed' AND upsell_shown > 0) AS upsell_shown,
		       count(*) FILTER (WHERE event = 'exposed' AND fallback) AS fallback_shown,
		       COALESCE(sum(amount) FILTER (WHERE event = 'purchased'), 0) AS revenue,
		       COALESCE(sum(margin) FILTER (WHERE event = 'purchased'), 0) AS margin,
		       COALESCE(bool_or(margin_available) FILTER (WHERE event = 'purchased'), false) AS margin_available
		FROM experiment_events
		WHERE experiment_id = $1
		GROUP BY variant`, experimentID)
	if err != nil {
		return nil, fmt.Errorf("experiment results: %w", err)
	}
	defer rows.Close()
	byVariant := map[string]Result{}
	for rows.Next() {
		var r Result
		if err := rows.Scan(&r.Variant, &r.Exposures, &r.Clients, &r.Clicks, &r.Purchases,
			&r.TargetExposures, &r.TargetPurchases,
			&r.RenewalShown, &r.UpsellShown, &r.FallbackShown,
			&r.Revenue, &r.Margin, &r.MarginAvailable); err != nil {
			return nil, err
		}
		if r.Exposures > 0 {
			r.CTR = float64(r.Clicks) / float64(r.Exposures)
			r.Conversion = float64(r.Purchases) / float64(r.Exposures)
		}
		if r.TargetExposures > 0 {
			r.TargetConversion = float64(r.TargetPurchases) / float64(r.TargetExposures)
		}
		if r.Exposures > 0 {
			r.RenewalRate = float64(r.RenewalShown) / float64(r.Exposures)
			r.UpsellRate = float64(r.UpsellShown) / float64(r.Exposures)
			r.FallbackRate = float64(r.FallbackShown) / float64(r.Exposures)
			r.MarginPerExposure = float64(r.Margin) / float64(r.Exposures)
		}
		byVariant[r.Variant] = r
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]Result, 0, 2)
	for _, name := range []string{"control", "treatment"} {
		if r, ok := byVariant[name]; ok {
			out = append(out, r)
		} else {
			out = append(out, Result{Variant: name})
		}
	}
	return out, nil
}

// Decide closes an experiment with a human decision, after guardrails.
func Decide(ctx context.Context, dbc *sql.DB, experimentID, decision, note string) (DecisionOutcome, error) {
	var out DecisionOutcome
	switch decision {
	case "promote", "reject", "extend":
	default:
		return out, fmt.Errorf("decision must be promote, reject or extend")
	}
	results, err := Results(ctx, dbc, experimentID)
	if err != nil {
		return out, err
	}
	check := CheckGuardrails(results)
	final := decision
	auto := false
	if decision == "promote" && check.Blocked {
		final = check.Suggested
		auto = true
	}
	if decision == "extend" && check.Suggested == "reject" {
		final = "reject"
		auto = true
	}
	status := "completed"
	endsAt := time.Now()
	if final == "extend" {
		status = "running"
		endsAt = time.Now().AddDate(0, 0, 14)
	}
	finalNote := note
	if auto && len(check.Reasons) > 0 {
		finalNote = "guardrail: " + strings.Join(check.Reasons, "; ") + "; " + note
	}
	res, err := dbc.ExecContext(ctx, `
		UPDATE experiments
		SET status = $2, decision = $3, decision_note = $4,
		    ends_at = $5, decided_at = now()
		WHERE id = $1`, experimentID, status, final, finalNote, endsAt)
	if err != nil {
		return out, fmt.Errorf("experiment decision: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return out, ErrNotFound
	}
	out = DecisionOutcome{
		Decision:  final,
		Auto:      auto,
		Reasons:   check.Reasons,
		Guardrail: check,
	}
	return out, nil
}

// AutoMonitor stops a clearly harmful experiment. It is called by a worker or
// the demo simulation; in production this would run on a schedule.
func AutoMonitor(ctx context.Context, dbc *sql.DB, experimentID string) (DecisionOutcome, bool, error) {
	results, err := Results(ctx, dbc, experimentID)
	if err != nil {
		return DecisionOutcome{}, false, err
	}
	check := CheckGuardrails(results)
	out := DecisionOutcome{Guardrail: check}
	if !check.Blocked || check.Suggested != "reject" {
		return out, false, nil
	}
	out, err = Decide(ctx, dbc, experimentID, "reject", "auto guardrail: "+strings.Join(check.Reasons, "; "))
	if err != nil {
		return out, false, err
	}
	return out, true, nil
}

// SearchKnowledge returns completed experiments filtered by text and decision.
func SearchKnowledge(ctx context.Context, dbc *sql.DB, query, decision string) ([]Experiment, error) {
	q := "%" + strings.TrimSpace(query) + "%"
	rows, err := dbc.QueryContext(ctx, `
		SELECT e.id, e.title, e.hypothesis, e.status, e.audience, e.industry,
		       e.strategy, e.metric, e.guardrails, e.variants::text, e.traffic_split::float8,
		       e.from_id, e.to_id, COALESCE(pf.title, ''), COALESCE(pt.title, ''),
		       e.relation_type, e.treatment, e.owner, e.created_at,
		       e.started_at, e.ends_at, e.decision, e.decision_note, e.decided_at
		FROM experiments e
		LEFT JOIN products pf ON pf.id = e.from_id
		LEFT JOIN products pt ON pt.id = e.to_id
		WHERE e.status = 'completed'
		  AND ($1 = '' OR e.decision = $1)
		  AND ($2 = '%%' OR e.title ILIKE $2 OR e.hypothesis ILIKE $2
		       OR e.audience ILIKE $2 OR e.industry ILIKE $2 OR e.strategy ILIKE $2
		       OR e.from_id ILIKE $2 OR e.to_id ILIKE $2)
		ORDER BY e.decided_at DESC NULLS LAST, e.created_at DESC`, decision, q)
	if err != nil {
		return nil, fmt.Errorf("experiment knowledge: %w", err)
	}
	defer rows.Close()
	var out []Experiment
	for rows.Next() {
		e, err := scanExperiment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		res, err := Results(ctx, dbc, out[i].ID)
		if err != nil {
			return nil, err
		}
		out[i].Results = res
		check := CheckGuardrails(res)
		out[i].Guardrail = &check
	}
	return out, nil
}

// SeedBadDemo creates a deliberately wrong hypothesis for time-to-reject tests.
func SeedBadDemo(ctx context.Context, dbc *sql.DB) error {
	const id = "exp-coreldraw-bad"
	if _, err := dbc.ExecContext(ctx, `DELETE FROM experiments WHERE id = $1`, id); err != nil {
		return fmt.Errorf("reset bad experiment: %w", err)
	}
	_, err := dbc.ExecContext(ctx, `
		INSERT INTO experiments
			(id, title, hypothesis, status, audience, industry, strategy, metric, guardrails,
			 variants, traffic_split, from_id, to_id, relation_type, treatment, owner, started_at)
		VALUES ($1, $2, $3, 'running', 'b2b', '', 'hybrid', 'conversion',
		        'renewal, upsell, fallback',
		        $4::jsonb, 0.5, 'prod-office-suite', 'prod-sl-43', 'alternative',
		        'force_relation', 'demo', now())`,
		id,
		"CorelDRAW как альтернатива Office для офисного сотрудника",
		"Показ CorelDRAW офисному сотруднику повышает конверсию",
		`[{"name":"control","weight":0.5},{"name":"treatment","weight":0.5}]`)
	if err != nil {
		return fmt.Errorf("seed bad experiment: %w", err)
	}
	return nil
}

// SeedDemo creates/resets the vertical-slice experiment.
func SeedDemo(ctx context.Context, dbc *sql.DB) error {
	const id = "exp-pdf-b2b"
	if _, err := dbc.ExecContext(ctx, `DELETE FROM experiments WHERE id = $1`, id); err != nil {
		return fmt.Errorf("reset demo experiment: %w", err)
	}
	_, err := dbc.ExecContext(ctx, `
		INSERT INTO experiments
			(id, title, hypothesis, status, audience, industry, strategy, metric, guardrails,
			 variants, traffic_split, from_id, to_id, relation_type, treatment, owner, started_at)
		VALUES ($1, $2, $3, 'running', 'b2b', '', 'hybrid', 'conversion',
		        'не терять renewal и upsell; следить за fallback',
		        $4::jsonb, 0.5, 'prod-office-suite', 'prod-pdf', 'adjacent_task',
		        'force_relation', 'demo', now())`,
		id,
		"PDF Toolkit как следующий шаг после Office",
		"Показ PDF Toolkit в слоте 3 повышает конверсию в B2B",
		`[{"name":"control","weight":0.5},{"name":"treatment","weight":0.5}]`)
	if err != nil {
		return fmt.Errorf("seed demo experiment: %w", err)
	}
	return nil
}
