package payments

import (
	"context"
	"testing"
)

func TestNormalizeProviderName(t *testing.T) {
	if got := normalizeProviderName("  STRIPE "); got != "stripe" {
		t.Fatalf("expected stripe, got %s", got)
	}
	if got := normalizeProviderName(""); got != "default" {
		t.Fatalf("expected default, got %s", got)
	}
}

func TestPassThroughProviderImplementsContract(t *testing.T) {
	p := passThroughProvider{}
	payment := Payment{ID: "p1", ExternalReference: "ext-1"}
	if _, err := p.Authorize(context.Background(), payment, 100); err != nil {
		t.Fatalf("authorize: %v", err)
	}
	if _, err := p.Capture(context.Background(), payment, 100); err != nil {
		t.Fatalf("capture: %v", err)
	}
	if _, err := p.Refund(context.Background(), payment, 100); err != nil {
		t.Fatalf("refund: %v", err)
	}
	if _, err := p.Void(context.Background(), payment); err != nil {
		t.Fatalf("void: %v", err)
	}
}

func TestDisputeTransitions(t *testing.T) {
	if !isValidDisputeTransition("open", "under_review") {
		t.Fatalf("expected open->under_review valid")
	}
	if isValidDisputeTransition("won", "lost") {
		t.Fatalf("expected won->lost invalid")
	}
}
