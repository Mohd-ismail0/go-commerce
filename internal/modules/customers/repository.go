package customers

import "sync"

type Repository struct {
	mu        sync.RWMutex
	customers map[string]Customer
}

func NewRepository() *Repository {
	return &Repository{customers: map[string]Customer{}}
}

func (r *Repository) Save(customer Customer) Customer {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.customers[customer.ID] = customer
	return customer
}

func (r *Repository) List(tenantID string) []Customer {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Customer, 0, len(r.customers))
	for _, c := range r.customers {
		if c.TenantID == tenantID {
			out = append(out, c)
		}
	}
	return out
}
