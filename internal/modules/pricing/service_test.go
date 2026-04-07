package pricing

import (
	"context"
	"testing"
	"time"

	"rewrite/internal/modules/promotions"
)

type fakePricingRepo struct {
	rate int64
}

func (f *fakePricingRepo) Save(entry PriceBookEntry) PriceBookEntry { return entry }
func (f *fakePricingRepo) List(_ string) []PriceBookEntry           { return nil }
func (f *fakePricingRepo) SaveTaxClass(item TaxClass) TaxClass      { return item }
func (f *fakePricingRepo) ListTaxClasses(_ string) []TaxClass       { return nil }
func (f *fakePricingRepo) SaveTaxRate(item TaxRate) TaxRate         { return item }
func (f *fakePricingRepo) ListTaxRates(_ string) []TaxRate          { return nil }
func (f *fakePricingRepo) TaxRateBasisPoints(_, _, _, _ string) int64 {
	return f.rate
}

type fakePromoRepo struct{}

func (f *fakePromoRepo) Save(item promotions.Promotion) promotions.Promotion { return item }
func (f *fakePromoRepo) List(_ string) []promotions.Promotion                { return nil }
func (f *fakePromoRepo) SaveRule(item promotions.PromotionRule) promotions.PromotionRule {
	return item
}
func (f *fakePromoRepo) ListRules(_ string) []promotions.PromotionRule { return nil }
func (f *fakePromoRepo) SaveVoucher(item promotions.Voucher) promotions.Voucher {
	return item
}
func (f *fakePromoRepo) ListVouchers(_ string) []promotions.Voucher { return nil }
func (f *fakePromoRepo) TryConsumeVoucher(_, _, _, _ string, _ time.Time) (promotions.Voucher, bool) {
	return promotions.Voucher{}, false
}
func (f *fakePromoRepo) GetPromotionByID(id, tenantID, regionID string) (promotions.Promotion, bool) {
	if id == "promo_a" && tenantID == "tenant_a" && regionID == "region_a" {
		return promotions.Promotion{ID: id, TenantID: tenantID, RegionID: regionID, ValueCents: 200}, true
	}
	return promotions.Promotion{}, false
}

func TestCalculateDeterministicDiscountAndTax(t *testing.T) {
	promoSvc := promotions.NewService(&fakePromoRepo{})
	svc := NewService(&fakePricingRepo{rate: 750}, promoSvc)
	resultA := svc.Calculate(context.Background(), CalculationInput{
		TenantID:        "tenant_a",
		RegionID:        "region_a",
		Currency:        "USD",
		BaseAmountCents: 1000,
		PromotionID:     "promo_a",
		TaxClassID:      "tax_std",
		CountryCode:     "US",
	})
	resultB := svc.Calculate(context.Background(), CalculationInput{
		TenantID:        "tenant_a",
		RegionID:        "region_a",
		Currency:        "USD",
		BaseAmountCents: 1000,
		PromotionID:     "promo_a",
		TaxClassID:      "tax_std",
		CountryCode:     "US",
	})
	if resultA != resultB {
		t.Fatalf("expected deterministic result, got %#v and %#v", resultA, resultB)
	}
	if resultA.DiscountCents != 200 || resultA.TaxCents != 60 || resultA.TotalCents != 860 {
		t.Fatalf("unexpected calculation result: %#v", resultA)
	}
}
