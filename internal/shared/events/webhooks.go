package events

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type Endpoint struct {
	ID    string
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
	d.endpoints = append(d.endpoints, Endpoint{ID: "", Event: event, URL: url})
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

type WebhookDeliveryInput struct {
	SubscriptionID string
	URL            string
}

type WebhookDeliveryResult struct {
	SubscriptionID string
	StatusCode     int
	ResponseBody   string
	Err            error
}

func (d *WebhookDispatcher) Deliver(ctx context.Context, event string, payload any, targets []WebhookDeliveryInput) []WebhookDeliveryResult {
	body, err := json.Marshal(map[string]any{
		"event":   event,
		"payload": payload,
		"sent_at": time.Now().UTC(),
	})
	if err != nil {
		return []WebhookDeliveryResult{{
			Err: fmt.Errorf("marshal webhook payload: %w", err),
		}}
	}
	out := make([]WebhookDeliveryResult, 0, len(targets))
	for _, target := range targets {
		result := WebhookDeliveryResult{SubscriptionID: target.SubscriptionID}
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, target.URL, bytes.NewReader(body))
		if reqErr != nil {
			result.Err = reqErr
			out = append(out, result)
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		resp, doErr := d.client.Do(req)
		if doErr != nil {
			result.Err = doErr
			out = append(out, result)
			continue
		}
		responseBody, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			result.Err = readErr
		}
		result.StatusCode = resp.StatusCode
		result.ResponseBody = string(responseBody)
		out = append(out, result)
	}
	return out
}
