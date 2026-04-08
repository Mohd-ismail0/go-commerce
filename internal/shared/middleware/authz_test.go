package middleware

import "testing"

func TestMatchPolicyRuleLongestPrefix(t *testing.T) {
	rules := []PolicyRule{
		{Prefix: "/payments", PermissionCode: "payments.manage"},
		{Prefix: "/payments/webhooks", PermissionCode: "other"},
	}
	code := MatchPolicyRule("/payments/webhooks/foo", rules)
	if code != "other" {
		t.Fatalf("expected other, got %q", code)
	}
}

func TestMatchPolicyRuleNoMatch(t *testing.T) {
	rules := []PolicyRule{{Prefix: "/payments", PermissionCode: "payments.manage"}}
	if MatchPolicyRule("/products", rules) != "" {
		t.Fatalf("expected empty match")
	}
}
