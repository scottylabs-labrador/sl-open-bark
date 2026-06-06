// Package tools holds the thin MCP handler. It maps input/output and calls the service. The tool
// is read-only and deterministic; its description is what the agent reads.
package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/scottylabs/scottylabs-agent/mcp-servers/finance-rules/internal/domain"
	"github.com/scottylabs/scottylabs-agent/mcp-servers/finance-rules/internal/service"
)

// Register wires the finance rules tool onto the server. The evaluator is injected (DI).
func Register(s *mcp.Server, ev *service.Evaluator) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "evaluate",
		Description: "Evaluate a parsed reimbursement request against ScottyLabs finance standards. " +
			"Returns a verdict (pass | fail | review): fail lists the specific failed standards with " +
			"evidence; review flags a borderline amount for a human. Read-only and deterministic — it " +
			"never approves or rejects money. scope: finance.read, impact: read.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in domain.ReimbursementRequest) (*mcp.CallToolResult, domain.Evaluation, error) {
		return nil, ev.Evaluate(in), nil
	})
}
