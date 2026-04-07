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
	TryConsumeVoucher(tenantID, regionID, code, currency string, at time.Time) (Voucher, bool)
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
		at := time.Now().UTC()
		if parsed, err := parseRFC3339OrZero(input.ReferenceTime); err == nil && !parsed.IsZero() {
			at = parsed.UTC()
		}
		voucher, ok := s.repo.TryConsumeVoucher(input.TenantID, input.RegionID, input.VoucherCode, input.Currency, at)
		if ok {
			return clampDiscount(input.BaseAmount, voucher.ValueCents)
		}
	}
	if strings.TrimSpace(input.PromotionID) != "" {
		if promo, ok := s.repo.GetPromotionByID(input.PromotionID, input.TenantID, input.RegionID); ok {
			return clampDiscount(input.BaseAmount, promo.ValueCents)
		}
	}
	return 0
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
