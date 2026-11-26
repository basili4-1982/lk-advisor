package catalog

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lib/pq"
)

// GraphNode is one public item in the demo relation graph.
type GraphNode struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Category      string `json:"category"`
	Family        string `json:"family"`
	ExternalCount int    `json:"external_count"`
	Buyers        int    `json:"buyers"`
	B2CBuyers     int    `json:"b2c_buyers"`
	Degree        int    `json:"degree"`
	Solo          bool   `json:"solo"`

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
	Type      string `json:"type,omitempty"`
	Status    string `json:"status,omitempty"`
	Manual    bool   `json:"manual,omitempty"`
}

// ExternalEdge is a link from the current page to a node on another page.
type ExternalEdge struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Title      string `json:"title"`
	TargetPage int    `json:"target_page"`
	Count      int    `json:"count"`
	B2BCount   int    `json:"b2b_count"`
	B2CCount   int    `json:"b2c_count"`
	B2CHidden  bool   `json:"b2c_hidden"`
}

// GraphPeriod is one selectable month in the demo timeline.
type GraphPeriod struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// DemoGraph is the public, privacy-filtered graph payload.
type GraphFilterOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

type DemoGraph struct {
	Audience      string              `json:"audience"`
	Industry      string              `json:"industry"`
	Segment       string              `json:"segment"`
	FilterLabel   string              `json:"filter_label"`
	Industries    []GraphFilterOption `json:"industries"`
	Segments      []GraphFilterOption `json:"segments"`
	Families      []GraphFilterOption `json:"families"`
	Nodes         []GraphNode         `json:"nodes"`
	Edges         []GraphEdge         `json:"edges"`
	Coverage      DatasetCoverage     `json:"coverage"`
	Connected     int                 `json:"connected"`
	PublicSolo    int                 `json:"public_solo"`
	ShownSolo     int                 `json:"shown_solo"`
	PublicItems   int                 `json:"public_items"`
	PublicPairs   int                 `json:"public_pairs"`
	SegmentItems  int                 `json:"segment_items"`
	SegmentHidden int                 `json:"segment_hidden"`
	Page          int                 `json:"page"`
	PageSize      int                 `json:"page_size"`
	TotalNodes    int                 `json:"total_nodes"`
	TotalPages    int                 `json:"total_pages"`
	ExternalEdges []ExternalEdge      `json:"external_edges"`

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
	return LoadDemoGraphFiltered(ctx, dbc, audience, "", "", "", soloLimit)
}

// LoadDemoGraphAt returns the public graph plus a monthly demand overlay.
func LoadDemoGraphAt(ctx context.Context, dbc *sql.DB, audience, period string, soloLimit int) (DemoGraph, error) {
	return LoadDemoGraphFiltered(ctx, dbc, audience, "", "", period, soloLimit)
}

// LoadDemoGraphFiltered returns the public graph for an audience and optional
// industry/segment. Segment filters recompute public items and pairs inside
// the segment, so k-anonymity is enforced per segment.
func LoadDemoGraphFiltered(ctx context.Context, dbc *sql.DB, audience, industry, segment, period string, soloLimit int) (DemoGraph, error) {
	return LoadDemoGraphPage(ctx, dbc, audience, industry, segment, period, 1, 1000, soloLimit)
}

// LoadDemoGraphPage paginates the public graph and returns external links to
// nodes that live on other pages.
func LoadDemoGraphPage(ctx context.Context, dbc *sql.DB, audience, industry, segment, period string, page, pageSize, soloLimit int) (DemoGraph, error) {
	audience = strings.ToLower(strings.TrimSpace(audience))
	if audience != "b2c" {
		audience = "b2b"
	}
	industry = strings.TrimSpace(industry)
	segment = strings.ToLower(strings.TrimSpace(segment))

	var graph DemoGraph
	var err error
	if industry == "" && segment == "" {
		graph, err = loadAggregateGraph(ctx, dbc, audience, soloLimit)
	} else {
		graph, err = loadSegmentGraph(ctx, dbc, audience, industry, segment, soloLimit)
	}
	if err != nil {
		return DemoGraph{}, err
	}
	graph.Audience = audience
	graph.Industry = industry
	graph.Segment = segment
	graph.FilterLabel = filterLabel(audience, industry, segment)
	graph.Industries, graph.Segments, err = loadGraphFilterOptions(ctx, dbc)
	if err != nil {
		return DemoGraph{}, err
	}
	if industry == "" && segment == "" {
		graph.PublicItems = graph.Coverage.PublicItems
		graph.PublicPairs = graph.Coverage.PublicPairs
		graph.SegmentItems = graph.Coverage.Items
		graph.SegmentHidden = graph.Coverage.SuppressedItems
	}
	if graph.PublicItems == 0 {
		graph.PublicItems = len(graph.Nodes)
	}
	if graph.PublicPairs == 0 {
		graph.PublicPairs = len(graph.Edges)
	}
	if err := appendManualEdges(ctx, dbc, &graph); err != nil {
		return DemoGraph{}, err
	}
	graph.Families = familyOptions(graph.Nodes)
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
		paginateGraph(&graph, page, pageSize)
		return graph, nil
	}
	start, err := time.Parse("2006-01", period)
	if err != nil {
		return DemoGraph{}, fmt.Errorf("invalid period %q: %w", period, err)
	}
	end := start.AddDate(0, 1, 0)
	prevStart := start.AddDate(0, -1, 0)
	periodItems, publicPeriod, err := applyPeriodDemand(ctx, dbc, audience, industry, segment, graph.Nodes, prevStart, start, end)
	if err != nil {
		return DemoGraph{}, err
	}
	graph.Period = period
	graph.PeriodLabel = monthNames[int(start.Month())] + " " + fmt.Sprintf("%d", start.Year())
	graph.PeriodItems = periodItems
	graph.PeriodPublicItems = publicPeriod
	graph.PeriodHiddenItems = periodItems - publicPeriod
	paginateGraph(&graph, page, pageSize)
	return graph, nil
}

func familyForTitle(title, category string) string {
	t := strings.ToLower(strings.ReplaceAll(title, "ё", "е"))
	has := func(parts ...string) bool {
		for _, p := range parts {
			if !strings.Contains(t, p) {
				return false
			}
		}
		return true
	}
	switch {
	case has("kaspersky", "endpoint"):
		return "Kaspersky Endpoint"
	case has("kaspersky", "small office"):
		return "Kaspersky Small Office"
	case has("kaspersky", "standard"):
		return "Kaspersky Standard"
	case has("kaspersky", "plus"):
		return "Kaspersky Plus"
	case has("kaspersky"):
		return "Kaspersky"
	case has("microsoft 365", "business"):
		return "Microsoft 365 Business"
	case has("microsoft 365", "premium"):
		return "Microsoft 365 Premium"
	case has("microsoft 365", "personal"):
		return "Microsoft 365 Personal"
	case has("microsoft 365", "family"):
		return "Microsoft 365 Family"
	case has("microsoft 365"):
		return "Microsoft 365"
	case has("office", "home"):
		return "Office Home"
	case has("office", "professional"):
		return "Office Professional"
	case has("office", "pro"):
		return "Office Professional"
	case has("office"):
		return "Microsoft Office"
	case has("windows", "server"):
		return "Windows Server"
	case has("win", "pro"):
		return "Windows Pro"
	case has("google workspace"):
		return "Google Workspace"
	case has("криптопро", "csp"):
		return "КриптоПро CSP"
	case has("криптоарм"):
		return "КриптоАРМ"
	case has("магпро"):
		return "МагПро"
	case has("dr.web") || has("drweb"):
		return "Dr.Web"
	case has("adobe"):
		return "Adobe"
	case has("coreldraw"):
		return "CorelDRAW"
	case has("nanocad"):
		return "nanoCAD"
	case has("anydesk"):
		return "AnyDesk"
	case has("visio"):
		return "Microsoft Visio"
	case has("project"):
		return "Microsoft Project"
	case has("pdf"):
		return "PDF Toolkit"
	}
	stop := map[string]bool{"лицензия": true, "дистрибутив": true, "программа": true,
		"программы": true, "эвм": true, "право": true, "использование": true,
		"использования": true, "предоставление": true, "доступа": true,
		"сервисам": true, "неисключительное": true, "бессрочная": true}
	for _, w := range strings.FieldsFunc(t, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= 'а' && r <= 'я') && !(r >= '0' && r <= '9')
	}) {
		if len(w) > 2 && !stop[w] {
			return strings.Title(w)
		}
	}
	if category != "" {
		return category
	}
	return "Другое"
}

func filterLabel(audience, industry, segment string) string {
	parts := []string{strings.ToUpper(audience)}
	if industry != "" {
		parts = append(parts, industry)
	}
	if segment != "" {
		parts = append(parts, segment)
	}
	return strings.Join(parts, " · ")
}

func appendManualEdges(ctx context.Context, dbc *sql.DB, g *DemoGraph) error {
	if len(g.Nodes) == 0 {
		return nil
	}
	ids := make([]string, 0, len(g.Nodes))
	for _, n := range g.Nodes {
		ids = append(ids, n.ID)
	}
	rows, err := dbc.QueryContext(ctx, `
		SELECT from_item_id, to_item_id, type, status, confidence::float8
		FROM lk_relation_candidates
		WHERE source = 'manual'
		  AND status IN ('quarantine', 'confirmed')
		  AND from_item_id = ANY($1)
		  AND to_item_id = ANY($1)`, pq.Array(ids))
	if err != nil {
		return fmt.Errorf("manual graph edges: %w", err)
	}
	defer rows.Close()
	manual := []GraphEdge{}
	for rows.Next() {
		var e GraphEdge
		var confidence float64
		if err := rows.Scan(&e.Source, &e.Target, &e.Type, &e.Status, &confidence); err != nil {
			return err
		}
		e.Manual = true
		e.Count = 1
		manual = append(manual, e)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(manual) == 0 {
		return nil
	}
	g.Edges = append(g.Edges, manual...)

	connected := map[string]bool{}
	degree := map[string]int{}
	for _, e := range g.Edges {
		degree[e.Source]++
		degree[e.Target]++
		connected[e.Source] = true
		connected[e.Target] = true
	}
	for i := range g.Nodes {
		g.Nodes[i].Degree = degree[g.Nodes[i].ID]
		g.Nodes[i].Solo = !connected[g.Nodes[i].ID]
	}
	g.Connected = len(connected)
	g.PublicSolo = 0
	for _, n := range g.Nodes {
		if n.Solo {
			g.PublicSolo++
		}
	}
	return nil
}

// CreateManualDatasetRelation stores a human-created relation in the constructor queue.
func CreateManualDatasetRelation(ctx context.Context, dbc *sql.DB, from, to, typ, status string, confidence float64, reason string) (bool, error) {
	from, to, typ = strings.TrimSpace(from), strings.TrimSpace(to), strings.TrimSpace(typ)
	if from == "" || to == "" || from == to || typ == "" {
		return false, fmt.Errorf("from_item_id, to_item_id and type are required")
	}
	if status != "quarantine" && status != "confirmed" {
		return false, fmt.Errorf("status must be quarantine or confirmed")
	}
	if confidence <= 0 || confidence > 1 {
		confidence = 0.8
	}
	rows, err := dbc.QueryContext(ctx, `
		SELECT item_id FROM lk_items WHERE item_id IN ($1, $2)`, from, to)
	if err != nil {
		return false, err
	}
	n := 0
	for rows.Next() {
		n++
	}
	rows.Close()
	if n != 2 {
		return false, fmt.Errorf("item not found")
	}
	res, err := dbc.ExecContext(ctx, `
		INSERT INTO lk_relation_candidates
			(from_item_id, to_item_id, type, confidence, status, reason, evidence, source)
		VALUES ($1, $2, $3, $4, $5, $6, '[]'::jsonb, 'manual')
		ON CONFLICT (from_item_id, to_item_id, type) DO UPDATE SET
			status = EXCLUDED.status,
			confidence = EXCLUDED.confidence,
			reason = EXCLUDED.reason,
			source = 'manual',
			reviewed_by = '',
			reviewed_at = NULL,
			review_note = ''`,
		from, to, typ, confidence, status, strings.TrimSpace(reason))
	if err != nil {
		return false, fmt.Errorf("create manual relation: %w", err)
	}
	affected, _ := res.RowsAffected()
	return affected > 0, nil
}

func familyOptions(nodes []GraphNode) []GraphFilterOption {
	counts := map[string]int{}
	for _, n := range nodes {
		if n.Family != "" {
			counts[n.Family]++
		}
	}
	out := make([]GraphFilterOption, 0, len(counts))
	for value, count := range counts {
		out = append(out, GraphFilterOption{Value: value, Label: value, Count: count})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Value < out[j].Value
	})
	if len(out) > 60 {
		out = out[:60]
	}
	return out
}

func loadGraphFilterOptions(ctx context.Context, dbc *sql.DB) ([]GraphFilterOption, []GraphFilterOption, error) {
	load := func(q string) ([]GraphFilterOption, error) {
		rows, err := dbc.QueryContext(ctx, q)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []GraphFilterOption{}
		for rows.Next() {
			var o GraphFilterOption
			if err := rows.Scan(&o.Value, &o.Count); err != nil {
				return nil, err
			}
			o.Label = o.Value
			out = append(out, o)
		}
		return out, rows.Err()
	}
	industries, err := load(`SELECT industry, count(*) FROM lk_clients WHERE industry <> '' GROUP BY industry ORDER BY count(*) DESC, industry`)
	if err != nil {
		return nil, nil, fmt.Errorf("graph industries: %w", err)
	}
	segments, err := load(`SELECT segment, count(*) FROM lk_clients WHERE segment <> '' GROUP BY segment ORDER BY count(*) DESC, segment`)
	if err != nil {
		return nil, nil, fmt.Errorf("graph segments: %w", err)
	}
	return industries, segments, nil
}

func loadSegmentGraph(ctx context.Context, dbc *sql.DB, audience, industry, segment string, soloLimit int) (DemoGraph, error) {
	if soloLimit < 0 {
		soloLimit = 0
	}
	if soloLimit > 1000 {
		soloLimit = 1000
	}
	group := "B2B"
	k := 3
	if audience == "b2c" {
		group = "B2C"
		k = 5
	}

	var segmentItems int
	if err := dbc.QueryRowContext(ctx, `
		SELECT count(DISTINCT o.item_id)
		FROM lk_orders o
		JOIN lk_clients c ON c.client_id = o.client_id
		WHERE c.client_group = $1
		  AND ($2 = '' OR c.industry = $2)
		  AND ($3 = '' OR c.segment = $3)`, group, industry, segment).Scan(&segmentItems); err != nil {
		return DemoGraph{}, fmt.Errorf("segment items: %w", err)
	}

	rows, err := dbc.QueryContext(ctx, `
		WITH buyers AS (
			SELECT o.item_id,
			       count(DISTINCT o.client_id) AS buyers,
			       count(DISTINCT o.client_id) FILTER (WHERE c.client_group = 'B2C') AS b2c_buyers
			FROM lk_orders o
			JOIN lk_clients c ON c.client_id = o.client_id
			WHERE c.client_group = $1
			  AND ($2 = '' OR c.industry = $2)
			  AND ($3 = '' OR c.segment = $3)
			GROUP BY o.item_id
		)
		SELECT i.item_id, i.title, i.category, b.buyers, b.b2c_buyers
		FROM buyers b
		JOIN lk_items i ON i.item_id = b.item_id
		WHERE CASE WHEN $1 = 'b2c' THEN b.b2c_buyers >= 5 ELSE b.buyers >= 3 END
		ORDER BY b.buyers DESC, i.item_id`, group, industry, segment)
	if err != nil {
		return DemoGraph{}, fmt.Errorf("segment nodes: %w", err)
	}
	var nodes []GraphNode
	nodeIDs := map[string]bool{}
	for rows.Next() {
		var n GraphNode
		if err := rows.Scan(&n.ID, &n.Title, &n.Category, &n.Buyers, &n.B2CBuyers); err != nil {
			rows.Close()
			return DemoGraph{}, err
		}
		n.Family = familyForTitle(n.Title, n.Category)
		nodes = append(nodes, n)
		nodeIDs[n.ID] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return DemoGraph{}, err
	}
	rows.Close()

	edgeRows, err := dbc.QueryContext(ctx, `
		WITH seg AS (
			SELECT DISTINCT o.order_id, o.order_week, o.item_id, o.client_id, c.client_group
			FROM lk_orders o
			JOIN lk_clients c ON c.client_id = o.client_id
			WHERE c.client_group = $1
			  AND ($2 = '' OR c.industry = $2)
			  AND ($3 = '' OR c.segment = $3)
		), buyers AS (
			SELECT item_id,
			       count(DISTINCT client_id) AS buyers,
			       count(DISTINCT client_id) FILTER (WHERE client_group = 'B2C') AS b2c_buyers
			FROM seg GROUP BY item_id
		), public AS (
			SELECT item_id FROM buyers
			WHERE CASE WHEN $1 = 'b2c' THEN b2c_buyers >= 5 ELSE buyers >= 3 END
		), pairs AS (
			SELECT a.item_id AS item_a, b.item_id AS item_b,
			       count(DISTINCT (a.order_id, a.order_week)) AS count
			FROM seg a
			JOIN seg b ON a.order_id = b.order_id AND a.order_week = b.order_week
			          AND a.item_id < b.item_id
			GROUP BY 1, 2
		)
		SELECT p.item_a, p.item_b, p.count,
		       CASE WHEN $1 = 'b2b' THEN p.count ELSE 0 END AS b2b_count,
		       CASE WHEN $1 = 'b2c' THEN p.count ELSE 0 END AS b2c_count
		FROM pairs p
		JOIN public pa ON pa.item_id = p.item_a
		JOIN public pb ON pb.item_id = p.item_b
		WHERE p.count >= $4
		ORDER BY p.count DESC, p.item_a, p.item_b
		LIMIT 2000`, group, industry, segment, k)
	if err != nil {
		return DemoGraph{}, fmt.Errorf("segment edges: %w", err)
	}
	var edges []GraphEdge
	degree := map[string]int{}
	connected := map[string]bool{}
	for edgeRows.Next() {
		var e GraphEdge
		if err := edgeRows.Scan(&e.Source, &e.Target, &e.Count, &e.B2BCount, &e.B2CCount); err != nil {
			edgeRows.Close()
			return DemoGraph{}, err
		}
		e.B2CHidden = e.B2CCount < 5
		edges = append(edges, e)
		degree[e.Source]++
		degree[e.Target]++
		connected[e.Source] = true
		connected[e.Target] = true
	}
	if err := edgeRows.Err(); err != nil {
		edgeRows.Close()
		return DemoGraph{}, err
	}
	edgeRows.Close()

	publicSolo, shownSolo := 0, 0
	filtered := make([]GraphNode, 0, len(nodes))
	for _, n := range nodes {
		n.Family = familyForTitle(n.Title, n.Category)
		n.Degree = degree[n.ID]
		n.Solo = !connected[n.ID]
		if n.Solo {
			publicSolo++
			if soloLimit > 0 && shownSolo >= soloLimit {
				continue
			}
			shownSolo++
		}
		filtered = append(filtered, n)
	}
	coverage, err := LoadDatasetCoverage(ctx, dbc)
	if err != nil {
		return DemoGraph{}, err
	}
	return DemoGraph{
		Nodes:         filtered,
		Edges:         edges,
		Coverage:      coverage,
		Connected:     len(connected),
		PublicSolo:    publicSolo,
		ShownSolo:     shownSolo,
		PublicItems:   len(nodes),
		PublicPairs:   len(edges),
		SegmentItems:  segmentItems,
		SegmentHidden: segmentItems - len(nodes),
	}, nil
}

func paginateGraph(g *DemoGraph, page, pageSize int) {
	if pageSize <= 0 {
		pageSize = 1000
	}
	if pageSize > 2000 {
		pageSize = 2000
	}
	sort.SliceStable(g.Nodes, func(i, j int) bool {
		a, b := g.Nodes[i], g.Nodes[j]
		if a.Degree != b.Degree {
			return a.Degree > b.Degree
		}
		if a.Buyers != b.Buyers {
			return a.Buyers > b.Buyers
		}
		return a.ID < b.ID
	})
	total := len(g.Nodes)
	g.TotalNodes = total
	g.TotalPages = (total + pageSize - 1) / pageSize
	if g.TotalPages == 0 {
		g.TotalPages = 1
	}
	if page < 1 {
		page = 1
	}
	if page > g.TotalPages {
		page = g.TotalPages
	}
	g.Page = page
	g.PageSize = pageSize
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	pageNodes := g.Nodes[start:end]
	pageIndex := map[string]int{}
	for i := range pageNodes {
		pageIndex[pageNodes[i].ID] = i
	}
	allIndex := map[string]int{}
	for i, n := range g.Nodes {
		allIndex[n.ID] = i
	}
	internal := make([]GraphEdge, 0, len(g.Edges))
	external := make([]ExternalEdge, 0)
	for _, e := range g.Edges {
		_, aIn := pageIndex[e.Source]
		_, bIn := pageIndex[e.Target]
		switch {
		case aIn && bIn:
			internal = append(internal, e)
		case aIn != bIn:
			from, to := e.Source, e.Target
			if !aIn {
				from, to = e.Target, e.Source
			}
			idx, ok := allIndex[to]
			if !ok {
				continue
			}
			targetPage := idx/pageSize + 1
			title := to
			for _, n := range g.Nodes {
				if n.ID == to {
					title = n.Title
					break
				}
			}
			external = append(external, ExternalEdge{
				From: from, To: to, Title: title, TargetPage: targetPage,
				Count: e.Count, B2BCount: e.B2BCount, B2CCount: e.B2CCount, B2CHidden: e.B2CHidden,
			})
			if i, ok := pageIndex[from]; ok {
				pageNodes[i].ExternalCount++
			}
		}
	}
	sort.Slice(external, func(i, j int) bool {
		if external[i].Count != external[j].Count {
			return external[i].Count > external[j].Count
		}
		if external[i].From != external[j].From {
			return external[i].From < external[j].From
		}
		return external[i].To < external[j].To
	})
	g.Nodes = pageNodes
	g.Edges = internal
	g.ExternalEdges = external
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
		n.Family = familyForTitle(n.Title, n.Category)
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

func applyPeriodDemand(ctx context.Context, dbc *sql.DB, audience, industry, segment string, nodes []GraphNode,
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
		  AND ($5 = '' OR c.industry = $5)
		  AND ($6 = '' OR c.segment = $6)
		  AND o.order_week >= $3 AND o.order_week < $2
		GROUP BY o.item_id`,
		start.Format("2006-01-02"), end.Format("2006-01-02"), prevStart.Format("2006-01-02"), group, industry, segment)
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
