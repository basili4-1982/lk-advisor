// Package emulator replays catalog relations against described client profiles.
//
// It is a simulation: no real traffic and no LLM calls. Relevance is a
// transparent score built from audience/industry match, a small task-concept
// dictionary and real purchase affinity from the dataset.
package emulator

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"
	"unicode"
)

// Profile is a described client used for a simulated run.
type Profile struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Audience    string `json:"audience"`
	Industry    string `json:"industry"`
	Description string `json:"description"`
}

// Options configures one emulation run.
type Options struct {
	ExperimentID string
	ProfileID    string
	Impressions  int
	Seed         int64
}

// VariantStats is the aggregate for control or treatment.
type VariantStats struct {
	Variant    string  `json:"variant"`
	Exposures  int     `json:"exposures"`
	Clicks     int     `json:"clicks"`
	Purchases  int     `json:"purchases"`
	Fit        float64 `json:"fit"`
	Conversion float64 `json:"conversion"`
	CTR        float64 `json:"ctr"`
}

// Session is one simulated client session.
type Session struct {
	N            int      `json:"n"`
	Variant      string   `json:"variant"`
	ProductID    string   `json:"product_id"`
	ProductTitle string   `json:"product_title"`
	Fit          float64  `json:"fit"`
	Clicked      bool     `json:"clicked"`
	Purchased    bool     `json:"purchased"`
	Matched      []string `json:"matched,omitempty"`
}

// Summary is stored in emulation_runs.summary and returned to the UI.
type Summary struct {
	ID             string       `json:"id"`
	ExperimentID   string       `json:"experiment_id"`
	ProfileID      string       `json:"profile_id"`
	ProfileName    string       `json:"profile_name"`
	Impressions    int          `json:"impressions"`
	Seed           int64        `json:"seed"`
	Fit            float64      `json:"fit"`
	Control        VariantStats `json:"control"`
	Treatment      VariantStats `json:"treatment"`
	Delta          float64      `json:"delta"`
	Uplift         float64      `json:"uplift"`
	Matched        []string     `json:"matched,omitempty"`
	Reasons        []string     `json:"reasons,omitempty"`
	ControlTitle   string       `json:"control_title"`
	TreatmentTitle string       `json:"treatment_title"`
	Sessions       []Session    `json:"sessions,omitempty"`
	CreatedAt      time.Time    `json:"created_at"`
}

// ErrNotFound means the experiment or profile does not exist.
var ErrNotFound = errors.New("emulation target not found")

var errNoControl = errors.New("no control product")

// Run simulates a hypothesis for one profile and writes the events into the
// experiment so the regular metrics and guardrails pick them up.
func Run(ctx context.Context, dbc *sql.DB, opts Options) (*Summary, error) {
	if opts.Impressions < 100 || opts.Impressions > 5000 {
		return nil, fmt.Errorf("количество показов должно быть от 100 до 5000")
	}
	var fromID, toID, relationType, expAudience, expIndustry string
	var split float64
	err := dbc.QueryRowContext(ctx, `
		SELECT from_id, to_id, relation_type, audience, industry, traffic_split::float8
		FROM experiments WHERE id = $1`, opts.ExperimentID).Scan(
		&fromID, &toID, &relationType, &expAudience, &expIndustry, &split)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load experiment: %w", err)
	}
	if split <= 0 || split >= 1 {
		split = 0.5
	}

	var profile Profile
	err = dbc.QueryRowContext(ctx, `
		SELECT id, name, audience, industry, description
		FROM client_profiles WHERE id = $1`, opts.ProfileID).Scan(
		&profile.ID, &profile.Name, &profile.Audience, &profile.Industry, &profile.Description)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load profile: %w", err)
	}

	target, err := loadItem(ctx, dbc, toID)
	if err != nil {
		return nil, fmt.Errorf("load target item: %w", err)
	}
	controlID, err := pickControl(ctx, dbc, fromID, toID)
	if err != nil {
		return nil, err
	}
	control, err := loadItem(ctx, dbc, controlID)
	if err != nil {
		return nil, fmt.Errorf("load control item: %w", err)
	}

	targetAffinity, targetAffinityNote := industryAffinity(ctx, dbc, target, profile.Industry)
	controlAffinity, _ := industryAffinity(ctx, dbc, control, profile.Industry)

	targetFit, targetMatched, _, reasons := scoreFit(profile, expAudience, expIndustry, relationType, target, targetAffinity)
	controlFit, controlMatched, _, _ := scoreFit(profile, expAudience, expIndustry, relationType, control, controlAffinity)

	seed := opts.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	rng := rand.New(rand.NewSource(seed))

	runID := fmt.Sprintf("emu-%d", time.Now().UnixNano())
	tx, err := dbc.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("emulation: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM experiment_events
		WHERE experiment_id = $1 AND client_id LIKE 'emu:%'`, opts.ExperimentID); err != nil {
		return nil, fmt.Errorf("emulation: clear previous events: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM emulation_runs WHERE experiment_id = $1`,
		opts.ExperimentID); err != nil {
		return nil, fmt.Errorf("emulation: clear previous runs: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO experiment_events
		(experiment_id, client_id, variant, trace_id, product_id, event,
		 renewal_shown, upsell_shown, fallback, margin_available)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, false)`)
	if err != nil {
		return nil, fmt.Errorf("emulation: prepare: %w", err)
	}
	defer stmt.Close()

	summary := &Summary{
		ID: runID, ExperimentID: opts.ExperimentID,
		ProfileID: profile.ID, ProfileName: profile.Name,
		Impressions: opts.Impressions, Seed: seed,
		Fit:            targetFit,
		Matched:        targetMatched,
		Reasons:        append(reasons, targetAffinityNote),
		Control:        VariantStats{Variant: "control", Fit: controlFit},
		Treatment:      VariantStats{Variant: "treatment", Fit: targetFit},
		ControlTitle:   control.Title,
		TreatmentTitle: target.Title,
		CreatedAt:      time.Now().UTC(),
	}

	for i := 0; i < opts.Impressions; i++ {
		variant := "control"
		fit := controlFit
		matched := controlMatched
		product := control
		if rng.Float64() < split {
			variant = "treatment"
			fit = targetFit
			matched = targetMatched
			product = target
		}
		stats := &summary.Control
		if variant == "treatment" {
			stats = &summary.Treatment
		}
		clientID := fmt.Sprintf("emu:%s:%d", runID, i)
		clicked := rng.Float64() < clickProbability(fit)
		purchased := clicked && rng.Float64() < purchaseProbability(fit)
		renewal := 0
		if rng.Float64() < 0.65 {
			renewal = 1
		}
		upsell := 0
		if rng.Float64() < 0.35 {
			upsell = 1
		}
		fallback := fit < 0.22 && rng.Float64() < 0.4

		if _, err := stmt.ExecContext(ctx, opts.ExperimentID, clientID, variant, runID,
			product.ID, "exposed", renewal, upsell, fallback); err != nil {
			return nil, fmt.Errorf("emulation: exposure: %w", err)
		}
		stats.Exposures++
		if clicked {
			if _, err := stmt.ExecContext(ctx, opts.ExperimentID, clientID, variant, runID,
				product.ID, "clicked", 0, 0, false); err != nil {
				return nil, fmt.Errorf("emulation: click: %w", err)
			}
			stats.Clicks++
		}
		if purchased {
			if _, err := stmt.ExecContext(ctx, opts.ExperimentID, clientID, variant, runID,
				product.ID, "purchased", 0, 0, false); err != nil {
				return nil, fmt.Errorf("emulation: purchase: %w", err)
			}
			stats.Purchases++
		}
		if len(summary.Sessions) < 8 {
			summary.Sessions = append(summary.Sessions, Session{
				N: i + 1, Variant: variant, ProductID: product.ID,
				ProductTitle: clip(product.Title, 70), Fit: fit,
				Clicked: clicked, Purchased: purchased, Matched: matched,
			})
		}
	}

	finalize(&summary.Control)
	finalize(&summary.Treatment)
	summary.Delta = summary.Treatment.Conversion - summary.Control.Conversion
	if summary.Control.Conversion > 0 {
		summary.Uplift = summary.Delta / summary.Control.Conversion
	}

	payload, err := json.Marshal(summary)
	if err != nil {
		return nil, fmt.Errorf("emulation: marshal summary: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO emulation_runs (id, experiment_id, profile_id, impressions, seed, fit, summary)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)`,
		runID, opts.ExperimentID, profile.ID, opts.Impressions, seed, targetFit, string(payload)); err != nil {
		return nil, fmt.Errorf("emulation: save run: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("emulation: commit: %w", err)
	}
	return summary, nil
}

func finalize(v *VariantStats) {
	if v.Exposures == 0 {
		return
	}
	v.Conversion = float64(v.Purchases) / float64(v.Exposures)
	v.CTR = float64(v.Clicks) / float64(v.Exposures)
}

func clickProbability(fit float64) float64 {
	return 0.08 + 0.62*math.Pow(clamp(fit, 0, 1), 0.9)
}

func purchaseProbability(fit float64) float64 {
	return 0.15 + 0.22*clamp(fit, 0, 1)
}

func loadItem(ctx context.Context, dbc *sql.DB, itemID string) (Item, error) {
	var it Item
	err := dbc.QueryRowContext(ctx, `
		SELECT item_id, title, category, subcategory, COALESCE(product_group, '')
		FROM lk_items WHERE item_id = $1`, itemID).Scan(
		&it.ID, &it.Title, &it.Category, &it.Subcategory, &it.Group)
	if errors.Is(err, sql.ErrNoRows) {
		return it, ErrNotFound
	}
	it.Title = cleanItemTitle(it.Title)
	return it, err
}

// cleanItemTitle removes marketplace boilerplate so the relevance dictionary
// does not match every item on words like "доступа".
func cleanItemTitle(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "(") {
		if i := strings.Index(s, ")"); i > 0 {
			s = strings.TrimSpace(s[i+1:])
		}
	}
	for _, p := range []string{
		"Предоставление доступа к сервисам ",
		"Предоставление доступа к ",
		"Программа контроля полномочий доступа к информационным ресурсам ",
		"Средство создания модели системы разграничения доступа ",
	} {
		if len(s) > len(p) && strings.HasPrefix(s, p) {
			return strings.TrimSpace(s[len(p):])
		}
	}
	return s
}

// Item is the catalog metadata used for scoring.
type Item struct {
	ID          string
	Title       string
	Category    string
	Subcategory string
	Group       string
}

// pickControl returns the generic bestseller that the recommender shows when
// the hypothesis is off. It is deliberately not another item from the same
// product line, otherwise the comparison would be meaningless.
func pickControl(ctx context.Context, dbc *sql.DB, fromID, toID string) (string, error) {
	var id string
	err := dbc.QueryRowContext(ctx, `
		SELECT o.item_id FROM lk_orders o
		WHERE o.item_id <> $1 AND o.item_id <> $2
		GROUP BY o.item_id ORDER BY count(*) DESC, o.item_id LIMIT 1`,
		fromID, toID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errNoControl
	}
	if err != nil {
		return "", fmt.Errorf("pick control: %w", err)
	}
	return id, nil
}

func industryAffinity(ctx context.Context, dbc *sql.DB, item Item, industry string) (float64, string) {
	if strings.TrimSpace(industry) == "" {
		return 0.3, "отрасль не указана: взята нейтральная отраслевая релевантность"
	}
	var buyers, indBuyers, indClients, total int
	err := dbc.QueryRowContext(ctx, `
		SELECT
			(SELECT count(DISTINCT client_id) FROM lk_orders WHERE item_id = $1),
			(SELECT count(DISTINCT o.client_id) FROM lk_orders o
			 JOIN lk_clients c ON c.client_id = o.client_id
			 WHERE o.item_id = $1 AND c.industry = $2),
			(SELECT count(*) FROM lk_clients WHERE industry = $2),
			(SELECT count(*) FROM lk_clients)`, item.ID, industry).Scan(
		&buyers, &indBuyers, &indClients, &total)
	if err == nil && buyers >= 10 && indBuyers >= 3 && indClients > 0 && total > 0 {
		base := float64(buyers) / float64(total)
		ind := float64(indBuyers) / float64(indClients)
		if base > 0 && ind > 0 {
			lift := ind / base
			return clamp((lift-0.5)/1.5, 0, 1),
				fmt.Sprintf("в отрасли «%s» товар берут в %.1f раза чаще среднего", industry, lift)
		}
	}
	if item.Group != "" {
		err = dbc.QueryRowContext(ctx, `
			SELECT
				(SELECT count(DISTINCT o.client_id) FROM lk_orders o
				 JOIN lk_items i ON i.item_id = o.item_id WHERE i.product_group = $1),
				(SELECT count(DISTINCT o.client_id) FROM lk_orders o
				 JOIN lk_items i ON i.item_id = o.item_id
				 JOIN lk_clients c ON c.client_id = o.client_id
				 WHERE i.product_group = $1 AND c.industry = $2),
				(SELECT count(*) FROM lk_clients WHERE industry = $2),
				(SELECT count(*) FROM lk_clients)`, item.Group, industry).Scan(
			&buyers, &indBuyers, &indClients, &total)
		if err == nil && buyers >= 20 && indBuyers >= 5 && indClients > 0 && total > 0 {
			base := float64(buyers) / float64(total)
			ind := float64(indBuyers) / float64(indClients)
			if base > 0 && ind > 0 {
				lift := ind / base
				return clamp((lift-0.5)/1.5, 0, 1),
					fmt.Sprintf("группа товаров в отрасли «%s» популярнее среднего в %.1f раза", industry, lift)
			}
		}
	}
	return 0.3, "мало отраслевых данных: взята нейтральная отраслевая релевантность"
}

func scoreFit(profile Profile, expAudience, expIndustry, relationType string, item Item, affinity float64) (float64, []string, []string, []string) {
	var reasons []string
	profileTokens := tokenize(profile.Description + " " + profile.Name)
	itemTokens := tokenize(item.Title + " " + item.Category + " " + item.Subcategory + " " + item.Group)

	concepts, words := matchTokens(profileTokens, itemTokens)
	conceptScore := math.Min(1, float64(len(concepts))/3)
	wordScore := math.Min(1, float64(len(words))/4)
	keywordScore := 0.8*conceptScore + 0.2*wordScore
	if len(concepts) == 0 {
		keywordScore = 0.5 * wordScore
	}

	fit := 0.0
	audience := strings.ToLower(strings.TrimSpace(expAudience))
	if audience == "" || strings.EqualFold(profile.Audience, audience) {
		fit += 0.15
		reasons = append(reasons, "аудитория профиля совпадает со скоупом гипотезы")
	} else {
		reasons = append(reasons, "аудитория профиля не совпадает со скоупом гипотезы")
	}

	industry := strings.TrimSpace(expIndustry)
	switch {
	case industry == "":
		fit += 0.05
	case strings.EqualFold(industry, profile.Industry):
		fit += 0.07
		reasons = append(reasons, "отрасль профиля совпадает со скоупом гипотезы")
	default:
		reasons = append(reasons, "отрасль профиля не совпадает со скоупом гипотезы")
	}

	fit += 0.65 * keywordScore
	fit += 0.10 * clamp(affinity, 0, 1)
	switch relationType {
	case "upgrade", "adjacent_task", "accessory", "bundle", "workflow_next_step":
		fit += 0.05
	case "substitute", "analog":
		fit -= 0.03
	}
	if len(concepts) > 0 {
		reasons = append(reasons, "смысловые совпадения: "+strings.Join(concepts, ", "))
	}
	if len(words) > 0 {
		reasons = append(reasons, "общие слова: "+strings.Join(words, ", "))
	}
	if audience != "" && !strings.EqualFold(profile.Audience, audience) {
		fit = math.Min(fit, 0.10)
	}
	if industry != "" && !strings.EqualFold(industry, profile.Industry) {
		fit = math.Min(fit, 0.5)
	}
	return clamp(fit, 0.02, 0.95), concepts, words, reasons
}

type concept struct {
	label string
	words []string
}

var concepts = []concept{
	{"почта", []string{"почт", "mail", "exchange", "outlook", "gmail", "workspace"}},
	{"офисные программы", []string{"офис", "office", "word", "excel", "документ", "workspace"}},
	{"антивирус и защита", []string{"антивирус", "защит", "security", "kaspersky", "eset", "dr.web", "угроз", "файрвол"}},
	{"резервное копирование", []string{"резервн", "backup", "архив", "восстановлен"}},
	{"контроль доступа", []string{"контрол", "полномоч", "ревизор", "идентификац", "разграничен"}},
	{"графика и дизайн", []string{"дизайн", "график", "coreldraw", "photoshop", "illustrator", "верстк"}},
	{"разработка", []string{"разработ", "программист", "ide", "git", "devtools", "отладк"}},
	{"обучение", []string{"обучен", "курс", "тренинг", "образован", "учебн"}},
	{"бухгалтерия", []string{"бухгалт", "налог"}},
	{"торговля и склад", []string{"касс", "склад", "торговл", "ритейл", "маркировк", "продаж"}},
	{"медицина", []string{"медицин", "клиник", "пациент", "стоматолог"}},
	{"инфраструктура", []string{"сервер", "виртуализац", "сетев", "linux"}},
}

func matchTokens(profileTokens, itemTokens []string) ([]string, []string) {
	var matchedConcepts []string
	seenConcept := map[string]bool{}
	for _, c := range concepts {
		profileHas := tokenListMatches(profileTokens, c.words)
		itemHas := tokenListMatches(itemTokens, c.words)
		if profileHas && itemHas && !seenConcept[c.label] {
			seenConcept[c.label] = true
			matchedConcepts = append(matchedConcepts, c.label)
		}
	}
	var words []string
	seenWord := map[string]bool{}
	for _, pt := range profileTokens {
		if seenWord[pt] || len([]rune(pt)) < 4 {
			continue
		}
		for _, it := range itemTokens {
			if tokenMatch(pt, it) {
				seenWord[pt] = true
				words = append(words, pt)
				break
			}
		}
	}
	if len(words) > 6 {
		words = words[:6]
	}
	return matchedConcepts, words
}

func tokenListMatches(tokens, words []string) bool {
	for _, t := range tokens {
		for _, w := range words {
			if tokenMatch(t, w) {
				return true
			}
		}
	}
	return false
}

func tokenMatch(a, b string) bool {
	if len([]rune(a)) < 4 || len([]rune(b)) < 4 {
		return false
	}
	return strings.HasPrefix(a, b) || strings.HasPrefix(b, a)
}

var stopwords = map[string]bool{
	"для": true, "что": true, "это": true, "как": true, "или": true, "при": true,
	"над": true, "под": true, "без": true, "через": true, "после": true, "перед": true,
	"нужны": true, "нужен": true, "нужна": true, "есть": true, "также": true,
	"очень": true, "более": true, "менее": true, "себя": true, "свой": true,
	"компания": true, "сотрудник": true, "сотрудников": true, "клиент": true,
	"покупатель": true, "бюджет": true, "цена": true, "цену": true,
}

func tokenize(s string) []string {
	s = strings.ToLower(strings.ReplaceAll(s, "ё", "е"))
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if len([]rune(f)) < 3 || stopwords[f] {
			continue
		}
		out = append(out, f)
	}
	return out
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clip(s string, limit int) string {
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
