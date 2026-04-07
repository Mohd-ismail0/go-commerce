package orders

import "sync"

type Repository struct {
	mu     sync.RWMutex
	orders map[string]Order
}

func NewRepository() *Repository {
	return &Repository{orders: map[string]Order{}}
}

func (r *Repository) Save(order Order) Order {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.orders[order.ID] = order
	return order
}

func (r *Repository) List(tenantID string) []Order {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Order, 0, len(r.orders))
	for _, o := range r.orders {
		if o.TenantID == tenantID {
			out = append(out, o)
		}
	}
	return out
}
