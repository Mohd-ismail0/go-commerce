package payments

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	sharederrors "rewrite/internal/shared/errors"
	"rewrite/internal/shared/utils"
)

type Service struct {
	repo      *Repository
	providers map[string]ProviderAdapter
}

func NewService(repo *Repository, adapters ...ProviderAdapter) *Service {
	providers := map[string]ProviderAdapter{
		"default": passThroughProvider{},
	}
	for _, a := range adapters {
		if a == nil {
			continue
		}
		providers[normalizeProviderName(a.Name())] = a
	}
	return &Service{repo: repo, providers: providers}
}

func (s *Service) Save(ctx context.Context, item Payment, idempotencyKey string) (Payment, error) {
	if strings.TrimSpace(item.Provider) == "" || item.AmountCents <= 0 || len(strings.TrimSpace(item.Currency)) != 3 {
		return Payment{}, sharederrors.BadRequest("invalid payment payload")
	}
	if item.Status == "" {
		item.Status = StatusAuthorized
	}
	provider, err := s.providerFor(item.Provider)
	if err != nil {
		return Payment{}, err
	}
	if idempotencyKey != "" {
		scope := "payments.upsert"
		resourceID, err := s.repo.GetIdempotency(ctx, item.TenantID, scope, idempotencyKey)
		if err == nil && resourceID != "" {
			existing, getErr := s.repo.GetByID(ctx, item.TenantID, resourceID)
			if getErr == nil {
				return existing, nil
			}
		}
	}
	if item.ID == "" {
		item.ID = utils.NewID("pay")
	}
	if item.ExternalReference == "" {
		authResult, authErr := provider.Authorize(ctx, item, item.AmountCents)
		if authErr != nil || !authResult.Success {
			return Payment{}, sharederrors.Conflict("payment provider authorization failed")
		}
		item.ExternalReference = authResult.ExternalReference
	}
	saved, err := s.repo.Save(ctx, item)
	if err != nil {
		return Payment{}, sharederrors.Internal("failed to save payment")
	}
	if idempotencyKey != "" {
		_ = s.repo.SaveIdempotency(ctx, item.TenantID, "payments.upsert", idempotencyKey, saved.ID)
	}
	return saved, nil
}

func (s *Service) List(ctx context.Context, tenantID string) ([]Payment, error) {
	return s.repo.List(ctx, tenantID)
}

func (s *Service) Get(ctx context.Context, tenantID, id string) (Payment, error) {
	p, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Payment{}, sharederrors.NotFound("payment not found")
		}
		return Payment{}, sharederrors.Internal("failed to load payment")
	}
	return p, nil
}

func (s *Service) ListTransactions(ctx context.Context, tenantID, paymentID string) ([]PaymentTransaction, error) {
	if _, err := s.Get(ctx, tenantID, paymentID); err != nil {
		return nil, err
	}
	return s.repo.ListTransactions(ctx, tenantID, paymentID)
}

func (s *Service) Capture(ctx context.Context, tenantID, paymentID string, in AmountActionInput, idempotencyKey string) (ActionResult, error) {
	if strings.TrimSpace(idempotencyKey) == "" {
		return ActionResult{}, sharederrors.BadRequest("Idempotency-Key header is required")
	}
	scope := "payments.capture." + paymentID
	if prev, err := s.repo.GetIdempotency(ctx, tenantID, scope, idempotencyKey); err == nil && prev != "" {
		p, _ := s.Get(ctx, tenantID, paymentID)
		txs, _ := s.repo.ListTransactions(ctx, tenantID, paymentID)
		for _, t := range txs {
			if t.ID == prev {
				return ActionResult{Payment: p, Transaction: t}, nil
			}
		}
	}

	p, err := s.Get(ctx, tenantID, paymentID)
	if err != nil {
		return ActionResult{}, err
	}
	if p.Status != StatusAuthorized && p.Status != StatusPartiallyCaptured {
		return ActionResult{}, sharederrors.BadRequest("capture not allowed for current status")
	}
	captured, _, err := s.repo.Totals(ctx, paymentID)
	if err != nil {
		return ActionResult{}, sharederrors.Internal("failed to read payment totals")
	}
	remaining := p.AmountCents - captured
	if remaining <= 0 {
		return ActionResult{}, sharederrors.BadRequest("nothing left to capture")
	}
	amount := remaining
	if in.AmountCents != nil {
		if *in.AmountCents <= 0 || *in.AmountCents > remaining {
			return ActionResult{}, sharederrors.BadRequest("invalid capture amount")
		}
		amount = *in.AmountCents
	}

	tx := PaymentTransaction{
		ID:          utils.NewID("ptx"),
		TenantID:    p.TenantID,
		RegionID:    p.RegionID,
		PaymentID:   p.ID,
		EventType:   EventCapture,
		AmountCents: amount,
		Currency:    p.Currency,
		Success:     true,
		RawPayload:  map[string]any{"op": "capture", "provider": p.Provider},
	}
	provider, perr := s.providerFor(p.Provider)
	if perr != nil {
		return ActionResult{}, perr
	}
	provRes, captureErr := provider.Capture(ctx, p, amount)
	if captureErr != nil || !provRes.Success {
		return ActionResult{}, sharederrors.Conflict("payment provider capture failed")
	}
	if provRes.RawPayload != nil {
		tx.RawPayload = provRes.RawPayload
	}
	savedTx, err := s.repo.InsertTransaction(ctx, tx)
	if err != nil {
		return ActionResult{}, sharederrors.Internal("failed to record capture")
	}
	newCaptured := captured + amount
	nextStatus := StatusPartiallyCaptured
	if newCaptured >= p.AmountCents {
		nextStatus = StatusCaptured
	}
	updated, err := s.repo.UpdatePaymentStatus(ctx, tenantID, paymentID, nextStatus)
	if err != nil {
		return ActionResult{}, sharederrors.Internal("failed to update payment status")
	}
	_ = s.repo.SaveIdempotency(ctx, tenantID, scope, idempotencyKey, savedTx.ID)
	return ActionResult{Payment: updated, Transaction: savedTx}, nil
}

func (s *Service) Refund(ctx context.Context, tenantID, paymentID string, in AmountActionInput, idempotencyKey string) (ActionResult, error) {
	if strings.TrimSpace(idempotencyKey) == "" {
		return ActionResult{}, sharederrors.BadRequest("Idempotency-Key header is required")
	}
	scope := "payments.refund." + paymentID
	if prev, err := s.repo.GetIdempotency(ctx, tenantID, scope, idempotencyKey); err == nil && prev != "" {
		p, _ := s.Get(ctx, tenantID, paymentID)
		txs, _ := s.repo.ListTransactions(ctx, tenantID, paymentID)
		for _, t := range txs {
			if t.ID == prev {
				return ActionResult{Payment: p, Transaction: t}, nil
			}
		}
	}

	p, err := s.Get(ctx, tenantID, paymentID)
	if err != nil {
		return ActionResult{}, err
	}
	if p.Status != StatusCaptured && p.Status != StatusPartiallyCaptured && p.Status != StatusPartiallyRefunded {
		return ActionResult{}, sharederrors.BadRequest("refund not allowed for current status")
	}
	captured, refunded, err := s.repo.Totals(ctx, paymentID)
	if err != nil {
		return ActionResult{}, sharederrors.Internal("failed to read payment totals")
	}
	refundable := captured - refunded
	if refundable <= 0 {
		return ActionResult{}, sharederrors.BadRequest("nothing to refund")
	}
	amount := refundable
	if in.AmountCents != nil {
		if *in.AmountCents <= 0 || *in.AmountCents > refundable {
			return ActionResult{}, sharederrors.BadRequest("invalid refund amount")
		}
		amount = *in.AmountCents
	}

	tx := PaymentTransaction{
		ID:          utils.NewID("ptx"),
		TenantID:    p.TenantID,
		RegionID:    p.RegionID,
		PaymentID:   p.ID,
		EventType:   EventRefund,
		AmountCents: amount,
		Currency:    p.Currency,
		Success:     true,
		RawPayload:  map[string]any{"op": "refund", "provider": p.Provider},
	}
	provider, perr := s.providerFor(p.Provider)
	if perr != nil {
		return ActionResult{}, perr
	}
	provRes, refundErr := provider.Refund(ctx, p, amount)
	if refundErr != nil || !provRes.Success {
		return ActionResult{}, sharederrors.Conflict("payment provider refund failed")
	}
	if provRes.RawPayload != nil {
		tx.RawPayload = provRes.RawPayload
	}
	savedTx, err := s.repo.InsertTransaction(ctx, tx)
	if err != nil {
		return ActionResult{}, sharederrors.Internal("failed to record refund")
	}
	newRefunded := refunded + amount
	nextStatus := StatusPartiallyRefunded
	if newRefunded >= captured {
		nextStatus = StatusRefunded
	}
	updated, err := s.repo.UpdatePaymentStatus(ctx, tenantID, paymentID, nextStatus)
	if err != nil {
		return ActionResult{}, sharederrors.Internal("failed to update payment status")
	}
	_ = s.repo.SaveIdempotency(ctx, tenantID, scope, idempotencyKey, savedTx.ID)
	return ActionResult{Payment: updated, Transaction: savedTx}, nil
}

func (s *Service) Void(ctx context.Context, tenantID, paymentID string, idempotencyKey string) (ActionResult, error) {
	if strings.TrimSpace(idempotencyKey) == "" {
		return ActionResult{}, sharederrors.BadRequest("Idempotency-Key header is required")
	}
	scope := "payments.void." + paymentID
	if prev, err := s.repo.GetIdempotency(ctx, tenantID, scope, idempotencyKey); err == nil && prev != "" {
		p, _ := s.Get(ctx, tenantID, paymentID)
		txs, _ := s.repo.ListTransactions(ctx, tenantID, paymentID)
		for _, t := range txs {
			if t.ID == prev {
				return ActionResult{Payment: p, Transaction: t}, nil
			}
		}
	}

	p, err := s.Get(ctx, tenantID, paymentID)
	if err != nil {
		return ActionResult{}, err
	}
	if p.Status != StatusAuthorized {
		return ActionResult{}, sharederrors.BadRequest("void only allowed from authorized status")
	}
	captured, _, err := s.repo.Totals(ctx, paymentID)
	if err != nil {
		return ActionResult{}, sharederrors.Internal("failed to read payment totals")
	}
	if captured > 0 {
		return ActionResult{}, sharederrors.BadRequest("cannot void after capture")
	}

	tx := PaymentTransaction{
		ID:          utils.NewID("ptx"),
		TenantID:    p.TenantID,
		RegionID:    p.RegionID,
		PaymentID:   p.ID,
		EventType:   EventVoid,
		AmountCents: p.AmountCents,
		Currency:    p.Currency,
		Success:     true,
		RawPayload:  map[string]any{"op": "void", "provider": p.Provider},
	}
	provider, perr := s.providerFor(p.Provider)
	if perr != nil {
		return ActionResult{}, perr
	}
	provRes, voidErr := provider.Void(ctx, p)
	if voidErr != nil || !provRes.Success {
		return ActionResult{}, sharederrors.Conflict("payment provider void failed")
	}
	if provRes.RawPayload != nil {
		tx.RawPayload = provRes.RawPayload
	}
	savedTx, err := s.repo.InsertTransaction(ctx, tx)
	if err != nil {
		return ActionResult{}, sharederrors.Internal("failed to record void")
	}
	updated, err := s.repo.UpdatePaymentStatus(ctx, tenantID, paymentID, StatusVoided)
	if err != nil {
		return ActionResult{}, sharederrors.Internal("failed to update payment status")
	}
	_ = s.repo.SaveIdempotency(ctx, tenantID, scope, idempotencyKey, savedTx.ID)
	return ActionResult{Payment: updated, Transaction: savedTx}, nil
}

func (s *Service) ProcessWebhook(ctx context.Context, tenantID, regionID string, in WebhookInput) (ActionResult, error) {
	if strings.TrimSpace(in.PaymentID) == "" || strings.TrimSpace(in.ProviderEventID) == "" {
		return ActionResult{}, sharederrors.BadRequest("payment_id and provider_event_id are required")
	}
	if _, err := s.repo.FindTransactionByProviderEvent(ctx, tenantID, in.ProviderEventID); err == nil {
		return ActionResult{}, sharederrors.Conflict("duplicate webhook delivery")
	} else if !errors.Is(err, sql.ErrNoRows) {
		return ActionResult{}, sharederrors.Internal("failed to check webhook idempotency")
	}

	p, err := s.Get(ctx, tenantID, in.PaymentID)
	if err != nil {
		return ActionResult{}, err
	}

	raw := in.Raw
	if raw == nil {
		raw = map[string]any{}
	}

	switch strings.ToLower(strings.TrimSpace(in.Event)) {
	case "capture", "captured":
		return s.webhookCapture(ctx, tenantID, regionID, p, in, raw)
	case "refund", "refunded":
		return s.webhookRefund(ctx, tenantID, regionID, p, in, raw)
	case "void", "voided", "cancelled":
		return s.webhookVoid(ctx, tenantID, regionID, p, in, raw)
	default:
		tx := PaymentTransaction{
			ID:              utils.NewID("ptx"),
			TenantID:        tenantID,
			RegionID:        regionID,
			PaymentID:       p.ID,
			EventType:       EventWebhook,
			AmountCents:     in.AmountCents,
			Currency:        pickCurrency(in.Currency, p.Currency),
			Success:         true,
			ProviderEventID: in.ProviderEventID,
			RawPayload:      raw,
		}
		saved, err := s.repo.InsertTransaction(ctx, tx)
		if err != nil {
			return ActionResult{}, sharederrors.Internal("failed to record webhook transaction")
		}
		updated, uerr := s.repo.GetByID(ctx, tenantID, p.ID)
		if uerr != nil {
			return ActionResult{}, sharederrors.Internal("failed to load payment")
		}
		return ActionResult{Payment: updated, Transaction: saved}, nil
	}
}

func (s *Service) webhookCapture(ctx context.Context, tenantID, regionID string, p Payment, in WebhookInput, raw map[string]any) (ActionResult, error) {
	if p.Status != StatusAuthorized && p.Status != StatusPartiallyCaptured {
		return ActionResult{}, sharederrors.BadRequest("capture not allowed for current status")
	}
	captured, _, err := s.repo.Totals(ctx, p.ID)
	if err != nil {
		return ActionResult{}, sharederrors.Internal("failed to read payment totals")
	}
	remaining := p.AmountCents - captured
	if remaining <= 0 {
		return ActionResult{}, sharederrors.BadRequest("nothing left to capture")
	}
	amount := remaining
	if in.AmountCents > 0 {
		if in.AmountCents > remaining {
			return ActionResult{}, sharederrors.BadRequest("invalid capture amount")
		}
		amount = in.AmountCents
	}

	tx := PaymentTransaction{
		ID:              utils.NewID("ptx"),
		TenantID:        tenantID,
		RegionID:        regionID,
		PaymentID:       p.ID,
		EventType:       EventCapture,
		AmountCents:     amount,
		Currency:        pickCurrency(in.Currency, p.Currency),
		Success:         true,
		ProviderEventID: in.ProviderEventID,
		RawPayload:      raw,
	}
	savedTx, err := s.repo.InsertTransaction(ctx, tx)
	if err != nil {
		return ActionResult{}, sharederrors.Internal("failed to record capture")
	}
	newCaptured := captured + amount
	nextStatus := StatusPartiallyCaptured
	if newCaptured >= p.AmountCents {
		nextStatus = StatusCaptured
	}
	updated, err := s.repo.UpdatePaymentStatus(ctx, tenantID, p.ID, nextStatus)
	if err != nil {
		return ActionResult{}, sharederrors.Internal("failed to update payment status")
	}
	return ActionResult{Payment: updated, Transaction: savedTx}, nil
}

func (s *Service) webhookRefund(ctx context.Context, tenantID, regionID string, p Payment, in WebhookInput, raw map[string]any) (ActionResult, error) {
	if p.Status != StatusCaptured && p.Status != StatusPartiallyCaptured && p.Status != StatusPartiallyRefunded {
		return ActionResult{}, sharederrors.BadRequest("refund not allowed for current status")
	}
	captured, refunded, err := s.repo.Totals(ctx, p.ID)
	if err != nil {
		return ActionResult{}, sharederrors.Internal("failed to read payment totals")
	}
	refundable := captured - refunded
	if refundable <= 0 {
		return ActionResult{}, sharederrors.BadRequest("nothing to refund")
	}
	amount := refundable
	if in.AmountCents > 0 {
		if in.AmountCents > refundable {
			return ActionResult{}, sharederrors.BadRequest("invalid refund amount")
		}
		amount = in.AmountCents
	}

	tx := PaymentTransaction{
		ID:              utils.NewID("ptx"),
		TenantID:        tenantID,
		RegionID:        regionID,
		PaymentID:       p.ID,
		EventType:       EventRefund,
		AmountCents:     amount,
		Currency:        pickCurrency(in.Currency, p.Currency),
		Success:         true,
		ProviderEventID: in.ProviderEventID,
		RawPayload:      raw,
	}
	savedTx, err := s.repo.InsertTransaction(ctx, tx)
	if err != nil {
		return ActionResult{}, sharederrors.Internal("failed to record refund")
	}
	newRefunded := refunded + amount
	nextStatus := StatusPartiallyRefunded
	if newRefunded >= captured {
		nextStatus = StatusRefunded
	}
	updated, err := s.repo.UpdatePaymentStatus(ctx, tenantID, p.ID, nextStatus)
	if err != nil {
		return ActionResult{}, sharederrors.Internal("failed to update payment status")
	}
	return ActionResult{Payment: updated, Transaction: savedTx}, nil
}

func (s *Service) webhookVoid(ctx context.Context, tenantID, regionID string, p Payment, in WebhookInput, raw map[string]any) (ActionResult, error) {
	if p.Status != StatusAuthorized {
		return ActionResult{}, sharederrors.BadRequest("void only allowed from authorized status")
	}
	captured, _, err := s.repo.Totals(ctx, p.ID)
	if err != nil {
		return ActionResult{}, sharederrors.Internal("failed to read payment totals")
	}
	if captured > 0 {
		return ActionResult{}, sharederrors.BadRequest("cannot void after capture")
	}
	tx := PaymentTransaction{
		ID:              utils.NewID("ptx"),
		TenantID:        tenantID,
		RegionID:        regionID,
		PaymentID:       p.ID,
		EventType:       EventVoid,
		AmountCents:     p.AmountCents,
		Currency:        pickCurrency(in.Currency, p.Currency),
		Success:         true,
		ProviderEventID: in.ProviderEventID,
		RawPayload:      raw,
	}
	savedTx, err := s.repo.InsertTransaction(ctx, tx)
	if err != nil {
		return ActionResult{}, sharederrors.Internal("failed to record void")
	}
	updated, err := s.repo.UpdatePaymentStatus(ctx, tenantID, p.ID, StatusVoided)
	if err != nil {
		return ActionResult{}, sharederrors.Internal("failed to update payment status")
	}
	return ActionResult{Payment: updated, Transaction: savedTx}, nil
}

func pickCurrency(in, fallback string) string {
	if len(strings.TrimSpace(in)) == 3 {
		return strings.TrimSpace(in)
	}
	return fallback
}

func (s *Service) Reconcile(ctx context.Context, tenantID, regionID string) (ReconciliationReport, error) {
	payments, err := s.repo.ListForRegion(ctx, tenantID, regionID)
	if err != nil {
		return ReconciliationReport{}, sharederrors.Internal("failed to load payments for reconciliation")
	}
	items := make([]ReconciliationItem, 0)
	for _, p := range payments {
		captured, refunded, err := s.repo.Totals(ctx, p.ID)
		if err != nil {
			return ReconciliationReport{}, sharederrors.Internal("failed to aggregate payment totals")
		}
		issues := detectReconciliationIssues(p, captured, refunded)
		for _, issue := range issues {
			items = append(items, ReconciliationItem{
				PaymentID:       p.ID,
				Status:          p.Status,
				AuthorizedCents: p.AmountCents,
				CapturedCents:   captured,
				RefundedCents:   refunded,
				Issue:           issue,
			})
		}
	}
	return ReconciliationReport{
		GeneratedAt: time.Now().UTC().Unix(),
		Items:       items,
	}, nil
}

func detectReconciliationIssues(p Payment, captured, refunded int64) []string {
	out := []string{}
	if captured > p.AmountCents {
		out = append(out, "captured_exceeds_authorized")
	}
	if refunded > captured {
		out = append(out, "refunded_exceeds_captured")
	}
	if p.Status == StatusCaptured && captured < p.AmountCents {
		out = append(out, "status_captured_but_partial_capture_totals")
	}
	if p.Status == StatusRefunded && refunded < captured {
		out = append(out, "status_refunded_but_partial_refund_totals")
	}
	if p.Status == StatusVoided && captured > 0 {
		out = append(out, "status_voided_with_capture_activity")
	}
	return out
}

func (s *Service) providerFor(name string) (ProviderAdapter, error) {
	key := normalizeProviderName(name)
	p, ok := s.providers[key]
	if ok {
		return p, nil
	}
	if def, ok := s.providers["default"]; ok {
		return def, nil
	}
	return nil, sharederrors.BadRequest("payment provider is not configured")
}

func (s *Service) SaveDispute(ctx context.Context, d Dispute) (Dispute, error) {
	if strings.TrimSpace(d.PaymentID) == "" || strings.TrimSpace(d.ProviderCaseID) == "" || d.AmountCents <= 0 || len(strings.TrimSpace(d.Currency)) != 3 {
		return Dispute{}, sharederrors.BadRequest("invalid dispute payload")
	}
	if d.ID == "" {
		d.ID = utils.NewID("pdp")
	}
	if d.Status == "" {
		d.Status = "open"
	}
	if !isValidDisputeStatus(d.Status) {
		return Dispute{}, sharederrors.BadRequest("invalid dispute status")
	}
	existing, err := s.repo.FindDisputeByCase(ctx, d.TenantID, d.Provider, d.ProviderCaseID)
	if err == nil {
		if !isValidDisputeTransition(existing.Status, d.Status) {
			return Dispute{}, sharederrors.Conflict("invalid dispute status transition")
		}
	}
	saved, err := s.repo.SaveDispute(ctx, d)
	if err != nil {
		return Dispute{}, sharederrors.Internal("failed to save dispute")
	}
	return saved, nil
}

func (s *Service) ListDisputes(ctx context.Context, tenantID, regionID string) ([]Dispute, error) {
	out, err := s.repo.ListDisputes(ctx, tenantID, regionID)
	if err != nil {
		return nil, sharederrors.Internal("failed to list disputes")
	}
	return out, nil
}

func (s *Service) ListReconciliationActions(ctx context.Context, tenantID, regionID string) ([]ReconciliationAction, error) {
	out, err := s.repo.ListReconciliationActions(ctx, tenantID, regionID)
	if err != nil {
		return nil, sharederrors.Internal("failed to list reconciliation actions")
	}
	return out, nil
}

func (s *Service) UpsertReconciliationAction(ctx context.Context, tenantID, regionID string, item ReconciliationItem) error {
	if _, err := s.repo.FindOpenReconciliationAction(ctx, tenantID, regionID, item.PaymentID, item.Issue); err == nil {
		return nil
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err := s.repo.SaveReconciliationAction(ctx, ReconciliationAction{
		ID:         utils.NewID("pra"),
		TenantID:   tenantID,
		RegionID:   regionID,
		PaymentID:  item.PaymentID,
		Issue:      item.Issue,
		ActionType: "investigate",
		Status:     "open",
		Note:       "Auto-created by reconciliation worker",
	})
	return err
}

func isValidDisputeStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "open", "under_review", "won", "lost", "reversed", "closed":
		return true
	default:
		return false
	}
}

func isValidDisputeTransition(from, to string) bool {
	from = strings.ToLower(strings.TrimSpace(from))
	to = strings.ToLower(strings.TrimSpace(to))
	switch from {
	case "open":
		return to == "open" || to == "under_review" || to == "won" || to == "lost" || to == "closed"
	case "under_review":
		return to == "under_review" || to == "won" || to == "lost" || to == "reversed" || to == "closed"
	case "won", "lost", "reversed", "closed":
		return to == from
	default:
		return false
	}
}
