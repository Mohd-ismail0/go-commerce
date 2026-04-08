package fulfillments

import (
	"context"
	"errors"
	"strings"
	"time"

	"rewrite/internal/modules/orders"
	sharederrors "rewrite/internal/shared/errors"
)

type Service struct {
	repo   Repository
	orders OrderStatusUpdater
}

type OrderStatusUpdater interface {
	GetByID(ctx context.Context, tenantID, orderID string) (orders.Order, error)
	UpdateStatus(ctx context.Context, tenantID string, input orders.StatusUpdateInput) (orders.Order, error)
}

func NewService(repo Repository, ordersSvc OrderStatusUpdater) *Service {
	return &Service{repo: repo, orders: ordersSvc}
}

func (s *Service) Create(ctx context.Context, in Fulfillment) (Fulfillment, error) {
	if strings.TrimSpace(in.OrderID) == "" {
		return Fulfillment{}, sharederrors.BadRequest("order_id is required")
	}
	if strings.TrimSpace(in.Status) == "" {
		in.Status = "fulfilled"
	}
	if s.orders == nil {
		return Fulfillment{}, sharederrors.Internal("orders service is not configured")
	}
	saved, err := s.repo.Create(ctx, in)
	if err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			return Fulfillment{}, sharederrors.NotFound(err.Error())
		}
		return Fulfillment{}, sharederrors.Internal("failed to create fulfillment")
	}
	currentOrder, err := s.orders.GetByID(ctx, in.TenantID, in.OrderID)
	if err != nil {
		return Fulfillment{}, err
	}
	updatedAt, parseErr := time.Parse(time.RFC3339Nano, currentOrder.UpdatedAt)
	if parseErr != nil {
		return Fulfillment{}, sharederrors.Internal("failed to parse order timestamp")
	}
	if currentOrder.Status == "created" {
		confirmed, updateErr := s.orders.UpdateStatus(ctx, in.TenantID, orders.StatusUpdateInput{
			ID:                in.OrderID,
			Status:            "confirmed",
			ExpectedUpdatedAt: updatedAt,
		})
		if updateErr != nil {
			return Fulfillment{}, updateErr
		}
		updatedAt, parseErr = time.Parse(time.RFC3339Nano, confirmed.UpdatedAt)
		if parseErr != nil {
			return Fulfillment{}, sharederrors.Internal("failed to parse order timestamp")
		}
	}
	if _, err := s.orders.UpdateStatus(ctx, in.TenantID, orders.StatusUpdateInput{
		ID:                in.OrderID,
		Status:            "completed",
		ExpectedUpdatedAt: updatedAt,
	}); err != nil {
		return Fulfillment{}, err
	}
	return saved, nil
}

func (s *Service) List(ctx context.Context, tenantID, regionID, orderID string) ([]Fulfillment, error) {
	return s.repo.List(ctx, tenantID, regionID, orderID)
}
