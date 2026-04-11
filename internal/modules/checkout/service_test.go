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
	completed             bool
	session               Session
	completeErr           error
	lines                 []Line
	paymentCovered        bool
	upsertErr             error
	updateCtxErr          error
	updateShippingErr     error
	updatePricingErr      error
	applyAddrErr          error
	applyAddrIdem         map[string]Session
	getSessionErr         error
	idem                  map[string]Session
	completePayloadByIdem map[string]OrderCreatedPayload
	lineUpsertIdem        map[string]Line
	recalcIdem            map[string]Session
	patchCtxIdem          map[string]Session
}

func (f *fakeRepo) CreateSession(_ context.Context, in Session, idempotencyKey string) (Session, error) {
	if strings.TrimSpace(idempotencyKey) == "" {
		return Session{}, ErrIdempotencyKeyRequired
	}
	k := in.TenantID + "|" + checkoutSessionCreateScope + "|" + strings.TrimSpace(idempotencyKey)
	if f.idem == nil {
		f.idem = make(map[string]Session)
	}
	if prev, ok := f.idem[k]; ok {
		return prev, nil
	}
	f.idem[k] = in
	return in, nil
}

func (f *fakeRepo) UpsertLine(_ context.Context, tenantID, _ string, line Line, idempotencyKey string) (Line, error) {
	if strings.TrimSpace(idempotencyKey) == "" {
		return Line{}, ErrCheckoutLineUpsertIdempotencyKeyRequired
	}
	if f.upsertErr != nil {
		return Line{}, f.upsertErr
	}
	k := tenantID + "|" + checkoutLineUpsertScope(line.CheckoutID, line.ID) + "|" + strings.TrimSpace(idempotencyKey)
	if f.lineUpsertIdem == nil {
		f.lineUpsertIdem = make(map[string]Line)
	}
	if prev, ok := f.lineUpsertIdem[k]; ok {
		return prev, nil
	}
	f.lineUpsertIdem[k] = line
	return line, nil
}

func (f *fakeRepo) GetSession(_ context.Context, _, _, checkoutID string) (Session, error) {
	if f.getSessionErr != nil {
		return Session{}, f.getSessionErr
	}
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

func (f *fakeRepo) ApplyCustomerAddressesToCheckout(_ context.Context, tenantID, _, checkoutID, shipID, billID, idempotencyKey string) (Session, error) {
	key := strings.TrimSpace(idempotencyKey)
	if key == "" {
		return Session{}, ErrCheckoutApplyAddressesIdempotencyKeyRequired
	}
	sk := tenantID + "|" + checkoutApplyCustomerAddressesScope(checkoutID) + "|" + key
	if f.applyAddrIdem == nil {
		f.applyAddrIdem = make(map[string]Session)
	}
	if prev, ok := f.applyAddrIdem[sk]; ok {
		return prev, nil
	}
	if f.applyAddrErr != nil {
		return Session{}, f.applyAddrErr
	}
	s := f.session
	if s.ID == "" {
		s = Session{ID: checkoutID, Status: "open", Currency: "USD", CustomerID: "cus_1"}
	}
	s.ID = checkoutID
	if s.Status == "" {
		s.Status = "open"
	}
	if s.Currency == "" {
		s.Currency = "USD"
	}
	if strings.TrimSpace(shipID) != "" {
		s.ShippingAddressCountry = "US"
		s.ShippingAddressPostalCode = "90210"
	}
	if strings.TrimSpace(billID) != "" {
		s.BillingAddressCountry = "CA"
		s.BillingAddressPostalCode = "M5V"
	}
	f.session = s
	f.applyAddrIdem[sk] = s
	return s, nil
}

func (f *fakeRepo) UpdateSessionContext(_ context.Context, tenantID, _, checkoutID string, in Session, idempotencyKey string) (Session, error) {
	key := strings.TrimSpace(idempotencyKey)
	if key == "" {
		return Session{}, ErrCheckoutPatchSessionIdempotencyKeyRequired
	}
	if f.updateCtxErr != nil {
		return Session{}, f.updateCtxErr
	}
	idemK := tenantID + "|" + checkoutPatchSessionScope(checkoutID) + "|" + key
	if f.patchCtxIdem != nil {
		if prev, ok := f.patchCtxIdem[idemK]; ok {
			return prev, nil
		}
	}
	in.ID = checkoutID
	in.Currency = "USD"
	in.SubtotalCents = 1000
	in.ShippingCents = 200
	out := in
	if f.patchCtxIdem == nil {
		f.patchCtxIdem = make(map[string]Session)
	}
	f.patchCtxIdem[idemK] = out
	return out, nil
}

func (f *fakeRepo) Recalculate(ctx context.Context, tenantID, _, checkoutID string, opts *RecalculateOptions, idempotencyKey string) (Session, error) {
	key := strings.TrimSpace(idempotencyKey)
	if key != "" {
		idemK := tenantID + "|" + checkoutRecalculateScope(checkoutID) + "|" + key
		if f.recalcIdem != nil {
			if prev, ok := f.recalcIdem[idemK]; ok {
				return prev, nil
			}
		}
	}
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
		if key != "" {
			if f.recalcIdem == nil {
				f.recalcIdem = make(map[string]Session)
			}
			f.recalcIdem[tenantID+"|"+checkoutRecalculateScope(checkoutID)+"|"+key] = s
		}
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
	if key != "" {
		if f.recalcIdem == nil {
			f.recalcIdem = make(map[string]Session)
		}
		f.recalcIdem[tenantID+"|"+checkoutRecalculateScope(checkoutID)+"|"+key] = session
	}
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

func (f *fakeRepo) ValidateCheckoutStock(context.Context, string, string, string) error {
	return nil
}

func (f *fakeRepo) Complete(_ context.Context, tenantID, regionID, checkoutID, idempotencyKey string) (CompleteOutcome, error) {
	key := strings.TrimSpace(idempotencyKey)
	if key == "" {
		return CompleteOutcome{}, ErrCheckoutCompleteIdempotencyKeyRequired
	}
	sk := tenantID + "|" + checkoutCompleteScope(checkoutID) + "|" + key
	if f.completePayloadByIdem == nil {
		f.completePayloadByIdem = make(map[string]OrderCreatedPayload)
	}
	if prev, ok := f.completePayloadByIdem[sk]; ok {
		return CompleteOutcome{Payload: prev, FromIdempotencyReplay: true}, nil
	}
	if f.completeErr != nil {
		return CompleteOutcome{}, f.completeErr
	}
	f.completed = true
	payload := OrderCreatedPayload{
		ID:         "ord_test_1",
		TenantID:   tenantID,
		RegionID:   regionID,
		CheckoutID: checkoutID,
		CustomerID: "cus_1",
		Status:     "created",
		TotalCents: 1200,
		Currency:   "USD",
	}
	f.completePayloadByIdem[sk] = payload
	return CompleteOutcome{Payload: payload, FromIdempotencyReplay: false}, nil
}

func TestValidateCheckoutReportsSessionNotOpen(t *testing.T) {
	repo := &fakeRepo{session: Session{ID: "chk_1", Status: "completed", Currency: "USD"}}
	svc := NewService(repo, events.NewBus(), &fakeCalculator{})
	report, err := svc.ValidateCheckout(context.Background(), "tenant_a", "us", "chk_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.Problems) != 1 || report.Problems[0].Code != "session_not_open" {
		t.Fatalf("expected session_not_open problem, got %+v", report.Problems)
	}
}

func TestValidateCheckoutEmptyCartWarning(t *testing.T) {
	repo := &fakeRepo{session: Session{ID: "chk_1", Status: "open", Currency: "USD"}}
	svc := NewService(repo, events.NewBus(), &fakeCalculator{})
	report, err := svc.ValidateCheckout(context.Background(), "tenant_a", "us", "chk_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var found bool
	for _, p := range report.Problems {
		if p.Code == "empty_cart" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected empty_cart warning, got %+v", report.Problems)
	}
}

func TestCompleteMapsChannelListingMismatchToConflict(t *testing.T) {
	repo := &fakeRepo{completeErr: ErrChannelListingMismatch, paymentCovered: true}
	svc := NewService(repo, events.NewBus(), &fakeCalculator{})
	_, err := svc.Complete(context.Background(), "tenant_a", "us", "chk_1", "idem-c")
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
	result, err := svc.Complete(context.Background(), "tenant_a", "us", "chk_1", "idem-c")
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
	_, err := svc.Complete(context.Background(), "tenant_a", "us", "chk_1", "idem-c")
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
	_, err := svc.Complete(context.Background(), "tenant_a", "us", "chk_1", "idem-c")
	if err == nil {
		t.Fatalf("expected error")
	}
	apiErr, ok := err.(sharederrors.APIError)
	if !ok || apiErr.Status != 409 {
		t.Fatalf("expected 409 API error, got %#v", err)
	}
}

func TestCompleteRequiresIdempotencyKey(t *testing.T) {
	svc := NewService(&fakeRepo{paymentCovered: true}, events.NewBus(), &fakeCalculator{})
	_, err := svc.Complete(context.Background(), "t", "r", "chk_1", "  ")
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(sharederrors.APIError)
	if !ok || apiErr.Status != 400 {
		t.Fatalf("expected 400 API error, got %#v", err)
	}
}

func TestCompleteIdempotentReplayDoesNotRepublishOrderCreated(t *testing.T) {
	repo := &fakeRepo{
		paymentCovered: true,
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
	bus := events.NewBus()
	var publishCount int
	published := make(chan struct{}, 8)
	bus.Subscribe(events.EventOrderCreated, func(_ context.Context, _ any) {
		publishCount++
		published <- struct{}{}
	})
	svc := NewService(repo, bus, &fakeCalculator{})
	if _, err := svc.Complete(context.Background(), "tenant_a", "us", "chk_1", "complete-key"); err != nil {
		t.Fatalf("first complete: %v", err)
	}
	select {
	case <-published:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected first order.created event")
	}
	if _, err := svc.Complete(context.Background(), "tenant_a", "us", "chk_1", "complete-key"); err != nil {
		t.Fatalf("second complete: %v", err)
	}
	select {
	case <-published:
		t.Fatal("unexpected second order.created on idempotent replay")
	case <-time.After(200 * time.Millisecond):
	}
	if publishCount != 1 {
		t.Fatalf("expected exactly one order.created publish, got %d", publishCount)
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
	updated, err := svc.Recalculate(context.Background(), "tenant_a", "us", "chk_1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.ShippingCents != 250 {
		t.Fatalf("expected shipping cents to be 250, got %d", updated.ShippingCents)
	}
}

func TestRecalculateIdempotentReplayReturnsFirstTotals(t *testing.T) {
	repo := &fakeRepo{
		session: Session{
			ID:       "chk_1",
			Status:   "open",
			Currency: "USD",
		},
	}
	svc := NewService(repo, events.NewBus(), &fakeCalculator{})
	first, err := svc.Recalculate(context.Background(), "tenant_a", "us", "chk_1", "recalc-idem-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	second, err := svc.Recalculate(context.Background(), "tenant_a", "us", "chk_1", "recalc-idem-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if first.TotalCents != second.TotalCents {
		t.Fatalf("expected idempotent replay, first=%+v second=%+v", first, second)
	}
}

func TestApplyCustomerAddressesRequiresAnID(t *testing.T) {
	svc := NewService(&fakeRepo{}, events.NewBus(), &fakeCalculator{})
	_, err := svc.ApplyCustomerAddresses(context.Background(), "t", "r", "chk_1", "", "  ", "idem-a")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestApplyCustomerAddressesMapsNotApplicableToNotFound(t *testing.T) {
	repo := &fakeRepo{applyAddrErr: ErrCustomerAddressNotApplicable}
	svc := NewService(repo, events.NewBus(), &fakeCalculator{})
	_, err := svc.ApplyCustomerAddresses(context.Background(), "t", "r", "chk_1", "adr_1", "", "idem-a")
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(sharederrors.APIError)
	if !ok || apiErr.Status != 404 {
		t.Fatalf("expected 404 API error, got %#v", err)
	}
}

func TestApplyCustomerAddressesRequiresIdempotencyKey(t *testing.T) {
	svc := NewService(&fakeRepo{}, events.NewBus(), nil)
	_, err := svc.ApplyCustomerAddresses(context.Background(), "t", "r", "chk_1", "adr_1", "", "  ")
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(sharederrors.APIError)
	if !ok || apiErr.Status != 400 {
		t.Fatalf("expected 400 API error, got %#v", err)
	}
}

func TestApplyCustomerAddressesIdempotentReplayReturnsFirstSession(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo, events.NewBus(), nil)
	first, err := svc.ApplyCustomerAddresses(context.Background(), "tenant_a", "us", "chk_1", "adr_1", "", "apply-key")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	second, err := svc.ApplyCustomerAddresses(context.Background(), "tenant_a", "us", "chk_1", "adr_9", "", "apply-key")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if first.ShippingAddressPostalCode != second.ShippingAddressPostalCode || first.ShippingAddressPostalCode != "90210" {
		t.Fatalf("expected replay of first apply, first=%+v second=%+v", first, second)
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
	_, err := svc.Recalculate(context.Background(), "tenant_a", "us", "chk_1", "")
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
	_, err := svc.Recalculate(context.Background(), "tenant_a", "us", "chk_1", "")
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
	}, "idem_patch_recalc")
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
	}, "idem_ln_unpub_v")
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
	}, "idem_ln_price")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpsertLineRequiresIdempotencyKey(t *testing.T) {
	svc := NewService(&fakeRepo{}, events.NewBus(), &fakeCalculator{})
	_, err := svc.UpsertLine(context.Background(), "tenant_a", "us", Line{
		ID:             "ln_1",
		CheckoutID:     "chk_1",
		VariantID:      "var_ok",
		Quantity:       1,
		UnitPriceCents: 1200,
		Currency:       "USD",
	}, "  ")
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(sharederrors.APIError)
	if !ok || apiErr.Status != 400 {
		t.Fatalf("expected 400 API error, got %#v", err)
	}
}

func TestUpsertLineIdempotentReplayReturnsFirstLine(t *testing.T) {
	repo := &fakeRepo{session: Session{ID: "chk_1", ChannelID: "web", Currency: "USD"}}
	svc := NewService(repo, events.NewBus(), &fakeCalculator{})
	first, err := svc.UpsertLine(context.Background(), "tenant_a", "us", Line{
		ID:             "ln_1",
		CheckoutID:     "chk_1",
		VariantID:      "var_ok",
		Quantity:       1,
		UnitPriceCents: 1200,
		Currency:       "USD",
	}, "same-line-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	second, err := svc.UpsertLine(context.Background(), "tenant_a", "us", Line{
		ID:             "ln_1",
		CheckoutID:     "chk_1",
		VariantID:      "var_ok",
		Quantity:       99,
		UnitPriceCents: 1200,
		Currency:       "USD",
	}, "same-line-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if first.Quantity != second.Quantity || first.Quantity != 1 {
		t.Fatalf("expected idempotent replay to return first line qty=1, got first=%+v second=%+v", first, second)
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
	}, "idem-channel")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestCreateSessionRequiresIdempotencyKey(t *testing.T) {
	svc := NewService(&fakeRepo{}, events.NewBus(), nil)
	_, err := svc.CreateSession(context.Background(), Session{
		TenantID:   "t",
		RegionID:   "r",
		CustomerID: "c",
		Currency:   "USD",
	}, "  ")
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(sharederrors.APIError)
	if !ok || apiErr.Status != 400 {
		t.Fatalf("expected 400 API error, got %#v", err)
	}
}

func TestCreateSessionIdempotentReplayReturnsFirstSession(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo, events.NewBus(), nil)
	first, err := svc.CreateSession(context.Background(), Session{
		ID:         "chk_first",
		TenantID:   "tenant_a",
		RegionID:   "us",
		CustomerID: "cus_1",
		Currency:   "USD",
	}, "same-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	second, err := svc.CreateSession(context.Background(), Session{
		ID:         "chk_second",
		TenantID:   "tenant_a",
		RegionID:   "us",
		CustomerID: "cus_1",
		Currency:   "USD",
	}, "same-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if first.ID != second.ID || first.ID != "chk_first" {
		t.Fatalf("expected replay of first session id, got first=%+v second=%+v", first, second)
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
	}, "idem_ln_prd_hidden")
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
	}, "idem_ln_curr")
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
	}, "idem_ln_var_mis")
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
	}, "idem_ln_autofill")
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
	}, "idem_ln_completed")
	if err == nil {
		t.Fatalf("expected error")
	}
	apiErr, ok := err.(sharederrors.APIError)
	if !ok || apiErr.Status != 409 {
		t.Fatalf("expected 409 API error, got %#v", err)
	}
}

func TestUpsertLineMapsInsufficientStockToConflict(t *testing.T) {
	repo := &fakeRepo{upsertErr: ErrInsufficientStock, session: Session{ID: "chk_1", Status: "open", Currency: "USD"}}
	svc := NewService(repo, events.NewBus(), &fakeCalculator{})
	_, err := svc.UpsertLine(context.Background(), "tenant_a", "us", Line{
		ID:             "ln_1",
		CheckoutID:     "chk_1",
		VariantID:      "var_ok",
		ProductID:      "prd_ok",
		Quantity:       1,
		UnitPriceCents: 1200,
		Currency:       "USD",
	}, "idem_ln_stock")
	if err == nil {
		t.Fatal("expected error")
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
	}, "idem_ln_not_open")
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
	_, err := svc.UpdateSessionContext(context.Background(), "tenant_a", "us", "chk_1", Session{ChannelID: "pos"}, "idem_patch_ch_bad")
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
	updated, err := svc.UpdateSessionContext(context.Background(), "tenant_a", "us", "chk_1", Session{ChannelID: "web"}, "idem_patch_ch_ok")
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
	}, "idem_patch_completed")
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
	}, "idem_patch_not_open")
	if err == nil {
		t.Fatalf("expected error")
	}
	apiErr, ok := err.(sharederrors.APIError)
	if !ok || apiErr.Status != 409 {
		t.Fatalf("expected 409 API error, got %#v", err)
	}
}

func TestUpdateSessionContextRequiresIdempotencyKey(t *testing.T) {
	svc := NewService(&fakeRepo{}, events.NewBus(), &fakeCalculator{})
	_, err := svc.UpdateSessionContext(context.Background(), "tenant_a", "us", "chk_1", Session{
		VoucherCode: "SAVE10",
	}, "  ")
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(sharederrors.APIError)
	if !ok || apiErr.Status != 400 {
		t.Fatalf("expected 400 API error, got %#v", err)
	}
}

func TestUpdateSessionContextIdempotentReplayReturnsFirstPayload(t *testing.T) {
	repo := &fakeRepo{session: Session{ID: "chk_1", ChannelID: "web", Currency: "USD"}}
	svc := NewService(repo, events.NewBus(), nil)
	first, err := svc.UpdateSessionContext(context.Background(), "tenant_a", "us", "chk_1", Session{
		VoucherCode: "FIRST",
	}, "same-patch-key")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	second, err := svc.UpdateSessionContext(context.Background(), "tenant_a", "us", "chk_1", Session{
		VoucherCode: "SECOND",
	}, "same-patch-key")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if first.VoucherCode != second.VoucherCode || first.VoucherCode != "FIRST" {
		t.Fatalf("expected idempotent replay of first patch, first=%+v second=%+v", first, second)
	}
}

func TestGetSessionRequiresCheckoutID(t *testing.T) {
	svc := NewService(&fakeRepo{}, events.NewBus(), nil)
	_, err := svc.GetSession(context.Background(), "t", "r", "  ")
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(sharederrors.APIError)
	if !ok || apiErr.Status != 400 {
		t.Fatalf("expected 400 API error, got %#v", err)
	}
}

func TestGetSessionMapsNotFoundToAPI(t *testing.T) {
	repo := &fakeRepo{getSessionErr: ErrSessionNotFound}
	svc := NewService(repo, events.NewBus(), nil)
	_, err := svc.GetSession(context.Background(), "t", "r", "chk_x")
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(sharederrors.APIError)
	if !ok || apiErr.Status != 404 {
		t.Fatalf("expected 404 API error, got %#v", err)
	}
}

func TestGetSessionReturnsStoredSession(t *testing.T) {
	repo := &fakeRepo{
		session: Session{
			ID:         "chk_1",
			Status:     "open",
			Currency:   "EUR",
			CustomerID: "cus_9",
		},
	}
	svc := NewService(repo, events.NewBus(), nil)
	sess, err := svc.GetSession(context.Background(), "tenant_a", "us", "chk_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.CustomerID != "cus_9" || sess.Currency != "EUR" {
		t.Fatalf("unexpected session: %+v", sess)
	}
}

func TestListLinesMapsNotFoundToAPI(t *testing.T) {
	repo := &fakeRepo{getSessionErr: ErrSessionNotFound}
	svc := NewService(repo, events.NewBus(), nil)
	_, err := svc.ListLines(context.Background(), "t", "r", "chk_x")
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(sharederrors.APIError)
	if !ok || apiErr.Status != 404 {
		t.Fatalf("expected 404 API error, got %#v", err)
	}
}

func TestListLinesReturnsEmptySliceWhenNoLines(t *testing.T) {
	repo := &fakeRepo{session: Session{ID: "chk_1", Status: "open", Currency: "USD"}}
	svc := NewService(repo, events.NewBus(), nil)
	lines, err := svc.ListLines(context.Background(), "t", "r", "chk_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lines == nil || len(lines) != 0 {
		t.Fatalf("expected empty non-nil slice, got %#v", lines)
	}
}

func TestListLinesReturnsItems(t *testing.T) {
	repo := &fakeRepo{
		session: Session{ID: "chk_1", Status: "open", Currency: "USD"},
		lines: []Line{
			{ID: "ln_1", CheckoutID: "chk_1", ProductID: "p1", Quantity: 2, UnitPriceCents: 500, Currency: "USD"},
		},
	}
	svc := NewService(repo, events.NewBus(), nil)
	lines, err := svc.ListLines(context.Background(), "t", "r", "chk_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lines) != 1 || lines[0].ID != "ln_1" {
		t.Fatalf("unexpected lines: %#v", lines)
	}
}
