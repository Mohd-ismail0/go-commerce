package webhooks

import (
	"context"
	"strings"

	sharederrors "rewrite/internal/shared/errors"
	"rewrite/internal/shared/events"
)

type webhookRepo interface {
	Save(ctx context.Context, in Subscription) (Subscription, error)
	List(ctx context.Context, tenantID, regionID string, onlyActive bool) ([]Subscription, error)
	GetByID(ctx context.Context, tenantID, regionID, id string) (Subscription, bool, error)
	AppExists(ctx context.Context, tenantID, regionID, appID string) (bool, error)
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
		lower := strings.ToLower(err.Error())
		if strings.Contains(lower, "ux_webhook_subscriptions_tenant_region_event_endpoint_app") ||
			strings.Contains(lower, "duplicate key") ||
			strings.Contains(lower, "unique constraint") {
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

func isAllowedEvent(name string) bool {
	switch strings.TrimSpace(name) {
	case events.EventOrderCreated, events.EventOrderCompleted, events.EventProductUpdated, events.EventInventoryChange:
		return true
	default:
		return false
	}
}
