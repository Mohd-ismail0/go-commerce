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
}

func ParseAndVerifyUserJWT(token, secret string, now time.Time) (UserClaims, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return UserClaims{}, errors.New("invalid jwt format")
	}

	signingInput := parts[0] + "." + parts[1]
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return UserClaims{}, errors.New("invalid jwt signature encoding")
	}

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(signingInput))
	expected := mac.Sum(nil)
	if !hmac.Equal(sig, expected) {
		return UserClaims{}, errors.New("invalid jwt signature")
	}

	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return UserClaims{}, errors.New("invalid jwt header")
	}
	var header struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return UserClaims{}, errors.New("invalid jwt header")
	}
	if !strings.EqualFold(header.Alg, "HS256") {
		return UserClaims{}, errors.New("unsupported jwt algorithm")
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
	}, nil
}
