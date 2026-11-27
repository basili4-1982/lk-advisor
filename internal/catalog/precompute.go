package catalog

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lib/pq"
)

// PrecomputeResult reports one full recommendation rebuild.
type PrecomputeResult struct {
	Clients int           `json:"clients"`
	Rows    int           `json:"rows"`
	Took    time.Duration `json:"-"`
}

type preItem struct {
	title     string
	available bool
	tokens    map[string]bool
}

var precomputeMu sync.Mutex

// RebuildClientRecommendations recalculates offline-model top-N for every
// catalog client and replaces client_recommendations in one transaction.
func RebuildClientRecommendations(ctx context.Context, dbc *sql.DB, topN int) (PrecomputeResult, error) {
	precomputeMu.Lock()
	defer precomputeMu.Unlock()
	start := time.Now()
	var res PrecomputeResult
	if topN <= 0 || topN > 36 {
		topN = 10
	}

	items := map[string]preItem{}
	rows, err := dbc.QueryContext(ctx, `SELECT item_id, title, available FROM lk_items`)
	if err != nil {
		return res, fmt.Errorf("precompute items: %w", err)
	}
	for rows.Next() {
		var id, title string
		var available bool
		if err := rows.Scan(&id, &title, &available); err != nil {
			rows.Close()
			return res, err
		}
		items[id] = preItem{title: title, available: available, tokens: titleTokenSet(title)}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return res, err
	}

	audience := map[string]string{}
	rows, err = dbc.QueryContext(ctx, `SELECT client_id, client_type FROM lk_clients`)
	if err != nil {
		return res, fmt.Errorf("precompute clients: %w", err)
	}
	for rows.Next() {
		var id, ctype string
		if err := rows.Scan(&id, &ctype); err != nil {
			rows.Close()
			return res, err
		}
		if strings.ToUpper(strings.TrimSpace(ctype)) == "ФЛ" {
			audience[id] = "b2c"
		} else {
			audience[id] = "b2b"
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return res, err
	}

	owned := map[string]map[string]bool{}
	baskets := map[string]map[string]bool{}
	popSets := map[string]map[string]map[string]bool{
		"b2b": {}, "b2c": {},
	}
	rows, err = dbc.QueryContext(ctx, `
		SELECT client_id, order_id, order_week::text, item_id FROM lk_orders`)
	if err != nil {
		return res, fmt.Errorf("precompute orders: %w", err)
	}
	for rows.Next() {
		var cid, oid, week, itemID string
		if err := rows.Scan(&cid, &oid, &week, &itemID); err != nil {
			rows.Close()
			return res, err
		}
		if owned[cid] == nil {
			owned[cid] = map[string]bool{}
		}
		owned[cid][itemID] = true
		key := oid + "|" + week
		if baskets[key] == nil {
			baskets[key] = map[string]bool{}
		}
		baskets[key][itemID] = true
		aud := audience[cid]
		if aud == "" {
			aud = "b2b"
		}
		if popSets[aud][itemID] == nil {
			popSets[aud][itemID] = map[string]bool{}
		}
		popSets[aud][itemID][cid] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return res, err
	}

	popularity := map[string]map[string]int{"b2b": {}, "b2c": {}}
	maxPop := map[string]int{"b2b": 1, "b2c": 1}
	popList := map[string][]string{}
	for aud, byItem := range popSets {
		for item, clients := range byItem {
			popularity[aud][item] = len(clients)
			if len(clients) > maxPop[aud] {
				maxPop[aud] = len(clients)
			}
		}
		popList[aud] = topPopularity(popularity[aud])
	}

	itemPop := map[string]int{}
	neighbourCounts := map[string]map[string]int{}
	for _, basket := range baskets {
		for item := range basket {
			itemPop[item]++
		}
		for a := range basket {
			if neighbourCounts[a] == nil {
				neighbourCounts[a] = map[string]int{}
			}
			for b := range basket {
				if a != b {
					neighbourCounts[a][b]++
				}
			}
		}
	}
	neighbours := map[string][]neighbour{}
	for a, counts := range neighbourCounts {
		list := make([]neighbour, 0, len(counts))
		for b, count := range counts {
			list = append(list, neighbour{item: b, count: count})
		}
		sort.Slice(list, func(i, j int) bool {
			if list[i].count == list[j].count {
				return list[i].item < list[j].item
			}
			return list[i].count > list[j].count
		})
		neighbours[a] = list
	}

	// Cache token intersections for the top-500 popular items.
	topTokens := map[string][]string{}
	for aud := range popList {
		list := popList[aud]
		if len(list) > 500 {
			list = list[:500]
		}
		for _, item := range list {
			topTokens[item] = keys(items[item].tokens)
		}
	}

	tx, err := dbc.BeginTx(ctx, nil)
	if err != nil {
		return res, fmt.Errorf("precompute tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `TRUNCATE client_recommendations`); err != nil {
		return res, fmt.Errorf("precompute truncate: %w", err)
	}
	stmt, err := tx.PrepareContext(ctx, pq.CopyIn("client_recommendations",
		"client_id", "item_id", "rank", "score", "source", "reason"))
	if err != nil {
		return res, fmt.Errorf("precompute copy: %w", err)
	}
	defer stmt.Close()

	for cid, aud := range audience {
		ownedSet := owned[cid]
		recs := recommendForPrecompute(aud, ownedSet, items, popularity[aud], maxPop[aud],
			popList[aud], neighbours, itemPop, topTokens, topN)
		for rank, item := range recs {
			if _, err := stmt.ExecContext(ctx, cid, item, rank+1, 1.0/float64(rank+1),
				"offline_model", "Подобрано по вашей истории"); err != nil {
				return res, fmt.Errorf("precompute insert: %w", err)
			}
			res.Rows++
		}
		res.Clients++
	}
	if _, err := stmt.ExecContext(ctx); err != nil {
		return res, fmt.Errorf("precompute flush: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return res, fmt.Errorf("precompute commit: %w", err)
	}
	res.Took = time.Since(start)
	return res, nil
}

type neighbour struct {
	item  string
	count int
}

func recommendForPrecompute(aud string, owned map[string]bool, items map[string]preItem,
	pop map[string]int, maxPop int, popList []string,
	neighbours map[string][]neighbour, itemPop map[string]int,
	topTokens map[string][]string, limit int) []string {
	if aud == "b2c" || len(owned) == 0 {
		out := make([]string, 0, limit)
		for _, item := range popList {
			if owned[item] || !items[item].available {
				continue
			}
			out = append(out, item)
			if len(out) >= limit {
				break
			}
		}
		return out
	}

	scores := map[string]float64{}
	for a := range owned {
		for _, nb := range neighbours[a] {
			b := nb.item
			if owned[b] || !items[b].available {
				continue
			}
			base := math.Max(1, float64(itemPop[a])) * math.Max(1, float64(itemPop[b]))
			scores[b] += 3.0 * float64(nb.count) / math.Sqrt(base)
		}
	}
	for b, count := range pop {
		if owned[b] || !items[b].available {
			continue
		}
		scores[b] += 0.5 * float64(count) / float64(maxPop)
	}
	top := popList
	if len(top) > 500 {
		top = top[:500]
	}
	for a := range owned {
		at := items[a].tokens
		for _, b := range top {
			if owned[b] || !items[b].available {
				continue
			}
			overlap := 0
			for _, token := range topTokens[b] {
				if at[token] {
					overlap++
				}
			}
			if overlap > 0 {
				scores[b] += 0.2 * float64(overlap)
			}
		}
	}

	type pair struct {
		item  string
		score float64
	}
	list := make([]pair, 0, len(scores))
	for item, score := range scores {
		list = append(list, pair{item, score})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].score == list[j].score {
			return list[i].item < list[j].item
		}
		return list[i].score > list[j].score
	})
	out := make([]string, 0, limit)
	seen := map[string]bool{}
	for _, p := range list {
		if seen[p.item] {
			continue
		}
		seen[p.item] = true
		out = append(out, p.item)
		if len(out) >= limit {
			return out
		}
	}
	for _, item := range popList {
		if len(out) >= limit {
			break
		}
		if owned[item] || seen[item] || !items[item].available {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

func topPopularity(pop map[string]int) []string {
	type pair struct {
		item  string
		count int
	}
	list := make([]pair, 0, len(pop))
	for item, count := range pop {
		list = append(list, pair{item, count})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].count == list[j].count {
			return list[i].item < list[j].item
		}
		return list[i].count > list[j].count
	})
	out := make([]string, 0, len(list))
	for _, p := range list {
		out = append(out, p.item)
	}
	return out
}

func titleTokenSet(s string) map[string]bool {
	out := map[string]bool{}
	for _, token := range titleTokens(s) {
		out[token] = true
	}
	return out
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
