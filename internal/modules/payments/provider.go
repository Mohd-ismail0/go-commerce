package payments

import (
	"context"
	"strings"
)

type ProviderAdapter interface {
	Name() string
	Authorize(ctx context.Context, p Payment, amountCents int64) (ProviderResult, error)
	Capture(ctx context.Context, p Payment, amountCents int64) (ProviderResult, error)
	Refund(ctx context.Context, p Payment, amountCents int64) (ProviderResult, error)
	Void(ctx context.Context, p Payment) (ProviderResult, error)
}

type ProviderResult struct {
	Success           bool
	ExternalReference string
	RawPayload        map[string]any
}

type passThroughProvider struct{}

func (p passThroughProvider) Name() string { return "default" }
func (p passThroughProvider) Authorize(_ context.Context, payment Payment, _ int64) (ProviderResult, error) {
	return ProviderResult{Success: true, ExternalReference: payment.ExternalReference, RawPayload: map[string]any{"provider": "default", "op": "authorize"}}, nil
}
func (p passThroughProvider) Capture(_ context.Context, payment Payment, amount int64) (ProviderResult, error) {
	return ProviderResult{Success: true, ExternalReference: payment.ExternalReference, RawPayload: map[string]any{"provider": "default", "op": "capture", "amount_cents": amount}}, nil
}
func (p passThroughProvider) Refund(_ context.Context, payment Payment, amount int64) (ProviderResult, error) {
	return ProviderResult{Success: true, ExternalReference: payment.ExternalReference, RawPayload: map[string]any{"provider": "default", "op": "refund", "amount_cents": amount}}, nil
}
func (p passThroughProvider) Void(_ context.Context, payment Payment) (ProviderResult, error) {
	return ProviderResult{Success: true, ExternalReference: payment.ExternalReference, RawPayload: map[string]any{"provider": "default", "op": "void"}}, nil
}

func normalizeProviderName(in string) string {
	name := strings.TrimSpace(strings.ToLower(in))
	if name == "" {
		return "default"
	}
	return name
}
