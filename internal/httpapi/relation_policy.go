package httpapi

import (
	"os"
	"strconv"
	"strings"
)

// RelationPolicy routes a proposed relation by confidence.
//
// Defaults: >= 0.85 is accepted automatically, 0.70..0.85 goes to a human as
// a candidate, below 0.70 is quarantined. Auto-accept can be disabled so every
// relation passes through human review.
type RelationPolicy struct {
	AutoConfirm          bool
	AutoConfirmThreshold float64
	ReviewThreshold      float64
}

// DefaultRelationPolicy returns the policy used when nothing is configured.
func DefaultRelationPolicy() RelationPolicy {
	return RelationPolicy{
		AutoConfirm:          true,
		AutoConfirmThreshold: 0.85,
		ReviewThreshold:      0.70,
	}
}

func relationPolicyFromEnv() RelationPolicy {
	p := DefaultRelationPolicy()
	if v, ok := os.LookupEnv("RELATION_AUTO_CONFIRM"); ok {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "0", "false", "no", "off":
			p.AutoConfirm = false
		}
	}
	if v, ok := os.LookupEnv("RELATION_AUTO_CONFIRM_THRESHOLD"); ok {
		if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil && f >= 0 && f <= 1 {
			p.AutoConfirmThreshold = f
		}
	}
	if v, ok := os.LookupEnv("RELATION_REVIEW_THRESHOLD"); ok {
		if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil && f >= 0 && f <= 1 {
			p.ReviewThreshold = f
		}
	}
	if p.ReviewThreshold > p.AutoConfirmThreshold {
		p.ReviewThreshold = p.AutoConfirmThreshold
	}
	return p
}

// Status maps a confidence score to a relation status.
func (p RelationPolicy) Status(confidence float64) string {
	if p.AutoConfirm && confidence >= p.AutoConfirmThreshold {
		return "confirmed"
	}
	if confidence >= p.ReviewThreshold {
		return "llm_suggested"
	}
	return "quarantine"
}

func (h *handler) relationPolicy() RelationPolicy {
	if h.relPolicy == nil {
		return DefaultRelationPolicy()
	}
	return *h.relPolicy
}
