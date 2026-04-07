package pricing

import (
	"context"
	"strings"

	"rewrite/internal/modules/promotions"
)

type Service struct {
	repo     serviceRepository
	promoSvc *promotions.Service
}

type serviceRepository interface {
	Save(entry PriceBookEntry) PriceBookEntry
	List(tenantID string) []PriceBookEntry
	SaveTaxClass(item TaxClass) TaxClass
	ListTaxClasses(tenantID string) []TaxClass
	SaveTaxRate(item TaxRate) TaxRate
	ListTaxRates(tenantID string) []TaxRate
	TaxRateBasisPoints(tenantID, regionID, taxClassID, countryCode string) int64
}

func NewService(repo serviceRepository, promoSvc *promotions.Service) *Service {
	return &Service{repo: repo, promoSvc: promoSvc}
}

func (s *Service) Save(_ context.Context, entry PriceBookEntry) PriceBookEntry {
	return s.repo.Save(entry)
}

func (s *Service) List(_ context.Context, tenantID string) []PriceBookEntry {
	return s.repo.List(tenantID)
}

func (s *Service) SaveTaxClass(_ context.Context, item TaxClass) TaxClass {
	return s.repo.SaveTaxClass(item)
}

func (s *Service) ListTaxClasses(_ context.Context, tenantID string) []TaxClass {
	return s.repo.ListTaxClasses(tenantID)
}

func (s *Service) SaveTaxRate(_ context.Context, item TaxRate) TaxRate {
	return s.repo.SaveTaxRate(item)
}

func (s *Service) ListTaxRates(_ context.Context, tenantID string) []TaxRate {
	return s.repo.ListTaxRates(tenantID)
}

func (s *Service) Calculate(ctx context.Context, in CalculationInput) CalculationResult {
	base := in.BaseAmountCents
	if base < 0 {
		base = 0
	}
	discount := int64(0)
	if s.promoSvc != nil {
		discount = s.promoSvc.ResolveDiscount(ctx, promotions.EligibilityInput{
			TenantID:    in.TenantID,
			RegionID:    in.RegionID,
			Currency:    in.Currency,
			BaseAmount:  base,
			VoucherCode: strings.TrimSpace(in.VoucherCode),
			PromotionID: strings.TrimSpace(in.PromotionID),
		})
	}
	taxable := base - discount
	if taxable < 0 {
		taxable = 0
	}
	rateBasisPoints := s.repo.TaxRateBasisPoints(in.TenantID, in.RegionID, in.TaxClassID, strings.ToUpper(strings.TrimSpace(in.CountryCode)))
	tax := roundBasisPoints(taxable, rateBasisPoints)
	return CalculationResult{
		BaseAmountCents: base,
		DiscountCents:   discount,
		TaxCents:        tax,
		TotalCents:      taxable + tax,
	}
}

func roundBasisPoints(amount, basisPoints int64) int64 {
	if amount <= 0 || basisPoints <= 0 {
		return 0
	}
	return (amount*basisPoints + 5000) / 10000
}
