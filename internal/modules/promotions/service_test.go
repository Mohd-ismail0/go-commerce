package promotions

import (
	"context"
	"testing"
	"time"
)

type fakeRepo struct {
	used  int64
	limit int64
	rules []PromotionRule
}

func (f *fakeRepo) Save(item Promotion) Promotion { return item }
func (f *fakeRepo) List(_ string) []Promotion     { return nil }
func (f *fakeRepo) SaveRule(item PromotionRule) PromotionRule {
	return item
}
func (f *fakeRepo) ListRules(_ string) []PromotionRule { return f.rules }
func (f *fakeRepo) SaveVoucher(item Voucher) Voucher {
	return item
}
func (f *fakeRepo) ListVouchers(_ string) []Voucher { return nil }
func (f *fakeRepo) GetPromotionByID(id, tenantID, regionID string) (Promotion, bool) {
	if id == "promo_1" && tenantID == "tenant_a" && regionID == "region_a" {
		return Promotion{ID: "promo_1", TenantID: tenantID, RegionID: regionID, ValueCents: 50}, true
	}
	return Promotion{}, false
}
func (f *fakeRepo) FindEligibleVoucher(tenantID, regionID, code, currency string, at time.Time) (Voucher, bool) {
	if tenantID != "tenant_a" || regionID != "region_a" || code != "SAVE10" || currency != "USD" {
		return Voucher{}, false
	}
	if f.limit > 0 && f.used >= f.limit {
		return Voucher{}, false
	}
	return Voucher{ID: "v_1", Code: code, DiscountType: "fixed", ValueCents: 100}, true
}
func (f *fakeRepo) ConsumeVoucherByID(voucherID string) bool {
	if voucherID == "" {
		return false
	}
	if f.limit > 0 && f.used >= f.limit {
		return false
	}
	f.used++
	return true
}

func TestResolveDiscountDoesNotConsumeVoucher(t *testing.T) {
	svc := NewService(&fakeRepo{limit: 1})
	input := EligibilityInput{
		TenantID:    "tenant_a",
		RegionID:    "region_a",
		Currency:    "USD",
		BaseAmount:  500,
		VoucherCode: "SAVE10",
	}
	first := svc.ResolveDiscount(context.Background(), input)
	second := svc.ResolveDiscount(context.Background(), input)
	if first != 100 {
		t.Fatalf("expected first discount to apply, got %d", first)
	}
	if second != 100 {
		t.Fatalf("expected second discount preview to stay deterministic, got %d", second)
	}
}

func TestResolveDiscountUsesBestPromotionRule(t *testing.T) {
	svc := NewService(&fakeRepo{
		rules: []PromotionRule{
			{ID: "r1", TenantID: "tenant_a", RegionID: "region_a", PromotionID: "promo_1", RuleType: "fixed", ValueCents: 120, Currency: "USD"},
			{ID: "r2", TenantID: "tenant_a", RegionID: "region_a", PromotionID: "promo_1", RuleType: "percentage", ValueCents: 20, Currency: "USD"},
		},
	})
	discount := svc.ResolveDiscount(context.Background(), EligibilityInput{
		TenantID:    "tenant_a",
		RegionID:    "region_a",
		Currency:    "USD",
		BaseAmount:  1000,
		PromotionID: "promo_1",
	})
	if discount != 200 {
		t.Fatalf("expected best rule discount 200, got %d", discount)
	}
}

func TestConsumeVoucherRespectsUsageLimit(t *testing.T) {
	svc := NewService(&fakeRepo{limit: 1})
	input := EligibilityInput{
		TenantID:    "tenant_a",
		RegionID:    "region_a",
		Currency:    "USD",
		BaseAmount:  500,
		VoucherCode: "SAVE10",
	}
	if ok := svc.ConsumeVoucher(context.Background(), input); !ok {
		t.Fatalf("expected first consume to succeed")
	}
	if ok := svc.ConsumeVoucher(context.Background(), input); ok {
		t.Fatalf("expected second consume to fail after limit")
	}
}
