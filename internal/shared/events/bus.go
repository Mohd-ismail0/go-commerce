package events

import (
	"context"
	"sync"
)

const (
	EventOrderCreated    = "order.created"
	EventOrderCompleted  = "order.completed"
	EventProductUpdated  = "product.updated"
	EventInventoryChange = "inventory.changed"
)

type Handler func(ctx context.Context, payload any)

type Bus struct {
	mu     sync.RWMutex
	subs   map[string][]Handler
	outbox *OutboxStore
}

func NewBus() *Bus {
	return &Bus{subs: map[string][]Handler{}}
}

func NewBusWithOutbox(outbox *OutboxStore) *Bus {
	return &Bus{subs: map[string][]Handler{}, outbox: outbox}
}

func (b *Bus) Subscribe(event string, handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs[event] = append(b.subs[event], handler)
}

func (b *Bus) Publish(ctx context.Context, event string, payload any) {
	if b.outbox != nil {
		b.outbox.Enqueue(ctx, tenantIDFromPayload(payload), regionIDFromPayload(payload), event, "domain", "", payload)
	}
	b.mu.RLock()
	handlers := append([]Handler{}, b.subs[event]...)
	b.mu.RUnlock()

	for _, h := range handlers {
		// Non-blocking dispatch keeps request paths fast.
		go h(ctx, payload)
	}
}

func tenantIDFromPayload(payload any) string {
	if p, ok := payload.(interface{ GetTenantID() string }); ok {
		return p.GetTenantID()
	}
	return "public"
}

func regionIDFromPayload(payload any) string {
	if p, ok := payload.(interface{ GetRegionID() string }); ok {
		return p.GetRegionID()
	}
	return "global"
}
