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
	ClientID    string
	Limit       int
	ForceItemID string
	ForceType   string
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
	sort.SliceStable(res.Items, func(i, j int) bool { return res.Items[i].Score > res.Items[j].Score })

	if opts.ForceItemID != "" && !ownedSet[opts.ForceItemID] {
		if forced, ok := forceCandidate(ctx, dbc, opts.ForceItemID, opts.ForceType); ok {
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

	if len(res.Items) < opts.Limit {
		popular, err := popularItems(ctx, dbc, audience, industry, owned, opts.Limit*3)
		if err != nil {
			return nil, err
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
		       COALESCE(i.system_requirements::text, '{}')
		FROM relations r
		JOIN lk_items i ON i.item_id = r.to_item_id AND i.available
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
		if err := rows.Scan(&it.SourceItemID, &it.ItemID, &it.RelationType, &confidence, &it.Title, &requirements); err != nil {
			return nil, err
		}
		it.Compatibility = compatibility(requirements)
		it.Type = "cross_sell"
		if it.RelationType == "upgrade" {
			it.Type = "upsell"
		}
		it.Score = confidence * relationWeight(it.RelationType)
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
