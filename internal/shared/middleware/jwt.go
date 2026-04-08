package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type UserClaims struct {
	Subject  string
	TenantID string
	Roles    []string
	Exp      int64
	KeyID    string
}

func ParseAndVerifyUserJWT(token, secret string, now time.Time) (UserClaims, error) {
	return ParseAndVerifyUserJWTWithKeys(token, []JWTKey{{ID: "default", Secret: secret}}, now)
}

type JWTKey struct {
	ID     string
	Secret string
}

func ParseAndVerifyUserJWTWithKeys(token string, keys []JWTKey, now time.Time) (UserClaims, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return UserClaims{}, errors.New("invalid jwt format")
	}

	signingInput := parts[0] + "." + parts[1]
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return UserClaims{}, errors.New("invalid jwt signature encoding")
	}

	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return UserClaims{}, errors.New("invalid jwt header")
	}
	var header struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return UserClaims{}, errors.New("invalid jwt header")
	}
	if !strings.EqualFold(header.Alg, "HS256") {
		return UserClaims{}, errors.New("unsupported jwt algorithm")
	}
	key, keyErr := resolveJWTKey(header.Kid, keys)
	if keyErr != nil {
		return UserClaims{}, keyErr
	}

	mac := hmac.New(sha256.New, []byte(key.Secret))
	_, _ = mac.Write([]byte(signingInput))
	expected := mac.Sum(nil)
	if !hmac.Equal(sig, expected) {
		return UserClaims{}, errors.New("invalid jwt signature")
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return UserClaims{}, errors.New("invalid jwt payload")
	}
	var payload struct {
		Sub      string   `json:"sub"`
		TenantID string   `json:"tenant_id"`
		Roles    []string `json:"roles"`
		Exp      int64    `json:"exp"`
	}
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return UserClaims{}, errors.New("invalid jwt payload")
	}
	if strings.TrimSpace(payload.Sub) == "" {
		return UserClaims{}, errors.New("jwt sub is required")
	}
	if payload.Exp > 0 && now.Unix() >= payload.Exp {
		return UserClaims{}, errors.New("jwt token is expired")
	}

	return UserClaims{
		Subject:  strings.TrimSpace(payload.Sub),
		TenantID: strings.TrimSpace(payload.TenantID),
		Roles:    payload.Roles,
		Exp:      payload.Exp,
		KeyID:    key.ID,
	}, nil
}

func resolveJWTKey(kid string, keys []JWTKey) (JWTKey, error) {
	if len(keys) == 0 {
		return JWTKey{}, errors.New("jwt keyset is empty")
	}
	kid = strings.TrimSpace(kid)
	if kid == "" {
		// Backward compatibility: use first key when no kid is present.
		return keys[0], nil
	}
	for _, k := range keys {
		if strings.TrimSpace(k.ID) == kid {
			return k, nil
		}
	}
	return JWTKey{}, errors.New("jwt kid not recognized")
}
