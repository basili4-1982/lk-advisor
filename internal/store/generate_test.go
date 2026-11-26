package store_test

import (
	"reflect"
	"testing"

	"github.com/basili4-1982/lk-advisor/internal/store"
)

func TestGenerateSeed42(t *testing.T) {
	a, err := store.Generate(42)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	b, err := store.Generate(42)
	if err != nil {
		t.Fatalf("generate second: %v", err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("dataset is not deterministic for the same seed")
	}

	counts := []struct {
		name string
		got  int
		want int
	}{
		{"clients", len(a.Clients), 50},
		{"products", len(a.Products), 200},
		{"purchases", len(a.Purchases), 500},
		{"licenses", len(a.Licenses), 100},
		{"orders", len(a.Orders), 200},
		{"events", len(a.Events), 1000},
		{"rules", len(a.Rules), 20},
		{"promotions", len(a.Promotions), 40},
		{"environment", len(a.Environment), 50},
	}
	for _, c := range counts {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
	if len(a.Relations) < 300 {
		t.Errorf("relations = %d, want at least 300", len(a.Relations))
	}

	covered := map[string]bool{}
	for _, g := range a.GroundTruth {
		covered[g.ClientID] = true
	}
	if len(covered) != 50 {
		t.Errorf("ground truth covers %d clients, want 50", len(covered))
	}
}
