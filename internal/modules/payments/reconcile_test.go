package payments

import "testing"

func TestDetectReconciliationIssues(t *testing.T) {
	p := Payment{ID: "p1", Status: StatusCaptured, AmountCents: 1000}
	issues := detectReconciliationIssues(p, 700, 800)
	if len(issues) == 0 {
		t.Fatalf("expected reconciliation issues")
	}
}
