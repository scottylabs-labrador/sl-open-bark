// Package service holds the (thin) use-case orchestration. The finance evaluation is pure, so this
// layer mostly holds the loaded standards and delegates to the domain. Audit is centralized at the
// gateway, so the server does not duplicate it.
package service

import (
	"github.com/scottylabs/scottylabs-agent/mcp-servers/finance-rules/internal/domain"
)

// Evaluator screens reimbursement requests against a fixed set of standards.
type Evaluator struct {
	standards domain.Standards
}

// New builds an Evaluator over reviewed standards.
func New(s domain.Standards) *Evaluator { return &Evaluator{standards: s} }

// Evaluate returns the deterministic verdict for a request.
func (e *Evaluator) Evaluate(r domain.ReimbursementRequest) domain.Evaluation {
	return domain.EvaluateReimbursement(r, e.standards)
}
