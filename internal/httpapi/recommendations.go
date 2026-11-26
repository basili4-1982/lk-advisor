package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	mathrand "math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/basili4-1982/lk-advisor/internal/analytics"
	"github.com/basili4-1982/lk-advisor/internal/catalog"
	"github.com/basili4-1982/lk-advisor/internal/experiment"
	"github.com/basili4-1982/lk-advisor/internal/explain"
	"github.com/basili4-1982/lk-advisor/internal/feedback"
	"github.com/basili4-1982/lk-advisor/internal/model"
	"github.com/basili4-1982/lk-advisor/internal/profile"
	"github.com/basili4-1982/lk-advisor/internal/recommend"
	"github.com/basili4-1982/lk-advisor/internal/strategy"
)

var allowedEvents = map[string]bool{
	"": true, "login": true, "license_view": true,
	"order_view": true, "catalog_view": true,
}

type recommendationRequest struct {
	ClientID string `json:"client_id"`
	Event    string `json:"event"`
	Limit    *int   `json:"limit"`
	Audience string `json:"audience"`
	Industry string `json:"industry"`
	Strategy string `json:"strategy"`
}

type recommendationResponse struct {
	TraceID         string                 `json:"trace_id"`
	ClientID        string                 `json:"client_id"`
	Strategy        string                 `json:"strategy"`
	Intent          string                 `json:"intent"`
	Weights         model.Weights          `json:"weights"`
	Recommendations []model.Recommendation `json:"recommendations"`
	Fallback        bool                   `json:"fallback"`
	GeneratedAt     time.Time              `json:"generated_at"`
}

func (h *handler) recommendations(w http.ResponseWriter, r *http.Request) {
	var req recommendationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(req.ClientID) == "" {
		writeError(w, http.StatusBadRequest, "client_id is required")
		return
	}
	if !allowedEvents[req.Event] {
		writeError(w, http.StatusBadRequest, "unknown event")
		return
	}
	limit := 3
	if req.Limit != nil {
		if *req.Limit < 1 || *req.Limit > 5 {
			writeError(w, http.StatusBadRequest, "limit must be between 1 and 5")
			return
		}
		limit = *req.Limit
	}
	if h.db == nil {
		writeError(w, http.StatusInternalServerError, "database is not configured")
		return
	}

	ctx := r.Context()
	traceID := newTraceID()
	p, err := profile.Get(ctx, h.db, req.ClientID)
	if err == nil {
		if strings.TrimSpace(req.Audience) != "" {
			p.Audience = strings.TrimSpace(req.Audience)
		}
		if strings.TrimSpace(req.Industry) != "" {
			p.Industry = strings.TrimSpace(req.Industry)
		}
	}
	if errors.Is(err, profile.ErrNotFound) {
		writeError(w, http.StatusNotFound, "client not found")
		return
	}
	if err != nil {
		h.logger.Error("profile", "err", err)
		writeError(w, http.StatusInternalServerError, "profile error")
		return
	}

	products, err := catalog.AllProducts(ctx, h.db)
	if err != nil {
		h.logger.Error("products", "err", err)
		writeError(w, http.StatusInternalServerError, "catalog error")
		return
	}
	rules, err := catalog.Rules(ctx, h.db)
	if err != nil {
		h.logger.Error("rules", "err", err)
		writeError(w, http.StatusInternalServerError, "rules error")
		return
	}
	audience := strings.TrimSpace(p.Audience)
	if audience == "" {
		audience = "b2b"
	}
	industry := strings.ToLower(strings.TrimSpace(p.Industry))
	strategyName := strategy.Default(audience)
	if forced := strings.TrimSpace(req.Strategy); forced != "" {
		if forced != "hybrid" && forced != "popular" {
			writeError(w, http.StatusBadRequest, "strategy must be hybrid or popular")
			return
		}
		strategyName = forced
	} else if auto, aerr := catalog.AutoStrategy(ctx, h.db); aerr == nil && auto {
		if stats, serr := catalog.StrategyStats(ctx, h.db); serr == nil {
			strategyName = strategy.Choose(audience, stats)
		}
	}
	weights, err := catalog.LoadWeightsForAudience(ctx, h.db, audience)
	if err != nil {
		h.logger.Error("weights", "err", err)
		writeError(w, http.StatusInternalServerError, "settings error")
		return
	}
	weightsVersion := strategyName + "/" + audience
	relationTypes, err := catalog.RelationTypeMap(ctx, h.db)
	if err != nil {
		h.logger.Error("relation types", "err", err)
		writeError(w, http.StatusInternalServerError, "relation types error")
		return
	}
	promotions, err := catalog.AllPromotions(ctx, h.db)
	if err != nil {
		h.logger.Error("promotions", "err", err)
		writeError(w, http.StatusInternalServerError, "promotions error")
		return
	}

	popularity, perr := catalog.PopularityScores(ctx, h.db, audience, h.now(), 8)
	if perr != nil {
		h.logger.Warn("popularity", "err", perr)
		popularity = nil
	}

	owned := ownedIDs(p)
	relations, err := catalog.RelationsScoped(ctx, h.db, owned, audience, industry, h.now())
	if err != nil {
		h.logger.Error("relations", "err", err)
		writeError(w, http.StatusInternalServerError, "relations error")
		return
	}

	var runningExperiment *experiment.Experiment
	var assignment experiment.Assignment
	if list, xerr := experiment.LoadRunning(ctx, h.db, audience, industry); xerr == nil && len(list) > 0 {
		e := list[0]
		if a, aerr := experiment.Assign(ctx, h.db, e, req.ClientID); aerr == nil {
			runningExperiment = &e
			assignment = a
		} else {
			h.logger.Warn("experiment assignment", "err", aerr)
		}
	}

	in := recommend.Input{
		Profile: p, Products: products, Relations: relations,
		Promotions: promotions, Rules: rules, RelationTypes: relationTypes,
		Weights: weights, Popularity: popularity, Now: h.now(),
	}
	if runningExperiment != nil && assignment.Treatment {
		in.Experiment = &recommend.ExperimentTreatment{
			ID:              runningExperiment.ID,
			Variant:         assignment.Variant,
			TargetProductID: runningExperiment.ToID,
			FromID:          runningExperiment.FromID,
			RelationType:    runningExperiment.RelationType,
		}
	}
	intent := recommend.Intent(in)

	var allowed []model.Candidate
	var rejected []model.Rejection
	if strategyName != "popular" {
		allowed, rejected = recommend.Rank(in)
	}
	if len(allowed) > 0 {
		ids := make([]string, 0, len(allowed))
		for _, c := range allowed {
			ids = append(ids, c.Product.ID)
		}
		if stats, ferr := feedback.Load(ctx, h.db, ids, h.now()); ferr == nil {
			feedback.Apply(allowed, stats)
			recommend.Sort(allowed)
		} else {
			h.logger.Warn("feedback", "err", ferr)
		}
	}
	cands := recommend.Diversify(allowed, limit)
	cands = recommend.FillPopular(in, cands, limit)
	cands = recommend.ForceExperiment(in, cands, limit)
	// Explore only the last cross-sell slot: never replace renewal, upsell,
	// analog or alternative, which carry the main business explanation.
	if limit >= 3 && len(cands) >= 3 && cands[limit-1].Type == "cross_sell" {
		if explore, ok := recommend.ExplorationSlot(allowed, cands, mathrand.Float64()); ok {
			cands[limit-1] = explore
		}
	}
	fallback := false
	if len(cands) == 0 {
		popular, err := catalog.PopularByAudience(ctx, h.db, audience, 3)
		if err != nil {
			h.logger.Error("popular", "err", err)
			writeError(w, http.StatusInternalServerError, "fallback error")
			return
		}
		if len(popular) == 0 {
			_ = analytics.Log(ctx, h.db, traceID, req.ClientID, intent,
				analytics.Meta{Strategy: strategyName, Audience: audience, Industry: industry, WeightsVersion: weightsVersion},
				true, nil, nil)
			writeError(w, http.StatusConflict, "no data for calculation")
			return
		}
		fallback = true
		for _, prod := range popular {
			c := model.Candidate{
				Product: prod, Type: "cross_sell", Relevance: 0.2,
				Affinity: 0.5, Confidence: 0.5,
				Evidence: []model.Evidence{{Source: "fallback", Value: "popular in segment"}},
			}
			cands = append(cands, recommend.ScoreWith(c, h.now(), weights))
		}
		// Keep the popularity order returned by the catalog; do not re-sort by margin.
		if limit < len(cands) {
			cands = cands[:limit]
		}
	}

	recs := make([]model.Recommendation, 0, len(cands))
	for i, c := range cands {
		rec := model.Recommendation{
			ProductID:          c.Product.ID,
			SKU:                c.Product.SKU,
			Title:              c.Product.Title,
			Type:               c.Type,
			Score:              c.Score,
			FeedbackMultiplier: c.FeedbackMultiplier,
			Exploration:        c.Exploration,
			SourceProductID:    c.SourceProductID,
			RelationType:       c.RelationType,
			CandidateID:        c.CandidateID,
			Rank:               i + 1,
			Evidence:           c.Evidence,
			Constraints:        c.Constraints,
			Price:              model.Price{Amount: c.Product.Price, Currency: "RUB"},
			Promo:              c.Promo,
			ExpiresAt:          h.expiresAt(c, p),
		}
		if rec.Evidence == nil {
			rec.Evidence = []model.Evidence{}
		}
		if rec.Constraints == nil {
			rec.Constraints = []string{}
		}
		reason, err := explain.Render(ctx, rec, rec.Evidence)
		if err != nil {
			reason = fmt.Sprintf("Рекомендуем %s.", rec.Title)
		}
		rec.Reason = reason
		rec.Factors = &model.ScoreFactors{
			Relevance: c.Relevance, Urgency: c.Urgency, Affinity: c.Affinity,
			Margin: c.Margin, Confidence: c.Confidence,
		}
		recs = append(recs, rec)
	}

	shownItems := make([]analytics.Item, 0, len(cands))
	shown := map[string]bool{}
	source := "engine"
	if fallback {
		source = "fallback"
	}
	for i, c := range cands {
		shown[c.Product.ID] = true
		detail := ""
		if i < len(recs) {
			detail = recs[i].Reason
		}
		itemSource := source
		if c.Exploration {
			itemSource = "exploration"
		}
		shownItems = append(shownItems, analytics.Item{
			ProductID: c.Product.ID, Type: c.Type, Reason: "top_3",
			Detail: detail, Source: itemSource, Rank: i + 1, Score: c.Score,
			CandidateID: c.CandidateID, SourceProductID: c.SourceProductID, RelationType: c.RelationType,
		})
	}
	skippedItems := make([]analytics.Item, 0, len(rejected)+len(allowed))
	for _, r := range rejected {
		skippedItems = append(skippedItems, analytics.Item{
			ProductID: r.ProductID, Reason: "rule:" + r.Rule, Detail: r.Reason,
		})
	}
	for _, c := range allowed {
		if shown[c.Product.ID] {
			continue
		}
		skippedItems = append(skippedItems, analytics.Item{
			ProductID: c.Product.ID, Type: c.Type, Reason: "below_top_3",
			Detail: "не вошёл в топ-3 после разнообразия", Source: "engine", Score: c.Score,
		})
	}
	if len(skippedItems) > 25 {
		skippedItems = skippedItems[:25]
	}
	meta := analytics.Meta{Strategy: strategyName, Audience: audience, Industry: industry, WeightsVersion: weightsVersion}
	if err := analytics.Log(ctx, h.db, traceID, req.ClientID, intent, meta, fallback, shownItems, skippedItems); err != nil {
		h.logger.Warn("analytics log", "err", err)
	}
	if runningExperiment != nil {
		exposedTarget := ""
		for _, c := range cands {
			if c.Product.ID == runningExperiment.ToID {
				exposedTarget = runningExperiment.ToID
				break
			}
		}
		if err := experiment.RecordExposure(ctx, h.db, assignment, traceID, exposedTarget); err != nil {
			h.logger.Warn("experiment exposure", "err", err)
		}
	}

	writeJSON(w, http.StatusOK, recommendationResponse{
		TraceID:         traceID,
		ClientID:        req.ClientID,
		Strategy:        strategyName,
		Intent:          intent,
		Weights:         weights,
		Recommendations: recs,
		Fallback:        fallback,
		GeneratedAt:     h.now().UTC(),
	})
}

func (h *handler) expiresAt(c model.Candidate, p model.Profile) time.Time {
	now := h.now()
	t := now.AddDate(0, 0, 30)
	for _, l := range p.Licenses {
		if l.ProductID == c.Product.ID && l.Status == "active" &&
			l.End.After(now) && l.End.Before(t) {
			t = l.End
		}
	}
	if c.Promo != nil && c.Promo.ValidTo.After(now) && c.Promo.ValidTo.Before(t) {
		t = c.Promo.ValidTo
	}
	return t
}

func ownedIDs(p model.Profile) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range p.Purchases {
		if !seen[x.ProductID] {
			seen[x.ProductID] = true
			out = append(out, x.ProductID)
		}
	}
	for _, x := range p.Licenses {
		if !seen[x.ProductID] {
			seen[x.ProductID] = true
			out = append(out, x.ProductID)
		}
	}
	return out
}

func newTraceID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("tr-%d", time.Now().UnixNano())
	}
	return "tr-" + hex.EncodeToString(b)
}
