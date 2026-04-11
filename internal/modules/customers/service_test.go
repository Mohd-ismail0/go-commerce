package customers

import (
	"context"
	"errors"
	"testing"
)

type stubRepo struct {
	saveIn            Customer
	saveOut           Customer
	saveErr           error
	listInTenant      string
	listInRegion      string
	listOut           []Customer
	listErr           error
	emailTakenIn      string
	emailTakenExclude string
	emailTakenOut     bool
	emailTakenErr     error
	listAddressesOut  []Address
	listAddressesErr  error
	saveAddressIn     Address
	saveAddressOut    Address
	saveAddressErr    error
	deleteAddressErr  error
}

func (s *stubRepo) Save(_ context.Context, customer Customer) (Customer, error) {
	s.saveIn = customer
	if s.saveOut.ID == "" {
		s.saveOut = customer
	}
	return s.saveOut, s.saveErr
}

func (s *stubRepo) List(_ context.Context, tenantID, regionID string) ([]Customer, error) {
	s.listInTenant = tenantID
	s.listInRegion = regionID
	return s.listOut, s.listErr
}

func (s *stubRepo) EmailTaken(_ context.Context, _ string, _ string, email, excludeID string) (bool, error) {
	s.emailTakenIn = email
	s.emailTakenExclude = excludeID
	return s.emailTakenOut, s.emailTakenErr
}

func (s *stubRepo) ListAddresses(_ context.Context, _, _, _ string) ([]Address, error) {
	if s.listAddressesErr != nil {
		return nil, s.listAddressesErr
	}
	return s.listAddressesOut, nil
}

func (s *stubRepo) SaveAddress(_ context.Context, a Address) (Address, error) {
	s.saveAddressIn = a
	if s.saveAddressErr != nil {
		return Address{}, s.saveAddressErr
	}
	if s.saveAddressOut.ID != "" {
		return s.saveAddressOut, nil
	}
	return a, nil
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
	})
	if err == nil {
		t.Fatal("expected error for missing email")
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
	})
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
	repo := &stubRepo{emailTakenOut: true}
	svc := NewService(repo)
	_, err := svc.Save(context.Background(), Customer{
		TenantID: "t1",
		RegionID: "r1",
		ID:       "c1",
		Email:    "jane@example.com",
		Name:     "Jane",
	})
	if err == nil {
		t.Fatal("expected conflict error for duplicate email")
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

func TestCreateAddressNormalizesAndAssignsID(t *testing.T) {
	repo := &stubRepo{}
	svc := NewService(repo)
	out, err := svc.CreateAddress(context.Background(), "t", "r", "c1", Address{
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
