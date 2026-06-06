package assembly_test

import (
	"strings"
	"testing"

	"github.com/scottylabs/scottylabs-agent/services/session-memory/internal/assembly"
)

// fixed makes a string of n characters (n/... ~ tokens under EstimateTokens: (len+3)/4).
func fixed(n int) string { return strings.Repeat("x", n) }

func TestAssembleUnboundedIncludesAll(t *testing.T) {
	parts := []assembly.Part{
		{Name: "a", Content: "hello"},
		{Name: "b", Content: "world"},
	}
	got := assembly.Assemble(parts, 0, nil) // budget 0 = unbounded
	if len(got.Included) != 2 || len(got.Dropped) != 0 {
		t.Fatalf("unbounded should include all: %+v", got)
	}
	if !strings.Contains(got.Text, "hello") || !strings.Contains(got.Text, "world") {
		t.Fatalf("text missing parts: %q", got.Text)
	}
}

func TestAssembleRespectsBudgetAndPriority(t *testing.T) {
	// Each 40-char part ~ 10 tokens. Budget 25 -> first two fit (20), third (non-trimmable) dropped.
	parts := []assembly.Part{
		{Name: "p1", Content: fixed(40)},
		{Name: "p2", Content: fixed(40)},
		{Name: "p3", Content: fixed(40)},
	}
	got := assembly.Assemble(parts, 25, nil)
	if got.Tokens > 25 {
		t.Fatalf("budget exceeded: %d > 25", got.Tokens)
	}
	if len(got.Included) != 2 || got.Included[0] != "p1" || got.Included[1] != "p2" {
		t.Fatalf("expected p1,p2 included in order: %+v", got.Included)
	}
	if len(got.Dropped) != 1 || got.Dropped[0] != "p3" {
		t.Fatalf("expected p3 dropped: %+v", got.Dropped)
	}
}

func TestAssembleTruncatesTrimmable(t *testing.T) {
	parts := []assembly.Part{
		{Name: "head", Content: fixed(40)},                   // ~10 tokens
		{Name: "tool", Content: fixed(400), Trimmable: true}, // ~100 tokens, must be trimmed
	}
	got := assembly.Assemble(parts, 25, nil)
	if got.Tokens > 25 {
		t.Fatalf("budget exceeded after trim: %d > 25", got.Tokens)
	}
	if len(got.Truncated) != 1 || got.Truncated[0] != "tool" {
		t.Fatalf("expected tool truncated: %+v", got)
	}
	if !strings.Contains(got.Text, "trimmed") {
		t.Fatalf("trimmed marker missing: %q", got.Text)
	}
}

func TestAssembleSkipsEmptyParts(t *testing.T) {
	parts := []assembly.Part{
		{Name: "a", Content: ""},
		{Name: "b", Content: "real"},
	}
	got := assembly.Assemble(parts, 0, nil)
	if len(got.Included) != 1 || got.Included[0] != "b" {
		t.Fatalf("empty parts should be skipped: %+v", got.Included)
	}
}

func TestEstimateTokens(t *testing.T) {
	if assembly.EstimateTokens("") != 0 {
		t.Fatal("empty should be 0 tokens")
	}
	if assembly.EstimateTokens("abcd") != 1 {
		t.Fatalf("4 chars ~ 1 token, got %d", assembly.EstimateTokens("abcd"))
	}
}
