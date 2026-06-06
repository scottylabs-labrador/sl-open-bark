package domain_test

import (
	"testing"

	"github.com/scottylabs/scottylabs-agent/mcp-servers/finance-rules/internal/domain"
	"github.com/scottylabs/scottylabs-agent/mcp-servers/finance-rules/internal/standards"
)

func TestLoadEmbeddedStandards(t *testing.T) {
	s, err := domain.LoadStandards(standards.Default())
	if err != nil {
		t.Fatalf("embedded standards must parse: %v", err)
	}
	if len(s.EligibleCategories) == 0 || s.MaxDaysToSubmit <= 0 {
		t.Fatalf("embedded standards look empty: %+v", s)
	}
	// A clear over-cap travel request fails against the real, reviewed standards.
	ev := domain.EvaluateReimbursement(domain.ReimbursementRequest{
		Category: "travel", AmountUSD: s.CategoryCapsUSD["travel"] + 100,
		HasItemizedReceipt: true, EventAssociated: true, SubmittedDaysAfterPurchase: 1,
	}, s)
	if ev.Verdict != domain.Fail {
		t.Fatalf("over-cap travel should fail under embedded standards, got %s", ev.Verdict)
	}
}

func TestLoadStandardsRejectsInvalid(t *testing.T) {
	cases := map[string]string{
		"not json":         "{not json",
		"no categories":    `{"eligible_categories":[],"max_days_to_submit":30,"review_band":0.1}`,
		"bad review band":  `{"eligible_categories":["food"],"max_days_to_submit":30,"review_band":1.5}`,
		"nonpositive days": `{"eligible_categories":["food"],"max_days_to_submit":0,"review_band":0.1}`,
		"negative review":  `{"eligible_categories":["food"],"max_days_to_submit":30,"review_band":-0.1}`,
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := domain.LoadStandards([]byte(doc)); err == nil {
				t.Fatalf("expected %s to be rejected", name)
			}
		})
	}
}
