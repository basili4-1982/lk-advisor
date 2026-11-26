package strategy_test

import (
	"testing"

	"github.com/basili4-1982/lk-advisor/internal/strategy"
)

func TestDefault(t *testing.T) {
	if got := strategy.Default("b2b"); got != "hybrid" {
		t.Fatalf("b2b = %s", got)
	}
	if got := strategy.Default("b2c"); got != "popular" {
		t.Fatalf("b2c = %s", got)
	}
}

func TestChooseUsesPosterior(t *testing.T) {
	stats := []strategy.Stat{
		{Audience: "b2b", Strategy: "hybrid", Enabled: true, Alpha: 1, Beta: 1, Shown: 100, Clicks: 10},
		{Audience: "b2b", Strategy: "popular", Enabled: true, Alpha: 1, Beta: 1, Shown: 100, Clicks: 2},
	}
	got := strategy.ChooseWith("b2b", stats, func() float64 { return 0.5 })
	if got != "hybrid" {
		t.Fatalf("got = %s, want hybrid", got)
	}
}

func TestChooseFallsBackToDefault(t *testing.T) {
	if got := strategy.ChooseWith("b2c", nil, func() float64 { return 0.5 }); got != "popular" {
		t.Fatalf("got = %s, want popular", got)
	}
}

func TestChooseSkipsDisabled(t *testing.T) {
	stats := []strategy.Stat{
		{Audience: "b2b", Strategy: "hybrid", Enabled: false, Alpha: 10, Beta: 1},
		{Audience: "b2b", Strategy: "popular", Enabled: true, Alpha: 1, Beta: 1},
	}
	got := strategy.ChooseWith("b2b", stats, func() float64 { return 0.5 })
	if got != "popular" {
		t.Fatalf("got = %s, want popular", got)
	}
}

func TestKeepsDefaultUntilEnoughImpressions(t *testing.T) {
	stats := []strategy.Stat{
		{Audience: "b2b", Strategy: "hybrid", Enabled: true, Alpha: 1, Beta: 1, Shown: 5, Clicks: 1},
		{Audience: "b2b", Strategy: "popular", Enabled: true, Alpha: 1, Beta: 1, Shown: 100, Clicks: 50},
	}
	got := strategy.ChooseWith("b2b", stats, func() float64 { return 0.5 })
	if got != "hybrid" {
		t.Fatalf("got = %s, want hybrid before 20 impressions", got)
	}
	stats[0].Shown = 25
	stats[0].Clicks = 1
	got = strategy.ChooseWith("b2b", stats, func() float64 { return 0.5 })
	if got != "popular" {
		t.Fatalf("got = %s, want popular after challenge", got)
	}
}
