package memory_test

import (
	"context"
	"testing"

	"github.com/scottylabs/scottylabs-agent/services/session-memory/internal/memory"
	"github.com/scottylabs/scottylabs-agent/services/session-memory/store"
)

type fakeStore struct {
	written   store.FactInput
	lastQuery store.FactQuery
	facts     []store.Fact
	forgotID  string
	forgotKey [3]string
}

func (f *fakeStore) WriteFact(_ context.Context, in store.FactInput) (store.Fact, error) {
	f.written = in
	return store.Fact{ID: "f1", ScopeType: in.ScopeType, ScopeID: in.ScopeID, Key: in.Key, Value: in.Value}, nil
}
func (f *fakeStore) SearchFacts(_ context.Context, q store.FactQuery) ([]store.Fact, error) {
	f.lastQuery = q
	return f.facts, nil
}
func (f *fakeStore) ForgetFact(_ context.Context, id string) (bool, error) {
	f.forgotID = id
	return true, nil
}
func (f *fakeStore) ForgetFactsByKey(_ context.Context, st, si, key string) (int64, error) {
	f.forgotKey = [3]string{st, si, key}
	return 2, nil
}

func TestWriteFactParsesExpiry(t *testing.T) {
	fs := &fakeStore{}
	h := memory.NewHandlers(fs)
	_, err := h.WriteFact(context.Background(), memory.WriteFactInput{
		ScopeType: store.ScopeUser, ScopeID: "alice", Key: "diet", Value: "vegan",
		Tags: []string{"pref"}, ExpiresAt: "2027-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if fs.written.Key != "diet" || fs.written.ExpiresAt == nil {
		t.Fatalf("write not mapped: %+v", fs.written)
	}

	if _, err := h.WriteFact(context.Background(), memory.WriteFactInput{
		ScopeType: store.ScopeUser, ScopeID: "a", Key: "k", ExpiresAt: "not-a-time",
	}); err == nil {
		t.Fatal("invalid expires_at should error")
	}
}

func TestSearchIsScoped(t *testing.T) {
	fs := &fakeStore{facts: []store.Fact{{Key: "k", Value: "v"}}}
	h := memory.NewHandlers(fs)
	out, err := h.Search(context.Background(), memory.SearchInput{
		ScopeType: store.ScopeCommittee, ScopeID: "finance", Tags: []string{"policy"}, Limit: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if fs.lastQuery.ScopeType != store.ScopeCommittee || fs.lastQuery.ScopeID != "finance" {
		t.Fatalf("search not scoped: %+v", fs.lastQuery)
	}
	if len(out.Facts) != 1 {
		t.Fatalf("expected 1 fact, got %d", len(out.Facts))
	}
}

func TestForget(t *testing.T) {
	fs := &fakeStore{}
	h := memory.NewHandlers(fs)

	byID, err := h.Forget(context.Background(), memory.ForgetInput{ID: "f1"})
	if err != nil || byID.Deleted != 1 || fs.forgotID != "f1" {
		t.Fatalf("forget by id failed: %+v err=%v", byID, err)
	}

	byKey, err := h.Forget(context.Background(), memory.ForgetInput{
		ScopeType: store.ScopeUser, ScopeID: "alice", Key: "diet",
	})
	if err != nil || byKey.Deleted != 2 || fs.forgotKey != [3]string{store.ScopeUser, "alice", "diet"} {
		t.Fatalf("forget by key failed: %+v err=%v", byKey, err)
	}

	if _, err := h.Forget(context.Background(), memory.ForgetInput{}); err == nil {
		t.Fatal("forget with neither id nor key should error")
	}
}
