package domain

import "testing"

var testStandards = Standards{
	EligibleCategories: []string{"travel", "food", "supplies"},
	CategoryCapsUSD:    map[string]float64{"travel": 500, "food": 250, "supplies": 300},
	MaxDaysToSubmit:    30,
	RequireReceipt:     true,
	RequireEventAssoc:  true,
	ReviewBand:         0.10,
}

func baseRequest() ReimbursementRequest {
	return ReimbursementRequest{
		RequestID:                  "r1",
		SubmitterEmail:             "a@cmu.edu",
		AmountUSD:                  100,
		Category:                   "food",
		HasItemizedReceipt:         true,
		EventAssociated:            true,
		SubmittedDaysAfterPurchase: 5,
	}
}

func TestEvaluateReimbursement(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*ReimbursementRequest)
		want   Verdict
	}{
		{"clean", func(r *ReimbursementRequest) {}, Pass},
		{"missing_receipt", func(r *ReimbursementRequest) { r.HasItemizedReceipt = false }, Fail},
		{"over_cap", func(r *ReimbursementRequest) { r.Category = "travel"; r.AmountUSD = 600 }, Fail},
		{"ineligible_category", func(r *ReimbursementRequest) { r.Category = "alcohol" }, Fail},
		{"late_submission", func(r *ReimbursementRequest) { r.SubmittedDaysAfterPurchase = 45 }, Fail},
		{"borderline", func(r *ReimbursementRequest) { r.Category = "travel"; r.AmountUSD = 470 }, Review},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := baseRequest()
			c.mutate(&r)
			got := EvaluateReimbursement(r, testStandards).Verdict
			if got != c.want {
				t.Fatalf("verdict = %s, want %s", got, c.want)
			}
		})
	}
}

func TestFailureListsAllBrokenStandards(t *testing.T) {
	r := baseRequest()
	r.Category = "travel"
	r.AmountUSD = 600
	r.HasItemizedReceipt = false
	ev := EvaluateReimbursement(r, testStandards)
	if ev.Verdict != Fail {
		t.Fatalf("want fail, got %s", ev.Verdict)
	}
	broken := map[string]bool{}
	for _, rr := range ev.FailedStandards {
		broken[rr.Standard] = true
	}
	if !broken["category_cap"] || !broken["itemized_receipt"] {
		t.Fatalf("expected cap and receipt failures, got %v", broken)
	}
}
