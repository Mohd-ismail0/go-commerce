package identity

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	sharederrors "rewrite/internal/shared/errors"
)

type Service struct {
	repo              *Repository
	jwtSecret         string
	jwtTTLMinute      int
	refreshTTLMinutes int
}

func NewService(repo *Repository, jwtSecret string, jwtTTLMinute, refreshTTLMinutes int) *Service {
	if jwtTTLMinute <= 0 {
		jwtTTLMinute = 60
	}
	if refreshTTLMinutes <= 0 {
		refreshTTLMinutes = 60 * 24 * 7
	}
	return &Service{
		repo:              repo,
		jwtSecret:         strings.TrimSpace(jwtSecret),
		jwtTTLMinute:      jwtTTLMinute,
		refreshTTLMinutes: refreshTTLMinutes,
	}
}

func (s *Service) Save(ctx context.Context, item User) (User, error) {
	if strings.TrimSpace(item.Email) == "" {
		return User{}, sharederrors.BadRequest("email is required")
	}
	passwordHash := ""
	if strings.TrimSpace(item.Password) != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(item.Password), bcrypt.DefaultCost)
		if err != nil {
			return User{}, sharederrors.Internal("failed to hash password")
		}
		passwordHash = string(hash)
	}
	return s.repo.Save(ctx, item, passwordHash)
}

func (s *Service) List(ctx context.Context, tenantID string) ([]User, error) {
	return s.repo.List(ctx, tenantID)
}

func (s *Service) Login(ctx context.Context, tenantID string, in LoginInput) (LoginResult, error) {
	if strings.TrimSpace(s.jwtSecret) == "" {
		return LoginResult{}, sharederrors.Internal("auth jwt secret is not configured")
	}
	if strings.TrimSpace(in.Email) == "" || strings.TrimSpace(in.Password) == "" {
		return LoginResult{}, sharederrors.BadRequest("email and password are required")
	}
	user, hash, err := s.repo.GetByEmail(ctx, tenantID, in.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			return LoginResult{}, sharederrors.BadRequest("invalid credentials")
		}
		return LoginResult{}, sharederrors.Internal("failed to load user")
	}
	if !user.IsActive {
		return LoginResult{}, sharederrors.BadRequest("user is inactive")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)); err != nil {
		return LoginResult{}, sharederrors.BadRequest("invalid credentials")
	}
	roles, err := s.repo.RolesForUser(ctx, tenantID, user.ID)
	if err != nil {
		return LoginResult{}, sharederrors.Internal("failed to load user roles")
	}
	return s.issueSessionTokens(ctx, tenantID, user.ID, roles)
}

func (s *Service) Refresh(ctx context.Context, tenantID string, in RefreshInput) (LoginResult, error) {
	if strings.TrimSpace(in.RefreshToken) == "" {
		return LoginResult{}, sharederrors.BadRequest("refresh_token is required")
	}
	hash := hashRefreshToken(in.RefreshToken)
	sessionID, userID, exp, err := s.repo.GetActiveSessionByRefreshHash(ctx, tenantID, hash)
	if err != nil {
		if err == sql.ErrNoRows {
			return LoginResult{}, sharederrors.BadRequest("invalid refresh token")
		}
		return LoginResult{}, sharederrors.Internal("failed to load auth session")
	}
	if time.Now().UTC().After(exp) {
		return LoginResult{}, sharederrors.BadRequest("refresh token expired")
	}
	roles, err := s.repo.RolesForUser(ctx, tenantID, userID)
	if err != nil {
		return LoginResult{}, sharederrors.Internal("failed to load user roles")
	}
	accessExp := time.Now().UTC().Add(time.Duration(s.jwtTTLMinute) * time.Minute)
	access, err := signHS256JWT(s.jwtSecret, map[string]any{
		"sub":       userID,
		"tenant_id": tenantID,
		"roles":     roles,
		"exp":       accessExp.Unix(),
	})
	if err != nil {
		return LoginResult{}, sharederrors.Internal("failed to issue token")
	}
	newRefreshToken, err := generateRandomToken(48)
	if err != nil {
		return LoginResult{}, sharederrors.Internal("failed to generate refresh token")
	}
	refreshExp := time.Now().UTC().Add(time.Duration(s.refreshTTLMinutes) * time.Minute)
	if err := s.repo.RotateSessionRefreshToken(ctx, sessionID, hashRefreshToken(newRefreshToken), refreshExp); err != nil {
		return LoginResult{}, sharederrors.Internal("failed to rotate refresh token")
	}
	return LoginResult{
		Token:            access,
		TokenType:        "Bearer",
		ExpiresAt:        accessExp.Unix(),
		RefreshToken:     newRefreshToken,
		RefreshExpiresAt: refreshExp.Unix(),
		UserID:           userID,
		TenantID:         tenantID,
		Roles:            roles,
	}, nil
}

func (s *Service) Logout(ctx context.Context, tenantID string, in RefreshInput) error {
	if strings.TrimSpace(in.RefreshToken) == "" {
		return sharederrors.BadRequest("refresh_token is required")
	}
	if err := s.repo.RevokeSessionByRefreshHash(ctx, tenantID, hashRefreshToken(in.RefreshToken)); err != nil {
		return sharederrors.Internal("failed to revoke auth session")
	}
	return nil
}

func (s *Service) issueSessionTokens(ctx context.Context, tenantID, userID string, roles []string) (LoginResult, error) {
	accessExp := time.Now().UTC().Add(time.Duration(s.jwtTTLMinute) * time.Minute)
	access, err := signHS256JWT(s.jwtSecret, map[string]any{
		"sub":       userID,
		"tenant_id": tenantID,
		"roles":     roles,
		"exp":       accessExp.Unix(),
	})
	if err != nil {
		return LoginResult{}, sharederrors.Internal("failed to issue token")
	}
	refreshToken, err := generateRandomToken(48)
	if err != nil {
		return LoginResult{}, sharederrors.Internal("failed to generate refresh token")
	}
	sessionRandom, err := generateRandomToken(6)
	if err != nil {
		return LoginResult{}, sharederrors.Internal("failed to generate session id")
	}
	sessionID := "sess_" + userID + "_" + strings.ToLower(sessionRandom)
	refreshExp := time.Now().UTC().Add(time.Duration(s.refreshTTLMinutes) * time.Minute)
	if err := s.repo.CreateAuthSession(ctx, sessionID, tenantID, userID, hashRefreshToken(refreshToken), refreshExp); err != nil {
		return LoginResult{}, sharederrors.Internal("failed to create auth session")
	}
	return LoginResult{
		Token:            access,
		TokenType:        "Bearer",
		ExpiresAt:        accessExp.Unix(),
		RefreshToken:     refreshToken,
		RefreshExpiresAt: refreshExp.Unix(),
		UserID:           userID,
		TenantID:         tenantID,
		Roles:            roles,
	}, nil
}

func signHS256JWT(secret string, payload map[string]any) (string, error) {
	headerJSON, err := json.Marshal(map[string]any{"alg": "HS256", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	head := base64.RawURLEncoding.EncodeToString(headerJSON)
	body := base64.RawURLEncoding.EncodeToString(payloadJSON)
	signingInput := head + "." + body
	mac, err := computeHMACSHA256([]byte(secret), []byte(signingInput))
	if err != nil {
		return "", err
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(mac), nil
}

func computeHMACSHA256(secret, msg []byte) ([]byte, error) {
	// local helper keeps signing implementation testable without extra dependencies.
	// mirrors middleware verifier algorithm.
	h := hmacSHA256(secret, msg)
	return h, nil
}

func hmacSHA256(secret, msg []byte) []byte {
	// duplicated tiny implementation boundary for simplicity
	// to avoid exposing middleware internals across modules.
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(msg)
	return mac.Sum(nil)
}

func hashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func generateRandomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
