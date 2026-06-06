// Package domain holds the finance rules as typed models and PURE FUNCTIONS. No I/O. This is the
// "rules as code" core (design 9.2): the model parses messy input and writes explanations; this
// code decides pass/fail/review deterministically, so the auto-return is auditable and testable.
package domain

import (
	"encoding/json"
	"fmt"
)

// Verdict is the outcome of evaluating a request.
type Verdict string

const (
	Pass   Verdict = "pass"
	Fail   Verdict = "fail"
	Review Verdict = "review"
)

// ReimbursementRequest is a parsed request. Parsing the messy form input happens upstream in the
// agent; json tags drive the MCP tool's input schema.
type ReimbursementRequest struct {
	RequestID                  string  `json:"request_id" jsonschema:"unique id of the request"`
	SubmitterEmail             string  `json:"submitter_email" jsonschema:"who submitted it"`
	AmountUSD                  float64 `json:"amount_usd" jsonschema:"requested amount in USD"`
	Category                   string  `json:"category" jsonschema:"expense category, e.g. travel"`
	HasItemizedReceipt         bool    `json:"has_itemized_receipt" jsonschema:"whether an itemized receipt is attached"`
	EventAssociated            bool    `json:"event_associated" jsonschema:"whether tied to a ScottyLabs event"`
	SubmittedDaysAfterPurchase int     `json:"submitted_days_after_purchase" jsonschema:"days between purchase and submission"`
}

// Standards is the finance policy, as data (loaded from a reviewed file, not hardcoded).
type Standards struct {
	EligibleCategories []string           `json:"eligible_categories"`
	CategoryCapsUSD    map[string]float64 `json:"category_caps_usd"`
	MaxDaysToSubmit    int                `json:"max_days_to_submit"`
	RequireReceipt     bool               `json:"require_receipt"`
	RequireEventAssoc  bool               `json:"require_event_assoc"`
	// ReviewBand: amounts within this fraction below the cap go to a human instead of auto-deciding.
	ReviewBand float64 `json:"review_band"`
}

// RuleResult is the outcome of one check, with evidence.
type RuleResult struct {
	Standard string `json:"standard"`
	Passed   bool   `json:"passed"`
	Detail   string `json:"detail"`
}

// Evaluation is the full result returned to the agent: a verdict, the standards that failed (with
// evidence), and any notes (for the review band).
type Evaluation struct {
	RequestID       string       `json:"request_id"`
	Verdict         Verdict      `json:"verdict"`
	FailedStandards []RuleResult `json:"failed_standards"`
	Notes           []string     `json:"notes"`
}

// LoadStandards parses and validates a standards JSON document. Pure function — no I/O. Unknown
// fields (like the "_comment" key in the reviewed file) are ignored.
func LoadStandards(data []byte) (Standards, error) {
	var s Standards
	if err := json.Unmarshal(data, &s); err != nil {
		return Standards{}, fmt.Errorf("domain: parse standards: %w", err)
	}
	if len(s.EligibleCategories) == 0 {
		return Standards{}, fmt.Errorf("domain: standards must list eligible_categories")
	}
	if s.MaxDaysToSubmit <= 0 {
		return Standards{}, fmt.Errorf("domain: standards max_days_to_submit must be positive")
	}
	if s.ReviewBand < 0 || s.ReviewBand >= 1 {
		return Standards{}, fmt.Errorf("domain: standards review_band must be in [0, 1)")
	}
	return s, nil
}
