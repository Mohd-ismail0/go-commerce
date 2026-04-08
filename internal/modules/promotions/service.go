package promotions

import (
	"context"
	"strings"
	"time"
)

type Service struct {
	repo serviceRepository
}

type serviceRepository interface {
	Save(item Promotion) Promotion
	List(tenantID string) []Promotion
	SaveRule(item PromotionRule) PromotionRule
	ListRules(tenantID string) []PromotionRule
	SaveVoucher(item Voucher) Voucher
	ListVouchers(tenantID string) []Voucher
	FindEligibleVoucher(tenantID, regionID, code, currency string, at time.Time) (Voucher, bool)
	ConsumeVoucherByID(voucherID string) bool
	GetPromotionByID(id, tenantID, regionID string) (Promotion, bool)
}

func NewService(repo serviceRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Save(_ context.Context, item Promotion) Promotion {
	return s.repo.Save(item)
}

func (s *Service) List(_ context.Context, tenantID string) []Promotion {
	return s.repo.List(tenantID)
}

func (s *Service) SaveRule(_ context.Context, item PromotionRule) PromotionRule {
	return s.repo.SaveRule(item)
}

func (s *Service) ListRules(_ context.Context, tenantID string) []PromotionRule {
	return s.repo.ListRules(tenantID)
}

func (s *Service) SaveVoucher(_ context.Context, item Voucher) Voucher {
	return s.repo.SaveVoucher(item)
}

func (s *Service) ListVouchers(_ context.Context, tenantID string) []Voucher {
	return s.repo.ListVouchers(tenantID)
}

func (s *Service) ResolveDiscount(_ context.Context, input EligibilityInput) int64 {
	if input.BaseAmount <= 0 {
		return 0
	}
	if strings.TrimSpace(input.VoucherCode) != "" {
		voucher, ok := s.resolveEligibleVoucher(input)
		if ok {
			return voucherDiscount(input.BaseAmount, voucher.DiscountType, voucher.ValueCents)
		}
	}
	if strings.TrimSpace(input.PromotionID) != "" {
		if promo, ok := s.repo.GetPromotionByID(input.PromotionID, input.TenantID, input.RegionID); ok {
			return s.resolvePromotionDiscount(input, promo)
		}
	}
	return 0
}

func (s *Service) ConsumeVoucher(_ context.Context, input EligibilityInput) bool {
	voucher, ok := s.resolveEligibleVoucher(input)
	if !ok {
		return false
	}
	return s.repo.ConsumeVoucherByID(voucher.ID)
}

func (s *Service) resolveEligibleVoucher(input EligibilityInput) (Voucher, bool) {
	at := time.Now().UTC()
	if parsed, err := parseRFC3339OrZero(input.ReferenceTime); err == nil && !parsed.IsZero() {
		at = parsed.UTC()
	}
	return s.repo.FindEligibleVoucher(input.TenantID, input.RegionID, input.VoucherCode, input.Currency, at)
}

func clampDiscount(base, discount int64) int64 {
	if discount < 0 {
		return 0
	}
	if discount > base {
		return base
	}
	return discount
}

func voucherDiscount(base int64, discountType string, value int64) int64 {
	switch strings.ToLower(strings.TrimSpace(discountType)) {
	case "percentage", "percent":
		return clampDiscount(base, (base*value)/100)
	default:
		return clampDiscount(base, value)
	}
}

func (s *Service) resolvePromotionDiscount(input EligibilityInput, promo Promotion) int64 {
	best := clampDiscount(input.BaseAmount, promo.ValueCents)
	rules := s.repo.ListRules(input.TenantID)
	for _, rule := range rules {
		if rule.PromotionID != promo.ID || !strings.EqualFold(strings.TrimSpace(rule.RegionID), strings.TrimSpace(input.RegionID)) {
			continue
		}
		if rule.Currency != "" && !strings.EqualFold(strings.TrimSpace(rule.Currency), strings.TrimSpace(input.Currency)) {
			continue
		}
		candidate := promotionRuleDiscount(input.BaseAmount, rule)
		if candidate > best {
			best = candidate
		}
	}
	return best
}

func promotionRuleDiscount(base int64, rule PromotionRule) int64 {
	switch strings.ToLower(strings.TrimSpace(rule.RuleType)) {
	case "percentage", "percent":
		return clampDiscount(base, (base*rule.ValueCents)/100)
	default:
		return clampDiscount(base, rule.ValueCents)
	}
}
