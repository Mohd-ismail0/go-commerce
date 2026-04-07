package promotions

type Promotion struct {
	ID         string `json:"id"`
	TenantID   string `json:"tenant_id"`
	RegionID   string `json:"region_id"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	ValueCents int64  `json:"value_cents"`
}

type PromotionRule struct {
	ID          string `json:"id"`
	TenantID    string `json:"tenant_id"`
	RegionID    string `json:"region_id"`
	PromotionID string `json:"promotion_id"`
	RuleType    string `json:"rule_type"`
	ValueCents  int64  `json:"value_cents"`
	Currency    string `json:"currency"`
}

type Voucher struct {
	ID           string `json:"id"`
	TenantID     string `json:"tenant_id"`
	RegionID     string `json:"region_id"`
	Code         string `json:"code"`
	DiscountType string `json:"discount_type"`
	ValueCents   int64  `json:"value_cents"`
	Currency     string `json:"currency"`
	UsageLimit   *int64 `json:"usage_limit,omitempty"`
	UsedCount    int64  `json:"used_count"`
	StartsAt     string `json:"starts_at,omitempty"`
	EndsAt       string `json:"ends_at,omitempty"`
}

type EligibilityInput struct {
	TenantID      string
	RegionID      string
	Currency      string
	BaseAmount    int64
	VoucherCode   string
	PromotionID   string
	ReferenceTime string
}
