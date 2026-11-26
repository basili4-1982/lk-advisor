package recommend_test

import (
	"testing"

	"github.com/basili4-1982/lk-advisor/internal/model"
	"github.com/basili4-1982/lk-advisor/internal/recommend"
)

func TestConsumerDominantSkipsBusinessUpgrade(t *testing.T) {
	c1 := prod("c1", 1000, 4, "active")
	c1.Audience = "consumer"
	c2 := prod("c2", 1000, 4, "active")
	c2.Audience = "consumer"
	c3 := prod("c3", 1000, 4, "active")
	c3.Audience = "consumer"
	b1 := prod("b1", 1000, 4, "active")
	b1.Audience = "business"
	businessTarget := prod("b2", 4000, 4, "active")
	businessTarget.Audience = "business"
	consumerTarget := prod("c4", 2000, 4, "active")
	consumerTarget.Audience = "consumer"

	in := input(profile(nil, []model.Purchase{
		purchase("c1", 10), purchase("c2", 10), purchase("c3", 10), purchase("b1", 10),
	}), []model.Product{c1, c2, c3, b1, businessTarget, consumerTarget},
		[]model.Relation{
			{FromID: "c1", ToID: "b2", Type: "upgrade", Status: "confirmed", Confidence: 0.9},
			{FromID: "c1", ToID: "c4", Type: "adjacent_task", Status: "confirmed", Confidence: 0.9},
		}, nil, nil)

	got, _ := recommend.Run(in)
	seen := map[string]bool{}
	for _, c := range got {
		seen[c.Product.ID] = true
	}
	if seen["b2"] {
		t.Fatalf("business upgrade leaked into consumer-dominant profile: %+v", got)
	}
	if !seen["c4"] {
		t.Fatalf("consumer complement missing: %+v", got)
	}
}
