package inventory

import "sync"

type Repository struct {
	mu    sync.RWMutex
	items map[string]StockItem
}

func NewRepository() *Repository {
	return &Repository{items: map[string]StockItem{}}
}

func (r *Repository) Save(item StockItem) StockItem {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[item.ID] = item
	return item
}

func (r *Repository) List(tenantID string) []StockItem {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]StockItem, 0, len(r.items))
	for _, i := range r.items {
		if i.TenantID == tenantID {
			out = append(out, i)
		}
	}
	return out
}
