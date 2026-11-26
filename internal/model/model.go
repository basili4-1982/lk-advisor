// Package model contains the domain types shared by storage, engine and API.
package model

import (
	"hash/fnv"
	"time"
)

// Profile is the materialized view of a client: purchases, licenses and environment.
type Profile struct {
	ClientID    string      `json:"client_id"`
	Segment     string      `json:"segment"`
	Industry    string      `json:"industry"`
	Size        string      `json:"size"`
	Audience    string      `json:"audience"`
	Licenses    []License   `json:"licenses"`
	Purchases   []Purchase  `json:"purchases"`
	Environment Environment `json:"environment"`
}

// License is an active or expired license.
type License struct {
	ID        string    `json:"id"`
	ClientID  string    `json:"client_id,omitempty"`
	ProductID string    `json:"product_id"`
	Seats     int       `json:"seats"`
	Start     time.Time `json:"start"`
	End       time.Time `json:"end"`
	Status    string    `json:"status"`
}

// Purchase is one line of purchase history.
type Purchase struct {
	ID        string    `json:"id"`
	ClientID  string    `json:"client_id,omitempty"`
	ProductID string    `json:"product_id"`
	Date      time.Time `json:"date"`
	Amount    int64     `json:"amount"`
	Quantity  int       `json:"quantity"`
	Seats     int       `json:"seats"`
}

// Environment is inferred from previous purchases.
type Environment struct {
	ClientID   string  `json:"client_id,omitempty"`
	OS         string  `json:"os"`
	RAMGB      int     `json:"ram_gb"`
	CPU        string  `json:"cpu"`
	GPU        string  `json:"gpu"`
	Source     string  `json:"source"`
	Confidence float64 `json:"confidence"`
}

// Client is one SMB customer.
type Client struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Segment  string `json:"segment"`
	Industry string `json:"industry"`
	Size     string `json:"size"`
	Region   string `json:"region"`
	Audience string `json:"audience"`
	Shard    int    `json:"shard"`
}

// Order is a purchase order.
type Order struct {
	ID          string      `json:"id"`
	ClientID    string      `json:"client_id"`
	Status      string      `json:"status"`
	CreatedAt   time.Time   `json:"created_at"`
	DeliveredAt *time.Time  `json:"delivered_at,omitempty"`
	Items       []OrderItem `json:"items"`
}

// OrderItem is one line of an order.
type OrderItem struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

// Event is a cabinet visit event.
type Event struct {
	ID       string            `json:"id"`
	ClientID string            `json:"client_id"`
	Type     string            `json:"type"`
	TS       time.Time         `json:"ts"`
	Context  map[string]string `json:"context"`
}

// Product is a catalog item.
type Product struct {
	ID           string       `json:"id"`
	SKU          string       `json:"sku"`
	Title        string       `json:"title"`
	Category     string       `json:"category"`
	Subcategory  string       `json:"subcategory"`
	Vendor       string       `json:"vendor"`
	Type         string       `json:"type"`
	Price        int64        `json:"price"`
	TermMonths   int          `json:"term_months"`
	Seats        int          `json:"seats"`
	Version      string       `json:"version"`
	Status       string       `json:"status"`
	Audience     string       `json:"audience"`
	Requirements Requirements `json:"system_requirements"`
}

// Requirements is the system requirements of a product.
type Requirements struct {
	OS     string `json:"os"`
	RAMGB  int    `json:"ram_gb"`
	CPU    string `json:"cpu"`
	GPU    string `json:"gpu"`
	DiskGB int    `json:"disk_gb"`
}

// Relation is an edge between two products.
type Relation struct {
	FromID     string     `json:"from_id"`
	ToID       string     `json:"to_id"`
	Type       string     `json:"type"`
	Direction  string     `json:"direction"`
	Confidence float64    `json:"confidence"`
	Status     string     `json:"status"`
	Source     string     `json:"source"`
	Reason     string     `json:"reason,omitempty"`
	Evidence   []Evidence `json:"evidence,omitempty"`
	ReviewedBy string     `json:"reviewed_by,omitempty"`
	ReviewedAt *time.Time `json:"reviewed_at,omitempty"`
	ReviewNote string     `json:"review_note,omitempty"`
}

// Promotion is an active discount with a validity window.
type Promotion struct {
	ID         string    `json:"id"`
	ProductID  string    `json:"product_id"`
	Discount   float64   `json:"discount"`
	ValidFrom  time.Time `json:"valid_from"`
	ValidTo    time.Time `json:"valid_to"`
	Conditions string    `json:"conditions"`
}

// Candidate is a product considered for recommendation.
type Candidate struct {
	Product     Product    `json:"product"`
	Type        string     `json:"type"`
	Relevance   float64    `json:"relevance"`
	Urgency     float64    `json:"urgency"`
	Affinity    float64    `json:"affinity"`
	Margin      float64    `json:"margin"`
	Confidence  float64    `json:"confidence"`
	Score       float64    `json:"score"`
	Promo       *Promotion `json:"promo,omitempty"`
	Constraints []string   `json:"constraints,omitempty"`
	Evidence    []Evidence `json:"-"`
}

// Evidence is one fact a reason may cite.
type Evidence struct {
	Source string `json:"source"`
	Value  string `json:"value"`
}

// Price is a money amount.
type Price struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

// Recommendation is one item of the API response.
type Recommendation struct {
	ProductID   string     `json:"product_id"`
	SKU         string     `json:"sku"`
	Title       string     `json:"title"`
	Type        string     `json:"type"`
	Score       float64    `json:"score"`
	Rank        int        `json:"rank"`
	Reason      string     `json:"reason"`
	Evidence    []Evidence `json:"evidence"`
	Constraints []string   `json:"constraints"`
	Price       Price      `json:"price"`
	Promo       *Promotion `json:"promo,omitempty"`
	ExpiresAt   time.Time  `json:"expires_at"`
}

// Rule is a business restriction.
type Rule struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Condition string `json:"condition"`
	Action    string `json:"action"`
}

// Rejection explains why a candidate was removed.
type Rejection struct {
	ProductID string `json:"product_id"`
	Rule      string `json:"rule"`
	Reason    string `json:"reason"`
}

// GroundTruthCase is one labelled evaluation case.
type GroundTruthCase struct {
	ClientID        string `json:"client_id"`
	Event           string `json:"event"`
	ExpectedProduct string `json:"expected_product_id"`
	ExpectedType    string `json:"expected_type"`
}

// Shard returns a stable shard for a client id.
func Shard(clientID string, n int) int {
	if n <= 0 {
		return 0
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(clientID))
	return int(h.Sum32() % uint32(n))
}

// Weights are the scoring coefficients of the recommendation formula.
type Weights struct {
	Relevance  float64 `json:"relevance"`
	Urgency    float64 `json:"urgency"`
	Affinity   float64 `json:"affinity"`
	Margin     float64 `json:"margin"`
	Confidence float64 `json:"confidence"`
}

// DefaultWeights matches the original fixed formula.
func DefaultWeights() Weights {
	return Weights{Relevance: 0.35, Urgency: 0.25, Affinity: 0.20, Margin: 0.10, Confidence: 0.10}
}

// Normalize clamps negative values and scales the weights to sum to one.
func (w Weights) Normalize() Weights {
	w.Relevance = maxFloat(0, w.Relevance)
	w.Urgency = maxFloat(0, w.Urgency)
	w.Affinity = maxFloat(0, w.Affinity)
	w.Margin = maxFloat(0, w.Margin)
	w.Confidence = maxFloat(0, w.Confidence)
	sum := w.Relevance + w.Urgency + w.Affinity + w.Margin + w.Confidence
	if sum <= 0 {
		return DefaultWeights()
	}
	w.Relevance /= sum
	w.Urgency /= sum
	w.Affinity /= sum
	w.Margin /= sum
	w.Confidence /= sum
	return w
}

// IsZero reports whether the weights were not configured.
func (w Weights) IsZero() bool {
	return w.Relevance == 0 && w.Urgency == 0 && w.Affinity == 0 && w.Margin == 0 && w.Confidence == 0
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// RelationType is an editable template for product links.
type RelationType struct {
	Code              string    `json:"code"`
	Title             string    `json:"title"`
	Description       string    `json:"description"`
	Examples          string    `json:"examples"`
	Behavior          string    `json:"behavior"`
	Direction         string    `json:"direction"`
	DefaultConfidence float64   `json:"default_confidence"`
	Status            string    `json:"status"`
	Builtin           bool      `json:"builtin"`
	UpdatedAt         time.Time `json:"updated_at"`
}
