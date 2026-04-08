package payments

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// VerifyWebhookSignature compares the hex-encoded HMAC-SHA256 of the body using secret.
// When secret is empty, verification is skipped (development only; callers should reject in production).
func VerifyWebhookSignature(secret string, body []byte, signatureHeader string) bool {
	if strings.TrimSpace(secret) == "" {
		return true
	}
	sig := strings.TrimSpace(signatureHeader)
	sig = strings.TrimPrefix(strings.ToLower(sig), "sha256=")
	decoded, err := hex.DecodeString(sig)
	if err != nil || len(decoded) != sha256.Size {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	expected := mac.Sum(nil)
	return hmac.Equal(decoded, expected)
}
