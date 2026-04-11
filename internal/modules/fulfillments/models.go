package fulfillments

import "rewrite/internal/modules/orders"

// CreateResult is returned when a fulfillment is created (or loaded from idempotency replay).
type CreateResult struct {
	Fulfillment           Fulfillment
	FinalOrder            orders.Order
	FromIdempotencyReplay bool
	// EmitOrderCompleted is true when this request applied a transition into completed from a non-completed status.
	EmitOrderCompleted bool
}

type Fulfillment struct {
	ID             string `json:"id"`
	TenantID       string `json:"tenant_id"`
	RegionID       string `json:"region_id"`
	OrderID        string `json:"order_id"`
	Status         string `json:"status"`
	TrackingNumber string `json:"tracking_number,omitempty"`
	UpdatedAt      string `json:"updated_at,omitempty"`
}
