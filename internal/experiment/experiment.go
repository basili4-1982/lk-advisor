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
	Emulation    *EmulationInfo  `json:"emulation,omitempty"`
}

// EmulationInfo is the latest profile run attached to an experiment.
type EmulationInfo struct {
	ID          string    `json:"id"`
	ProfileID   string    `json:"profile_id"`
	ProfileName string    `json:"profile_name"`
	Impressions int       `json:"impressions"`
	Fit         float64   `json:"fit"`
	CreatedAt   time.Time `json:"created_at"`
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
		       e.from_id, e.to_id, COALESCE(lf.title, pf.title, ''), COALESCE(lt.title, pt.title, ''),
		       e.relation_type, e.treatment, e.owner, e.created_at,
		       e.started_at, e.ends_at, e.decision, e.decision_note, e.decided_at
		FROM experiments e
		LEFT JOIN lk_items lf ON lf.item_id = e.from_id
		LEFT JOIN lk_items lt ON lt.item_id = e.to_id
		LEFT JOIN products pf ON pf.id = e.from_id
		LEFT JOIN products pt ON pt.id = e.to_id
		WHERE e.status = 'running'
		  AND (e.audience = '' OR e.audience = $1)
		  AND (e.industry = '' OR e.industry = $2)
		  AND ($3 = '' OR e.strategy = $3)
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
		       e.from_id, e.to_id, COALESCE(lf.title, pf.title, ''), COALESCE(lt.title, pt.title, ''),
		       e.relation_type, e.treatment, e.owner, e.created_at,
		       e.started_at, e.ends_at, e.decision, e.decision_note, e.decided_at
		FROM experiments e
		LEFT JOIN lk_items lf ON lf.item_id = e.from_id
		LEFT JOIN lk_items lt ON lt.item_id = e.to_id
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
		var em EmulationInfo
		err = dbc.QueryRowContext(ctx, `
			SELECT r.id, r.profile_id, p.name, r.impressions, r.fit::float8, r.created_at
			FROM emulation_runs r
			JOIN client_profiles p ON p.id = r.profile_id
			WHERE r.experiment_id = $1
			ORDER BY r.created_at DESC LIMIT 1`, out[i].ID).Scan(
			&em.ID, &em.ProfileID, &em.ProfileName, &em.Impressions, &em.Fit, &em.CreatedAt)
		if err == nil {
			out[i].Emulation = &em
		} else if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
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
		       COALESCE(p.price, 0),
		       CASE WHEN p.cost > 0 THEN p.price - p.cost ELSE 0 END,
		       (p.cost > 0)
		FROM experiment_events e
		LEFT JOIN products p ON p.id = $2
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
		       e.from_id, e.to_id, COALESCE(lf.title, pf.title, ''), COALESCE(lt.title, pt.title, ''),
		       e.relation_type, e.treatment, e.owner, e.created_at,
		       e.started_at, e.ends_at, e.decision, e.decision_note, e.decided_at
		FROM experiments e
		LEFT JOIN lk_items lf ON lf.item_id = e.from_id
		LEFT JOIN lk_items lt ON lt.item_id = e.to_id
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
		var em EmulationInfo
		err = dbc.QueryRowContext(ctx, `
			SELECT r.id, r.profile_id, p.name, r.impressions, r.fit::float8, r.created_at
			FROM emulation_runs r
			JOIN client_profiles p ON p.id = r.profile_id
			WHERE r.experiment_id = $1
			ORDER BY r.created_at DESC LIMIT 1`, out[i].ID).Scan(
			&em.ID, &em.ProfileID, &em.ProfileName, &em.Impressions, &em.Fit, &em.CreatedAt)
		if err == nil {
			out[i].Emulation = &em
		} else if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}
	return out, nil
}

// CreateFromRelationOptions describes a hypothesis created from the form or
// from a catalog relation card.
type CreateFromRelationOptions struct {
	FromID       string
	ToID         string
	RelationType string
	Title        string
	Hypothesis   string
	Audience     string
	Industry     string
	Metric       string
	Guardrails   string
	TrafficSplit float64
	SeedDemo     bool
}

func CreateFromRelation(ctx context.Context, dbc *sql.DB, opts CreateFromRelationOptions) (string, bool, error) {
	if strings.TrimSpace(opts.FromID) == "" || strings.TrimSpace(opts.ToID) == "" {
		return "", false, fmt.Errorf("from_id и to_id обязательны")
	}
	var existing string
	if err := dbc.QueryRowContext(ctx, `
		SELECT id FROM experiments
		WHERE from_id = $1 AND to_id = $2 AND status = 'running'
		LIMIT 1`, opts.FromID, opts.ToID).Scan(&existing); err == nil {
		return existing, false, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return "", false, fmt.Errorf("check running experiment: %w", err)
	}
	fromName := clipName(catalogItemName(ctx, dbc, opts.FromID), 54)
	toName := clipName(catalogItemName(ctx, dbc, opts.ToID), 54)
	if fromName == "" || toName == "" {
		return "", false, fmt.Errorf("товар не найден в каталоге")
	}
	audience := strings.TrimSpace(opts.Audience)
	if audience == "" {
		audience = "b2b"
	}
	title := strings.TrimSpace(opts.Title)
	if title == "" {
		title = "Показ «" + toName + "» после «" + fromName + "»"
	}
	hypothesis := strings.TrimSpace(opts.Hypothesis)
	if hypothesis == "" {
		hypothesis = fmt.Sprintf("Показ «%s» после «%s» повышает конверсию", toName, fromName)
	}
	metric := strings.TrimSpace(opts.Metric)
	switch metric {
	case "conversion", "target_conversion", "ctr":
	default:
		metric = "conversion"
	}
	guardrails := strings.TrimSpace(opts.Guardrails)
	if guardrails == "" {
		guardrails = "renewal, upsell, fallback"
	}
	split := opts.TrafficSplit
	if split <= 0 || split >= 1 {
		split = 0.5
	}
	variants := fmt.Sprintf(`[{"name":"control","weight":%.3f},{"name":"treatment","weight":%.3f}]`, 1-split, split)
	id := fmt.Sprintf("exp-catalog-%d", time.Now().UnixNano())
	tx, err := dbc.BeginTx(ctx, nil)
	if err != nil {
		return "", false, fmt.Errorf("create experiment: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO experiments
			(id, title, hypothesis, status, audience, industry, strategy, metric, guardrails,
			 variants, traffic_split, from_id, to_id, relation_type, treatment, owner, started_at)
		VALUES ($1, $2, $3, 'running', $4, $5, 'hybrid', $6,
		        $7, $8::jsonb, $9, $10, $11, $12,
		        'force_relation', '', now())`,
		id, title, hypothesis,
		audience, strings.TrimSpace(opts.Industry), metric,
		guardrails, variants, split,
		opts.FromID, opts.ToID, opts.RelationType); err != nil {
		return "", false, fmt.Errorf("create experiment: insert: %w", err)
	}
	if opts.SeedDemo {
		if err := seedVariantEvents(ctx, tx, id, opts.FromID, opts.ToID, "control", 150, 100, 24, 8, 120, 40); err != nil {
			return "", false, err
		}
		if err := seedVariantEvents(ctx, tx, id, opts.FromID, opts.ToID, "treatment", 150, 120, 48, 15, 118, 42); err != nil {
			return "", false, err
		}
	}
	if err := mirrorProducts(ctx, tx, opts.FromID, opts.ToID); err != nil {
		return "", false, err
	}
	if err := tx.Commit(); err != nil {
		return "", false, fmt.Errorf("create experiment: commit: %w", err)
	}
	return id, true, nil
}

// mirrorProducts copies catalog items into the legacy products table so the
// recommendation engine can force them during treatment.
func mirrorProducts(ctx context.Context, tx *sql.Tx, itemIDs ...string) error {
	for _, itemID := range itemIDs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO products (id, sku, title, category, subcategory, type, price, status, audience)
			SELECT i.item_id,
			       'lk-' || i.item_id,
			       i.title,
			       i.category,
			       COALESCE(i.subcategory, ''),
			       'license',
			       0,
			       CASE WHEN i.available THEN 'active' ELSE 'discontinued' END,
			       'any'
			FROM lk_items i
			WHERE i.item_id = $1
			ON CONFLICT DO NOTHING`, itemID); err != nil {
			return fmt.Errorf("mirror catalog item %s: %w", itemID, err)
		}
	}
	return nil
}

func SeedBadDemo(ctx context.Context, dbc *sql.DB) error {
	return seedCatalogExperiment(ctx, dbc, "exp-coreldraw-bad", true)
}

// SeedDemo creates/resets the vertical-slice experiment on a catalog pair.
func SeedDemo(ctx context.Context, dbc *sql.DB) error {
	return seedCatalogExperiment(ctx, dbc, "exp-pdf-b2b", false)
}

func seedCatalogExperiment(ctx context.Context, dbc *sql.DB, id string, bad bool) error {
	tx, err := dbc.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("seed experiment: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var fromID, toID, relationType string
	order := "DESC"
	if bad {
		order = "ASC"
	}
	err = tx.QueryRowContext(ctx, `
		SELECT r.from_item_id, r.to_item_id, r.type
		FROM relations r
		WHERE r.status IN ('candidate','active') AND r.type <> 'variant_of'
		ORDER BY r.confidence `+order+`, r.id LIMIT 1`).Scan(&fromID, &toID, &relationType)
	if err != nil {
		return fmt.Errorf("seed experiment: no catalog relation: %w", err)
	}
	fromName := clipName(catalogItemName(ctx, tx, fromID), 54)
	toName := clipName(catalogItemName(ctx, tx, toID), 54)
	title := fmt.Sprintf("Показ «%s» после «%s»", toName, fromName)
	hypothesis := fmt.Sprintf("Показ «%s» после «%s» повышает конверсию в B2B", toName, fromName)
	status, decision, note := "running", "", ""
	if bad {
		status, decision = "completed", "reject"
		note = "auto guardrail: primary metric хуже контроля на 4.7 п.п."
		hypothesis = fmt.Sprintf("Показ «%s» офисному клиенту повышает конверсию", toName)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM experiments WHERE id = $1`, id); err != nil {
		return fmt.Errorf("seed experiment: reset: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO experiments
			(id, title, hypothesis, status, audience, industry, strategy, metric, guardrails,
			 variants, traffic_split, from_id, to_id, relation_type, treatment, owner,
			 started_at, decision, decision_note, decided_at)
		VALUES ($1, $2, $3, $4, 'b2b', '', 'hybrid', 'conversion',
		        'renewal, upsell, fallback', $5::jsonb, 0.5, $6, $7, $8,
		        'force_relation', 'demo', now(), $9, $10,
		        CASE WHEN $9 = '' THEN NULL ELSE now() END)`,
		id, title, hypothesis, status,
		`[{"name":"control","weight":0.5},{"name":"treatment","weight":0.5}]`,
		fromID, toID, relationType, decision, note); err != nil {
		return fmt.Errorf("seed experiment: insert: %w", err)
	}
	if err := mirrorProducts(ctx, tx, fromID, toID); err != nil {
		return err
	}
	if bad {
		if err := seedVariantEvents(ctx, tx, id, fromID, toID, "control", 150, 8, 86, 47, 108, 23); err != nil {
			return err
		}
		if err := seedVariantEvents(ctx, tx, id, fromID, toID, "treatment", 150, 127, 84, 40, 114, 24); err != nil {
			return err
		}
	} else {
		if err := seedVariantEvents(ctx, tx, id, fromID, toID, "control", 250, 29, 21, 7, 190, 50); err != nil {
			return err
		}
		if err := seedVariantEvents(ctx, tx, id, fromID, toID, "treatment", 250, 233, 170, 66, 180, 60); err != nil {
			return err
		}
	}
	return tx.Commit()
}

var genericNamePrefixes = []string{
	"Предоставление доступа к сервисам ",
	"Предоставление доступа к ",
	"Программа контроля полномочий доступа к информационным ресурсам ",
	"Средство создания модели системы разграничения доступа ",
}

// cleanCatalogName drops boilerplate that every item in a product line shares,
// so generated hypothesis titles stay readable.
func cleanCatalogName(s string) string {
	s = strings.TrimSpace(s)
	for _, p := range genericNamePrefixes {
		if len(s) > len(p) && strings.HasPrefix(s, p) {
			return strings.TrimSpace(s[len(p):])
		}
	}
	return s
}

func clipName(s string, limit int) string {
	s = cleanCatalogName(s)
	r := []rune(strings.TrimSpace(s))
	if len(r) <= limit {
		return string(r)
	}
	cut := string(r[:limit])
	if i := strings.LastIndex(cut, " "); i > limit/2 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ,.;:-") + "…"
}

func catalogItemName(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, itemID string) string {
	var name string
	_ = q.QueryRowContext(ctx, `
		SELECT COALESCE(NULLIF(a.short_name, ''), i.title)
		FROM lk_items i
		LEFT JOIN item_aliases a ON a.item_id = i.item_id
		WHERE i.item_id = $1`, itemID).Scan(&name)
	return name
}

func seedVariantEvents(ctx context.Context, tx *sql.Tx, expID, fromID, toID, variant string,
	exposures, targetExposures, clicks, purchases, renewalShown, upsellShown int) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO experiment_events
			(experiment_id, client_id, variant, product_id, event, renewal_shown, upsell_shown, fallback)
		SELECT $1, 'demo-' || ((g % 25) + 1), $2,
		       CASE WHEN g <= $3 THEN $4 ELSE $5 END, 'exposed',
		       CASE WHEN g <= $6 THEN 1 ELSE 0 END,
		       CASE WHEN g <= $7 THEN 1 ELSE 0 END, false
		FROM generate_series(1, $8) g`,
		expID, variant, targetExposures, toID, fromID, renewalShown, upsellShown, exposures); err != nil {
		return fmt.Errorf("seed experiment: exposures: %w", err)
	}
	if clicks > 0 {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO experiment_events (experiment_id, client_id, variant, product_id, event)
			SELECT $1, 'demo-' || ((g % 25) + 1), $2, $3, 'clicked'
			FROM generate_series(1, $4) g`, expID, variant, toID, clicks); err != nil {
			return fmt.Errorf("seed experiment: clicks: %w", err)
		}
	}
	if purchases > 0 {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO experiment_events (experiment_id, client_id, variant, product_id, event)
			SELECT $1, 'demo-' || ((g % 25) + 1), $2, $3, 'purchased'
			FROM generate_series(1, $4) g`, expID, variant, toID, purchases); err != nil {
			return fmt.Errorf("seed experiment: purchases: %w", err)
		}
	}
	return nil
}
