package recommend_test

import (
	"testing"

	"github.com/basili4-1982/lk-advisor/internal/model"
	"github.com/basili4-1982/lk-advisor/internal/recommend"
)

func TestIntent(t *testing.T) {
	cases := []struct {
		name string
		in   recommend.Input
		want string
	}{
		{
			name: "renewal",
			in: input(profile([]model.License{licenseFor("prod-antivirus", 14)}, nil),
				[]model.Product{prod("prod-antivirus", 15000, 4, "active")}, nil, nil, nil),
			want: "renewal",
		},
		{
			name: "upsell",
			in: input(profile(nil, []model.Purchase{purchase("prod-office-suite", 60)}),
				[]model.Product{prod("prod-office-suite", 35000, 4, "active"), prod("prod-office-pack", 50000, 4, "active")},
				[]model.Relation{{FromID: "prod-office-suite", ToID: "prod-office-pack", Type: "upgrade", Status: "confirmed", Confidence: 0.9}},
				nil, nil),
			want: "upsell",
		},
		{
			name: "cross_sell",
			in: input(profile(nil, []model.Purchase{purchase("prod-raster", 10)}),
				[]model.Product{prod("prod-raster", 22000, 4, "active"), prod("prod-vector", 20000, 4, "active")},
				[]model.Relation{{FromID: "prod-raster", ToID: "prod-vector", Type: "adjacent_task", Status: "confirmed", Confidence: 0.9}},
				nil, nil),
			want: "cross_sell",
		},
		{
			name: "analog",
			in: input(profile(nil, []model.Purchase{purchase("prod-old", 400)}),
				[]model.Product{prod("prod-old", 10000, 4, "discontinued"), prod("prod-new", 12000, 4, "active")},
				[]model.Relation{{FromID: "prod-old", ToID: "prod-new", Type: "analog", Status: "confirmed", Confidence: 0.9}},
				nil, nil),
			want: "analog",
		},
		{
			name: "none",
			in:   input(profile(nil, nil), nil, nil, nil, nil),
			want: "none",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := recommend.Intent(tc.in); got != tc.want {
				t.Fatalf("intent = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCandidatesForCrossSellSkipSubstitute(t *testing.T) {
	in := input(profile(nil, []model.Purchase{purchase("prod-raster", 10)}),
		[]model.Product{prod("prod-raster", 22000, 4, "active"), prod("prod-vector", 20000, 4, "active"), prod("prod-raster-2", 24000, 4, "active")},
		[]model.Relation{
			{FromID: "prod-raster", ToID: "prod-vector", Type: "adjacent_task", Status: "confirmed", Confidence: 0.9},
			{FromID: "prod-raster", ToID: "prod-raster-2", Type: "substitute", Status: "confirmed", Confidence: 0.9},
		}, nil, nil)

	cands := recommend.Candidates(in)
	if len(cands) != 1 || cands[0].Product.ID != "prod-vector" {
		t.Fatalf("candidates = %+v, want only prod-vector", cands)
	}
}

func TestCandidatesForUpsell(t *testing.T) {
	in := input(profile(nil, []model.Purchase{purchase("prod-vector", 20)}),
		[]model.Product{prod("prod-vector", 20000, 4, "active"), prod("prod-vector-pro", 32000, 4, "active")},
		[]model.Relation{{FromID: "prod-vector", ToID: "prod-vector-pro", Type: "upgrade", Status: "confirmed", Confidence: 0.9}},
		nil, nil)

	cands := recommend.Candidates(in)
	if len(cands) != 1 || cands[0].Type != "upsell" || cands[0].Product.ID != "prod-vector-pro" {
		t.Fatalf("candidates = %+v", cands)
	}
}
