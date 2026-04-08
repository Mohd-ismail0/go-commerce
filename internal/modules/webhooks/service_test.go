package webhooks

import (
	"context"
	"testing"
)

func TestServiceRejectsInvalidURL(t *testing.T) {
	svc := NewService(&Repository{})
	_, err := svc.Save(context.Background(), Subscription{
		ID:          "whs_1",
		EventName:   "order.created",
		EndpointURL: "ftp://example.test",
	})
	if err == nil {
		t.Fatalf("expected invalid endpoint error")
	}
}
