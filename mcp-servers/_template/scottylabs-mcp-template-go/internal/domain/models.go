// Package domain holds typed models and pure-function business rules. No I/O lives here.
package domain

// Verdict is the outcome of evaluating a request.
type Verdict string

const (
	Pass   Verdict = "pass"
	Fail   Verdict = "fail"
	Review Verdict = "review"
)

// ReimbursementRequest is a parsed request. The messy parsing happens upstream in the agent;
// json tags drive the MCP tool's input schema.
type ReimbursementRequest struct {
	RequestID                  string  `json:"request_id" jsonschema:"unique id of the request"`
	SubmitterEmail             string  `json:"submitter_email"`
	AmountUSD                  float64 `json:"amount_usd"`
	Category                   string  `json:"category"`
	HasItemizedReceipt         bool    `json:"has_itemized_receipt"`
	EventAssociated            bool    `json:"event_associated"`
	SubmittedDaysAfterPurchase int     `json:"submitted_days_after_purchase"`
}

// Standards is the finance policy, as data. Version-controlled and reviewable.
type Standards struct {
	EligibleCategories []string
	CategoryCapsUSD    map[string]float64
	MaxDaysToSubmit    int
	RequireReceipt     bool
	RequireEventAssoc  bool
	// ReviewBand: amounts within this fraction of the cap go to a human instead of auto-deciding.
	ReviewBand float64
}

// RuleResult is the outcome of one check.
type RuleResult struct {
	Standard string `json:"standard"`
	Passed   bool   `json:"passed"`
	Detail   string `json:"detail"`
}

// Evaluation is the full result returned to the agent.
type Evaluation struct {
	RequestID       string       `json:"request_id"`
	Verdict         Verdict      `json:"verdict"`
	FailedStandards []RuleResult `json:"failed_standards"`
	Notes           []string     `json:"notes"`
}

// DefaultStandards returns example standards. In a real server, load these from a reviewed file.
func DefaultStandards() Standards {
	return Standards{
		EligibleCategories: []string{"travel", "food", "supplies", "swag"},
		CategoryCapsUSD:    map[string]float64{"travel": 500, "food": 250, "supplies": 300, "swag": 400},
		MaxDaysToSubmit:    30,
		RequireReceipt:     true,
		RequireEventAssoc:  true,
		ReviewBand:         0.10,
	}
}
