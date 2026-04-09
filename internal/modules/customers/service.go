package customers

import (
	"context"
	"strings"

	sharederrors "rewrite/internal/shared/errors"
	"rewrite/internal/shared/utils"
)

type serviceRepository interface {
	Save(ctx context.Context, customer Customer) (Customer, error)
	List(ctx context.Context, tenantID, regionID string) ([]Customer, error)
	EmailTaken(ctx context.Context, tenantID, regionID, email, excludeID string) (bool, error)
}

type Service struct {
	repo serviceRepository
}

func NewService(repo serviceRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Save(ctx context.Context, customer Customer) (Customer, error) {
	customer.Email = strings.ToLower(strings.TrimSpace(customer.Email))
	customer.Name = strings.TrimSpace(customer.Name)
	if customer.Email == "" {
		return Customer{}, sharederrors.BadRequest("email is required")
	}
	if !strings.Contains(customer.Email, "@") {
		return Customer{}, sharederrors.BadRequest("email must be valid")
	}
	if customer.Name == "" {
		return Customer{}, sharederrors.BadRequest("name is required")
	}
	customer.ID = strings.TrimSpace(customer.ID)
	if customer.ID == "" {
		customer.ID = utils.NewID("cus")
	}
	taken, err := s.repo.EmailTaken(ctx, customer.TenantID, customer.RegionID, customer.Email, customer.ID)
	if err != nil {
		return Customer{}, sharederrors.Internal("failed to validate customer email")
	}
	if taken {
		return Customer{}, sharederrors.Conflict("customer email already exists in tenant/region")
	}
	saved, err := s.repo.Save(ctx, customer)
	if err != nil {
		return Customer{}, sharederrors.Internal("failed to save customer")
	}
	return saved, nil
}

func (s *Service) List(ctx context.Context, tenantID, regionID string) ([]Customer, error) {
	items, err := s.repo.List(ctx, tenantID, regionID)
	if err != nil {
		return nil, sharederrors.Internal("failed to list customers")
	}
	return items, nil
}
