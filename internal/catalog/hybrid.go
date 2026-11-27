package catalog

import (
	"context"
	"database/sql"
	"math"
	"sort"
	"strings"

	"github.com/lib/pq"
)

// ServeHybrid is the serving version of the offline model: co-purchase lift,
// popularity and title overlap, with confirmed relations and FSTEC as boosts.
func ServeHybrid(ctx context.Context, dbc *sql.DB, opts ServeOptions) (*ServeResult, error) {
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
		return nil, sql.ErrNoRows
	}
	res := &ServeResult{ClientID: opts.ClientID, Audience: audience, Industry: industry}

	owned, err := ownedItems(ctx, dbc, opts.ClientID)
	if err != nil {
		return nil, err
	}
	res.ColdStart = len(owned) == 0
	ownedSet := map[string]bool{}
	for _, id := range owned {
		ownedSet[id] = true
	}

	items, err := allServeItems(ctx, dbc)
	if err != nil {
		return nil, err
	}
	popularity, maxPop, err := popularityByAudience(ctx, dbc, audience)
	if err != nil {
		return nil, err
	}
	popList := sortedPopularity(popularity)

	if pre, err := precomputedForClient(ctx, dbc, opts.ClientID, ownedSet, items, opts); err == nil && len(pre) > 0 {
		res.Items = applyForced(ctx, dbc, pre, opts, ownedSet)
		return res, nil
	}

	scores := map[string]float64{}
	sources := map[string]string{}
	types := map[string]string{}
	reasons := map[string]string{}

	if audience == "b2c" || len(owned) == 0 {
		for _, id := range popList {
			if ownedSet[id] || !items[id].available {
				continue
			}
			scores[id] = 0.35
			sources[id] = "popular"
			types[id] = "popular"
			reasons[id] = "Популярно у клиентов вашего профиля"
			if len(scores) >= opts.Limit*4 {
				break
			}
		}
	} else {
		// Co-purchase from the shared pair table.
		pairs, err := coPurchaseForItems(ctx, dbc, owned)
		if err != nil {
			return nil, err
		}
		for _, p := range pairs {
			neighbor := p.other
			if ownedSet[neighbor] || !items[neighbor].available {
				continue
			}
			count := p.count
			if audience == "b2b" && p.b2b > 0 {
				count = p.b2b
			} else if audience == "b2c" && p.b2c > 0 {
				count = p.b2c
			}
			base := math.Max(1, float64(popularity[p.from])) * math.Max(1, float64(popularity[neighbor]))
			scores[neighbor] += 3.0 * float64(count) / math.Sqrt(base)
			sources[neighbor] = "co_purchase"
			types[neighbor] = "co_purchase"
			reasons[neighbor] = "Связь с купленным товаром"
		}

		// Popularity component.
		for id, count := range popularity {
			if ownedSet[id] || !items[id].available {
				continue
			}
			scores[id] += 0.5 * float64(count) / float64(maxPop)
			if _, ok := sources[id]; !ok {
				sources[id] = "popular"
				types[id] = "popular"
				reasons[id] = "Популярно у клиентов вашего профиля"
			}
		}

		// Title overlap: the offline model sums overlap per owned item.
		tokenCache := map[string]map[string]bool{}
		tokensOf := func(id string) map[string]bool {
			if ts, ok := tokenCache[id]; ok {
				return ts
			}
			ts := map[string]bool{}
			for _, t := range titleTokens(items[id].title) {
				ts[t] = true
			}
			tokenCache[id] = ts
			return ts
		}
		for _, ownedID := range owned {
			at := tokensOf(ownedID)
			checked := 0
			for _, id := range popList {
				if checked >= 500 {
					break
				}
				checked++
				if ownedSet[id] || !items[id].available {
					continue
				}
				overlap := 0
				for t := range at {
					if tokensOf(id)[t] {
						overlap++
					}
				}
				if overlap > 0 {
					scores[id] += 0.2 * float64(overlap)
					if sources[id] == "popular" {
						sources[id] = "content"
						types[id] = "content"
						reasons[id] = "Похоже на то, что вы уже покупали"
					}
				}
			}
		}

		// Confirmed relations add a small, explainable component.
		rels, err := activeRelationsForItems(ctx, dbc, owned, audience, industry)
		if err != nil {
			return nil, err
		}
		for _, r := range rels {
			if ownedSet[r.toID] || !items[r.toID].available {
				continue
			}
			scores[r.toID] += 0.5 * r.confidence * relationWeight(r.rtype)
			sources[r.toID] = "relation"
			types[r.toID] = r.rtype
			reasons[r.toID] = r.reason
		}
	}

	// Normalise to 0..1 so certificate boosts are comparable.
	maxScore := 0.0
	for _, score := range scores {
		if score > maxScore {
			maxScore = score
		}
	}
	ranked := make([]ServeItem, 0, len(scores))
	for id, score := range scores {
		item := items[id]
		ranked = append(ranked, ServeItem{
			ItemID: id, Title: item.title, Type: types[id], Reason: reasons[id],
			Score: score / math.Max(maxScore, 0.0001), SourceItemID: "",
			RelationType: types[id],
		})
	}
	ranked = annotateCertificates(ctx, dbc, ranked)
	if opts.CertifiedOnly {
		ranked = filterCertified(ranked)
	}
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].Score > ranked[j].Score })
	if len(ranked) > opts.Limit {
		ranked = ranked[:opts.Limit]
	}

	ranked = applyForced(ctx, dbc, ranked, opts, ownedSet)
	res.Items = ranked
	return res, nil
}

func precomputedForClient(ctx context.Context, dbc *sql.DB, clientID string, owned map[string]bool, items map[string]serveItemMeta, opts ServeOptions) ([]ServeItem, error) {
	rows, err := dbc.QueryContext(ctx, `
		SELECT item_id, rank FROM client_recommendations
		WHERE client_id = $1 ORDER BY rank LIMIT $2`, clientID, opts.Limit*3)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ServeItem
	for rows.Next() {
		var id string
		var rank int
		if err := rows.Scan(&id, &rank); err != nil {
			return nil, err
		}
		meta, ok := items[id]
		if !ok || !meta.available || owned[id] {
			continue
		}
		out = append(out, ServeItem{
			ItemID: id, Title: meta.title, Type: "personal",
			Reason: "Подобрано по вашей истории",
			Score:  1.0 / float64(rank+1),
		})
	}
	out = annotateCertificates(ctx, dbc, out)
	if opts.CertifiedOnly {
		out = filterCertified(out)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if len(out) > opts.Limit {
		out = out[:opts.Limit]
	}
	return out, rows.Err()
}

func applyForced(ctx context.Context, dbc *sql.DB, ranked []ServeItem, opts ServeOptions, owned map[string]bool) []ServeItem {
	if opts.ForceItemID == "" || owned[opts.ForceItemID] {
		return ranked
	}
	forced, ok := forceCandidate(ctx, dbc, opts.ForceItemID, opts.ForceType)
	if !ok {
		return ranked
	}
	forced = annotateCertificates(ctx, dbc, []ServeItem{forced})[0]
	if opts.CertifiedOnly && !forced.Certified {
		return ranked
	}
	pos := 2
	if pos > len(ranked) {
		pos = len(ranked)
	}
	ranked = append(ranked, ServeItem{})
	copy(ranked[pos+1:], ranked[pos:])
	ranked[pos] = forced
	if len(ranked) > opts.Limit {
		ranked = ranked[:opts.Limit]
	}
	return ranked
}

type serveItemMeta struct {
	title     string
	available bool
}

func allServeItems(ctx context.Context, dbc *sql.DB) (map[string]serveItemMeta, error) {
	rows, err := dbc.QueryContext(ctx, `SELECT item_id, title, available FROM lk_items`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]serveItemMeta{}
	for rows.Next() {
		var id, title string
		var available bool
		if err := rows.Scan(&id, &title, &available); err != nil {
			return nil, err
		}
		out[id] = serveItemMeta{title: title, available: available}
	}
	return out, rows.Err()
}

func popularityByAudience(ctx context.Context, dbc *sql.DB, audience string) (map[string]int, int, error) {
	query := `
		SELECT o.item_id, count(DISTINCT o.client_id)
		FROM lk_orders o
		JOIN lk_clients c ON c.client_id = o.client_id
		WHERE (($1 = 'b2b' AND c.client_type IN ('ЮЛ', 'ИП'))
		    OR ($1 = 'b2c' AND c.client_type = 'ФЛ'))
		GROUP BY o.item_id`
	rows, err := dbc.QueryContext(ctx, query, audience)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := map[string]int{}
	max := 1
	for rows.Next() {
		var id string
		var count int
		if err := rows.Scan(&id, &count); err != nil {
			return nil, 0, err
		}
		out[id] = count
		if count > max {
			max = count
		}
	}
	return out, max, rows.Err()
}

func sortedPopularity(pop map[string]int) []string {
	type pair struct {
		id    string
		count int
	}
	list := make([]pair, 0, len(pop))
	for id, count := range pop {
		list = append(list, pair{id, count})
	}
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].count == list[j].count {
			return list[i].id < list[j].id
		}
		return list[i].count > list[j].count
	})
	out := make([]string, 0, len(list))
	for _, p := range list {
		out = append(out, p.id)
	}
	return out
}

type coPair struct {
	from, other string
	count       int
	b2b, b2c    int
}

func coPurchaseForItems(ctx context.Context, dbc *sql.DB, owned []string) ([]coPair, error) {
	rows, err := dbc.QueryContext(ctx, `
		SELECT item_a, item_b, count, b2b_count, b2c_count FROM co_purchase_pairs
		WHERE item_a = ANY($1)
		UNION
		SELECT item_b, item_a, count, b2b_count, b2c_count FROM co_purchase_pairs
		WHERE item_b = ANY($1)`, pq.Array(owned))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []coPair
	for rows.Next() {
		var p coPair
		if err := rows.Scan(&p.from, &p.other, &p.count, &p.b2b, &p.b2c); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

type activeRel struct {
	toID       string
	rtype      string
	confidence float64
	reason     string
}

func activeRelationsForItems(ctx context.Context, dbc *sql.DB, owned []string, audience, industry string) ([]activeRel, error) {
	rows, err := dbc.QueryContext(ctx, `
		SELECT to_item_id, type, confidence::float8, reason FROM relations
		WHERE from_item_id = ANY($1)
		  AND status IN ('active', 'confirmed')
		  AND type <> 'variant_of'
		  AND (audience = '' OR audience = $2)
		  AND (industry = '' OR industry = $3)`, pq.Array(owned), audience, industry)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []activeRel
	for rows.Next() {
		var r activeRel
		if err := rows.Scan(&r.toID, &r.rtype, &r.confidence, &r.reason); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

var titleReplacer = strings.NewReplacer(
	"ё", "е", ",", " ", ".", " ", "-", " ", "(", " ", ")", " ",
)

func titleTokens(s string) []string {
	return strings.Fields(titleReplacer.Replace(strings.ToLower(s)))
}
