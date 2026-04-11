package customers

import (
	"context"
	"errors"
	"strings"
	"unicode"

	sharederrors "rewrite/internal/shared/errors"
	"rewrite/internal/shared/utils"
)

type serviceRepository interface {
	Save(ctx context.Context, customer Customer, idempotencyKey string) (Customer, error)
	List(ctx context.Context, tenantID, regionID string) ([]Customer, error)
	ListAddresses(ctx context.Context, tenantID, regionID, customerID string) ([]Address, error)
	SaveAddress(ctx context.Context, a Address) (Address, error)
	DeleteAddress(ctx context.Context, tenantID, regionID, customerID, addressID string) error
}

type Service struct {
	repo serviceRepository
}

func NewService(repo serviceRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Save(ctx context.Context, customer Customer, idempotencyKey string) (Customer, error) {
	if strings.TrimSpace(idempotencyKey) == "" {
		return Customer{}, sharederrors.BadRequest("Idempotency-Key is required")
	}
	key := strings.TrimSpace(idempotencyKey)
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
	saved, err := s.repo.Save(ctx, customer, key)
	if err != nil {
		if errors.Is(err, ErrIdempotencyKeyRequired) {
			return Customer{}, sharederrors.BadRequest("Idempotency-Key is required")
		}
		if errors.Is(err, ErrCustomerIdempotencyOrphan) {
			return Customer{}, sharederrors.Internal("customer idempotency record is inconsistent")
		}
		if errors.Is(err, ErrCustomerEmailTaken) {
			return Customer{}, sharederrors.Conflict("customer email already exists in tenant/region")
		}
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

func (s *Service) ListAddresses(ctx context.Context, tenantID, regionID, customerID string) ([]Address, error) {
	if strings.TrimSpace(customerID) == "" {
		return nil, sharederrors.BadRequest("customer_id is required")
	}
	items, err := s.repo.ListAddresses(ctx, tenantID, regionID, customerID)
	if err != nil {
		if errors.Is(err, ErrCustomerNotFound) {
			return nil, sharederrors.NotFound(err.Error())
		}
		return nil, sharederrors.Internal("failed to list customer addresses")
	}
	return items, nil
}

// CreateAddress persists a new address. tenant_id and region_id on the input are ignored and taken from arguments.
func (s *Service) CreateAddress(ctx context.Context, tenantID, regionID, customerID string, in Address) (Address, error) {
	if strings.TrimSpace(customerID) == "" {
		return Address{}, sharederrors.BadRequest("customer_id is required")
	}
	in.ID = strings.TrimSpace(in.ID)
	if in.ID == "" {
		in.ID = utils.NewID("adr")
	}
	in.TenantID = tenantID
	in.RegionID = regionID
	in.CustomerID = customerID
	if err := normalizeAndValidateAddress(&in); err != nil {
		return Address{}, err
	}
	saved, err := s.repo.SaveAddress(ctx, in)
	if err != nil {
		if errors.Is(err, ErrCustomerNotFound) {
			return Address{}, sharederrors.NotFound(err.Error())
		}
		return Address{}, sharederrors.Internal("failed to save customer address")
	}
	return saved, nil
}

// ReplaceAddress upserts an address with a fixed id from the path (TOCTOU-safe default flags under customer lock).
func (s *Service) ReplaceAddress(ctx context.Context, tenantID, regionID, customerID, addressID string, in Address) (Address, error) {
	if strings.TrimSpace(customerID) == "" || strings.TrimSpace(addressID) == "" {
		return Address{}, sharederrors.BadRequest("customer_id and address_id are required")
	}
	in.ID = addressID
	in.TenantID = tenantID
	in.RegionID = regionID
	in.CustomerID = customerID
	if err := normalizeAndValidateAddress(&in); err != nil {
		return Address{}, err
	}
	saved, err := s.repo.SaveAddress(ctx, in)
	if err != nil {
		if errors.Is(err, ErrCustomerNotFound) {
			return Address{}, sharederrors.NotFound(err.Error())
		}
		if errors.Is(err, ErrAddressNotFound) {
			return Address{}, sharederrors.NotFound(err.Error())
		}
		return Address{}, sharederrors.Internal("failed to save customer address")
	}
	return saved, nil
}

func (s *Service) DeleteAddress(ctx context.Context, tenantID, regionID, customerID, addressID string) error {
	if strings.TrimSpace(customerID) == "" || strings.TrimSpace(addressID) == "" {
		return sharederrors.BadRequest("customer_id and address_id are required")
	}
	err := s.repo.DeleteAddress(ctx, tenantID, regionID, customerID, addressID)
	if err != nil {
		if errors.Is(err, ErrCustomerNotFound) {
			return sharederrors.NotFound(err.Error())
		}
		if errors.Is(err, ErrAddressNotFound) {
			return sharederrors.NotFound(err.Error())
		}
		return sharederrors.Internal("failed to delete customer address")
	}
	return nil
}

func normalizeAndValidateAddress(a *Address) error {
	a.FirstName = strings.TrimSpace(a.FirstName)
	a.LastName = strings.TrimSpace(a.LastName)
	a.Company = strings.TrimSpace(a.Company)
	a.StreetLine1 = strings.TrimSpace(a.StreetLine1)
	a.StreetLine2 = strings.TrimSpace(a.StreetLine2)
	a.City = strings.TrimSpace(a.City)
	a.PostalCode = strings.TrimSpace(a.PostalCode)
	a.CountryCode = strings.ToUpper(strings.TrimSpace(a.CountryCode))
	a.Phone = strings.TrimSpace(a.Phone)
	if a.FirstName == "" {
		return sharederrors.BadRequest("first_name is required")
	}
	if a.LastName == "" {
		return sharederrors.BadRequest("last_name is required")
	}
	if a.StreetLine1 == "" {
		return sharederrors.BadRequest("street_line_1 is required")
	}
	if a.City == "" {
		return sharederrors.BadRequest("city is required")
	}
	if a.PostalCode == "" {
		return sharederrors.BadRequest("postal_code is required")
	}
	if len(a.CountryCode) != 2 {
		return sharederrors.BadRequest("country_code must be a 2-letter ISO code")
	}
	for _, r := range a.CountryCode {
		if !unicode.IsLetter(r) {
			return sharederrors.BadRequest("country_code must be alphabetic")
		}
	}
	return nil
}
