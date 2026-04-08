package identity

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	sharederrors "rewrite/internal/shared/errors"
)

type Service struct {
	repo         *Repository
	jwtSecret    string
	jwtTTLMinute int
}

func NewService(repo *Repository, jwtSecret string, jwtTTLMinute int) *Service {
	if jwtTTLMinute <= 0 {
		jwtTTLMinute = 60
	}
	return &Service{repo: repo, jwtSecret: strings.TrimSpace(jwtSecret), jwtTTLMinute: jwtTTLMinute}
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
	exp := time.Now().UTC().Add(time.Duration(s.jwtTTLMinute) * time.Minute)
	token, err := signHS256JWT(s.jwtSecret, map[string]any{
		"sub":       user.ID,
		"tenant_id": tenantID,
		"roles":     roles,
		"exp":       exp.Unix(),
	})
	if err != nil {
		return LoginResult{}, sharederrors.Internal("failed to issue token")
	}
	return LoginResult{
		Token:     token,
		TokenType: "Bearer",
		ExpiresAt: exp.Unix(),
		UserID:    user.ID,
		TenantID:  tenantID,
		Roles:     roles,
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
