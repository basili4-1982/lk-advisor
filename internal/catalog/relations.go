package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lib/pq"
)

// RelationFilter narrows the relation registry listing.
// Zero values mean "no filter"; user input always goes through SQL parameters.
type RelationFilter struct {
	Query         string   `json:"query"`
	IDs           []int64  `json:"ids"`
	Types         []string `json:"types"`
	Statuses      []string `json:"statuses"`
	Provenance    []string `json:"provenance"`
	Family        string   `json:"family"`
	Audience      string   `json:"audience"`
	OrgType       string   `json:"org_type"`
	OwnerID       string   `json:"owner_id"`
	MinConfidence float64  `json:"min_confidence"`
	MaxConfidence float64  `json:"max_confidence"`
	ExpiringDays  int      `json:"expiring_days"`
	Limit         int      `json:"limit"`
	Offset        int      `json:"offset"`
	Sort          string   `json:"sort"`
}

// RelationEvidence is one reason the registry keeps an edge.
// Legacy rows store {"source","value"} and are mapped to Kind/Summary.
type RelationEvidence struct {
	Kind    string         `json:"kind"`
	Summary string         `json:"summary"`
	Details map[string]any `json:"details,omitempty"`
}

// RegistryRelation is one edge of the v5 registry with both item cards.
type RegistryRelation struct {
	ID              int64              `json:"id"`
	FromID          string             `json:"from_item_id"`
	ToID            string             `json:"to_item_id"`
	Type            string             `json:"type"`
	Status          string             `json:"status"`
	ProvenanceKind  string             `json:"provenance_kind"`
	Reason          string             `json:"reason"`
	Confidence      float64            `json:"confidence"`
	Evidence        []RelationEvidence `json:"evidence"`
	Audience        string             `json:"audience"`
	Industry        string             `json:"industry"`
	OrgType         string             `json:"org_type"`
	OwnerID         string             `json:"owner_id"`
	Version         int                `json:"version"`
	Position        int                `json:"position"`
	ValidUntil      *time.Time         `json:"valid_until,omitempty"`
	RevalidateAfter *time.Time         `json:"revalidate_after,omitempty"`
	RejectedUntil   *time.Time         `json:"rejected_until,omitempty"`
	RejectionReason string             `json:"rejection_reason"`
	ReviewNote      string             `json:"review_note"`
	ReviewedBy      string             `json:"reviewed_by"`
	ApprovedBy      string             `json:"approved_by"`
	ApprovedAt      *time.Time         `json:"approved_at,omitempty"`
	FromTitle       string             `json:"from_title"`
	ToTitle         string             `json:"to_title"`
	FromFamily      string             `json:"from_family"`
	ToFamily        string             `json:"to_family"`
	FromCategory    string             `json:"from_category"`
	ToCategory      string             `json:"to_category"`
	UpdatedAt       time.Time          `json:"updated_at"`
}

// RegistryPage is one page of the registry listing.
type RegistryPage struct {
	Items  []RegistryRelation `json:"items"`
	Total  int                `json:"total"`
	Limit  int                `json:"limit"`
	Offset int                `json:"offset"`
}

// RegistryStats is the workbench summary.
// The brief calls this type RegistryMetrics, but Go cannot declare a type and a
// function with the same name in one package, and the function is what the HTTP
// layer consumes. The function keeps the specified name.
type RegistryStats struct {
	Total           int                 `json:"total"`
	Active          int                 `json:"active"`
	Candidates      int                 `json:"candidates"`
	Rejected        int                 `json:"rejected"`
	Stale           int                 `json:"stale"`
	Expiring30      int                 `json:"expiring_30"`
	ItemsWithActive int                 `json:"items_with_active"`
	ItemsTotal      int                 `json:"items_total"`
	CoveragePct     float64             `json:"coverage_pct"`
	HubTop1Pct      float64             `json:"hub_top1_pct"`
	StaleShare      float64             `json:"stale_share"`
	ByType          map[string]int      `json:"by_type"`
	ByProvenance    map[string]int      `json:"by_provenance"`
	Families        []string            `json:"families"`
	FamilyOptions   []GraphFilterOption `json:"family_options"`
}

const registryColumns = `r.id, r.from_item_id, r.to_item_id, r.type, r.status,
	r.provenance_kind, r.reason, r.confidence::float8, r.evidence,
	r.audience, r.industry, r.org_type, r.owner_id, r.version, r.position,
	r.valid_until, r.revalidate_after, r.rejected_until,
	r.rejection_reason, r.review_note, r.reviewed_by, r.approved_by, r.approved_at,
	COALESCE(f.title, r.from_item_id), COALESCE(t.title, r.to_item_id),
	COALESCE(f.product_group, ''), COALESCE(t.product_group, ''),
	COALESCE(f.category, ''), COALESCE(t.category, ''),
	r.updated_at`

// decodeEvidence parses the evidence jsonb column.
// New rows use {"kind","summary","details"}; legacy rows use {"source","value"}.
func decodeEvidence(raw []byte) ([]RelationEvidence, error) {
	out := []RelationEvidence{}
	if len(raw) == 0 {
		return out, nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("decode evidence: %w", err)
	}
	for _, item := range items {
		var plain string
		if err := json.Unmarshal(item, &plain); err == nil {
			out = append(out, RelationEvidence{Summary: plain})
			continue
		}
		var e struct {
			Kind    string         `json:"kind"`
			Summary string         `json:"summary"`
			Details map[string]any `json:"details"`
			Source  string         `json:"source"`
			Value   string         `json:"value"`
		}
		if err := json.Unmarshal(item, &e); err != nil {
			return nil, fmt.Errorf("decode evidence item: %w", err)
		}
		ev := RelationEvidence{Kind: e.Kind, Summary: e.Summary, Details: e.Details}
		if ev.Kind == "" {
			ev.Kind = e.Source
		}
		if ev.Summary == "" {
			ev.Summary = e.Value
		}
		out = append(out, ev)
	}
	return out, nil
}

// scanRegistryRelation reads one row of registryColumns plus count(*) OVER().
func scanRegistryRelation(rows *sql.Rows) (RegistryRelation, int, error) {
	var rel RegistryRelation
	var evidence []byte
	var total int
	if err := rows.Scan(&rel.ID, &rel.FromID, &rel.ToID, &rel.Type, &rel.Status,
		&rel.ProvenanceKind, &rel.Reason, &rel.Confidence, &evidence,
		&rel.Audience, &rel.Industry, &rel.OrgType, &rel.OwnerID, &rel.Version, &rel.Position,
		&rel.ValidUntil, &rel.RevalidateAfter, &rel.RejectedUntil,
		&rel.RejectionReason, &rel.ReviewNote, &rel.ReviewedBy, &rel.ApprovedBy, &rel.ApprovedAt,
		&rel.FromTitle, &rel.ToTitle,
		&rel.FromFamily, &rel.ToFamily,
		&rel.FromCategory, &rel.ToCategory,
		&rel.UpdatedAt, &total); err != nil {
		return RegistryRelation{}, 0, err
	}
	ev, err := decodeEvidence(evidence)
	if err != nil {
		return RegistryRelation{}, 0, err
	}
	rel.Evidence = ev
	return rel, total, nil
}

// buildRelationWhere builds the WHERE clause from parameters only.
func buildRelationWhere(f RelationFilter) (string, []any) {
	var conds []string
	var args []any

	add := func(v any) int {
		args = append(args, v)
		return len(args)
	}

	if len(f.IDs) > 0 {
		n := add(pq.Array(f.IDs))
		conds = append(conds, fmt.Sprintf(`r.id = ANY($%d)`, n))
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		n := add("%" + q + "%")
		conds = append(conds, fmt.Sprintf(
			`(f.title ILIKE $%d OR t.title ILIKE $%d OR r.from_item_id ILIKE $%d OR r.to_item_id ILIKE $%d)`,
			n, n, n, n))
	}
	if len(f.Types) > 0 {
		n := add(pq.Array(f.Types))
		conds = append(conds, fmt.Sprintf(`r.type = ANY($%d)`, n))
	}
	if len(f.Statuses) > 0 {
		n := add(pq.Array(f.Statuses))
		conds = append(conds, fmt.Sprintf(`r.status = ANY($%d)`, n))
	}
	if len(f.Provenance) > 0 {
		n := add(pq.Array(f.Provenance))
		conds = append(conds, fmt.Sprintf(`r.provenance_kind = ANY($%d)`, n))
	}
	if fam := strings.TrimSpace(f.Family); fam != "" {
		n := add(fam)
		conds = append(conds, fmt.Sprintf(`f.product_group = $%d`, n))
	}
	if aud := strings.TrimSpace(f.Audience); aud != "" {
		n := add(aud)
		conds = append(conds, fmt.Sprintf(`(r.audience = '' OR r.audience = $%d)`, n))
	}
	if org := strings.TrimSpace(f.OrgType); org != "" {
		n := add(org)
		conds = append(conds, fmt.Sprintf(`(r.org_type = '' OR r.org_type = $%d)`, n))
	}
	if owner := strings.TrimSpace(f.OwnerID); owner != "" {
		n := add(owner)
		conds = append(conds, fmt.Sprintf(`r.owner_id = $%d`, n))
	}
	if f.MinConfidence > 0 {
		n := add(f.MinConfidence)
		conds = append(conds, fmt.Sprintf(`r.confidence >= $%d`, n))
	}
	if f.MaxConfidence > 0 {
		n := add(f.MaxConfidence)
		conds = append(conds, fmt.Sprintf(`r.confidence <= $%d`, n))
	}
	if f.ExpiringDays > 0 {
		n := add(f.ExpiringDays)
		conds = append(conds, fmt.Sprintf(
			`r.valid_until IS NOT NULL AND r.valid_until <= now() + ($%d || ' days')::interval`, n))
	}
	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// orderBy maps a whitelisted sort key to SQL. Unknown keys fall back to the default.
func orderBy(sort string) string {
	switch sort {
	case "updated_desc":
		return "r.updated_at DESC, r.id DESC"
	case "expiry_asc":
		return "r.valid_until ASC NULLS LAST, r.id"
	case "confidence_asc":
		return "r.confidence ASC, r.id"
	case "confidence_desc":
		return "r.confidence DESC, r.id"
	default:
		return "r.confidence DESC, r.id"
	}
}

// ListRegistryRelations returns one page of the v5 relation registry.
func ListRegistryRelations(ctx context.Context, dbc *sql.DB, f RelationFilter) (RegistryPage, error) {
	where, args := buildRelationWhere(f)

	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}
	from := ` FROM relations r
		LEFT JOIN lk_items f ON f.item_id = r.from_item_id
		LEFT JOIN lk_items t ON t.item_id = r.to_item_id`
	whereArgs := args
	args = append(args, limit, offset)

	query := `SELECT ` + registryColumns + `, count(*) OVER() AS total` + from + where +
		` ORDER BY ` + orderBy(f.Sort) +
		fmt.Sprintf(` LIMIT $%d OFFSET $%d`, len(args)-1, len(args))

	rows, err := dbc.QueryContext(ctx, query, args...)
	if err != nil {
		return RegistryPage{}, fmt.Errorf("registry relations: %w", err)
	}
	defer rows.Close()

	page := RegistryPage{Items: []RegistryRelation{}, Limit: limit, Offset: offset}
	for rows.Next() {
		rel, total, err := scanRegistryRelation(rows)
		if err != nil {
			return RegistryPage{}, fmt.Errorf("scan registry relation: %w", err)
		}
		page.Items = append(page.Items, rel)
		page.Total = total
	}
	if err := rows.Err(); err != nil {
		return RegistryPage{}, err
	}
	// count(*) OVER() disappears with the rows when OFFSET runs past the end;
	// fall back to a plain count so pagination keeps the real total.
	if len(page.Items) == 0 {
		if err := dbc.QueryRowContext(ctx,
			`SELECT count(*)`+from+where, whereArgs...).Scan(&page.Total); err != nil {
			return RegistryPage{}, fmt.Errorf("registry relations count: %w", err)
		}
	}
	return page, nil
}

// RegistryMetrics summarises the v5 registry for the workbench header.
func RegistryMetrics(ctx context.Context, dbc *sql.DB) (RegistryStats, error) {
	var m RegistryStats

	if err := dbc.QueryRowContext(ctx, `
		SELECT count(*),
		       count(*) FILTER (WHERE status = 'active'),
		       count(*) FILTER (WHERE status = 'candidate'),
		       count(*) FILTER (WHERE status = 'rejected'),
		       count(*) FILTER (WHERE status = 'stale'),
		       count(*) FILTER (WHERE status = 'active'
		                          AND valid_until IS NOT NULL
		                          AND valid_until <= now() + interval '30 days')
		FROM relations`).Scan(&m.Total, &m.Active, &m.Candidates,
		&m.Rejected, &m.Stale, &m.Expiring30); err != nil {
		return m, fmt.Errorf("registry metrics: %w", err)
	}

	m.ByType = map[string]int{}
	if err := fillCounts(ctx, dbc,
		`SELECT type, count(*) FROM relations GROUP BY type`, m.ByType); err != nil {
		return m, err
	}

	m.ByProvenance = map[string]int{}
	if err := fillCounts(ctx, dbc,
		`SELECT provenance_kind, count(*) FROM relations
		 WHERE provenance_kind <> '' GROUP BY provenance_kind`, m.ByProvenance); err != nil {
		return m, err
	}

	if err := dbc.QueryRowContext(ctx, `SELECT count(*) FROM lk_items`).
		Scan(&m.ItemsTotal); err != nil {
		return m, fmt.Errorf("registry items total: %w", err)
	}

	if err := dbc.QueryRowContext(ctx, `
		SELECT count(*) FROM (
			SELECT from_item_id AS item_id FROM relations WHERE status = 'active'
			UNION
			SELECT to_item_id AS item_id FROM relations WHERE status = 'active'
		) x`).Scan(&m.ItemsWithActive); err != nil {
		return m, fmt.Errorf("registry items with active: %w", err)
	}

	if err := dbc.QueryRowContext(ctx, `
		WITH deg AS (
			SELECT to_item_id, count(*) AS d
			FROM relations WHERE status = 'active' AND type <> 'variant_of'
			GROUP BY to_item_id
		), ranked AS (
			SELECT to_item_id,
			       row_number() OVER (ORDER BY d DESC, to_item_id) AS rn,
			       count(*) OVER () AS n
			FROM deg
		), top AS (
			SELECT to_item_id FROM ranked
			WHERE rn <= GREATEST(1, ceil(n * 0.01)::int)
		)
		SELECT COALESCE(
		         100.0 * count(*) FILTER (WHERE r.to_item_id IN (SELECT to_item_id FROM top))
		         / NULLIF(count(*), 0), 0)
		FROM relations r
		WHERE r.status = 'active' AND r.type <> 'variant_of'`).Scan(&m.HubTop1Pct); err != nil {
		return m, fmt.Errorf("registry hub share: %w", err)
	}

	rows, err := dbc.QueryContext(ctx, `
		SELECT product_group, title, category FROM lk_items
		WHERE product_group <> '' ORDER BY product_group`)
	if err != nil {
		return m, fmt.Errorf("registry families: %w", err)
	}
	defer rows.Close()
	type familyInfo struct {
		labels map[string]int
		count  int
	}
	groups := map[string]*familyInfo{}
	var groupOrder []string
	for rows.Next() {
		var group, title, category string
		if err := rows.Scan(&group, &title, &category); err != nil {
			return m, err
		}
		info := groups[group]
		if info == nil {
			info = &familyInfo{labels: map[string]int{}}
			groups[group] = info
			groupOrder = append(groupOrder, group)
		}
		label := familyForTitle(title, category)
		if label == "" {
			label = group
		}
		info.labels[label]++
		info.count++
	}
	if err := rows.Err(); err != nil {
		return m, err
	}
	m.Families = groupOrder
	if len(m.Families) > 100 {
		m.Families = m.Families[:100]
	}
	m.FamilyOptions = make([]GraphFilterOption, 0, len(groups))
	for group, info := range groups {
		label, best := group, 0
		for candidate, n := range info.labels {
			if n > best || (n == best && candidate < label) {
				label, best = candidate, n
			}
		}
		m.FamilyOptions = append(m.FamilyOptions, GraphFilterOption{
			Value: group, Label: label, Count: info.count,
		})
	}
	sort.Slice(m.FamilyOptions, func(i, j int) bool {
		if m.FamilyOptions[i].Count != m.FamilyOptions[j].Count {
			return m.FamilyOptions[i].Count > m.FamilyOptions[j].Count
		}
		return m.FamilyOptions[i].Label < m.FamilyOptions[j].Label
	})
	if len(m.FamilyOptions) > 100 {
		m.FamilyOptions = m.FamilyOptions[:100]
	}

	if m.ItemsTotal > 0 {
		m.CoveragePct = 100 * float64(m.ItemsWithActive) / float64(m.ItemsTotal)
	}
	if denom := m.Active + m.Stale; denom > 0 {
		m.StaleShare = 100 * float64(m.Stale) / float64(denom)
	}
	return m, nil
}

// fillCounts runs a (key, count) query into dst.
func fillCounts(ctx context.Context, dbc *sql.DB, query string, dst map[string]int) error {
	rows, err := dbc.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("registry counts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var n int
		if err := rows.Scan(&key, &n); err != nil {
			return err
		}
		dst[key] = n
	}
	return rows.Err()
}
