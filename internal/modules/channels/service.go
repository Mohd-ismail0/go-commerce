package channels

import (
	"context"
	"strings"

	sharederrors "rewrite/internal/shared/errors"
	"rewrite/internal/shared/utils"
)

type repo interface {
	SlugTaken(ctx context.Context, tenantID, regionID, slug, excludeID string) (bool, error)
	Save(ctx context.Context, ch Channel) (Channel, error)
	List(ctx context.Context, tenantID, regionID string) ([]Channel, error)
}

type Service struct {
	repo repo
}

func NewService(r repo) *Service {
	return &Service{repo: r}
}

func (s *Service) List(ctx context.Context, tenantID, regionID string) ([]Channel, error) {
	return s.repo.List(ctx, tenantID, regionID)
}

func (s *Service) Save(ctx context.Context, ch Channel) (Channel, error) {
	if strings.TrimSpace(ch.Slug) == "" || strings.TrimSpace(ch.Name) == "" {
		return Channel{}, sharederrors.BadRequest("slug and name are required")
	}
	ch.Slug = strings.TrimSpace(ch.Slug)
	ch.Name = strings.TrimSpace(ch.Name)
	ch.DefaultCurrency = strings.ToUpper(strings.TrimSpace(ch.DefaultCurrency))
	ch.DefaultCountry = strings.ToUpper(strings.TrimSpace(ch.DefaultCountry))
	if len(ch.DefaultCurrency) != 3 {
		return Channel{}, sharederrors.BadRequest("default_currency must be ISO 4217 (3 letters)")
	}
	if len(ch.DefaultCountry) != 2 {
		return Channel{}, sharederrors.BadRequest("default_country must be a 2-letter ISO code")
	}
	if ch.ID == "" {
		ch.ID = utils.NewID("chn")
	}
	taken, err := s.repo.SlugTaken(ctx, ch.TenantID, ch.RegionID, ch.Slug, ch.ID)
	if err != nil {
		return Channel{}, sharederrors.Internal("failed to validate channel slug")
	}
	if taken {
		return Channel{}, sharederrors.Conflict("channel slug already exists in tenant/region")
	}
	saved, err := s.repo.Save(ctx, ch)
	if err != nil {
		return Channel{}, sharederrors.Internal("failed to save channel")
	}
	return saved, nil
}
