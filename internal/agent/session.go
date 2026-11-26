package agent

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/basili4-1982/lk-advisor/internal/model"
)

type session struct {
	ID           string
	ClientID     string
	LastIntent   string
	LastProducts []string
}

func loadSession(ctx context.Context, dbc *sql.DB, id, clientID string) (session, error) {
	if id == "" {
		return session{ID: newSessionID(), ClientID: clientID}, nil
	}

	var s session
	var raw []byte
	err := dbc.QueryRowContext(ctx, `
		SELECT client_id, last_intent, last_product_ids
		FROM agent_sessions WHERE session_id = $1`, id).
		Scan(&s.ClientID, &s.LastIntent, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return session{ID: id, ClientID: clientID}, nil
	}
	if err != nil {
		return session{}, err
	}
	if s.ClientID != clientID {
		return session{}, ErrClientMismatch
	}
	_ = json.Unmarshal(raw, &s.LastProducts)
	s.ID = id
	return s, nil
}

func saveSession(ctx context.Context, dbc *sql.DB, s session) error {
	raw, err := json.Marshal(s.LastProducts)
	if err != nil {
		return err
	}
	_, err = dbc.ExecContext(ctx, `
		INSERT INTO agent_sessions (session_id, client_id, last_intent, last_product_ids, updated_at)
		VALUES ($1,$2,$3,$4,now())
		ON CONFLICT (session_id) DO UPDATE
		SET client_id = EXCLUDED.client_id,
		    last_intent = EXCLUDED.last_intent,
		    last_product_ids = EXCLUDED.last_product_ids,
		    updated_at = now()`,
		s.ID, s.ClientID, s.LastIntent, string(raw))
	return err
}

func newSessionID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "ses-fallback"
	}
	return "ses-" + hex.EncodeToString(b)
}

func topIDs(cands []model.Candidate) []string {
	out := make([]string, 0, 3)
	for _, c := range cands {
		out = append(out, c.Product.ID)
		if len(out) == 3 {
			break
		}
	}
	return out
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
