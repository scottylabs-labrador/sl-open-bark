// Package service holds use-case orchestration. It depends on the domain (pure functions) and
// on collaborator interfaces it defines itself (Go idiom: define interfaces at the consumer).
package service

import (
	"context"

	"github.com/scottylabs/scottylabs-mcp-example/internal/domain"
)

// AuditSink is the collaborator this service needs. Concrete implementations live in the
// clients package; defining the interface here keeps dependencies pointing inward and makes the
// service testable with a fake.
type AuditSink interface {
	Record(ctx context.Context, actor, action string, payload map[string]any) error
}

// Reimbursement orchestrates the reimbursement use case. Collaborators are injected (DI).
type Reimbursement struct {
	standards domain.Standards
	audit     AuditSink
}

func New(standards domain.Standards, audit AuditSink) *Reimbursement {
	return &Reimbursement{standards: standards, audit: audit}
}

// Evaluate runs the pure rules and records the outcome.
func (s *Reimbursement) Evaluate(ctx context.Context, r domain.ReimbursementRequest) (domain.Evaluation, error) {
	ev := domain.EvaluateReimbursement(r, s.standards)
	_ = s.audit.Record(ctx, "reimbursement-service", "evaluate",
		map[string]any{"request_id": r.RequestID, "verdict": string(ev.Verdict)})
	return ev, nil
}

// RecordReviewDecision records a human's decision on an edge case. The only side effect is the
// audit record; it never moves money. Any high-impact downstream action stays gated at the gateway.
func (s *Reimbursement) RecordReviewDecision(ctx context.Context, requestID, decision, decidedBy string) error {
	return s.audit.Record(ctx, decidedBy, "review_decision",
		map[string]any{"request_id": requestID, "decision": decision})
}
