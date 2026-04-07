package catalog

import "sync"

type Repository struct {
	mu       sync.RWMutex
	products map[string]Product
}

func NewRepository() *Repository {
	return &Repository{products: map[string]Product{}}
}

func (r *Repository) Upsert(product Product) Product {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.products[product.ID] = product
	return product
}

func (r *Repository) List(tenantID string) []Product {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Product, 0, len(r.products))
	for _, p := range r.products {
		if p.TenantID == tenantID {
			out = append(out, p)
		}
	}
	return out
}
