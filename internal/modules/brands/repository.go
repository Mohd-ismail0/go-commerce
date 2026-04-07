package brands

import "sync"

type Repository struct {
	mu     sync.RWMutex
	brands map[string]Brand
}

func NewRepository() *Repository {
	return &Repository{brands: map[string]Brand{}}
}

func (r *Repository) Save(brand Brand) Brand {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.brands[brand.ID] = brand
	return brand
}

func (r *Repository) List(tenantID string) []Brand {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Brand, 0, len(r.brands))
	for _, b := range r.brands {
		if b.TenantID == tenantID {
			out = append(out, b)
		}
	}
	return out
}
