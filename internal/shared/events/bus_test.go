package events

import (
	"context"
	"testing"
	"time"
)

func TestPublishIsNonBlocking(t *testing.T) {
	bus := NewBus()
	bus.Subscribe(EventOrderCreated, func(_ context.Context, _ any) {
		time.Sleep(100 * time.Millisecond)
	})

	start := time.Now()
	bus.Publish(context.Background(), EventOrderCreated, map[string]string{"id": "ord_1"})
	elapsed := time.Since(start)
	if elapsed > 25*time.Millisecond {
		t.Fatalf("publish should be non-blocking, took %s", elapsed)
	}
}
