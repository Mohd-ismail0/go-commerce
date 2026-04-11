package shop

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	sharederrors "rewrite/internal/shared/errors"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Get(ctx context.Context, tenantID, regionID string) (Settings, error) {
	settings, err := s.repo.Get(ctx, tenantID, regionID)
	if err != nil {
		return Settings{}, sharederrors.Internal("failed to load shop settings")
	}
	return settings, nil
}

func (s *Service) Patch(ctx context.Context, tenantID, regionID string, patch PatchInput, idempotencyKey string) (Settings, error) {
	if strings.TrimSpace(idempotencyKey) == "" {
		return Settings{}, sharederrors.BadRequest("Idempotency-Key is required")
	}
	key := strings.TrimSpace(idempotencyKey)
	if patch.Metadata != nil && len(*patch.Metadata) > 0 && !json.Valid(*patch.Metadata) {
		return Settings{}, sharederrors.BadRequest("metadata must be valid JSON")
	}
	if patch.Metadata != nil && len(*patch.Metadata) == 0 {
		patch.Metadata = nil
	}
	out, err := s.repo.Patch(ctx, tenantID, regionID, patch, key)
	if err != nil {
		if errors.Is(err, ErrShopSettingsIdempotencyKeyRequired) {
			return Settings{}, sharederrors.BadRequest("Idempotency-Key is required")
		}
		if errors.Is(err, ErrShopSettingsIdempotencyMismatch) || errors.Is(err, ErrShopSettingsIdempotencyOrphan) {
			return Settings{}, sharederrors.Internal("shop settings idempotency record is inconsistent")
		}
		return Settings{}, sharederrors.Internal("failed to update shop settings")
	}
	return out, nil
}
