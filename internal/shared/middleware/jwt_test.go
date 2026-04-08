package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func TestParseAndVerifyUserJWT(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	token := buildJWT(t, "secret", map[string]any{
		"sub":       "user_1",
		"tenant_id": "public",
		"roles":     []string{"admin"},
		"exp":       now.Add(time.Hour).Unix(),
	})

	claims, err := ParseAndVerifyUserJWT(token, "secret", now)
	if err != nil {
		t.Fatalf("expected valid token, got error: %v", err)
	}
	if claims.Subject != "user_1" || claims.TenantID != "public" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestParseAndVerifyUserJWTExpired(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	token := buildJWT(t, "secret", map[string]any{
		"sub": "user_1",
		"exp": now.Add(-time.Minute).Unix(),
	})
	if _, err := ParseAndVerifyUserJWT(token, "secret", now); err == nil {
		t.Fatalf("expected expiration error")
	}
}

func buildJWT(t *testing.T, secret string, payload map[string]any) string {
	t.Helper()
	headerRaw, _ := json.Marshal(map[string]any{"alg": "HS256", "typ": "JWT"})
	payloadRaw, _ := json.Marshal(payload)
	head := base64.RawURLEncoding.EncodeToString(headerRaw)
	body := base64.RawURLEncoding.EncodeToString(payloadRaw)
	signingInput := head + "." + body
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(signingInput))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return signingInput + "." + sig
}
