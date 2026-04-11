package customers

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type stubRepo struct {
	saveIn           Customer
	saveOut          Customer
	saveErr          error
	idem             map[string]Customer
	listInTenant     string
	listInRegion     string
	listOut          []Customer
	listErr          error
	listAddressesOut []Address
	listAddressesErr error
	saveAddressIn    Address
	saveAddressOut   Address
	saveAddressErr   error
	addressIdem      map[string]Address
	deleteAddressErr error
}

func (s *stubRepo) Save(_ context.Context, customer Customer, idempotencyKey string) (Customer, error) {
	if strings.TrimSpace(idempotencyKey) == "" {
		return Customer{}, ErrIdempotencyKeyRequired
	}
	k := customer.TenantID + "|" + customerSaveIdempotencyScope + "|" + strings.TrimSpace(idempotencyKey)
	if s.idem == nil {
		s.idem = make(map[string]Customer)
	}
	if prev, ok := s.idem[k]; ok {
		return prev, nil
	}
	if s.saveErr != nil {
		return Customer{}, s.saveErr
	}
	s.saveIn = customer
	if s.saveOut.ID == "" {
		s.saveOut = customer
	}
	s.idem[k] = s.saveOut
	return s.saveOut, nil
}

func (s *stubRepo) List(_ context.Context, tenantID, regionID string) ([]Customer, error) {
	s.listInTenant = tenantID
	s.listInRegion = regionID
	return s.listOut, s.listErr
}

func (s *stubRepo) ListAddresses(_ context.Context, _, _, _ string) ([]Address, error) {
	if s.listAddressesErr != nil {
		return nil, s.listAddressesErr
	}
	return s.listAddressesOut, nil
}

func (s *stubRepo) SaveAddress(_ context.Context, a Address, idempotencyKey string) (Address, error) {
	key := strings.TrimSpace(idempotencyKey)
	if key != "" {
		sk := a.TenantID + "|" + addressCreateIdempotencyScope(a.CustomerID) + "|" + key
		if s.addressIdem == nil {
			s.addressIdem = make(map[string]Address)
		}
		if prev, ok := s.addressIdem[sk]; ok {
			return prev, nil
		}
	}
	if s.saveAddressErr != nil {
		return Address{}, s.saveAddressErr
	}
	s.saveAddressIn = a
	var out Address
	if s.saveAddressOut.ID != "" {
		out = s.saveAddressOut
	} else {
		out = a
	}
	if key != "" {
		sk := a.TenantID + "|" + addressCreateIdempotencyScope(a.CustomerID) + "|" + key
		if s.addressIdem == nil {
			s.addressIdem = make(map[string]Address)
		}
		s.addressIdem[sk] = out
	}
	return out, nil
}

func (s *stubRepo) DeleteAddress(_ context.Context, _, _, _, _ string) error {
	return s.deleteAddressErr
}

func TestServiceSaveRequiresEmail(t *testing.T) {
	svc := NewService(&stubRepo{})
	_, err := svc.Save(context.Background(), Customer{
		TenantID: "t1",
		RegionID: "r1",
		Name:     "Jane",
	}, "idem-1")
	if err == nil {
		t.Fatal("expected error for missing email")
	}
}

func TestServiceSaveRequiresIdempotencyKey(t *testing.T) {
	svc := NewService(&stubRepo{})
	_, err := svc.Save(context.Background(), Customer{
		TenantID: "t1",
		RegionID: "r1",
		Email:    "a@b.co",
		Name:     "N",
	}, "  ")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestServiceSaveNormalizesEmailAndGeneratesID(t *testing.T) {
	repo := &stubRepo{}
	svc := NewService(repo)
	out, err := svc.Save(context.Background(), Customer{
		TenantID: "t1",
		RegionID: "r1",
		Email:    "  Jane@Example.COM ",
		Name:     " Jane Doe ",
	}, "idem-norm")
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}
	if out.ID == "" {
		t.Fatal("expected generated id")
	}
	if repo.saveIn.Email != "jane@example.com" {
		t.Fatalf("expected normalized email, got %q", repo.saveIn.Email)
	}
	if repo.saveIn.Name != "Jane Doe" {
		t.Fatalf("expected trimmed name, got %q", repo.saveIn.Name)
	}
}

func TestServiceSaveRejectsDuplicateEmail(t *testing.T) {
	repo := &stubRepo{saveErr: ErrCustomerEmailTaken}
	svc := NewService(repo)
	_, err := svc.Save(context.Background(), Customer{
		TenantID: "t1",
		RegionID: "r1",
		ID:       "c1",
		Email:    "jane@example.com",
		Name:     "Jane",
	}, "idem-dup")
	if err == nil {
		t.Fatal("expected conflict error for duplicate email")
	}
}

func TestServiceSaveIdempotentReplayReturnsFirstCustomer(t *testing.T) {
	repo := &stubRepo{}
	svc := NewService(repo)
	first, err := svc.Save(context.Background(), Customer{
		TenantID: "t1",
		RegionID: "r1",
		ID:       "cus_first",
		Email:    "a@b.co",
		Name:     "First",
	}, "same-key")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	second, err := svc.Save(context.Background(), Customer{
		TenantID: "t1",
		RegionID: "r1",
		ID:       "cus_second",
		Email:    "a@b.co",
		Name:     "Second",
	}, "same-key")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if first.ID != second.ID || first.Name != "First" {
		t.Fatalf("expected replay of first customer, got first=%+v second=%+v", first, second)
	}
}

func TestServiceListPassesTenantRegion(t *testing.T) {
	repo := &stubRepo{listOut: []Customer{{ID: "c1"}}}
	svc := NewService(repo)
	out, err := svc.List(context.Background(), "tenant-a", "region-eu")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected one item, got %d", len(out))
	}
	if repo.listInTenant != "tenant-a" || repo.listInRegion != "region-eu" {
		t.Fatalf("unexpected scope: tenant=%s region=%s", repo.listInTenant, repo.listInRegion)
	}
}

func TestServiceListReturnsInternalOnRepoFailure(t *testing.T) {
	repo := &stubRepo{listErr: errors.New("db down")}
	svc := NewService(repo)
	_, err := svc.List(context.Background(), "tenant-a", "region-eu")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListAddressesMapsCustomerNotFound(t *testing.T) {
	repo := &stubRepo{listAddressesErr: ErrCustomerNotFound}
	svc := NewService(repo)
	_, err := svc.ListAddresses(context.Background(), "t", "r", "c1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCreateAddressRequiresIdempotencyKey(t *testing.T) {
	svc := NewService(&stubRepo{})
	_, err := svc.CreateAddress(context.Background(), "t", "r", "c1", " ", Address{
		FirstName: "A", LastName: "B", StreetLine1: "1", City: "C", PostalCode: "1", CountryCode: "US",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCreateAddressIdempotentReplayReturnsFirstAddress(t *testing.T) {
	repo := &stubRepo{}
	svc := NewService(repo)
	first, err := svc.CreateAddress(context.Background(), "t", "r", "c1", "addr-key", Address{
		ID:          "adr_first",
		FirstName:   "A",
		LastName:    "B",
		StreetLine1: "1",
		City:        "C",
		PostalCode:  "1",
		CountryCode: "US",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	second, err := svc.CreateAddress(context.Background(), "t", "r", "c1", "addr-key", Address{
		ID:          "adr_second",
		FirstName:   "X",
		LastName:    "Y",
		StreetLine1: "2",
		City:        "D",
		PostalCode:  "2",
		CountryCode: "CA",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if first.ID != second.ID || first.FirstName != "A" {
		t.Fatalf("expected replay of first address, got first=%+v second=%+v", first, second)
	}
}

func TestCreateAddressNormalizesAndAssignsID(t *testing.T) {
	repo := &stubRepo{}
	svc := NewService(repo)
	out, err := svc.CreateAddress(context.Background(), "t", "r", "c1", "idem-adr", Address{
		FirstName:   "  A ",
		LastName:    "B",
		StreetLine1: "1 Main",
		City:        "NYC",
		PostalCode:  "10001",
		CountryCode: "us",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.ID == "" {
		t.Fatal("expected id")
	}
	if repo.saveAddressIn.CountryCode != "US" {
		t.Fatalf("country: %q", repo.saveAddressIn.CountryCode)
	}
	if repo.saveAddressIn.CustomerID != "c1" {
		t.Fatalf("customer id not scoped: %q", repo.saveAddressIn.CustomerID)
	}
}

func TestReplaceAddressMapsAddressNotFound(t *testing.T) {
	repo := &stubRepo{saveAddressErr: ErrAddressNotFound}
	svc := NewService(repo)
	_, err := svc.ReplaceAddress(context.Background(), "t", "r", "c1", "a1", Address{
		FirstName: "A", LastName: "B", StreetLine1: "1", City: "C", PostalCode: "1", CountryCode: "US",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDeleteAddressMapsNotFound(t *testing.T) {
	repo := &stubRepo{deleteAddressErr: ErrAddressNotFound}
	svc := NewService(repo)
	err := svc.DeleteAddress(context.Background(), "t", "r", "c1", "a1")
	if err == nil {
		t.Fatal("expected error")
	}
}
