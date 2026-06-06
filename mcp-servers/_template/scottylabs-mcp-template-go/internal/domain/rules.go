package domain

import "fmt"

// Each check is a PURE FUNCTION of (request, standards): deterministic, no I/O, easy to test.
// This is the "rules as code" pattern: the model parses and explains, the code decides.

func checkCategoryEligible(r ReimbursementRequest, s Standards) RuleResult {
	for _, c := range s.EligibleCategories {
		if c == r.Category {
			return RuleResult{Standard: "eligible_category", Passed: true, Detail: "ok"}
		}
	}
	return RuleResult{Standard: "eligible_category", Passed: false,
		Detail: fmt.Sprintf("category %q is not eligible", r.Category)}
}

func checkReceipt(r ReimbursementRequest, s Standards) RuleResult {
	ok := !s.RequireReceipt || r.HasItemizedReceipt
	detail := "ok"
	if !ok {
		detail = "missing itemized receipt"
	}
	return RuleResult{Standard: "itemized_receipt", Passed: ok, Detail: detail}
}

func checkEventAssociation(r ReimbursementRequest, s Standards) RuleResult {
	ok := !s.RequireEventAssoc || r.EventAssociated
	detail := "ok"
	if !ok {
		detail = "no associated event"
	}
	return RuleResult{Standard: "event_association", Passed: ok, Detail: detail}
}

func checkDeadline(r ReimbursementRequest, s Standards) RuleResult {
	ok := r.SubmittedDaysAfterPurchase <= s.MaxDaysToSubmit
	detail := "ok"
	if !ok {
		detail = fmt.Sprintf("submitted %d days after purchase (max %d)",
			r.SubmittedDaysAfterPurchase, s.MaxDaysToSubmit)
	}
	return RuleResult{Standard: "submission_deadline", Passed: ok, Detail: detail}
}

func checkCategoryCap(r ReimbursementRequest, s Standards) RuleResult {
	limit, ok := s.CategoryCapsUSD[r.Category]
	if !ok {
		return RuleResult{Standard: "category_cap", Passed: true, Detail: "no cap for category"}
	}
	if r.AmountUSD <= limit {
		return RuleResult{Standard: "category_cap", Passed: true, Detail: "ok"}
	}
	return RuleResult{Standard: "category_cap", Passed: false,
		Detail: fmt.Sprintf("amount $%.2f exceeds cap $%.2f", r.AmountUSD, limit)}
}

// checks is the ordered rule set. Add a rule by appending a pure function.
var checks = []func(ReimbursementRequest, Standards) RuleResult{
	checkCategoryEligible,
	checkReceipt,
	checkEventAssociation,
	checkDeadline,
	checkCategoryCap,
}

func isBorderline(r ReimbursementRequest, s Standards) bool {
	limit, ok := s.CategoryCapsUSD[r.Category]
	if !ok {
		return false
	}
	return r.AmountUSD >= limit*(1-s.ReviewBand) && r.AmountUSD <= limit
}

// EvaluateReimbursement returns Pass, Fail (with the specific failed standards), or Review.
func EvaluateReimbursement(r ReimbursementRequest, s Standards) Evaluation {
	var failed []RuleResult
	for _, check := range checks {
		if res := check(r, s); !res.Passed {
			failed = append(failed, res)
		}
	}
	if len(failed) > 0 {
		return Evaluation{RequestID: r.RequestID, Verdict: Fail, FailedStandards: failed}
	}
	if isBorderline(r, s) {
		return Evaluation{RequestID: r.RequestID, Verdict: Review,
			Notes: []string{"amount is within the review band of the category cap; needs a human"}}
	}
	return Evaluation{RequestID: r.RequestID, Verdict: Pass}
}
