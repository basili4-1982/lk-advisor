package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/lib/pq"
)

// ClientAudience resolves the real catalog client and its audience/industry.
func ClientAudience(ctx context.Context, dbc *sql.DB, clientID string) (audience, industry string, ok bool, err error) {
	var clientType string
	err = dbc.QueryRowContext(ctx, `
		SELECT client_type, COALESCE(industry, '')
		FROM lk_clients WHERE client_id = $1`, clientID).Scan(&clientType, &industry)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, fmt.Errorf("catalog client: %w", err)
	}
	return AudienceForClientType(clientType), industry, true, nil
}

// AudienceForClientType maps the dataset client type to a recommendation scope.
func AudienceForClientType(clientType string) string {
	switch strings.ToUpper(strings.TrimSpace(clientType)) {
	case "ФЛ":
		return "b2c"
	case "ЮЛ", "ИП":
		return "b2b"
	}
	return "b2b"
}

// ServeOptions configures one catalog recommendation request.
type ServeOptions struct {
	ClientID      string
	Limit         int
	ForceItemID   string
	ForceType     string
	CertifiedOnly bool
}

// ServeItem is one recommendation from the catalog graph.
type ServeItem struct {
	ItemID        string
	Title         string
	Type          string
	Reason        string
	Score         float64
	SourceItemID  string
	RelationType  string
	Compatibility string
	Certified     bool
	CertExpired   bool
	Forced        bool
}

// ServeResult is the ranked block for one client.
type ServeResult struct {
	ClientID  string
	Audience  string
	Industry  string
	ColdStart bool
	Items     []ServeItem
}

// Serve recommends catalog items from confirmed relations and purchase history.
// It is the serving counterpart of the admin workbench: the same relations that
// are reviewed there are shown to real clients.
func Serve(ctx context.Context, dbc *sql.DB, opts ServeOptions) (*ServeResult, error) {
	if opts.Limit <= 0 {
		opts.Limit = 3
	}
	if opts.Limit > 36 {
		opts.Limit = 36
	}
	audience, industry, ok, err := ClientAudience(ctx, dbc, opts.ClientID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("клиент не найден в каталоге")
	}
	res := &ServeResult{ClientID: opts.ClientID, Audience: audience, Industry: industry}

	owned, err := ownedItems(ctx, dbc, opts.ClientID)
	if err != nil {
		return nil, err
	}
	res.ColdStart = len(owned) == 0
	ownedSet := make(map[string]bool, len(owned))
	for _, id := range owned {
		ownedSet[id] = true
	}

	if len(owned) > 0 {
		items, err := relationCandidates(ctx, dbc, owned, audience, industry)
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			if ownedSet[it.ItemID] {
				continue
			}
			res.Items = append(res.Items, it)
		}
	}

	res.Items = dedupe(res.Items)
	res.Items, err = annotateCertificates(ctx, dbc, res.Items)
	if err != nil {
		return nil, err
	}
	if opts.CertifiedOnly {
		res.Items = filterCertified(res.Items)
	}
	sort.SliceStable(res.Items, func(i, j int) bool {
		if res.Items[i].Score == res.Items[j].Score {
			return res.Items[i].ItemID < res.Items[j].ItemID
		}
		return res.Items[i].Score > res.Items[j].Score
	})

	if opts.ForceItemID != "" && !ownedSet[opts.ForceItemID] {
		if forced, ok := forceCandidate(ctx, dbc, opts.ForceItemID, opts.ForceType); ok {
			forcedItems, err := annotateCertificates(ctx, dbc, []ServeItem{forced})
			if err != nil {
				return nil, err
			}
			forced = forcedItems[0]
			if !opts.CertifiedOnly || forced.Certified {
				res.Items = removeItem(res.Items, forced.ItemID)
				pos := 2
				if pos > len(res.Items) {
					pos = len(res.Items)
				}
				res.Items = append(res.Items, ServeItem{})
				copy(res.Items[pos+1:], res.Items[pos:])
				res.Items[pos] = forced
			}
		}
	}

	if len(res.Items) < opts.Limit {
		popular, err := popularItems(ctx, dbc, audience, industry, owned, opts.Limit*3)
		if err != nil {
			return nil, err
		}
		popular, err = annotateCertificates(ctx, dbc, popular)
		if err != nil {
			return nil, err
		}
		if opts.CertifiedOnly {
			popular = filterCertified(popular)
		}
		chosen := make(map[string]bool, len(res.Items))
		for _, it := range res.Items {
			chosen[it.ItemID] = true
		}
		for _, it := range popular {
			if len(res.Items) >= opts.Limit*2 {
				break
			}
			if chosen[it.ItemID] {
				continue
			}
			chosen[it.ItemID] = true
			res.Items = append(res.Items, it)
		}
	}
	if len(res.Items) > opts.Limit {
		res.Items = res.Items[:opts.Limit]
	}
	return res, nil
}

func ownedItems(ctx context.Context, dbc *sql.DB, clientID string) ([]string, error) {
	rows, err := dbc.QueryContext(ctx, `
		SELECT DISTINCT item_id FROM lk_orders WHERE client_id = $1`, clientID)
	if err != nil {
		return nil, fmt.Errorf("owned items: %w", err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func relationCandidates(ctx context.Context, dbc *sql.DB, owned []string, audience, industry string) ([]ServeItem, error) {
	rows, err := dbc.QueryContext(ctx, `
		SELECT r.from_item_id, r.to_item_id, r.type, r.confidence::float8, i.title,
		       COALESCE(i.system_requirements::text, '{}'),
		       COALESCE(p.shown, 0), COALESCE(p.clicks, 0), COALESCE(p.purchases, 0)
		FROM relations r
		JOIN lk_items i ON i.item_id = r.to_item_id AND i.available
		LEFT JOIN (
			SELECT source_product_id, product_id, relation_type,
			       count(*) FILTER (WHERE event = 'shown') AS shown,
			       count(*) FILTER (WHERE event = 'clicked') AS clicks,
			       count(*) FILTER (WHERE event = 'purchased') AS purchases
			FROM recommendation_events
			WHERE candidate_id <> ''
			GROUP BY source_product_id, product_id, relation_type
		) p ON p.source_product_id = r.from_item_id
		   AND p.product_id = r.to_item_id
		   AND p.relation_type = r.type
		WHERE r.from_item_id = ANY($1)
		  AND r.status IN ('active', 'confirmed')
		  AND r.type <> 'variant_of'
		  AND (r.audience = '' OR r.audience = $2)
		  AND (r.industry = '' OR r.industry = $3)`,
		pq.Array(owned), audience, industry)
	if err != nil {
		return nil, fmt.Errorf("catalog candidates: %w", err)
	}
	defer rows.Close()
	var out []ServeItem
	for rows.Next() {
		var it ServeItem
		var confidence float64
		var requirements string
		var shown, clicks, purchases int
		if err := rows.Scan(&it.SourceItemID, &it.ItemID, &it.RelationType, &confidence, &it.Title, &requirements,
			&shown, &clicks, &purchases); err != nil {
			return nil, err
		}
		it.Compatibility = compatibility(requirements)
		it.Type = "cross_sell"
		if it.RelationType == "upgrade" {
			it.Type = "upsell"
		}
		weight := RelationPerformanceMultiplier(shown, clicks, purchases)
		it.Score = clampNum(confidence*relationWeight(it.RelationType)*weight, 0, 1)
		it.Reason = fmt.Sprintf("Связь «%s» с купленным товаром, уверенность %.0f%%",
			typeLabel(it.RelationType), confidence*100)
		out = append(out, it)
	}
	return out, rows.Err()
}

func forceCandidate(ctx context.Context, dbc *sql.DB, itemID, relationType string) (ServeItem, bool) {
	var title, requirements string
	err := dbc.QueryRowContext(ctx, `
		SELECT title, COALESCE(system_requirements::text, '{}')
		FROM lk_items WHERE item_id = $1 AND available`, itemID).Scan(&title, &requirements)
	if err != nil {
		return ServeItem{}, false
	}
	typ := "cross_sell"
	if relationType == "upgrade" {
		typ = "upsell"
	}
	return ServeItem{
		ItemID: itemID, Title: title, Type: typ,
		Reason:        "Гипотеза: показ в эксперименте",
		Score:         0.99,
		RelationType:  relationType,
		Compatibility: compatibility(requirements),
		Forced:        true,
	}, true
}

func popularItems(ctx context.Context, dbc *sql.DB, audience, industry string, owned []string, limit int) ([]ServeItem, error) {
	if limit <= 0 {
		limit = 9
	}
	rows, err := dbc.QueryContext(ctx, `
		SELECT i.item_id, i.title, COALESCE(i.system_requirements::text, '{}'), count(*) AS purchases
		FROM lk_orders o
		JOIN lk_items i ON i.item_id = o.item_id AND i.available
		JOIN lk_clients c ON c.client_id = o.client_id
		WHERE ($1 = '' OR c.industry = $1)
		  AND (($2 = 'b2b' AND c.client_type IN ('ЮЛ', 'ИП'))
		       OR ($2 = 'b2c' AND c.client_type = 'ФЛ'))
		  AND o.item_id <> ALL($3)
		GROUP BY i.item_id, i.title
		ORDER BY purchases DESC, i.item_id
		LIMIT $4`, industry, audience, pq.Array(owned), limit)
	if err != nil {
		return nil, fmt.Errorf("popular items: %w", err)
	}
	defer rows.Close()
	var out []ServeItem
	for rows.Next() {
		var it ServeItem
		var purchases int
		var requirements string
		if err := rows.Scan(&it.ItemID, &it.Title, &requirements, &purchases); err != nil {
			return nil, err
		}
		it.Compatibility = compatibility(requirements)
		it.Type = "popular"
		it.Score = 0.35
		it.Reason = "Популярно у клиентов вашего профиля"
		out = append(out, it)
	}
	return out, rows.Err()
}

// annotateCertificates marks FSTEC status and uses it as a ranking signal:
// a valid certificate adds a bonus, an expired-only product gets a small
// penalty. The flag is stored on the item for the client card.
func annotateCertificates(ctx context.Context, dbc *sql.DB, items []ServeItem) ([]ServeItem, error) {
	if len(items) == 0 {
		return items, nil
	}
	ids := make([]string, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.ItemID)
	}
	valid := map[string]bool{}
	expired := map[string]bool{}
	rows, err := dbc.QueryContext(ctx, `
		SELECT m.item_id,
		       bool_or(c.valid_until >= current_date) AS has_valid,
		       bool_or(c.valid_until < current_date) AS has_expired
		FROM fstec_certificate_items m
		JOIN fstec_certificates c ON c.id = m.certificate_id
		WHERE m.item_id = ANY($1)
		GROUP BY m.item_id`, pq.Array(ids))
	if err != nil {
		return nil, fmt.Errorf("fstec status: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var v, e sql.NullBool
		if err := rows.Scan(&id, &v, &e); err != nil {
			return nil, fmt.Errorf("fstec status scan: %w", err)
		}
		valid[id] = v.Valid && v.Bool
		expired[id] = e.Valid && e.Bool
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fstec status rows: %w", err)
	}
	for i := range items {
		items[i].Certified = valid[items[i].ItemID]
		items[i].CertExpired = expired[items[i].ItemID] && !items[i].Certified
		// A certificate is a guardrail and a trust badge, not a general
		// ranking boost: the offline test showed +0.06 slightly lowers P@3.
		// The certified-only mode filters by it explicitly.
	}
	return items, nil
}

func clampNum(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func filterCertified(items []ServeItem) []ServeItem {
	out := items[:0]
	for _, it := range items {
		if it.Certified {
			out = append(out, it)
		}
	}
	return out
}

func compatibility(requirements string) string {
	req := strings.TrimSpace(requirements)
	if req == "" || req == "{}" || req == "null" {
		return "unverified"
	}
	return "ok"
}

func relationWeight(relType string) float64 {
	switch relType {
	case "upgrade":
		return 1.20
	case "accessory":
		return 1.10
	case "bundle":
		return 1.05
	case "adjacent_task":
		return 1.00
	case "workflow_next_step":
		return 0.95
	case "same_task":
		return 0.90
	case "substitute", "analog":
		return 0.85
	}
	return 0.90
}

func typeLabel(relType string) string {
	switch relType {
	case "upgrade":
		return "Апгрейд"
	case "accessory":
		return "Аксессуар"
	case "bundle":
		return "Комплект"
	case "adjacent_task":
		return "Дополнение"
	case "workflow_next_step":
		return "Следующий шаг"
	case "same_task":
		return "Та же задача"
	case "substitute":
		return "Замена"
	case "analog":
		return "Аналог"
	}
	return relType
}

func dedupe(items []ServeItem) []ServeItem {
	best := map[string]ServeItem{}
	for _, it := range items {
		if cur, ok := best[it.ItemID]; !ok || it.Score > cur.Score {
			best[it.ItemID] = it
		}
	}
	out := make([]ServeItem, 0, len(best))
	for _, it := range best {
		out = append(out, it)
	}
	return out
}

func removeItem(items []ServeItem, itemID string) []ServeItem {
	out := items[:0]
	for _, it := range items {
		if it.ItemID != itemID {
			out = append(out, it)
		}
	}
	return out
}
