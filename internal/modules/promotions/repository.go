package promotions

import "sync"

type Repository struct {
	mu    sync.RWMutex
	items map[string]Promotion
}

func NewRepository() *Repository {
	return &Repository{items: map[string]Promotion{}}
}

func (r *Repository) Save(item Promotion) Promotion {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[item.ID] = item
	return item
}

func (r *Repository) List(tenantID string) []Promotion {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Promotion, 0, len(r.items))
	for _, p := range r.items {
		if p.TenantID == tenantID {
			out = append(out, p)
		}
	}
	return out
}
