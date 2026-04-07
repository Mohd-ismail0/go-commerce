package pricing

type PriceBookEntry struct {
	ID          string `json:"id"`
	TenantID    string `json:"tenant_id"`
	RegionID    string `json:"region_id"`
	ProductID   string `json:"product_id"`
	Currency    string `json:"currency"`
	AmountCents int64  `json:"amount_cents"`
}

type TaxClass struct {
	ID       string `json:"id"`
	TenantID string `json:"tenant_id"`
	RegionID string `json:"region_id"`
	Name     string `json:"name"`
}

type TaxRate struct {
	ID              string `json:"id"`
	TenantID        string `json:"tenant_id"`
	RegionID        string `json:"region_id"`
	TaxClassID      string `json:"tax_class_id"`
	CountryCode     string `json:"country_code"`
	RateBasisPoints int64  `json:"rate_basis_points"`
}

type CalculationInput struct {
	TenantID        string `json:"tenant_id"`
	RegionID        string `json:"region_id"`
	Currency        string `json:"currency"`
	BaseAmountCents int64  `json:"base_amount_cents"`
	VoucherCode     string `json:"voucher_code,omitempty"`
	PromotionID     string `json:"promotion_id,omitempty"`
	TaxClassID      string `json:"tax_class_id,omitempty"`
	CountryCode     string `json:"country_code,omitempty"`
}

type CalculationResult struct {
	BaseAmountCents int64 `json:"base_amount_cents"`
	DiscountCents   int64 `json:"discount_cents"`
	TaxCents        int64 `json:"tax_cents"`
	TotalCents      int64 `json:"total_cents"`
}
