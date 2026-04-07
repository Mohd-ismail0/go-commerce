package events

import (
	"context"
	"time"
)

type Worker struct {
	store      *OutboxStore
	dispatcher *WebhookDispatcher
}

func NewWorker(store *OutboxStore, dispatcher *WebhookDispatcher) *Worker {
	return &Worker{store: store, dispatcher: dispatcher}
}

func (w *Worker) Start(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.tick(ctx)
			}
		}
	}()
}

func (w *Worker) tick(ctx context.Context) {
	events, err := w.store.DequeuePending(ctx, 50)
	if err != nil {
		return
	}
	for _, item := range events {
		w.dispatcher.dispatch(ctx, item.EventName, map[string]any{"raw": item.Payload})
		w.store.MarkDone(ctx, item.ID)
	}
}
