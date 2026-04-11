package shipping

import (
	"context"
	"strings"

	sharederrors "rewrite/internal/shared/errors"
	"rewrite/internal/shared/utils"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) SaveZone(ctx context.Context, z ShippingZone) (ShippingZone, error) {
	if strings.TrimSpace(z.Name) == "" {
		return ShippingZone{}, sharederrors.BadRequest("zone name is required")
	}
	if z.ID == "" {
		z.ID = utils.NewID("shz")
	}
	saved, err := s.repo.SaveZone(ctx, z)
	if err != nil {
		return ShippingZone{}, sharederrors.Internal("failed to save shipping zone")
	}
	return saved, nil
}

func (s *Service) ListZones(ctx context.Context, tenantID string) ([]ShippingZone, error) {
	return s.repo.ListZones(ctx, tenantID)
}

func (s *Service) SaveMethod(ctx context.Context, m ShippingMethod) (ShippingMethod, error) {
	if strings.TrimSpace(m.Name) == "" || m.PriceCents < 0 || len(strings.TrimSpace(m.Currency)) != 3 {
		return ShippingMethod{}, sharederrors.BadRequest("invalid shipping method payload")
	}
	if m.DeliveryDaysMin != nil && m.DeliveryDaysMax != nil && *m.DeliveryDaysMin > *m.DeliveryDaysMax {
		return ShippingMethod{}, sharederrors.BadRequest("delivery_days_min cannot exceed delivery_days_max")
	}
	if m.WeightSurchargePerKgCents != nil && *m.WeightSurchargePerKgCents < 0 {
		return ShippingMethod{}, sharederrors.BadRequest("weight_surcharge_per_kg_cents must be non-negative")
	}
	if m.ID == "" {
		m.ID = utils.NewID("shm")
	}
	saved, err := s.repo.SaveMethod(ctx, m)
	if err != nil {
		return ShippingMethod{}, sharederrors.Internal("failed to save shipping method")
	}
	return saved, nil
}

func (s *Service) ListMethods(ctx context.Context, tenantID string) ([]ShippingMethod, error) {
	return s.repo.ListMethods(ctx, tenantID)
}

func (s *Service) ResolveEligible(ctx context.Context, tenantID, regionID string, in ResolveInput) ([]ShippingMethod, error) {
	if len(strings.TrimSpace(in.CountryCode)) != 2 {
		return nil, sharederrors.BadRequest("country_code must be a 2-letter ISO code")
	}
	if in.OrderTotalCents < 0 || len(strings.TrimSpace(in.Currency)) != 3 {
		return nil, sharederrors.BadRequest("invalid order totals")
	}
	zones, err := s.repo.ListZones(ctx, tenantID)
	if err != nil {
		return nil, sharederrors.Internal("failed to list zones")
	}
	methods, err := s.repo.ListMethodsForTenantRegion(ctx, tenantID, regionID)
	if err != nil {
		return nil, sharederrors.Internal("failed to list methods")
	}
	zoneByID := map[string]ShippingZone{}
	for _, z := range zones {
		zoneByID[z.ID] = z
	}
	out := []ShippingMethod{}
	for _, m := range methods {
		if strings.TrimSpace(m.Currency) != strings.TrimSpace(in.Currency) {
			continue
		}
		z, ok := zoneByID[m.ShippingZoneID]
		if !ok {
			continue
		}
		if !ZoneMatchesCountry(z, in.CountryCode) {
			continue
		}
		if !MethodMatchesChannel(m, in.ChannelID) {
			continue
		}
		if !MethodMatchesPostal(m, in.PostalCode) {
			continue
		}
		if !MethodMatchesOrderTotal(m, in.OrderTotalCents) {
			continue
		}
		cp := m
		if m.WeightSurchargePerKgCents != nil && *m.WeightSurchargePerKgCents != 0 &&
			in.TotalWeightGrams != nil && *in.TotalWeightGrams > 0 {
			q := m.PriceCents + (*in.TotalWeightGrams**m.WeightSurchargePerKgCents)/1000
			cp.QuotedPriceCents = &q
		}
		out = append(out, cp)
	}
	return out, nil
}
