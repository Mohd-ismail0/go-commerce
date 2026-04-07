package pricing

import "sync"

type Repository struct {
	mu      sync.RWMutex
	entries map[string]PriceBookEntry
}

func NewRepository() *Repository {
	return &Repository{entries: map[string]PriceBookEntry{}}
}

func (r *Repository) Save(entry PriceBookEntry) PriceBookEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[entry.ID] = entry
	return entry
}

func (r *Repository) List(tenantID string) []PriceBookEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]PriceBookEntry, 0, len(r.entries))
	for _, e := range r.entries {
		if e.TenantID == tenantID {
			out = append(out, e)
		}
	}
	return out
}
