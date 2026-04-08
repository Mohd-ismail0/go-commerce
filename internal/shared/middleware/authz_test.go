package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

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

func TestResolveIdentityRejectsTenantMismatch(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/payments", nil)
	req = req.WithContext(WithTenantID(req.Context(), "tenant_a"))
	token := buildJWT(t, "secret", map[string]any{
		"sub":       "u1",
		"tenant_id": "tenant_b",
	})
	req.Header.Set("X-User-JWT", token)
	_, _, err := resolveIdentity(req, PolicyOptions{UserJWTSecret: "secret"})
	if err == nil {
		t.Fatalf("expected tenant mismatch error")
	}
}

func TestResolveIdentityLegacyBypassDisabled(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/payments", nil)
	req.Header.Set("X-Role", "admin")
	userID, roles, err := resolveIdentity(req, PolicyOptions{AllowLegacyRoleBypass: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if userID != "" || len(roles) != 0 {
		t.Fatalf("expected no legacy identity when disabled")
	}
}

func TestUserJWTFromRequestSupportsBearer(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/payments", nil)
	req.Header.Set("Authorization", "Bearer token_abc")
	token := UserJWTFromRequest(req)
	if token != "token_abc" {
		t.Fatalf("expected bearer token, got %q", token)
	}
}

func TestResolveIdentityAcceptsBearerToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/payments", nil)
	req = req.WithContext(WithTenantID(req.Context(), "tenant_a"))
	token := buildJWT(t, "secret", map[string]any{
		"sub":       "u1",
		"tenant_id": "tenant_a",
	})
	req.Header.Set("Authorization", "Bearer "+token)
	userID, _, err := resolveIdentity(req, PolicyOptions{UserJWTSecret: "secret"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if userID != "u1" {
		t.Fatalf("expected user id u1, got %q", userID)
	}
}
