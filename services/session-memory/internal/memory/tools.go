// Package memory is the Memory MCP: it exposes the platform's scoped long-term memory
// (write_fact, search, forget) as MCP tools, backed by the WP-01 store, so recipes can read and
// write memory like any other capability (design Section 4.4). The handlers are thin and testable;
// scoping (one principal's facts never surface for another) is enforced by the store.
package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/scottylabs/scottylabs-agent/services/session-memory/store"
)

// FactStore is the slice of the store the Memory MCP needs (defined at the consumer).
type FactStore interface {
	WriteFact(ctx context.Context, in store.FactInput) (store.Fact, error)
	SearchFacts(ctx context.Context, q store.FactQuery) ([]store.Fact, error)
	ForgetFact(ctx context.Context, id string) (bool, error)
	ForgetFactsByKey(ctx context.Context, scopeType, scopeID, key string) (int64, error)
}

// Handlers implement the Memory MCP use cases over a FactStore.
type Handlers struct{ store FactStore }

// NewHandlers builds the handlers.
func NewHandlers(fs FactStore) *Handlers { return &Handlers{store: fs} }

// WriteFactInput writes a durable, scoped fact. scope_type is user | committee | org.
type WriteFactInput struct {
	ScopeType string   `json:"scope_type" jsonschema:"user | committee | org"`
	ScopeID   string   `json:"scope_id" jsonschema:"the id within the scope, e.g. a user id or committee name"`
	Key       string   `json:"key" jsonschema:"the fact key"`
	Value     string   `json:"value" jsonschema:"the fact value"`
	Tags      []string `json:"tags" jsonschema:"tags for retrieval (optional)"`
	Source    string   `json:"source" jsonschema:"where the fact came from (optional)"`
	ExpiresAt string   `json:"expires_at" jsonschema:"RFC3339 expiry (optional)"`
}

// WriteFact stores a fact.
func (h *Handlers) WriteFact(ctx context.Context, in WriteFactInput) (store.Fact, error) {
	fact := store.FactInput{
		ScopeType: in.ScopeType, ScopeID: in.ScopeID, Key: in.Key, Value: in.Value,
		Tags: in.Tags, Source: in.Source,
	}
	if in.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, in.ExpiresAt)
		if err != nil {
			return store.Fact{}, fmt.Errorf("memory: invalid expires_at: %w", err)
		}
		fact.ExpiresAt = &t
	}
	return h.store.WriteFact(ctx, fact)
}

// SearchInput searches one scope for top-k facts by tag and recency.
type SearchInput struct {
	ScopeType string   `json:"scope_type" jsonschema:"user | committee | org"`
	ScopeID   string   `json:"scope_id" jsonschema:"the id within the scope"`
	Tags      []string `json:"tags" jsonschema:"only facts carrying one of these tags (optional)"`
	Limit     int      `json:"limit" jsonschema:"max facts to return (optional)"`
}

// SearchOutput is the scoped result set.
type SearchOutput struct {
	Facts []store.Fact `json:"facts"`
}

// Search returns scoped facts. The store always filters on (scope_type, scope_id), so results
// never cross scopes.
func (h *Handlers) Search(ctx context.Context, in SearchInput) (SearchOutput, error) {
	facts, err := h.store.SearchFacts(ctx, store.FactQuery{
		ScopeType: in.ScopeType, ScopeID: in.ScopeID, Tags: in.Tags, Limit: in.Limit,
	})
	if err != nil {
		return SearchOutput{}, err
	}
	if facts == nil {
		facts = []store.Fact{}
	}
	return SearchOutput{Facts: facts}, nil
}

// ForgetInput removes a fact by id, or all facts for a (scope, key).
type ForgetInput struct {
	ID        string `json:"id" jsonschema:"a specific fact id to forget (optional)"`
	ScopeType string `json:"scope_type" jsonschema:"scope for key-based forget (optional)"`
	ScopeID   string `json:"scope_id" jsonschema:"scope id for key-based forget (optional)"`
	Key       string `json:"key" jsonschema:"forget all facts with this key in the scope (optional)"`
}

// ForgetOutput reports how many facts were removed.
type ForgetOutput struct {
	Deleted int64 `json:"deleted"`
}

// Forget removes a fact by id, or every fact with a key in a scope.
func (h *Handlers) Forget(ctx context.Context, in ForgetInput) (ForgetOutput, error) {
	switch {
	case in.ID != "":
		ok, err := h.store.ForgetFact(ctx, in.ID)
		if err != nil {
			return ForgetOutput{}, err
		}
		if ok {
			return ForgetOutput{Deleted: 1}, nil
		}
		return ForgetOutput{Deleted: 0}, nil
	case in.ScopeType != "" && in.ScopeID != "" && in.Key != "":
		n, err := h.store.ForgetFactsByKey(ctx, in.ScopeType, in.ScopeID, in.Key)
		return ForgetOutput{Deleted: n}, err
	default:
		return ForgetOutput{}, fmt.Errorf("memory: forget requires either id or (scope_type, scope_id, key)")
	}
}

// Register wires the Memory MCP tools onto the server.
func Register(s *mcp.Server, h *Handlers) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "write_fact",
		Description: "Store a durable, scoped fact (preference, prior decision, standing instruction). " +
			"scope: memory.write, impact: write.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in WriteFactInput) (*mcp.CallToolResult, store.Fact, error) {
		f, err := h.WriteFact(ctx, in)
		return nil, f, err
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "search",
		Description: "Search a single scope's facts, top-k by tag and recency. Scoped — never returns " +
			"another principal's facts. scope: memory.read, impact: read.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in SearchInput) (*mcp.CallToolResult, SearchOutput, error) {
		out, err := h.Search(ctx, in)
		return nil, out, err
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "forget",
		Description: "Forget a fact by id, or all facts for a (scope, key). scope: memory.write, impact: write.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ForgetInput) (*mcp.CallToolResult, ForgetOutput, error) {
		out, err := h.Forget(ctx, in)
		return nil, out, err
	})
}
