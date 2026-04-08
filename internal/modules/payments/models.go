package payments

type Payment struct {
	ID                string `json:"id"`
	TenantID          string `json:"tenant_id"`
	RegionID          string `json:"region_id"`
	OrderID           string `json:"order_id,omitempty"`
	CheckoutID        string `json:"checkout_id,omitempty"`
	Provider          string `json:"provider"`
	Status            string `json:"status"`
	AmountCents       int64  `json:"amount_cents"`
	Currency          string `json:"currency"`
	ExternalReference string `json:"external_reference,omitempty"`
}

// PaymentTransaction records authorize/capture/refund/void and provider callbacks.
type PaymentTransaction struct {
	ID              string         `json:"id"`
	TenantID        string         `json:"tenant_id"`
	RegionID        string         `json:"region_id"`
	PaymentID       string         `json:"payment_id"`
	EventType       string         `json:"event_type"`
	AmountCents     int64          `json:"amount_cents"`
	Currency        string         `json:"currency"`
	Success         bool           `json:"success"`
	ProviderEventID string         `json:"provider_event_id,omitempty"`
	RawPayload      map[string]any `json:"raw_payload,omitempty"`
}

const (
	StatusPending           = "pending"
	StatusAuthorized        = "authorized"
	StatusPartiallyCaptured = "partially_captured"
	StatusCaptured          = "captured"
	StatusPartiallyRefunded = "partially_refunded"
	StatusRefunded          = "refunded"
	StatusVoided            = "voided"
	StatusFailed            = "failed"
)

const (
	EventAuthorize = "authorize"
	EventCapture   = "capture"
	EventRefund    = "refund"
	EventVoid      = "void"
	EventWebhook   = "webhook"
)

type AmountActionInput struct {
	AmountCents *int64 // nil = full remaining amount for capture/refund
}

type ActionResult struct {
	Payment     Payment            `json:"payment"`
	Transaction PaymentTransaction `json:"transaction"`
}

type WebhookInput struct {
	PaymentID         string         `json:"payment_id"`
	Event             string         `json:"event"`
	AmountCents       int64          `json:"amount_cents"`
	Currency          string         `json:"currency"`
	ProviderEventID   string         `json:"provider_event_id"`
	ExternalReference string         `json:"external_reference,omitempty"`
	Raw               map[string]any `json:"raw,omitempty"`
}

type ReconciliationItem struct {
	PaymentID       string `json:"payment_id"`
	Status          string `json:"status"`
	AuthorizedCents int64  `json:"authorized_cents"`
	CapturedCents   int64  `json:"captured_cents"`
	RefundedCents   int64  `json:"refunded_cents"`
	Issue           string `json:"issue"`
}

type ReconciliationReport struct {
	GeneratedAt int64                `json:"generated_at"`
	Items       []ReconciliationItem `json:"items"`
}

type Dispute struct {
	ID             string `json:"id"`
	TenantID       string `json:"tenant_id"`
	RegionID       string `json:"region_id"`
	PaymentID      string `json:"payment_id"`
	Provider       string `json:"provider"`
	ProviderCaseID string `json:"provider_case_id"`
	Reason         string `json:"reason"`
	Status         string `json:"status"`
	AmountCents    int64  `json:"amount_cents"`
	Currency       string `json:"currency"`
}
