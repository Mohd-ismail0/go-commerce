package payments

import (
	"context"
	"time"

	"rewrite/internal/shared/events"
)

const EventPaymentReconciliationIssue = "payment.reconciliation.issue"

type ReconciliationEvent struct {
	TenantID string               `json:"tenant_id"`
	RegionID string               `json:"region_id"`
	Items    []ReconciliationItem `json:"items"`
}

func (e ReconciliationEvent) GetTenantID() string { return e.TenantID }
func (e ReconciliationEvent) GetRegionID() string { return e.RegionID }

func StartReconciliationWorker(ctx context.Context, svc *Service, bus *events.Bus, tenantID, regionID string, interval time.Duration) {
	if svc == nil || bus == nil || interval <= 0 {
		return
	}
	t := time.NewTicker(interval)
	go func() {
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				report, err := svc.Reconcile(ctx, tenantID, regionID)
				if err != nil || len(report.Items) == 0 {
					continue
				}
				for _, item := range report.Items {
					_ = svc.UpsertReconciliationAction(ctx, tenantID, regionID, item)
				}
				bus.Publish(ctx, EventPaymentReconciliationIssue, ReconciliationEvent{
					TenantID: tenantID,
					RegionID: regionID,
					Items:    report.Items,
				})
			}
		}
	}()
}
