package promotions

import (
	"context"
	"testing"
	"time"
)

type fakeRepo struct {
	used  int64
	limit int64
}

func (f *fakeRepo) Save(item Promotion) Promotion { return item }
func (f *fakeRepo) List(_ string) []Promotion     { return nil }
func (f *fakeRepo) SaveRule(item PromotionRule) PromotionRule {
	return item
}
func (f *fakeRepo) ListRules(_ string) []PromotionRule { return nil }
func (f *fakeRepo) SaveVoucher(item Voucher) Voucher {
	return item
}
func (f *fakeRepo) ListVouchers(_ string) []Voucher { return nil }
func (f *fakeRepo) GetPromotionByID(_, _, _ string) (Promotion, bool) {
	return Promotion{}, false
}
func (f *fakeRepo) TryConsumeVoucher(tenantID, regionID, code, currency string, at time.Time) (Voucher, bool) {
	if tenantID != "tenant_a" || regionID != "region_a" || code != "SAVE10" || currency != "USD" {
		return Voucher{}, false
	}
	if f.limit > 0 && f.used >= f.limit {
		return Voucher{}, false
	}
	f.used++
	return Voucher{Code: code, ValueCents: 100}, true
}

func TestResolveDiscountRespectsVoucherUsageLimit(t *testing.T) {
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
	if second != 0 {
		t.Fatalf("expected second discount to be rejected after usage limit, got %d", second)
	}
}
