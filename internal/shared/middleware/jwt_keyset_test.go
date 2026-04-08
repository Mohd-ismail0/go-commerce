package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func TestParseAndVerifyUserJWTWithKeysHonorsKid(t *testing.T) {
	token := buildJWTWithKid(t, "k2", "secret-2", map[string]any{
		"sub":       "u1",
		"tenant_id": "public",
		"exp":       time.Now().UTC().Add(time.Hour).Unix(),
	})
	claims, err := ParseAndVerifyUserJWTWithKeys(token, []JWTKey{
		{ID: "k1", Secret: "secret-1"},
		{ID: "k2", Secret: "secret-2"},
	}, time.Now().UTC())
	if err != nil {
		t.Fatalf("expected verify success, got: %v", err)
	}
	if claims.KeyID != "k2" {
		t.Fatalf("expected key id k2, got %s", claims.KeyID)
	}
}

func buildJWTWithKid(t *testing.T, kid, secret string, payload map[string]any) string {
	t.Helper()
	headerRaw, _ := json.Marshal(map[string]any{"alg": "HS256", "typ": "JWT", "kid": kid})
	payloadRaw, _ := json.Marshal(payload)
	h := base64.RawURLEncoding.EncodeToString(headerRaw)
	p := base64.RawURLEncoding.EncodeToString(payloadRaw)
	signingInput := h + "." + p
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(signingInput))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return signingInput + "." + sig
}
