package catalog

import "testing"

func TestRelationPerformanceMultiplierStartsFull(t *testing.T) {
	if got := RelationPerformanceMultiplier(0, 0, 0); got != 1 {
		t.Fatalf("multiplier = %v, want 1", got)
	}
	if got := RelationPerformanceMultiplier(5, 0, 0); got >= 1 || got < 0.7 {
		t.Fatalf("multiplier after 5 shown = %v, want a gentle decay", got)
	}
}

func TestRelationPerformanceMultiplierDecaysSmoothly(t *testing.T) {
	previous := 1.0
	for _, shown := range []int{20, 50, 100, 200, 500} {
		got := RelationPerformanceMultiplier(shown, 0, 0)
		if got >= previous {
			t.Fatalf("shown=%d: multiplier %v did not decrease from %v", shown, got, previous)
		}
		if got < 0 || got > 1 {
			t.Fatalf("shown=%d: multiplier out of range: %v", shown, got)
		}
		previous = got
	}
}

func TestRelationPerformanceMultiplierRewardsClicksAndPurchases(t *testing.T) {
	ignored := RelationPerformanceMultiplier(100, 0, 0)
	clicked := RelationPerformanceMultiplier(100, 5, 0)
	if clicked <= ignored {
		t.Fatalf("clicked=%v ignored=%v", clicked, ignored)
	}
	if got := RelationPerformanceMultiplier(500, 0, 1); got != 1 {
		t.Fatalf("purchase multiplier = %v, want 1", got)
	}
}
