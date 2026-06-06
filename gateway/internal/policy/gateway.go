// Package policy is the thin ScottyLabs policy layer the gateway wraps around the adopted MCP
// gateway (ContextForge): it resolves a caller to committee roles, enforces per-tool scope and
// committee visibility, gates impact:high tools behind a recorded human approval, proxies the
// authorized call downstream, and audits everything. The transport, OAuth 2.1/PKCE, and downstream
// credential injection are provided by ContextForge (design Section 6.6); this is the committee-role
// policy it does not express off the shelf.
package policy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/scottylabs/scottylabs-agent/gateway/internal/limits"
	"github.com/scottylabs/scottylabs-agent/services/session-memory/store"
)

// Identity is the resolved caller: a member (via OAuth) or the deployed agent (service creds),
// carrying the committee roles that determine what it may see and call.
type Identity struct {
	Subject    string   // user email or the agent's service id
	Committees []string // resolved committee roles, e.g. ["finance"]
	IsAgent    bool
}

// CallRequest is a request to invoke a tool through the gateway. SessionID ties a high-impact call
// to its approval record.
type CallRequest struct {
	Identity   Identity
	SessionID  string
	ServerName string
	ToolName   string
	Args       json.RawMessage
}

// CallResult is a successful tool result plus the id of its audit row.
type CallResult struct {
	Output  json.RawMessage
	AuditID int64
}

var (
	// ErrUnknownTool is returned when no such tool is registered.
	ErrUnknownTool = errors.New("gateway: unknown tool")
	// ErrUnauthorized is returned when the caller's roles do not permit the tool.
	ErrUnauthorized = errors.New("gateway: caller not authorized for tool")
	// ErrRateLimited is returned when a caller exceeds the per-committee or global rate limit.
	ErrRateLimited = errors.New("gateway: rate limited")
)

// ApprovalRequiredError is returned when an impact:high tool is called without an approved
// approval. It carries the pending approval id so the caller can surface it for a human decision.
type ApprovalRequiredError struct {
	ApprovalID string
	Tool       string
}

func (e *ApprovalRequiredError) Error() string {
	return fmt.Sprintf("gateway: tool %q requires human approval (approval %s)", e.Tool, e.ApprovalID)
}

// Store is the persistence the gateway needs, defined here at the consumer (Go idiom).
// *store.Repository satisfies it.
type Store interface {
	RegisterServer(ctx context.Context, in store.ServerInput) (store.Server, error)
	ListVisibleTools(ctx context.Context, committees []string) ([]store.VisibleTool, error)
	ResolveTool(ctx context.Context, serverName, toolName string) (store.ToolBinding, error)
	CreateApproval(ctx context.Context, sessionID, tool string, argsRedacted json.RawMessage) (store.Approval, error)
	ListApprovalsBySession(ctx context.Context, sessionID string) ([]store.Approval, error)
	WriteAudit(ctx context.Context, in store.AuditInput) (store.AuditEntry, error)
}

// ToolCaller proxies an authorized call to a downstream MCP server (injecting that server's
// credentials). The real implementation speaks MCP over HTTP; tests inject a fake.
type ToolCaller interface {
	Call(ctx context.Context, server store.Server, tool store.Tool, args json.RawMessage) (json.RawMessage, error)
}

// Gateway is the policy engine. Construct it with New.
type Gateway struct {
	store   Store
	caller  ToolCaller
	redact  func(json.RawMessage) json.RawMessage
	limiter *limits.Limiter
}

// Option configures a Gateway.
type Option func(*Gateway)

// WithLimiter adds per-committee and global rate limiting (design 10.2).
func WithLimiter(l *limits.Limiter) Option { return func(g *Gateway) { g.limiter = l } }

// New builds a Gateway over a Store and a downstream ToolCaller.
func New(s Store, c ToolCaller, opts ...Option) *Gateway {
	g := &Gateway{store: s, caller: c, redact: RedactArgs}
	for _, o := range opts {
		o(g)
	}
	return g
}

// Register adds or updates a server in the registry (it lands 'proposed' until a maintainer
// promotes it) and audits the change.
func (g *Gateway) Register(ctx context.Context, in store.ServerInput) (store.Server, error) {
	s, err := g.store.RegisterServer(ctx, in)
	if err != nil {
		return store.Server{}, err
	}
	_, _ = g.store.WriteAudit(ctx, store.AuditInput{
		Actor: "registry", Tool: "register:" + in.Name, Result: "ok",
	})
	return s, nil
}

// ListTools returns the tools the identity is allowed to see — only those on approved, enabled
// servers granted to one of the caller's committees. A caller never even lists tools its role
// does not permit.
func (g *Gateway) ListTools(ctx context.Context, id Identity) ([]store.VisibleTool, error) {
	return g.store.ListVisibleTools(ctx, id.Committees)
}

// rateKey picks the per-committee rate-limit key for a caller: its first committee, falling back to
// its subject.
func rateKey(id Identity) string {
	if len(id.Committees) > 0 {
		return id.Committees[0]
	}
	return id.Subject
}

// Call authorizes, gates, proxies, and audits a tool invocation:
//  1. resolve the tool (unknown -> ErrUnknownTool, audited);
//  2. authorize: server approved+enabled and granted to one of the caller's committees
//     (else ErrUnauthorized, audited) — the tool never runs;
//  3. if impact:high, require an approved approval for this session+tool; if none, record a
//     pending approval and return ApprovalRequiredError (the tool never runs);
//  4. proxy downstream and audit the outcome (actor, tool, redacted args, result, latency).
func (g *Gateway) Call(ctx context.Context, req CallRequest) (CallResult, error) {
	start := time.Now()

	// Cost/rate control (design 10.2): a runaway or abused caller hits a wall before any work.
	if g.limiter != nil && !g.limiter.Allow(rateKey(req.Identity)) {
		g.audit(ctx, req, "denied: rate limited", start)
		return CallResult{}, ErrRateLimited
	}

	binding, err := g.store.ResolveTool(ctx, req.ServerName, req.ToolName)
	if errors.Is(err, store.ErrNotFound) {
		g.audit(ctx, req, "denied: unknown tool", start)
		return CallResult{}, ErrUnknownTool
	}
	if err != nil {
		return CallResult{}, fmt.Errorf("gateway: resolve tool: %w", err)
	}

	if !authorized(binding.Server, req.Identity) {
		g.audit(ctx, req, "denied: unauthorized", start)
		return CallResult{}, ErrUnauthorized
	}

	if binding.Tool.Impact == "high" {
		approved, err := g.hasApproval(ctx, req.SessionID, req.ToolName)
		if err != nil {
			return CallResult{}, err
		}
		if !approved {
			ap, err := g.store.CreateApproval(ctx, req.SessionID, req.ToolName, g.redact(req.Args))
			if err != nil {
				return CallResult{}, fmt.Errorf("gateway: create approval: %w", err)
			}
			g.audit(ctx, req, "gated: awaiting approval", start)
			return CallResult{}, &ApprovalRequiredError{ApprovalID: ap.ID, Tool: req.ToolName}
		}
	}

	out, callErr := g.caller.Call(ctx, binding.Server, binding.Tool, req.Args)
	result := "ok"
	if callErr != nil {
		result = "error: " + callErr.Error()
	}
	auditID := g.audit(ctx, req, result, start)
	if callErr != nil {
		return CallResult{AuditID: auditID}, fmt.Errorf("gateway: downstream call: %w", callErr)
	}
	return CallResult{Output: out, AuditID: auditID}, nil
}

// authorized reports whether an identity may use a tool on the given server: the server must be
// approved and enabled, and one of the caller's committees must be granted the server.
func authorized(s store.Server, id Identity) bool {
	if s.Lifecycle != store.LifecycleApproved || !s.Enabled {
		return false
	}
	for _, c := range id.Committees {
		for _, granted := range s.Committees {
			if c == granted {
				return true
			}
		}
	}
	return false
}

func (g *Gateway) hasApproval(ctx context.Context, sessionID, tool string) (bool, error) {
	if sessionID == "" {
		return false, nil
	}
	approvals, err := g.store.ListApprovalsBySession(ctx, sessionID)
	if err != nil {
		return false, fmt.Errorf("gateway: list approvals: %w", err)
	}
	for _, a := range approvals {
		if a.Tool == tool && a.Status == "approved" {
			return true, nil
		}
	}
	return false, nil
}

func (g *Gateway) audit(ctx context.Context, req CallRequest, result string, start time.Time) int64 {
	e, err := g.store.WriteAudit(ctx, store.AuditInput{
		Actor:        req.Identity.Subject,
		Tool:         req.ServerName + "/" + req.ToolName,
		ArgsRedacted: g.redact(req.Args),
		Result:       result,
		LatencyMS:    int(time.Since(start).Milliseconds()),
	})
	if err != nil {
		return 0
	}
	return e.ID
}
