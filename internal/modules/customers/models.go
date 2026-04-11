package customers

type Customer struct {
	ID       string `json:"id"`
	TenantID string `json:"tenant_id"`
	RegionID string `json:"region_id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
}

// Address is a customer-owned postal address (Saleor-style account address).
type Address struct {
	ID                string `json:"id"`
	TenantID          string `json:"tenant_id"`
	RegionID          string `json:"region_id"`
	CustomerID        string `json:"customer_id"`
	IsDefaultShipping bool   `json:"is_default_shipping"`
	IsDefaultBilling  bool   `json:"is_default_billing"`
	FirstName         string `json:"first_name"`
	LastName          string `json:"last_name"`
	Company           string `json:"company,omitempty"`
	StreetLine1       string `json:"street_line_1"`
	StreetLine2       string `json:"street_line_2,omitempty"`
	City              string `json:"city"`
	PostalCode        string `json:"postal_code"`
	CountryCode       string `json:"country_code"`
	Phone             string `json:"phone,omitempty"`
	CreatedAt         string `json:"created_at,omitempty"`
	UpdatedAt         string `json:"updated_at,omitempty"`
}
