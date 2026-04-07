package regions

import "sync"

type Repository struct {
	mu    sync.RWMutex
	items map[string]Region
}

func NewRepository() *Repository {
	return &Repository{items: map[string]Region{}}
}

func (r *Repository) Save(item Region) Region {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[item.ID] = item
	return item
}

func (r *Repository) List(tenantID string) []Region {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Region, 0, len(r.items))
	for _, i := range r.items {
		if i.TenantID == tenantID {
			out = append(out, i)
		}
	}
	return out
}
