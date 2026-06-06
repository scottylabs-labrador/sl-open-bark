package domain_test

import (
	"testing"

	"github.com/scottylabs/scottylabs-agent/mcp-servers/finance-rules/internal/domain"
	"github.com/scottylabs/scottylabs-agent/mcp-servers/finance-rules/internal/standards"
)

// TestSeededSheetSplits is the WP-08 reference check: a seeded "sheet" of reimbursement responses,
// evaluated against the real reviewed standards, produces the correct pass / fail / review splits —
// clear failures name their standard (for an auto-drafted return), edge cases are flagged for a
// human (never auto-decided), and clean requests pass.
func TestSeededSheetSplits(t *testing.T) {
	std, err := domain.LoadStandards(standards.Default())
	if err != nil {
		t.Fatal(err)
	}
	clean := func(id, cat string, amt float64) domain.ReimbursementRequest {
		return domain.ReimbursementRequest{RequestID: id, Category: cat, AmountUSD: amt,
			HasItemizedReceipt: true, EventAssociated: true, SubmittedDaysAfterPurchase: 5}
	}

	// The seeded sheet (10 rows) with the verdict each should receive.
	sheet := []struct {
		req         domain.ReimbursementRequest
		want        domain.Verdict
		wantFailing string // a standard expected for fail rows
	}{
		{clean("r1", "travel", 300), domain.Pass, ""},
		{clean("r2", "food", 20), domain.Pass, ""},
		{func() domain.ReimbursementRequest {
			r := clean("r3", "food", 20)
			r.HasItemizedReceipt = false
			return r
		}(), domain.Fail, "itemized_receipt"},
		{clean("r4", "travel", 600), domain.Fail, "category_cap"},
		{clean("r5", "alcohol", 50), domain.Fail, "eligible_category"},
		{func() domain.ReimbursementRequest {
			r := clean("r6", "supplies", 80)
			r.SubmittedDaysAfterPurchase = 45
			return r
		}(), domain.Fail, "submission_deadline"},
		{func() domain.ReimbursementRequest { r := clean("r7", "food", 20); r.EventAssociated = false; return r }(), domain.Fail, "event_association"},
		{clean("r8", "travel", 470), domain.Review, ""},
		{clean("r9", "printing", 150), domain.Review, ""},
		{clean("r10", "swag", 100), domain.Pass, ""},
	}

	counts := map[domain.Verdict]int{}
	for _, row := range sheet {
		ev := domain.EvaluateReimbursement(row.req, std)
		counts[ev.Verdict]++
		if ev.Verdict != row.want {
			t.Fatalf("%s: verdict = %s, want %s (%+v)", row.req.RequestID, ev.Verdict, row.want, ev)
		}
		switch ev.Verdict {
		case domain.Fail:
			if len(ev.FailedStandards) == 0 {
				t.Fatalf("%s: a returned request must name its failed standard", row.req.RequestID)
			}
			found := false
			for _, f := range ev.FailedStandards {
				if f.Standard == row.wantFailing {
					found = true
				}
				if f.Detail == "" || f.Detail == "ok" {
					t.Fatalf("%s: failed standard %q lacks evidence", row.req.RequestID, f.Standard)
				}
			}
			if !found {
				t.Fatalf("%s: expected failed standard %q, got %+v", row.req.RequestID, row.wantFailing, ev.FailedStandards)
			}
		case domain.Review:
			if len(ev.Notes) == 0 {
				t.Fatalf("%s: an edge case must carry a recommendation note for a human", row.req.RequestID)
			}
		case domain.Pass:
			if len(ev.FailedStandards) != 0 {
				t.Fatalf("%s: a pass must have no failed standards", row.req.RequestID)
			}
		}
	}

	if counts[domain.Pass] != 3 || counts[domain.Fail] != 5 || counts[domain.Review] != 2 {
		t.Fatalf("splits = pass:%d fail:%d review:%d, want 3/5/2", counts[domain.Pass], counts[domain.Fail], counts[domain.Review])
	}
}
