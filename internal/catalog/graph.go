package catalog

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
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

	PeriodBuyers  int     `json:"period_buyers"`
	PrevBuyers    int     `json:"prev_buyers"`
	Growth        float64 `json:"growth"`
	DemandHidden  bool    `json:"demand_hidden"`
	GrowthHidden  bool    `json:"growth_hidden"`
	RenewalShare  float64 `json:"renewal_share"`
	RenewalDays   int     `json:"renewal_days"`
	RenewalHidden bool    `json:"renewal_hidden"`
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

// GraphPeriod is one selectable month in the demo timeline.
type GraphPeriod struct {
	Value string `json:"value"`
	Label string `json:"label"`
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

	Period            string        `json:"period"`
	PeriodLabel       string        `json:"period_label"`
	Periods           []GraphPeriod `json:"periods"`
	PeriodItems       int           `json:"period_items"`
	PeriodPublicItems int           `json:"period_public_items"`
	PeriodHiddenItems int           `json:"period_hidden_items"`
}

var monthNames = []string{
	"", "январь", "февраль", "март", "апрель", "май", "июнь",
	"июль", "август", "сентябрь", "октябрь", "ноябрь", "декабрь",
}

// LoadDemoGraph returns the all-time public graph.
func LoadDemoGraph(ctx context.Context, dbc *sql.DB, audience string, soloLimit int) (DemoGraph, error) {
	return LoadDemoGraphAt(ctx, dbc, audience, "", soloLimit)
}

// LoadDemoGraphAt returns the public graph plus a monthly demand overlay.
// The graph structure is all-time and public; only the demand numbers change.
func LoadDemoGraphAt(ctx context.Context, dbc *sql.DB, audience, period string, soloLimit int) (DemoGraph, error) {
	audience = strings.ToLower(strings.TrimSpace(audience))
	if audience != "b2c" {
		audience = "b2b"
	}
	graph, err := loadAggregateGraph(ctx, dbc, audience, soloLimit)
	if err != nil {
		return DemoGraph{}, err
	}
	periods, err := loadGraphPeriods(ctx, dbc)
	if err != nil {
		return DemoGraph{}, err
	}
	graph.Periods = periods
	graph.Period = ""
	graph.PeriodLabel = "Все время"
	graph.PeriodItems = graph.Coverage.Items
	graph.PeriodPublicItems = graph.Coverage.PublicItems
	graph.PeriodHiddenItems = graph.Coverage.SuppressedItems

	if err := applyRenewal(ctx, dbc, graph.Nodes); err != nil {
		return DemoGraph{}, err
	}

	period = strings.TrimSpace(period)
	if period == "" || period == "all" {
		return graph, nil
	}
	start, err := time.Parse("2006-01", period)
	if err != nil {
		return DemoGraph{}, fmt.Errorf("invalid period %q: %w", period, err)
	}
	end := start.AddDate(0, 1, 0)
	prevStart := start.AddDate(0, -1, 0)
	periodItems, publicPeriod, err := applyPeriodDemand(ctx, dbc, audience, graph.Nodes, prevStart, start, end)
	if err != nil {
		return DemoGraph{}, err
	}
	graph.Period = period
	graph.PeriodLabel = monthNames[int(start.Month())] + " " + fmt.Sprintf("%d", start.Year())
	graph.PeriodItems = periodItems
	graph.PeriodPublicItems = publicPeriod
	graph.PeriodHiddenItems = periodItems - publicPeriod
	return graph, nil
}

func loadAggregateGraph(ctx context.Context, dbc *sql.DB, audience string, soloLimit int) (DemoGraph, error) {
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

func loadGraphPeriods(ctx context.Context, dbc *sql.DB) ([]GraphPeriod, error) {
	rows, err := dbc.QueryContext(ctx, `
		SELECT DISTINCT to_char(order_week, 'YYYY-MM')
		FROM lk_orders
		ORDER BY 1`)
	if err != nil {
		return nil, fmt.Errorf("graph periods: %w", err)
	}
	defer rows.Close()
	out := []GraphPeriod{{Value: "", Label: "Все время"}}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		t, err := time.Parse("2006-01", v)
		if err != nil {
			continue
		}
		out = append(out, GraphPeriod{
			Value: v,
			Label: fmt.Sprintf("%s %d", monthNames[int(t.Month())], t.Year()),
		})
	}
	return out, rows.Err()
}

func applyRenewal(ctx context.Context, dbc *sql.DB, nodes []GraphNode) error {
	rows, err := dbc.QueryContext(ctx, `
		WITH gaps AS (
			SELECT client_id, item_id, order_week,
			       order_week - lag(order_week) OVER (
			           PARTITION BY client_id, item_id ORDER BY order_week
			       ) AS gap
			FROM lk_orders
		)
		SELECT item_id,
		       count(*) FILTER (WHERE gap IS NOT NULL) AS repeats,
		       count(*) FILTER (WHERE gap BETWEEN 300 AND 430) AS renewals,
		       COALESCE(percentile_cont(0.5) WITHIN GROUP (ORDER BY gap)
		                FILTER (WHERE gap BETWEEN 300 AND 430), 0)::float8 AS median_days
		FROM gaps
		GROUP BY item_id`)
	if err != nil {
		return fmt.Errorf("renewal stats: %w", err)
	}
	defer rows.Close()
	type stat struct {
		repeats, renewals int
		median            float64
	}
	stats := map[string]stat{}
	for rows.Next() {
		var id string
		var s stat
		if err := rows.Scan(&id, &s.repeats, &s.renewals, &s.median); err != nil {
			return err
		}
		stats[id] = s
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range nodes {
		s, ok := stats[nodes[i].ID]
		if !ok || s.repeats < 5 {
			nodes[i].RenewalHidden = true
			continue
		}
		nodes[i].RenewalShare = float64(s.renewals) / float64(s.repeats)
		nodes[i].RenewalDays = int(s.median)
	}
	return nil
}

func applyPeriodDemand(ctx context.Context, dbc *sql.DB, audience string, nodes []GraphNode,
	prevStart, start, end time.Time) (int, int, error) {
	group := "B2B"
	if audience == "b2c" {
		group = "B2C"
	}
	rows, err := dbc.QueryContext(ctx, `
		SELECT o.item_id,
		       count(DISTINCT o.client_id) FILTER (WHERE o.order_week >= $1 AND o.order_week < $2) AS cur,
		       count(DISTINCT o.client_id) FILTER (WHERE o.order_week >= $3 AND o.order_week < $1) AS prev,
		       count(*) FILTER (WHERE o.order_week >= $1 AND o.order_week < $2) AS rows
		FROM lk_orders o
		JOIN lk_clients c ON c.client_id = o.client_id
		WHERE c.client_group = $4
		  AND o.order_week >= $3 AND o.order_week < $2
		GROUP BY o.item_id`,
		start.Format("2006-01-02"), end.Format("2006-01-02"), prevStart.Format("2006-01-02"), group)
	if err != nil {
		return 0, 0, fmt.Errorf("period demand: %w", err)
	}
	defer rows.Close()
	type demand struct {
		cur, prev, rows int
	}
	demandByItem := map[string]demand{}
	periodItems := 0
	for rows.Next() {
		var id string
		var d demand
		if err := rows.Scan(&id, &d.cur, &d.prev, &d.rows); err != nil {
			return 0, 0, err
		}
		demandByItem[id] = d
		if d.rows > 0 {
			periodItems++
		}
	}
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}

	k := 3
	if audience == "b2c" {
		k = 5
	}
	publicPeriod := 0
	for i := range nodes {
		d := demandByItem[nodes[i].ID]
		if d.cur < k {
			nodes[i].DemandHidden = true
			nodes[i].PeriodBuyers = 0
			continue
		}
		publicPeriod++
		nodes[i].PeriodBuyers = d.cur
		nodes[i].PrevBuyers = d.prev
		if d.prev < k {
			nodes[i].GrowthHidden = true
			continue
		}
		nodes[i].Growth = float64(d.cur-d.prev) / float64(d.prev)
	}
	return periodItems, publicPeriod, nil
}
