package assembly

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/scottylabs/scottylabs-agent/services/session-memory/store"
)

// Store is the persistence the builder needs (defined at the consumer). *store.Repository satisfies
// it. Memory retrieval goes through SearchFacts, which is always scoped — so assembled context can
// never include another principal's facts.
type Store interface {
	SearchFacts(ctx context.Context, q store.FactQuery) ([]store.Fact, error)
	GetSummary(ctx context.Context, sessionID string) (store.Summary, error)
	ListTurns(ctx context.Context, sessionID string, limit int) ([]store.Turn, error)
	UpsertSummary(ctx context.Context, sessionID, summary string) (store.Summary, error)
	WriteFact(ctx context.Context, in store.FactInput) (store.Fact, error)
}

// Builder assembles per-turn context from the store and static inputs.
type Builder struct {
	store Store
	count TokenCounter
}

// NewBuilder builds a Builder. A nil counter uses EstimateTokens.
func NewBuilder(s Store, count TokenCounter) *Builder {
	if count == nil {
		count = EstimateTokens
	}
	return &Builder{store: s, count: count}
}

// TurnInput describes one turn's context request. The static parts (system, hints, recipe) come
// from the runtime; memory, summary, and recent turns are retrieved from the store, scoped to
// (ScopeType, ScopeID). ToolOutput is the only trimmable part.
type TurnInput struct {
	SessionID          string
	ScopeType          string
	ScopeID            string
	MemoryTags         []string
	MemoryTopK         int
	RecentTurns        int
	SystemPrompt       string
	GooseHints         string
	RecipeInstructions string
	ToolOutput         string
	Budget             int
}

// Build retrieves scoped memory, the rolling summary, and recent turns, then assembles them with
// the static parts in priority order under the token budget (design Section 4.4).
func (b *Builder) Build(ctx context.Context, in TurnInput) (Assembled, error) {
	var memoryText, summaryText, turnsText string

	if in.ScopeType != "" && in.ScopeID != "" {
		facts, err := b.store.SearchFacts(ctx, store.FactQuery{
			ScopeType: in.ScopeType, ScopeID: in.ScopeID, Tags: in.MemoryTags, Limit: in.MemoryTopK,
		})
		if err != nil {
			return Assembled{}, fmt.Errorf("assembly: retrieve memory: %w", err)
		}
		memoryText = formatFacts(facts)
	}

	if in.SessionID != "" {
		sum, err := b.store.GetSummary(ctx, in.SessionID)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return Assembled{}, fmt.Errorf("assembly: get summary: %w", err)
		}
		if err == nil {
			summaryText = strings.TrimSpace(sum.Summary)
		}

		if in.RecentTurns > 0 {
			turns, err := b.store.ListTurns(ctx, in.SessionID, in.RecentTurns)
			if err != nil {
				return Assembled{}, fmt.Errorf("assembly: list turns: %w", err)
			}
			turnsText = formatTurns(turns)
		}
	}

	// Priority order, most relevant first (design Section 4.4).
	parts := []Part{
		{Name: "system", Content: in.SystemPrompt},
		{Name: "goosehints", Content: in.GooseHints},
		{Name: "recipe", Content: in.RecipeInstructions},
		{Name: "memory", Content: memoryText},
		{Name: "summary", Content: prefixed("Conversation summary:", summaryText)},
		{Name: "recent_turns", Content: turnsText},
		{Name: "tool_output", Content: prefixed("Tool results:", in.ToolOutput), Trimmable: true},
	}
	return Assemble(parts, in.Budget, b.count), nil
}

// PersistTurnResult writes back what the turn learned: an updated rolling summary and any new
// durable facts (design Section 4.4 — "the agent gets smarter about a user over time").
func (b *Builder) PersistTurnResult(ctx context.Context, sessionID, summary string, facts []store.FactInput) error {
	if summary != "" {
		if _, err := b.store.UpsertSummary(ctx, sessionID, summary); err != nil {
			return fmt.Errorf("assembly: persist summary: %w", err)
		}
	}
	for _, f := range facts {
		if _, err := b.store.WriteFact(ctx, f); err != nil {
			return fmt.Errorf("assembly: persist fact %q: %w", f.Key, err)
		}
	}
	return nil
}

func formatFacts(facts []store.Fact) string {
	if len(facts) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Relevant memory:")
	for _, f := range facts {
		b.WriteString("\n- ")
		b.WriteString(f.Key)
		b.WriteString(": ")
		b.WriteString(f.Value)
		if len(f.Tags) > 0 {
			fmt.Fprintf(&b, " [%s]", strings.Join(f.Tags, ", "))
		}
	}
	return b.String()
}

func formatTurns(turns []store.Turn) string {
	if len(turns) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Recent conversation:")
	for _, t := range turns {
		fmt.Fprintf(&b, "\n%s: %s", t.Role, t.Content)
	}
	return b.String()
}

func prefixed(prefix, body string) string {
	if strings.TrimSpace(body) == "" {
		return ""
	}
	return prefix + "\n" + body
}
