package events

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type Endpoint struct {
	Event string
	URL   string
}

type WebhookDispatcher struct {
	client    *http.Client
	mu        sync.RWMutex
	endpoints []Endpoint
}

func NewWebhookDispatcher(timeout time.Duration) *WebhookDispatcher {
	return &WebhookDispatcher{
		client: &http.Client{Timeout: timeout},
	}
}

func (d *WebhookDispatcher) Register(event, url string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.endpoints = append(d.endpoints, Endpoint{Event: event, URL: url})
}

func (d *WebhookDispatcher) Attach(bus *Bus) {
	for _, event := range []string{
		EventOrderCreated, EventOrderCompleted, EventProductUpdated, EventInventoryChange,
	} {
		eventName := event
		bus.Subscribe(eventName, func(ctx context.Context, payload any) {
			d.dispatch(ctx, eventName, payload)
		})
	}
}

func (d *WebhookDispatcher) dispatch(ctx context.Context, event string, payload any) {
	d.mu.RLock()
	targets := append([]Endpoint{}, d.endpoints...)
	d.mu.RUnlock()

	body, _ := json.Marshal(map[string]any{
		"event":   event,
		"payload": payload,
		"sent_at": time.Now().UTC(),
	})

	for _, target := range targets {
		if target.Event != event {
			continue
		}
		go func(url string) {
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
			if err != nil {
				return
			}
			req.Header.Set("Content-Type", "application/json")
			_, _ = d.client.Do(req)
		}(target.URL)
	}
}
