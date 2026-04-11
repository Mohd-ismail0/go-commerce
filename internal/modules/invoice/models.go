package invoice

import "encoding/json"

// Invoice is an order-linked billing document (Saleor invoice subset; no PDF pipeline).
type Invoice struct {
	ID            string          `json:"id"`
	TenantID      string          `json:"tenant_id"`
	RegionID      string          `json:"region_id"`
	OrderID       string          `json:"order_id"`
	InvoiceNumber string          `json:"invoice_number"`
	Status        string          `json:"status"`
	TotalCents    int64           `json:"total_cents"`
	Currency      string          `json:"currency"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
	CreatedAt     string          `json:"created_at,omitempty"`
	UpdatedAt     string          `json:"updated_at,omitempty"`
}
