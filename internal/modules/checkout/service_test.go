package checkout

import (
	"context"
	"strings"
	"testing"
	"time"

	"rewrite/internal/modules/pricing"
	sharederrors "rewrite/internal/shared/errors"
	"rewrite/internal/shared/events"
)

type fakeRepo struct {
	completed         bool
	session           Session
	completeErr       error
	lines             []Line
	paymentCovered    bool
	upsertErr         error
	updateCtxErr      error
	updateShippingErr error
	updatePricingErr  error
}

func (f *fakeRepo) CreateSession(_ context.Context, in Session) (Session, error) {
	return in, nil
}

func (f *fakeRepo) UpsertLine(_ context.Context, _, _ string, line Line) (Line, error) {
	if f.upsertErr != nil {
		return Line{}, f.upsertErr
	}
	return line, nil
}

func (f *fakeRepo) GetSession(_ context.Context, _, _, checkoutID string) (Session, error) {
	if f.session.ID == "" {
		f.session = Session{ID: checkoutID, Status: "open", Currency: "USD"}
	}
	if f.session.ID == "" {
		f.session.ID = checkoutID
	}
	if f.session.Status == "" {
		f.session.Status = "open"
	}
	return f.session, nil
}

func (f *fakeRepo) ListLines(_ context.Context, _, _, checkoutID string) ([]Line, error) {
	if len(f.lines) == 0 {
		return nil, nil
	}
	out := make([]Line, 0, len(f.lines))
	for _, l := range f.lines {
		if l.CheckoutID == "" || l.CheckoutID == checkoutID {
			out = append(out, l)
		}
	}
	return out, nil
}

func (f *fakeRepo) ChannelIsActive(_ context.Context, _, _, channelID string) (bool, error) {
	return channelID == "web" || channelID == "pos" || channelID == "", nil
}

func (f *fakeRepo) GetProductChannelListing(_ context.Context, _, _, channelID, productID string) (bool, bool, error) {
	if channelID == "web" && productID == "prd_ok" {
		return true, true, nil
	}
	if channelID == "web" && productID == "prd_hidden" {
		return false, true, nil
	}
	return false, false, nil
}

func (f *fakeRepo) GetVariantChannelListing(_ context.Context, _, _, channelID, variantID string) (int64, string, bool, bool, error) {
	if channelID == "web" && variantID == "var_ok" {
		return 1200, "USD", true, true, nil
	}
	if channelID == "web" && variantID == "var_unpublished" {
		return 1200, "USD", false, true, nil
	}
	return 0, "", false, false, nil
}

func (f *fakeRepo) GetVariantProductID(_ context.Context, _, _, variantID string) (string, bool, error) {
	if variantID == "var_ok" || variantID == "var_unpublished" {
		return "prd_ok", true, nil
	}
	if variantID == "var_other_product" {
		return "prd_other", true, nil
	}
	return "", false, nil
}

func (f *fakeRepo) UpdateSessionContext(_ context.Context, _, _, checkoutID string, in Session) (Session, error) {
	if f.updateCtxErr != nil {
		return Session{}, f.updateCtxErr
	}
	in.ID = checkoutID
	in.Currency = "USD"
	in.SubtotalCents = 1000
	in.ShippingCents = 200
	return in, nil
}

func (f *fakeRepo) Recalculate(ctx context.Context, _, _, checkoutID string, opts *RecalculateOptions) (Session, error) {
	s := f.session
	if s.ID == "" {
		s = Session{ID: checkoutID, Status: "open", Currency: "USD", SubtotalCents: 1000, ShippingCents: 200, TotalCents: 1200}
	}
	s.ID = checkoutID
	if s.Status == "" {
		s.Status = "open"
	}
	if s.Currency == "" {
		s.Currency = "USD"
	}
	if s.SubtotalCents == 0 {
		s.SubtotalCents = 1000
	}
	if s.Status != "open" {
		return Session{}, ErrSessionNotOpen
	}
	if opts == nil || opts.ComputePricing == nil {
		f.session = s
		return s, nil
	}
	session := s
	var shippingCents int64
	if strings.TrimSpace(session.ShippingMethodID) != "" {
		if strings.TrimSpace(session.ShippingAddressCountry) == "" {
			return Session{}, ErrShippingAddressCountryRequired
		}
		price, ok, _ := f.ResolveShippingMethodPrice(ctx, "", "", session.ShippingMethodID, session.ShippingAddressCountry, session.ChannelID, session.ShippingAddressPostalCode, session.Currency, session.SubtotalCents)
		if !ok {
			return Session{}, ErrShippingMethodNotEligible
		}
		if f.updateShippingErr != nil {
			return Session{}, f.updateShippingErr
		}
		shippingCents = price
	} else if session.ShippingCents != 0 {
		if f.updateShippingErr != nil {
			return Session{}, f.updateShippingErr
		}
		shippingCents = 0
	}
	session.ShippingCents = shippingCents
	baseAmount := session.SubtotalCents + session.ShippingCents
	taxCents, totalCents, pErr := opts.ComputePricing(ctx, session, baseAmount)
	if pErr != nil {
		return Session{}, pErr
	}
	if f.updatePricingErr != nil {
		return Session{}, f.updatePricingErr
	}
	session.TaxCents = taxCents
	session.TotalCents = totalCents
	f.session = session
	return session, nil
}

func (f *fakeRepo) UpdatePricing(_ context.Context, _, _, checkoutID string, taxCents, totalCents int64) (Session, error) {
	if f.updatePricingErr != nil {
		return Session{}, f.updatePricingErr
	}
	shippingCents := f.session.ShippingCents
	if shippingCents == 0 {
		shippingCents = 200
	}
	return Session{ID: checkoutID, Status: "open", Currency: "USD", SubtotalCents: 1000, ShippingCents: shippingCents, TaxCents: taxCents, TotalCents: totalCents}, nil
}

func (f *fakeRepo) ResolveShippingMethodPrice(_ context.Context, _, _, shippingMethodID, countryCode, channelID, postalCode, currency string, subtotalCents int64) (int64, bool, error) {
	if shippingMethodID == "" {
		return 0, false, nil
	}
	if shippingMethodID == "ship_std" && countryCode == "US" && currency == "USD" {
		return 250, true, nil
	}
	return 0, false, nil
}

func (f *fakeRepo) UpdateShippingCents(_ context.Context, _, _, checkoutID string, shippingCents int64) (Session, error) {
	if f.updateShippingErr != nil {
		return Session{}, f.updateShippingErr
	}
	f.session.ShippingCents = shippingCents
	f.session.ID = checkoutID
	if f.session.Status == "" {
		f.session.Status = "open"
	}
	if f.session.Currency == "" {
		f.session.Currency = "USD"
	}
	return f.session, nil
}

func (f *fakeRepo) HasAuthorizedPaymentCoverage(_ context.Context, _, _, _ string, _ int64) (bool, error) {
	return f.paymentCovered, nil
}

func (f *fakeRepo) Complete(_ context.Context, tenantID, regionID, checkoutID, orderID string) (OrderCreatedPayload, error) {
	if f.completeErr != nil {
		return OrderCreatedPayload{}, f.completeErr
	}
	f.completed = true
	return OrderCreatedPayload{
		ID:         orderID,
		TenantID:   tenantID,
		RegionID:   regionID,
		CheckoutID: checkoutID,
		CustomerID: "cus_1",
		Status:     "created",
		TotalCents: 1200,
		Currency:   "USD",
	}, nil
}

func TestCompleteMapsChannelListingMismatchToConflict(t *testing.T) {
	repo := &fakeRepo{completeErr: ErrChannelListingMismatch, paymentCovered: true}
	svc := NewService(repo, events.NewBus(), &fakeCalculator{})
	_, err := svc.Complete(context.Background(), "tenant_a", "us", "chk_1")
	if err == nil {
		t.Fatalf("expected error")
	}
	apiErr, ok := err.(sharederrors.APIError)
	if !ok || apiErr.Status != 409 {
		t.Fatalf("expected 409 API error, got %#v", err)
	}
}

type fakeCalculator struct{}

func (f *fakeCalculator) Calculate(_ context.Context, in pricing.CalculationInput) pricing.CalculationResult {
	return pricing.CalculationResult{
		BaseAmountCents: in.BaseAmountCents,
		DiscountCents:   100,
		TaxCents:        50,
		TotalCents:      in.BaseAmountCents - 100 + 50,
	}
}

func TestCompletePublishesOrderCreatedAndReturnsOrderID(t *testing.T) {
	repo := &fakeRepo{paymentCovered: true}
	bus := events.NewBus()
	done := make(chan struct{}, 1)
	bus.Subscribe(events.EventOrderCreated, func(_ context.Context, payload any) {
		evt, ok := payload.(OrderCreatedPayload)
		if !ok {
			t.Fatalf("unexpected payload type")
		}
		if evt.CheckoutID != "chk_1" {
			t.Fatalf("expected checkout id chk_1, got %s", evt.CheckoutID)
		}
		done <- struct{}{}
	})
	svc := NewService(repo, bus, &fakeCalculator{})
	result, err := svc.Complete(context.Background(), "tenant_a", "us", "chk_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.OrderID == "" {
		t.Fatalf("expected order id")
	}
	if !repo.completed {
		t.Fatalf("expected repository complete to be called")
	}
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("expected order.created event to be published")
	}
}

func TestCompleteRequiresAuthorizedPaymentCoverage(t *testing.T) {
	repo := &fakeRepo{
		paymentCovered: false,
		session: Session{
			ID:                     "chk_1",
			Status:                 "open",
			Currency:               "USD",
			ShippingMethodID:       "ship_std",
			ShippingAddressCountry: "US",
		},
		lines: []Line{
			{ID: "ln_1", CheckoutID: "chk_1", ProductID: "prd_ok", Quantity: 1, UnitPriceCents: 1000, Currency: "USD"},
		},
	}
	svc := NewService(repo, events.NewBus(), &fakeCalculator{})
	_, err := svc.Complete(context.Background(), "tenant_a", "us", "chk_1")
	if err == nil {
		t.Fatalf("expected error")
	}
	apiErr, ok := err.(sharederrors.APIError)
	if !ok || apiErr.Status != 409 {
		t.Fatalf("expected 409 API error, got %#v", err)
	}
}

func TestCompleteRequiresShippingContextWhenCheckoutHasLines(t *testing.T) {
	repo := &fakeRepo{
		paymentCovered: true,
		session: Session{
			ID:       "chk_1",
			Status:   "open",
			Currency: "USD",
		},
		lines: []Line{
			{ID: "ln_1", CheckoutID: "chk_1", ProductID: "prd_ok", Quantity: 1, UnitPriceCents: 1000, Currency: "USD"},
		},
	}
	svc := NewService(repo, events.NewBus(), &fakeCalculator{})
	_, err := svc.Complete(context.Background(), "tenant_a", "us", "chk_1")
	if err == nil {
		t.Fatalf("expected error")
	}
	apiErr, ok := err.(sharederrors.APIError)
	if !ok || apiErr.Status != 409 {
		t.Fatalf("expected 409 API error, got %#v", err)
	}
}

func TestRecalculateAppliesEligibleShippingMethod(t *testing.T) {
	repo := &fakeRepo{
		session: Session{
			ID:                     "chk_1",
			Status:                 "open",
			Currency:               "USD",
			ShippingMethodID:       "ship_std",
			ShippingAddressCountry: "US",
		},
	}
	svc := NewService(repo, events.NewBus(), &fakeCalculator{})
	updated, err := svc.Recalculate(context.Background(), "tenant_a", "us", "chk_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.ShippingCents != 250 {
		t.Fatalf("expected shipping cents to be 250, got %d", updated.ShippingCents)
	}
}

func TestRecalculateMapsUpdateShippingSessionNotOpenToConflict(t *testing.T) {
	repo := &fakeRepo{
		updateShippingErr: ErrSessionNotOpen,
		session: Session{
			ID:                     "chk_1",
			Status:                 "open",
			Currency:               "USD",
			ShippingMethodID:       "ship_std",
			ShippingAddressCountry: "US",
		},
	}
	svc := NewService(repo, events.NewBus(), &fakeCalculator{})
	_, err := svc.Recalculate(context.Background(), "tenant_a", "us", "chk_1")
	if err == nil {
		t.Fatalf("expected error")
	}
	apiErr, ok := err.(sharederrors.APIError)
	if !ok || apiErr.Status != 409 {
		t.Fatalf("expected 409 API error, got %#v", err)
	}
}

func TestRecalculateMapsUpdatePricingSessionNotOpenToConflict(t *testing.T) {
	repo := &fakeRepo{
		updatePricingErr: ErrSessionNotOpen,
		session: Session{
			ID:       "chk_1",
			Status:   "open",
			Currency: "USD",
		},
	}
	svc := NewService(repo, events.NewBus(), &fakeCalculator{})
	_, err := svc.Recalculate(context.Background(), "tenant_a", "us", "chk_1")
	if err == nil {
		t.Fatalf("expected error")
	}
	apiErr, ok := err.(sharederrors.APIError)
	if !ok || apiErr.Status != 409 {
		t.Fatalf("expected 409 API error, got %#v", err)
	}
}

func TestUpdateSessionContextRecalculatesTotals(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo, events.NewBus(), &fakeCalculator{})
	updated, err := svc.UpdateSessionContext(context.Background(), "tenant_a", "us", "chk_1", Session{
		VoucherCode: "SAVE10",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.TaxCents == 0 || updated.TotalCents == 0 {
		t.Fatalf("expected recalculated totals, got %+v", updated)
	}
}

func TestUpsertLineRejectsUnpublishedVariantForSessionChannel(t *testing.T) {
	repo := &fakeRepo{session: Session{ID: "chk_1", ChannelID: "web", Currency: "USD"}}
	svc := NewService(repo, events.NewBus(), &fakeCalculator{})
	_, err := svc.UpsertLine(context.Background(), "tenant_a", "us", Line{
		ID:             "ln_1",
		CheckoutID:     "chk_1",
		VariantID:      "var_unpublished",
		Quantity:       1,
		UnitPriceCents: 1200,
		Currency:       "USD",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpsertLineRejectsPriceMismatchWithChannelListing(t *testing.T) {
	repo := &fakeRepo{session: Session{ID: "chk_1", ChannelID: "web", Currency: "USD"}}
	svc := NewService(repo, events.NewBus(), &fakeCalculator{})
	_, err := svc.UpsertLine(context.Background(), "tenant_a", "us", Line{
		ID:             "ln_1",
		CheckoutID:     "chk_1",
		VariantID:      "var_ok",
		Quantity:       1,
		UnitPriceCents: 999,
		Currency:       "USD",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreateSessionRejectsInactiveChannel(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo, events.NewBus(), &fakeCalculator{})
	_, err := svc.CreateSession(context.Background(), Session{
		ID:         "chk_1",
		TenantID:   "tenant_a",
		RegionID:   "us",
		CustomerID: "cus_1",
		ChannelID:  "inactive",
		Currency:   "USD",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpsertLineRejectsUnpublishedProductForSessionChannel(t *testing.T) {
	repo := &fakeRepo{session: Session{ID: "chk_1", ChannelID: "web", Currency: "USD"}}
	svc := NewService(repo, events.NewBus(), &fakeCalculator{})
	_, err := svc.UpsertLine(context.Background(), "tenant_a", "us", Line{
		ID:             "ln_1",
		CheckoutID:     "chk_1",
		ProductID:      "prd_hidden",
		Quantity:       1,
		UnitPriceCents: 1200,
		Currency:       "USD",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpsertLineRejectsSessionCurrencyMismatch(t *testing.T) {
	repo := &fakeRepo{session: Session{ID: "chk_1", ChannelID: "web", Currency: "USD"}}
	svc := NewService(repo, events.NewBus(), &fakeCalculator{})
	_, err := svc.UpsertLine(context.Background(), "tenant_a", "us", Line{
		ID:             "ln_1",
		CheckoutID:     "chk_1",
		ProductID:      "prd_ok",
		Quantity:       1,
		UnitPriceCents: 1200,
		Currency:       "EUR",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpsertLineRejectsVariantProductMismatch(t *testing.T) {
	repo := &fakeRepo{session: Session{ID: "chk_1", ChannelID: "web", Currency: "USD"}}
	svc := NewService(repo, events.NewBus(), &fakeCalculator{})
	_, err := svc.UpsertLine(context.Background(), "tenant_a", "us", Line{
		ID:             "ln_1",
		CheckoutID:     "chk_1",
		ProductID:      "prd_ok",
		VariantID:      "var_other_product",
		Quantity:       1,
		UnitPriceCents: 1200,
		Currency:       "USD",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpsertLineAutoFillsProductFromVariant(t *testing.T) {
	repo := &fakeRepo{session: Session{ID: "chk_1", ChannelID: "web", Currency: "USD"}}
	svc := NewService(repo, events.NewBus(), &fakeCalculator{})
	line, err := svc.UpsertLine(context.Background(), "tenant_a", "us", Line{
		ID:             "ln_1",
		CheckoutID:     "chk_1",
		VariantID:      "var_ok",
		Quantity:       1,
		UnitPriceCents: 1200,
		Currency:       "USD",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if line.ProductID != "prd_ok" {
		t.Fatalf("expected product_id derived from variant, got %q", line.ProductID)
	}
}

func TestUpsertLineRejectsCompletedSession(t *testing.T) {
	repo := &fakeRepo{session: Session{ID: "chk_1", Status: "completed", ChannelID: "web", Currency: "USD"}}
	svc := NewService(repo, events.NewBus(), &fakeCalculator{})
	_, err := svc.UpsertLine(context.Background(), "tenant_a", "us", Line{
		ID:             "ln_1",
		CheckoutID:     "chk_1",
		VariantID:      "var_ok",
		Quantity:       1,
		UnitPriceCents: 1200,
		Currency:       "USD",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	apiErr, ok := err.(sharederrors.APIError)
	if !ok || apiErr.Status != 409 {
		t.Fatalf("expected 409 API error, got %#v", err)
	}
}

func TestUpsertLineMapsRepositorySessionNotOpenToConflict(t *testing.T) {
	repo := &fakeRepo{
		upsertErr: ErrSessionNotOpen,
		session:   Session{ID: "chk_1", Status: "open", Currency: "USD"},
	}
	svc := NewService(repo, events.NewBus(), &fakeCalculator{})
	_, err := svc.UpsertLine(context.Background(), "tenant_a", "us", Line{
		ID:             "ln_1",
		CheckoutID:     "chk_1",
		ProductID:      "prd_ok",
		Quantity:       1,
		UnitPriceCents: 1000,
		Currency:       "USD",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	apiErr, ok := err.(sharederrors.APIError)
	if !ok || apiErr.Status != 409 {
		t.Fatalf("expected 409 API error, got %#v", err)
	}
}

func TestUpdateSessionContextRejectsChannelSwitchWithIncompatibleLines(t *testing.T) {
	repo := &fakeRepo{
		session: Session{ID: "chk_1", ChannelID: "web", Currency: "USD"},
		lines: []Line{
			{ID: "ln_1", CheckoutID: "chk_1", VariantID: "var_ok", ProductID: "prd_ok", Quantity: 1, UnitPriceCents: 1200, Currency: "USD"},
		},
	}
	svc := NewService(repo, events.NewBus(), &fakeCalculator{})
	_, err := svc.UpdateSessionContext(context.Background(), "tenant_a", "us", "chk_1", Session{ChannelID: "pos"})
	if err == nil {
		t.Fatalf("expected error")
	}
	apiErr, ok := err.(sharederrors.APIError)
	if !ok || apiErr.Status != 409 {
		t.Fatalf("expected 409 API error, got %#v", err)
	}
}

func TestUpdateSessionContextAllowsCompatibleChannelSwitch(t *testing.T) {
	repo := &fakeRepo{
		session: Session{ID: "chk_1", ChannelID: "", Currency: "USD"},
		lines: []Line{
			{ID: "ln_1", CheckoutID: "chk_1", VariantID: "var_ok", ProductID: "prd_ok", Quantity: 1, UnitPriceCents: 1200, Currency: "USD"},
		},
	}
	svc := NewService(repo, events.NewBus(), &fakeCalculator{})
	updated, err := svc.UpdateSessionContext(context.Background(), "tenant_a", "us", "chk_1", Session{ChannelID: "web"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.ID != "chk_1" {
		t.Fatalf("expected checkout id chk_1, got %q", updated.ID)
	}
}

func TestUpdateSessionContextRejectsCompletedSession(t *testing.T) {
	repo := &fakeRepo{
		session: Session{ID: "chk_1", Status: "completed", ChannelID: "web", Currency: "USD"},
	}
	svc := NewService(repo, events.NewBus(), &fakeCalculator{})
	_, err := svc.UpdateSessionContext(context.Background(), "tenant_a", "us", "chk_1", Session{
		VoucherCode: "SAVE10",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	apiErr, ok := err.(sharederrors.APIError)
	if !ok || apiErr.Status != 409 {
		t.Fatalf("expected 409 API error, got %#v", err)
	}
}

func TestUpdateSessionContextMapsRepositorySessionNotOpenToConflict(t *testing.T) {
	repo := &fakeRepo{
		updateCtxErr: ErrSessionNotOpen,
		session:      Session{ID: "chk_1", Status: "open", Currency: "USD"},
	}
	svc := NewService(repo, events.NewBus(), &fakeCalculator{})
	_, err := svc.UpdateSessionContext(context.Background(), "tenant_a", "us", "chk_1", Session{
		VoucherCode: "SAVE10",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	apiErr, ok := err.(sharederrors.APIError)
	if !ok || apiErr.Status != 409 {
		t.Fatalf("expected 409 API error, got %#v", err)
	}
}
