package catalog

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// GraphNode is one public item in the demo relation graph.
type GraphNode struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Category  string `json:"category"`
	Buyers    int    `json:"buyers"`
	B2CBuyers int    `json:"b2c_buyers"`
	Degree    int    `json:"degree"`
	Solo      bool   `json:"solo"`
}

// GraphEdge is one public co-purchase link.
type GraphEdge struct {
	Source    string `json:"source"`
	Target    string `json:"target"`
	Count     int    `json:"count"`
	B2BCount  int    `json:"b2b_count"`
	B2CCount  int    `json:"b2c_count"`
	B2CHidden bool   `json:"b2c_hidden"`
}

// DemoGraph is the public, privacy-filtered graph payload.
type DemoGraph struct {
	Audience   string          `json:"audience"`
	Nodes      []GraphNode     `json:"nodes"`
	Edges      []GraphEdge     `json:"edges"`
	Coverage   DatasetCoverage `json:"coverage"`
	Connected  int             `json:"connected"`
	PublicSolo int             `json:"public_solo"`
	ShownSolo  int             `json:"shown_solo"`
}

// LoadDemoGraph returns public nodes and edges for the demo graph.
// Suppressed items and edges below the audience k-threshold never leave this function.
func LoadDemoGraph(ctx context.Context, dbc *sql.DB, audience string, soloLimit int) (DemoGraph, error) {
	audience = strings.ToLower(strings.TrimSpace(audience))
	if audience != "b2c" {
		audience = "b2b"
	}
	if soloLimit < 0 {
		soloLimit = 0
	}
	if soloLimit > 1000 {
		soloLimit = 1000
	}

	edges, err := loadGraphEdges(ctx, dbc, audience)
	if err != nil {
		return DemoGraph{}, err
	}
	degree := map[string]int{}
	connected := map[string]bool{}
	for _, e := range edges {
		degree[e.Source]++
		degree[e.Target]++
		connected[e.Source] = true
		connected[e.Target] = true
	}

	rows, err := dbc.QueryContext(ctx, `
		SELECT i.item_id, i.title, i.category, p.buyers, p.b2c_buyers
		FROM lk_items i
		JOIN lk_item_privacy p ON p.item_id = i.item_id
		WHERE CASE WHEN $1 = 'b2c' THEN p.public_b2c ELSE p.public_b2b END
		ORDER BY p.buyers DESC, i.item_id`, audience)
	if err != nil {
		return DemoGraph{}, fmt.Errorf("demo graph nodes: %w", err)
	}
	defer rows.Close()

	publicSolo, shownSolo := 0, 0
	nodes := make([]GraphNode, 0, len(edges)*2+64)
	nodeIDs := map[string]bool{}
	for rows.Next() {
		var n GraphNode
		if err := rows.Scan(&n.ID, &n.Title, &n.Category, &n.Buyers, &n.B2CBuyers); err != nil {
			return DemoGraph{}, err
		}
		n.Degree = degree[n.ID]
		n.Solo = !connected[n.ID]
		if n.Solo {
			publicSolo++
			if soloLimit > 0 && shownSolo >= soloLimit {
				continue
			}
			shownSolo++
		}
		nodes = append(nodes, n)
		nodeIDs[n.ID] = true
	}
	if err := rows.Err(); err != nil {
		return DemoGraph{}, err
	}

	// Keep only edges whose endpoints made it into the payload.
	filtered := make([]GraphEdge, 0, len(edges))
	for _, e := range edges {
		if nodeIDs[e.Source] && nodeIDs[e.Target] {
			filtered = append(filtered, e)
		}
	}

	coverage, err := LoadDatasetCoverage(ctx, dbc)
	if err != nil {
		return DemoGraph{}, err
	}
	return DemoGraph{
		Audience:   audience,
		Nodes:      nodes,
		Edges:      filtered,
		Coverage:   coverage,
		Connected:  len(connected),
		PublicSolo: publicSolo,
		ShownSolo:  shownSolo,
	}, nil
}

func loadGraphEdges(ctx context.Context, dbc *sql.DB, audience string) ([]GraphEdge, error) {
	rows, err := dbc.QueryContext(ctx, `
		SELECT p.item_a, p.item_b, p.count, p.b2b_count, p.b2c_count, (p.b2c_count < 5)
		FROM co_purchase_pairs p
		JOIN lk_item_privacy a ON a.item_id = p.item_a
		JOIN lk_item_privacy b ON b.item_id = p.item_b
		WHERE CASE WHEN $1 = 'b2c'
		           THEN a.public_b2c AND b.public_b2c AND p.b2c_count >= 5
		           ELSE a.public_b2b AND b.public_b2b AND p.b2b_count >= 3 END
		ORDER BY p.count DESC, p.item_a, p.item_b
		LIMIT 2000`, audience)
	if err != nil {
		return nil, fmt.Errorf("demo graph edges: %w", err)
	}
	defer rows.Close()
	out := []GraphEdge{}
	for rows.Next() {
		var e GraphEdge
		if err := rows.Scan(&e.Source, &e.Target, &e.Count, &e.B2BCount, &e.B2CCount, &e.B2CHidden); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
