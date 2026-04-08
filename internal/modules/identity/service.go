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
	"rewrite/internal/shared/middleware"
)

type Service struct {
	repo              *Repository
	signingKey        JWTSigningKey
	verifyKeys        []JWTSigningKey
	jwtTTLMinute      int
	refreshTTLMinutes int
}

type JWTSigningKey struct {
	ID     string
	Secret string
}

func NewService(repo *Repository, jwtSecret, jwtKeyset string, jwtTTLMinute, refreshTTLMinutes int) *Service {
	if jwtTTLMinute <= 0 {
		jwtTTLMinute = 60
	}
	if refreshTTLMinutes <= 0 {
		refreshTTLMinutes = 60 * 24 * 7
	}
	keys := parseJWTSigningKeys(jwtSecret, jwtKeyset)
	primary := JWTSigningKey{ID: "legacy", Secret: strings.TrimSpace(jwtSecret)}
	if len(keys) > 0 {
		primary = keys[0]
	}
	return &Service{
		repo:              repo,
		signingKey:        primary,
		verifyKeys:        keys,
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
	if strings.TrimSpace(s.signingKey.Secret) == "" {
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
	return s.issueSessionTokens(ctx, tenantID, user.ID, roles, in.DeviceID, "", "")
}

func (s *Service) Refresh(ctx context.Context, tenantID string, in RefreshInput) (LoginResult, error) {
	if strings.TrimSpace(in.RefreshToken) == "" {
		return LoginResult{}, sharederrors.BadRequest("refresh_token is required")
	}
	hash := hashRefreshToken(in.RefreshToken)
	sessionID, userID, deviceID, exp, err := s.repo.GetActiveSessionByRefreshHash(ctx, tenantID, hash)
	if err != nil {
		if err == sql.ErrNoRows {
			replayed, replayErr := s.repo.RevokeSessionByPreviousRefreshHash(ctx, tenantID, hash)
			if replayErr == nil && replayed {
				return LoginResult{}, sharederrors.Conflict("refresh token replay detected; session revoked")
			}
			return LoginResult{}, sharederrors.BadRequest("invalid refresh token")
		}
		return LoginResult{}, sharederrors.Internal("failed to load auth session")
	}
	if time.Now().UTC().After(exp) {
		return LoginResult{}, sharederrors.BadRequest("refresh token expired")
	}
	if strings.TrimSpace(in.DeviceID) != "" && strings.TrimSpace(deviceID) != "" && strings.TrimSpace(in.DeviceID) != strings.TrimSpace(deviceID) {
		return LoginResult{}, sharederrors.Conflict("refresh token device mismatch")
	}
	roles, err := s.repo.RolesForUser(ctx, tenantID, userID)
	if err != nil {
		return LoginResult{}, sharederrors.Internal("failed to load user roles")
	}
	accessExp := time.Now().UTC().Add(time.Duration(s.jwtTTLMinute) * time.Minute)
	access, err := signHS256JWT(s.signingKey, map[string]any{
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

func (s *Service) issueSessionTokens(ctx context.Context, tenantID, userID string, roles []string, deviceID, ipHash, userAgent string) (LoginResult, error) {
	accessExp := time.Now().UTC().Add(time.Duration(s.jwtTTLMinute) * time.Minute)
	access, err := signHS256JWT(s.signingKey, map[string]any{
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
	if err := s.repo.CreateAuthSession(ctx, sessionID, tenantID, userID, hashRefreshToken(refreshToken), deviceID, ipHash, userAgent, refreshExp); err != nil {
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

func (s *Service) ListSessions(ctx context.Context, tenantID, userJWT string) ([]SessionInfo, error) {
	userID, _, err := s.parseUserJWT(tenantID, userJWT)
	if err != nil {
		return nil, err
	}
	out, listErr := s.repo.ListSessionsByUser(ctx, tenantID, userID)
	if listErr != nil {
		return nil, sharederrors.Internal("failed to list sessions")
	}
	return out, nil
}

func (s *Service) RevokeSession(ctx context.Context, tenantID, userJWT, sessionID string) error {
	userID, _, err := s.parseUserJWT(tenantID, userJWT)
	if err != nil {
		return err
	}
	if strings.TrimSpace(sessionID) == "" {
		return sharederrors.BadRequest("session_id is required")
	}
	if err := s.repo.RevokeSessionByID(ctx, tenantID, userID, sessionID); err != nil {
		return sharederrors.Internal("failed to revoke session")
	}
	return nil
}

func (s *Service) RevokeOtherSessions(ctx context.Context, tenantID, userJWT, currentSessionRefreshToken string) error {
	userID, _, err := s.parseUserJWT(tenantID, userJWT)
	if err != nil {
		return err
	}
	if strings.TrimSpace(currentSessionRefreshToken) == "" {
		return sharederrors.BadRequest("refresh_token is required")
	}
	hash := hashRefreshToken(currentSessionRefreshToken)
	sessionID, sessionUserID, _, _, err := s.repo.GetActiveSessionByRefreshHash(ctx, tenantID, hash)
	if err != nil || sessionUserID != userID {
		return sharederrors.BadRequest("invalid current session token")
	}
	if err := s.repo.RevokeOtherSessions(ctx, tenantID, userID, sessionID); err != nil {
		return sharederrors.Internal("failed to revoke other sessions")
	}
	return nil
}

func (s *Service) parseUserJWT(tenantID, userJWT string) (string, []string, error) {
	token := strings.TrimSpace(userJWT)
	if token == "" {
		return "", nil, sharederrors.BadRequest("X-User-JWT header is required")
	}
	keys := make([]middleware.JWTKey, 0, len(s.verifyKeys))
	for _, k := range s.verifyKeys {
		keys = append(keys, middleware.JWTKey{ID: k.ID, Secret: k.Secret})
	}
	claims, err := middleware.ParseAndVerifyUserJWTWithKeys(token, keys, time.Now().UTC())
	if err != nil {
		return "", nil, sharederrors.BadRequest("invalid user jwt")
	}
	if claims.TenantID != "" && !strings.EqualFold(claims.TenantID, tenantID) {
		return "", nil, sharederrors.BadRequest("jwt tenant mismatch")
	}
	return claims.Subject, claims.Roles, nil
}

func signHS256JWT(key JWTSigningKey, payload map[string]any) (string, error) {
	headerJSON, err := json.Marshal(map[string]any{"alg": "HS256", "typ": "JWT", "kid": key.ID})
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
	mac, err := computeHMACSHA256([]byte(key.Secret), []byte(signingInput))
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

func parseJWTSigningKeys(legacySecret, keyset string) []JWTSigningKey {
	out := []JWTSigningKey{}
	for _, pair := range strings.Split(strings.TrimSpace(keyset), ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			continue
		}
		kid := strings.TrimSpace(parts[0])
		secret := strings.TrimSpace(parts[1])
		if kid == "" || secret == "" {
			continue
		}
		out = append(out, JWTSigningKey{ID: kid, Secret: secret})
	}
	if len(out) == 0 && strings.TrimSpace(legacySecret) != "" {
		out = append(out, JWTSigningKey{ID: "legacy", Secret: strings.TrimSpace(legacySecret)})
	}
	return out
}
