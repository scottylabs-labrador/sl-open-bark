// Package tools holds the thin MCP handlers. They map inputs and outputs and delegate to a
// service. No business logic lives here. Tool descriptions are what the agent reads, so keep
// them accurate and include scope and impact.
package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/scottylabs/scottylabs-mcp-example/internal/domain"
	"github.com/scottylabs/scottylabs-mcp-example/internal/service"
)

type recordDecisionInput struct {
	RequestID string `json:"request_id" jsonschema:"the reimbursement request id"`
	Decision  string `json:"decision" jsonschema:"pass, fail, or review"`
	DecidedBy string `json:"decided_by" jsonschema:"email of the human who decided"`
}

type recordDecisionOutput struct {
	RequestID string `json:"request_id"`
	Decision  string `json:"decision"`
	Recorded  bool   `json:"recorded"`
}

// Register wires this capability's tools onto the server. The service is injected (DI) so the
// handlers stay thin and the composition root owns the wiring.
func Register(s *mcp.Server, svc *service.Reimbursement) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "evaluate_reimbursement",
		Description: "Evaluate a reimbursement request against ScottyLabs finance standards. " +
			"Returns a verdict (pass | fail | review) with the specific failed standards. " +
			"Read-only and deterministic. scope: finance.read, impact: read.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in domain.ReimbursementRequest) (*mcp.CallToolResult, domain.Evaluation, error) {
		ev, err := svc.Evaluate(ctx, in)
		return nil, ev, err
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "record_review_decision",
		Description: "Record a human's decision on an edge-case reimbursement. Writes an audit " +
			"record only; it does not move money or email anyone. scope: finance.write, impact: write.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in recordDecisionInput) (*mcp.CallToolResult, recordDecisionOutput, error) {
		if err := svc.RecordReviewDecision(ctx, in.RequestID, in.Decision, in.DecidedBy); err != nil {
			return nil, recordDecisionOutput{}, err
		}
		return nil, recordDecisionOutput{RequestID: in.RequestID, Decision: in.Decision, Recorded: true}, nil
	})
}
