package webhooks

import (
	"context"
	"strings"

	shareddb "rewrite/internal/shared/db"
	sharederrors "rewrite/internal/shared/errors"
	"rewrite/internal/shared/events"
)

type webhookRepo interface {
	Save(ctx context.Context, in Subscription) (Subscription, error)
	List(ctx context.Context, tenantID, regionID string, onlyActive bool) ([]Subscription, error)
	GetByID(ctx context.Context, tenantID, regionID, id string) (Subscription, bool, error)
	AppExists(ctx context.Context, tenantID, regionID, appID string) (bool, error)
	ListDeliveries(ctx context.Context, tenantID, regionID, status, eventName string, limit int) ([]Delivery, error)
	RetryDeadOutbox(ctx context.Context, tenantID, regionID, outboxID, reason, requestedBy string) (bool, error)
}

type Service struct {
	repo webhookRepo
}

func NewService(repo webhookRepo) *Service {
	return &Service{repo: repo}
}

func (s *Service) Save(ctx context.Context, in Subscription, patch bool) (Subscription, error) {
	in.EventName = strings.TrimSpace(in.EventName)
	in.EndpointURL = strings.TrimSpace(in.EndpointURL)
	in.AppID = strings.TrimSpace(in.AppID)
	in.Secret = strings.TrimSpace(in.Secret)
	if patch {
		existing, ok, err := s.repo.GetByID(ctx, in.TenantID, in.RegionID, strings.TrimSpace(in.ID))
		if err != nil {
			return Subscription{}, sharederrors.Internal("failed to load webhook subscription")
		}
		if !ok {
			return Subscription{}, sharederrors.NotFound("webhook subscription not found")
		}
		merged := existing
		if in.EventName != "" {
			merged.EventName = in.EventName
		}
		if in.EndpointURL != "" {
			merged.EndpointURL = in.EndpointURL
		}
		if in.AppID != "" {
			merged.AppID = in.AppID
		}
		if in.Secret != "" {
			merged.Secret = in.Secret
		}
		if in.IsActive != existing.IsActive {
			merged.IsActive = in.IsActive
		}
		return s.validateAndSave(ctx, merged)
	}
	if in.EventName == "" || in.EndpointURL == "" {
		return Subscription{}, sharederrors.BadRequest("event_name and endpoint_url are required")
	}
	return s.validateAndSave(ctx, in)
}

func (s *Service) SetActive(ctx context.Context, tenantID, regionID, id string, active bool) (Subscription, error) {
	item, ok, err := s.repo.GetByID(ctx, tenantID, regionID, strings.TrimSpace(id))
	if err != nil {
		return Subscription{}, sharederrors.Internal("failed to load webhook subscription")
	}
	if !ok {
		return Subscription{}, sharederrors.NotFound("webhook subscription not found")
	}
	item.IsActive = active
	return s.validateAndSave(ctx, item)
}

func (s *Service) validateAndSave(ctx context.Context, in Subscription) (Subscription, error) {
	if !isAllowedEvent(in.EventName) {
		return Subscription{}, sharederrors.BadRequest("event_name is not supported")
	}
	if !strings.HasPrefix(strings.ToLower(in.EndpointURL), "http://") &&
		!strings.HasPrefix(strings.ToLower(in.EndpointURL), "https://") {
		return Subscription{}, sharederrors.BadRequest("endpoint_url must be http/https")
	}
	if in.AppID != "" {
		found, err := s.repo.AppExists(ctx, in.TenantID, in.RegionID, in.AppID)
		if err != nil {
			return Subscription{}, sharederrors.Internal("failed to validate app_id")
		}
		if !found {
			return Subscription{}, sharederrors.BadRequest("app_id does not exist in tenant/region")
		}
	}
	saved, err := s.repo.Save(ctx, in)
	if err != nil {
		if shareddb.IsUniqueConstraintViolation(err, "ux_webhook_subscriptions_tenant_region_event_endpoint_app") {
			return Subscription{}, sharederrors.Conflict("duplicate webhook subscription for event and endpoint")
		}
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

func (s *Service) ListDeliveries(ctx context.Context, tenantID, regionID, status, eventName string, limit int) ([]Delivery, error) {
	items, err := s.repo.ListDeliveries(ctx, tenantID, regionID, strings.TrimSpace(status), strings.TrimSpace(eventName), limit)
	if err != nil {
		return nil, sharederrors.Internal("failed to list webhook deliveries")
	}
	return items, nil
}

func (s *Service) RetryDeadOutbox(ctx context.Context, tenantID, regionID, outboxID, reason, requestedBy string) error {
	outboxID = strings.TrimSpace(outboxID)
	reason = strings.TrimSpace(reason)
	requestedBy = strings.TrimSpace(requestedBy)
	if outboxID == "" {
		return sharederrors.BadRequest("outbox id is required")
	}
	if reason == "" {
		return sharederrors.BadRequest("reason is required")
	}
	ok, err := s.repo.RetryDeadOutbox(ctx, tenantID, regionID, outboxID, reason, requestedBy)
	if err != nil {
		return sharederrors.Internal("failed to retry outbox event")
	}
	if !ok {
		return sharederrors.NotFound("dead outbox event not found")
	}
	return nil
}

func isAllowedEvent(name string) bool {
	switch strings.TrimSpace(name) {
	case events.EventOrderCreated, events.EventOrderCompleted, events.EventProductUpdated, events.EventInventoryChange:
		return true
	default:
		return false
	}
}
