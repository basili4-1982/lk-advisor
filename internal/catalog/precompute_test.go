package catalog

import "testing"

func TestRecommendForPrecomputeUsesCoPurchase(t *testing.T) {
	items := map[string]preItem{
		"A": {title: "Купленный товар", available: true, tokens: map[string]bool{"купленный": true}},
		"B": {title: "Связанный товар", available: true, tokens: map[string]bool{"связанный": true}},
		"C": {title: "Популярный товар", available: true, tokens: map[string]bool{"популярный": true}},
	}
	owned := map[string]bool{"A": true}
	pop := map[string]int{"B": 10, "C": 20}
	neighbours := map[string][]neighbour{"A": {{item: "B", count: 5}}}
	itemPop := map[string]int{"A": 10, "B": 10, "C": 10}
	out := recommendForPrecompute("b2b", owned, items, pop, 20,
		[]string{"C", "B"}, neighbours, itemPop, map[string][]string{}, 2)
	if len(out) != 2 || out[0] != "B" {
		t.Fatalf("out = %v, want B first", out)
	}
	if out[0] == "A" || out[1] == "A" {
		t.Fatalf("owned item leaked: %v", out)
	}
}

func TestPlaceForcedDoesNotDuplicate(t *testing.T) {
	ranked := []ServeItem{{ItemID: "A"}, {ItemID: "B"}, {ItemID: "C"}}
	out := placeForced(ranked, ServeItem{ItemID: "B", Forced: true}, 3)
	seen := map[string]int{}
	for _, it := range out {
		seen[it.ItemID]++
	}
	if seen["B"] != 1 || len(out) != 3 {
		t.Fatalf("out = %+v", out)
	}
}
