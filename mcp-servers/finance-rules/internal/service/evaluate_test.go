package service_test

import (
	"testing"

	"github.com/scottylabs/scottylabs-agent/mcp-servers/finance-rules/internal/domain"
	"github.com/scottylabs/scottylabs-agent/mcp-servers/finance-rules/internal/service"
	"github.com/scottylabs/scottylabs-agent/mcp-servers/finance-rules/internal/standards"
)

func TestEvaluatorUsesLoadedStandards(t *testing.T) {
	s, err := domain.LoadStandards(standards.Default())
	if err != nil {
		t.Fatal(err)
	}
	ev := service.New(s)

	pass := ev.Evaluate(domain.ReimbursementRequest{
		RequestID: "ok", Category: "food", AmountUSD: 10,
		HasItemizedReceipt: true, EventAssociated: true, SubmittedDaysAfterPurchase: 2,
	})
	if pass.Verdict != domain.Pass {
		t.Fatalf("clean request should pass, got %s", pass.Verdict)
	}

	fail := ev.Evaluate(domain.ReimbursementRequest{
		RequestID: "bad", Category: "food", AmountUSD: 10,
		HasItemizedReceipt: false, EventAssociated: true, SubmittedDaysAfterPurchase: 2,
	})
	if fail.Verdict != domain.Fail || fail.RequestID != "bad" {
		t.Fatalf("missing-receipt request should fail, got %+v", fail)
	}
}
