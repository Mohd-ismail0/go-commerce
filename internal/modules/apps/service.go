package apps

import (
	"context"
	"strings"

	sharederrors "rewrite/internal/shared/errors"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Save(ctx context.Context, in App) (App, error) {
	if strings.TrimSpace(in.Name) == "" {
		return App{}, sharederrors.BadRequest("name is required")
	}
	return s.repo.Save(ctx, in)
}

func (s *Service) List(ctx context.Context, tenantID, regionID string, activeOnly bool) ([]App, error) {
	return s.repo.List(ctx, tenantID, regionID, activeOnly)
}
