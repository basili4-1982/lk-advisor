package httpapi

import "testing"

func TestRelationPolicyStatus(t *testing.T) {
	policy := DefaultRelationPolicy()
	cases := []struct {
		confidence float64
		want       string
	}{
		{0.95, "confirmed"},
		{0.85, "confirmed"},
		{0.849, "llm_suggested"},
		{0.70, "llm_suggested"},
		{0.699, "quarantine"},
	}
	for _, tc := range cases {
		if got := policy.Status(tc.confidence); got != tc.want {
			t.Fatalf("Status(%v) = %q, want %q", tc.confidence, got, tc.want)
		}
	}
}

func TestRelationPolicyAutoConfirmDisabled(t *testing.T) {
	policy := DefaultRelationPolicy()
	policy.AutoConfirm = false
	if got := policy.Status(0.99); got != "llm_suggested" {
		t.Fatalf("Status(0.99) = %q, want llm_suggested", got)
	}
}

func TestRelationPolicyFromEnv(t *testing.T) {
	t.Setenv("RELATION_AUTO_CONFIRM", "false")
	t.Setenv("RELATION_AUTO_CONFIRM_THRESHOLD", "0.9")
	t.Setenv("RELATION_REVIEW_THRESHOLD", "0.75")

	policy := relationPolicyFromEnv()
	if policy.AutoConfirm {
		t.Fatal("auto confirm should be disabled")
	}
	if policy.AutoConfirmThreshold != 0.9 || policy.ReviewThreshold != 0.75 {
		t.Fatalf("policy = %+v", policy)
	}
}

func TestRelationPolicyInvalidEnvKeepsDefaults(t *testing.T) {
	t.Setenv("RELATION_AUTO_CONFIRM_THRESHOLD", "abc")
	t.Setenv("RELATION_REVIEW_THRESHOLD", "2")

	policy := relationPolicyFromEnv()
	def := DefaultRelationPolicy()
	if policy.AutoConfirmThreshold != def.AutoConfirmThreshold || policy.ReviewThreshold != def.ReviewThreshold {
		t.Fatalf("policy = %+v", policy)
	}
}
