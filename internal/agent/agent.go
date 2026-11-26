// Package agent implements the grounded /v1/ask endpoint.
package agent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/basili4-1982/lk-advisor/internal/catalog"
	"github.com/basili4-1982/lk-advisor/internal/llm"
	"github.com/basili4-1982/lk-advisor/internal/model"
	"github.com/basili4-1982/lk-advisor/internal/profile"
	"github.com/basili4-1982/lk-advisor/internal/recommend"
)

// ErrInvalid marks a malformed request.
var ErrInvalid = errors.New("invalid request")

// ErrClientMismatch means the session belongs to another client.
var ErrClientMismatch = errors.New("session client mismatch")

// AskRequest is the /v1/ask payload.
type AskRequest struct {
	ClientID  string `json:"client_id"`
	SessionID string `json:"session_id"`
	Question  string `json:"question"`
}

// ToolCall is one step the agent performed.
type ToolCall struct {
	Tool string         `json:"tool"`
	Args map[string]any `json:"args"`
}

// AskResponse is the grounded answer with its trace.
type AskResponse struct {
	SessionID string           `json:"session_id"`
	Intent    string           `json:"intent"`
	Answer    string           `json:"answer"`
	ToolCalls []ToolCall       `json:"tool_calls"`
	Evidence  []model.Evidence `json:"evidence"`
	Fallback  bool             `json:"fallback"`
}

// Agent answers questions about one client's recommendations.
type Agent struct {
	DB     *sql.DB
	Now    func() time.Time
	LLM    *llm.Client
	Logger *slog.Logger
}

// New returns an agent with the real clock.
func New(dbc *sql.DB) *Agent {
	return &Agent{DB: dbc, Now: time.Now}
}

// NewWithClock returns an agent with an injected clock.
func NewWithClock(dbc *sql.DB, now func() time.Time) *Agent {
	return &Agent{DB: dbc, Now: now}
}

// NewWithDeps returns an agent with a clock and an optional LLM client.
func NewWithDeps(dbc *sql.DB, now func() time.Time, client *llm.Client) *Agent {
	return &Agent{DB: dbc, Now: now, LLM: client}
}

// Ask runs the grounded retrieval pipeline for one question.
func (a *Agent) Ask(ctx context.Context, req AskRequest) (AskResponse, error) {
	if a == nil || a.DB == nil {
		return AskResponse{}, errors.New("agent has no database")
	}
	start := time.Now()
	req.ClientID = trim(req.ClientID)
	req.Question = trim(req.Question)
	if req.ClientID == "" || req.Question == "" || len(req.Question) > 500 {
		return AskResponse{}, ErrInvalid
	}

	sess, err := loadSession(ctx, a.DB, req.SessionID, req.ClientID)
	if err != nil {
		return AskResponse{}, err
	}

	state, err := a.loadState(ctx, req.ClientID)
	if err != nil {
		return AskResponse{}, err
	}

	ex := extract(req.Question, state.products)
	ids := ex.Products
	if len(ids) == 0 {
		ids = sess.LastProducts
	}
	if len(ids) == 0 {
		for _, c := range state.candidates {
			ids = append(ids, c.Product.ID)
		}
	}

	resp := AskResponse{SessionID: sess.ID, Intent: ex.Type, ToolCalls: []ToolCall{}, Evidence: []model.Evidence{}, Fallback: state.fallback}

	switch ex.Type {
	case "off_topic":
		resp.Answer = "Я отвечаю только по рекомендациям и данным клиента."
	case "what_if":
		resp.ToolCalls = []ToolCall{
			{Tool: "get_profile", Args: map[string]any{"client_id": req.ClientID}},
			{Tool: "get_evidence", Args: map[string]any{"product_ids": ids}},
		}
		resp.Answer, resp.Evidence = answerWhatIf(state.profile, ids, a.Now())
	case "alternatives":
		resp.ToolCalls = []ToolCall{
			{Tool: "get_relations", Args: map[string]any{"product_ids": ids}},
			{Tool: "apply_rules", Args: map[string]any{"client_id": req.ClientID}},
		}
		resp.Answer, resp.Evidence = answerAlternatives(state.products, state.relations, state.rejected, ids)
	default:
		resp.ToolCalls = []ToolCall{
			{Tool: "get_recommendation", Args: map[string]any{"client_id": req.ClientID}},
			{Tool: "get_evidence", Args: map[string]any{"product_ids": ids}},
		}
		resp.Answer, resp.Evidence = answerWhy(state.candidates, ids, state.products)
	}

	allow := make([]string, 0, len(state.products)+len(state.candidates))
	for _, p := range state.products {
		allow = append(allow, p.Title)
	}
	for _, c := range state.candidates {
		allow = append(allow, fmt.Sprintf("%.2f", c.Score))
	}

	baseAnswer := resp.Answer
	f := formulation{Answer: baseAnswer}
	if ex.Type != "off_topic" {
		f = a.formulate(ctx, req.Question, ex, state, baseAnswer, resp.Evidence)
		resp.Answer = f.Answer
		if f.Failed {
			resp.Fallback = true
		}
	}

	if err := Guard(resp.Answer, resp.Evidence, allow...); err != nil {
		resp.Answer = fallbackAnswer(state.candidates)
		resp.Evidence = topEvidence(state.candidates)
		resp.Fallback = true
		f.GuardError = "final: " + err.Error()
	}

	sess.LastIntent = ex.Type
	sess.LastProducts = topIDs(state.candidates)
	if err := saveSession(ctx, a.DB, sess); err != nil {
		return AskResponse{}, err
	}

	a.recordCall(ctx, sess, req, ex, resp, baseAnswer, f, time.Since(start))
	return resp, nil
}

type state struct {
	profile    model.Profile
	products   map[string]model.Product
	relations  []model.Relation
	rejected   []model.Rejection
	candidates []model.Candidate
	fallback   bool
}

func (a *Agent) loadState(ctx context.Context, clientID string) (state, error) {
	p, err := profile.Get(ctx, a.DB, clientID)
	if err != nil {
		return state{}, err
	}
	products, err := catalog.AllProducts(ctx, a.DB)
	if err != nil {
		return state{}, err
	}
	rules, err := catalog.Rules(ctx, a.DB)
	if err != nil {
		return state{}, err
	}
	promotions, err := catalog.AllPromotions(ctx, a.DB)
	if err != nil {
		return state{}, err
	}
	weights, err := catalog.LoadWeights(ctx, a.DB)
	if err != nil {
		return state{}, err
	}
	relationTypes, err := catalog.RelationTypeMap(ctx, a.DB)
	if err != nil {
		return state{}, err
	}
	owned := ownedIDs(p)
	relations, err := catalog.RelationsScoped(ctx, a.DB, owned, p.Audience, p.Industry, a.Now())
	if err != nil {
		return state{}, err
	}
	in := recommend.Input{
		Profile: p, Products: products, Relations: relations,
		Promotions: promotions, Rules: rules, RelationTypes: relationTypes,
		Weights: weights, Now: a.Now(),
	}
	cands, rejected := recommend.Run(in)
	usedFallback := false
	if len(cands) == 0 {
		popular, err := catalog.PopularByAudience(ctx, a.DB, p.Audience, 3)
		if err == nil {
			for _, prod := range popular {
				cands = append(cands, model.Candidate{
					Product: prod, Type: "cross_sell", Relevance: 0.2,
					Affinity: 0.5, Confidence: 0.5,
					Evidence: []model.Evidence{{Source: "fallback", Value: "популярное в сегменте"}},
				})
			}
			usedFallback = len(cands) > 0
		}
	}
	return state{
		profile: p, products: products, relations: relations,
		rejected: rejected, candidates: cands, fallback: usedFallback,
	}, nil
}
