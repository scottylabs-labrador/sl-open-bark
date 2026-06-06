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

// The design Section 9 matrix: every clear-failure standard, the review band, and clean passes.
func TestEvaluateMatrix(t *testing.T) {
	cases := []struct {
		name      string
		mutate    func(*ReimbursementRequest)
		want      Verdict
		wantBroke string // a standard expected in FailedStandards (for Fail cases)
	}{
		{"clean_pass", func(r *ReimbursementRequest) {}, Pass, ""},
		{"missing_receipt", func(r *ReimbursementRequest) { r.HasItemizedReceipt = false }, Fail, "itemized_receipt"},
		{"over_cap", func(r *ReimbursementRequest) { r.Category = "travel"; r.AmountUSD = 600 }, Fail, "category_cap"},
		{"ineligible_category", func(r *ReimbursementRequest) { r.Category = "alcohol" }, Fail, "eligible_category"},
		{"late_submission", func(r *ReimbursementRequest) { r.SubmittedDaysAfterPurchase = 45 }, Fail, "submission_deadline"},
		{"no_event_association", func(r *ReimbursementRequest) { r.EventAssociated = false }, Fail, "event_association"},
		{"borderline_review", func(r *ReimbursementRequest) { r.Category = "travel"; r.AmountUSD = 470 }, Review, ""},
		{"exactly_at_cap_review", func(r *ReimbursementRequest) { r.Category = "travel"; r.AmountUSD = 500 }, Review, ""},
		{"just_below_band_pass", func(r *ReimbursementRequest) { r.Category = "travel"; r.AmountUSD = 400 }, Pass, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := baseRequest()
			c.mutate(&r)
			ev := EvaluateReimbursement(r, testStandards)
			if ev.Verdict != c.want {
				t.Fatalf("verdict = %s, want %s (%+v)", ev.Verdict, c.want, ev)
			}
			if c.wantBroke != "" && !brokeStandard(ev, c.wantBroke) {
				t.Fatalf("expected %q in failed standards, got %+v", c.wantBroke, ev.FailedStandards)
			}
			if c.want == Fail && len(ev.FailedStandards) == 0 {
				t.Fatal("a Fail verdict must list the failed standards (explainable)")
			}
			if c.want == Review && len(ev.Notes) == 0 {
				t.Fatal("a Review verdict should carry a note for the human")
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
	if !brokeStandard(ev, "category_cap") || !brokeStandard(ev, "itemized_receipt") {
		t.Fatalf("expected cap and receipt failures, got %+v", ev.FailedStandards)
	}
	// Every failed standard carries evidence.
	for _, f := range ev.FailedStandards {
		if f.Detail == "" || f.Detail == "ok" {
			t.Fatalf("failed standard %q lacks evidence: %+v", f.Standard, f)
		}
	}
}

// An eligible category with no configured cap passes on amount (no cap to exceed, not borderline).
func TestUncappedEligibleCategory(t *testing.T) {
	s := testStandards
	s.EligibleCategories = append([]string{"stickers"}, s.EligibleCategories...)
	r := baseRequest()
	r.Category = "stickers"
	r.AmountUSD = 9999
	if ev := EvaluateReimbursement(r, s); ev.Verdict != Pass {
		t.Fatalf("uncapped eligible category should pass, got %s (%+v)", ev.Verdict, ev)
	}
}

func TestDeterministic(t *testing.T) {
	r := baseRequest()
	r.Category = "travel"
	r.AmountUSD = 470
	first := EvaluateReimbursement(r, testStandards).Verdict
	for i := 0; i < 100; i++ {
		if got := EvaluateReimbursement(r, testStandards).Verdict; got != first {
			t.Fatalf("non-deterministic verdict: %s vs %s", got, first)
		}
	}
}

func brokeStandard(ev Evaluation, standard string) bool {
	for _, f := range ev.FailedStandards {
		if f.Standard == standard {
			return true
		}
	}
	return false
}
