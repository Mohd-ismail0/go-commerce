package apps

import (
	"context"
	"errors"
	"strings"

	sharederrors "rewrite/internal/shared/errors"
)

type Service struct {
	repo appRepo
}

type appRepo interface {
	Save(ctx context.Context, in App) (App, error)
	List(ctx context.Context, tenantID, regionID string, activeOnly bool) ([]App, error)
	GetByID(ctx context.Context, tenantID, regionID, id string) (App, bool, error)
}

func NewService(repo appRepo) *Service {
	return &Service{repo: repo}
}

func (s *Service) Save(ctx context.Context, in App, patch bool) (App, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.AuthToken = strings.TrimSpace(in.AuthToken)
	if patch {
		existing, ok, err := s.repo.GetByID(ctx, in.TenantID, in.RegionID, strings.TrimSpace(in.ID))
		if err != nil {
			return App{}, sharederrors.Internal("failed to load app")
		}
		if !ok {
			return App{}, sharederrors.NotFound("app not found")
		}
		merged := existing
		if in.Name != "" {
			merged.Name = in.Name
		}
		// Keep existing token unless an explicit new token is provided.
		if in.AuthToken != "" {
			merged.AuthToken = in.AuthToken
		}
		if in.IsActive != existing.IsActive {
			merged.IsActive = in.IsActive
		}
		saved, saveErr := s.repo.Save(ctx, merged)
		if saveErr != nil {
			if isUniqueViolation(saveErr) {
				return App{}, sharederrors.Conflict("app name already exists in tenant/region")
			}
			return App{}, sharederrors.Internal("failed to save app")
		}
		return saved, nil
	}
	if strings.TrimSpace(in.Name) == "" {
		return App{}, sharederrors.BadRequest("name is required")
	}
	saved, err := s.repo.Save(ctx, in)
	if err != nil {
		if isUniqueViolation(err) {
			return App{}, sharederrors.Conflict("app name already exists in tenant/region")
		}
		return App{}, sharederrors.Internal("failed to save app")
	}
	return saved, nil
}

func (s *Service) List(ctx context.Context, tenantID, regionID string, activeOnly bool) ([]App, error) {
	return s.repo.List(ctx, tenantID, regionID, activeOnly)
}

func (s *Service) SetActive(ctx context.Context, tenantID, regionID, appID string, active bool) (App, error) {
	existing, ok, err := s.repo.GetByID(ctx, tenantID, regionID, strings.TrimSpace(appID))
	if err != nil {
		return App{}, sharederrors.Internal("failed to load app")
	}
	if !ok {
		return App{}, sharederrors.NotFound("app not found")
	}
	existing.IsActive = active
	saved, saveErr := s.repo.Save(ctx, existing)
	if saveErr != nil {
		return App{}, sharederrors.Internal("failed to update app state")
	}
	return saved, nil
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return !errors.Is(err, context.DeadlineExceeded) && (strings.Contains(msg, "duplicate key") || strings.Contains(msg, "unique constraint") || strings.Contains(msg, "ux_apps_tenant_region_name_ci"))
}
