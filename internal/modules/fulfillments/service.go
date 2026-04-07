package fulfillments

import (
	"context"
	"errors"
	"strings"

	sharederrors "rewrite/internal/shared/errors"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, in Fulfillment) (Fulfillment, error) {
	if strings.TrimSpace(in.OrderID) == "" {
		return Fulfillment{}, sharederrors.BadRequest("order_id is required")
	}
	if strings.TrimSpace(in.Status) == "" {
		in.Status = "fulfilled"
	}
	saved, err := s.repo.Create(ctx, in)
	if err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			return Fulfillment{}, sharederrors.NotFound(err.Error())
		}
		return Fulfillment{}, sharederrors.Internal("failed to create fulfillment")
	}
	return saved, nil
}

func (s *Service) List(ctx context.Context, tenantID, regionID, orderID string) ([]Fulfillment, error) {
	return s.repo.List(ctx, tenantID, regionID, orderID)
}
