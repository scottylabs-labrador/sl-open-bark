package service

import (
	"context"
	"testing"

	"github.com/scottylabs/scottylabs-mcp-example/internal/domain"
)

// fakeAudit is injected in place of a real client (dependency injection) so we can assert the
// side effect without any network.
type fakeAudit struct {
	calls      int
	lastAction string
}

func (f *fakeAudit) Record(_ context.Context, _, action string, _ map[string]any) error {
	f.calls++
	f.lastAction = action
	return nil
}

func testStandards() domain.Standards {
	return domain.Standards{
		EligibleCategories: []string{"food"},
		CategoryCapsUSD:    map[string]float64{"food": 250},
		MaxDaysToSubmit:    30,
		RequireReceipt:     true,
		RequireEventAssoc:  true,
		ReviewBand:         0.10,
	}
}

func TestEvaluateAudits(t *testing.T) {
	fa := &fakeAudit{}
	svc := New(testStandards(), fa)

	ev, err := svc.Evaluate(context.Background(), domain.ReimbursementRequest{
		RequestID:                  "r2",
		Category:                   "food",
		AmountUSD:                  20,
		HasItemizedReceipt:         true,
		EventAssociated:            true,
		SubmittedDaysAfterPurchase: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if ev.Verdict != domain.Pass {
		t.Fatalf("verdict = %s, want pass", ev.Verdict)
	}
	if fa.calls != 1 || fa.lastAction != "evaluate" {
		t.Fatalf("expected one evaluate audit, got calls=%d action=%s", fa.calls, fa.lastAction)
	}
}

func TestRecordReviewDecisionAudits(t *testing.T) {
	fa := &fakeAudit{}
	svc := New(testStandards(), fa)
	if err := svc.RecordReviewDecision(context.Background(), "r9", "pass", "lead@cmu.edu"); err != nil {
		t.Fatal(err)
	}
	if fa.lastAction != "review_decision" {
		t.Fatalf("want review_decision, got %s", fa.lastAction)
	}
}
