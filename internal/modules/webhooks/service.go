package webhooks

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

func (s *Service) Save(ctx context.Context, in Subscription) (Subscription, error) {
	if strings.TrimSpace(in.EventName) == "" || strings.TrimSpace(in.EndpointURL) == "" {
		return Subscription{}, sharederrors.BadRequest("event_name and endpoint_url are required")
	}
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(in.EndpointURL)), "http://") &&
		!strings.HasPrefix(strings.ToLower(strings.TrimSpace(in.EndpointURL)), "https://") {
		return Subscription{}, sharederrors.BadRequest("endpoint_url must be http/https")
	}
	saved, err := s.repo.Save(ctx, in)
	if err != nil {
		return Subscription{}, sharederrors.Internal("failed to save webhook subscription")
	}
	return saved, nil
}

func (s *Service) List(ctx context.Context, tenantID, regionID string, onlyActive bool) ([]Subscription, error) {
	items, err := s.repo.List(ctx, tenantID, regionID, onlyActive)
	if err != nil {
		return nil, sharederrors.Internal("failed to list webhook subscriptions")
	}
	return items, nil
}
