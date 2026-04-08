package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"rewrite/internal/modules/payments"
	"rewrite/internal/shared/db"
)

// Exercise capture + idempotency against a real Postgres when RUN_INTEGRATION=1 and DATABASE_URL is set.
func TestPaymentCaptureLifecycle(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION") != "1" {
		t.Skip("set RUN_INTEGRATION=1 and DATABASE_URL to run integration tests")
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is required for integration tests")
	}
	ctx := context.Background()
	conn, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	repo := payments.NewRepository(conn)
	svc := payments.NewService(repo)

	payID := fmt.Sprintf("pay_integration_test_%d", time.Now().UnixNano())
	const tenant = "public"
	const region = "global"

	_, _ = conn.ExecContext(ctx, `DELETE FROM payment_transactions WHERE payment_id = $1`, payID)
	_, _ = conn.ExecContext(ctx, `DELETE FROM payments WHERE id = $1`, payID)

	_, err = repo.Save(ctx, payments.Payment{
		ID:          payID,
		TenantID:    tenant,
		RegionID:    region,
		Provider:    "test",
		Status:      payments.StatusAuthorized,
		AmountCents: 500,
		Currency:    "USD",
	})
	if err != nil {
		t.Fatalf("seed payment: %v", err)
	}

	res1, err := svc.Capture(ctx, tenant, payID, payments.AmountActionInput{}, "idem-capture-1")
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if res1.Payment.Status != payments.StatusCaptured {
		t.Fatalf("expected captured, got %s", res1.Payment.Status)
	}

	res2, err := svc.Capture(ctx, tenant, payID, payments.AmountActionInput{}, "idem-capture-1")
	if err != nil {
		t.Fatalf("capture idempotent: %v", err)
	}
	if res2.Transaction.ID != res1.Transaction.ID {
		t.Fatalf("expected same transaction on replay")
	}
}
