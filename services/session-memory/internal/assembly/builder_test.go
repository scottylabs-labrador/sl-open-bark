package assembly_test

import (
	"context"
	"strings"
	"testing"

	"github.com/scottylabs/scottylabs-agent/services/session-memory/internal/assembly"
	"github.com/scottylabs/scottylabs-agent/services/session-memory/store"
)

// fakeStore implements assembly.Store, recording the scope a search was made in so the test can
// assert retrieval is scoped.
type fakeStore struct {
	facts        []store.Fact
	summary      store.Summary
	summaryErr   error
	turns        []store.Turn
	lastQuery    store.FactQuery
	savedSummary string
	savedFacts   []store.FactInput
}

func (f *fakeStore) SearchFacts(_ context.Context, q store.FactQuery) ([]store.Fact, error) {
	f.lastQuery = q
	return f.facts, nil
}
func (f *fakeStore) GetSummary(_ context.Context, _ string) (store.Summary, error) {
	return f.summary, f.summaryErr
}
func (f *fakeStore) ListTurns(_ context.Context, _ string, _ int) ([]store.Turn, error) {
	return f.turns, nil
}
func (f *fakeStore) UpsertSummary(_ context.Context, _, summary string) (store.Summary, error) {
	f.savedSummary = summary
	return store.Summary{Summary: summary}, nil
}
func (f *fakeStore) WriteFact(_ context.Context, in store.FactInput) (store.Fact, error) {
	f.savedFacts = append(f.savedFacts, in)
	return store.Fact{Key: in.Key}, nil
}

func TestBuildScopedAndBudgeted(t *testing.T) {
	fs := &fakeStore{
		facts:   []store.Fact{{Key: "fiscal_year", Value: "starts July", Tags: []string{"policy"}}},
		summary: store.Summary{Summary: "Discussed travel caps."},
		turns:   []store.Turn{{Role: "user", Content: "what's the cap?"}},
	}
	b := assembly.NewBuilder(fs, nil)

	got, err := b.Build(context.Background(), assembly.TurnInput{
		SessionID: "s1", ScopeType: store.ScopeCommittee, ScopeID: "finance",
		MemoryTags: []string{"policy"}, MemoryTopK: 5, RecentTurns: 5,
		SystemPrompt: "You are the ScottyLabs agent.", Budget: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Retrieval was scoped to exactly (committee, finance) — never another scope.
	if fs.lastQuery.ScopeType != store.ScopeCommittee || fs.lastQuery.ScopeID != "finance" {
		t.Fatalf("memory retrieval not scoped correctly: %+v", fs.lastQuery)
	}
	for _, want := range []string{"ScottyLabs agent", "fiscal_year", "starts July", "Discussed travel caps", "what's the cap?"} {
		if !strings.Contains(got.Text, want) {
			t.Fatalf("assembled context missing %q:\n%s", want, got.Text)
		}
	}
	if got.Tokens > 1000 {
		t.Fatalf("over budget: %d", got.Tokens)
	}
}

func TestBuildTightBudgetKeepsHighestPriority(t *testing.T) {
	fs := &fakeStore{
		facts:   []store.Fact{{Key: "k", Value: strings.Repeat("v", 400)}},
		summary: store.Summary{Summary: strings.Repeat("s", 400)},
	}
	b := assembly.NewBuilder(fs, nil)
	got, err := b.Build(context.Background(), assembly.TurnInput{
		SessionID: "s1", ScopeType: store.ScopeUser, ScopeID: "alice",
		MemoryTopK: 5, SystemPrompt: "SYS", Budget: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Tokens > 10 {
		t.Fatalf("tight budget exceeded: %d", got.Tokens)
	}
	// The system prompt (highest priority) survives.
	if !strings.Contains(got.Text, "SYS") {
		t.Fatalf("highest-priority part dropped under tight budget: %q", got.Text)
	}
}

func TestBuildHandlesMissingSummary(t *testing.T) {
	fs := &fakeStore{summaryErr: store.ErrNotFound}
	b := assembly.NewBuilder(fs, nil)
	got, err := b.Build(context.Background(), assembly.TurnInput{
		SessionID: "s1", SystemPrompt: "hi", Budget: 100,
	})
	if err != nil {
		t.Fatalf("missing summary must not error: %v", err)
	}
	if strings.Contains(got.Text, "Conversation summary") {
		t.Fatal("no summary should be included when none exists")
	}
}

func TestPersistTurnResult(t *testing.T) {
	fs := &fakeStore{}
	b := assembly.NewBuilder(fs, nil)
	err := b.PersistTurnResult(context.Background(), "s1", "new summary", []store.FactInput{
		{ScopeType: store.ScopeUser, ScopeID: "alice", Key: "pref", Value: "vegetarian"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if fs.savedSummary != "new summary" || len(fs.savedFacts) != 1 || fs.savedFacts[0].Key != "pref" {
		t.Fatalf("turn result not persisted: summary=%q facts=%+v", fs.savedSummary, fs.savedFacts)
	}
}
