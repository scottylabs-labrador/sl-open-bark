package store_test

import (
	"errors"
	"testing"

	"github.com/scottylabs/scottylabs-agent/services/session-memory/store"
)

func financeServer() store.ServerInput {
	return store.ServerInput{
		Name:        "finance.rules",
		Owner:       "finance-committee",
		Description: "Deterministic reimbursement screening.",
		Endpoint:    "http://finance-rules.railway.internal:8080/mcp",
		Tools: []store.ToolInput{
			{Name: "evaluate", Scope: "finance.read", Impact: "read"},
			{Name: "record_decision", Scope: "finance.write", Impact: "write"},
		},
		Committees: []string{"finance", "leadership"},
	}
}

func TestRegisterAndResolve(t *testing.T) {
	reset(t)
	r := testRepo

	s, err := r.RegisterServer(ctx(), financeServer())
	if err != nil {
		t.Fatal(err)
	}
	if s.Lifecycle != store.LifecycleProposed || !s.Enabled {
		t.Fatalf("new server should be proposed+enabled: %+v", s)
	}
	if len(s.Tools) != 2 || len(s.Committees) != 2 {
		t.Fatalf("tools/committees not stored: %+v", s)
	}

	binding, err := r.ResolveTool(ctx(), "finance.rules", "evaluate")
	if err != nil {
		t.Fatal(err)
	}
	if binding.Tool.Impact != "read" || binding.Server.Endpoint == "" {
		t.Fatalf("unexpected binding: %+v", binding)
	}
	if _, err := r.ResolveTool(ctx(), "finance.rules", "nope"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound for missing tool, got %v", err)
	}
}

// TestVisibilityLifecycleAndScope covers the discovery surface: a tool is visible only when its
// server is approved, enabled, and granted to one of the caller's committees.
func TestVisibilityLifecycleAndScope(t *testing.T) {
	reset(t)
	r := testRepo
	if _, err := r.RegisterServer(ctx(), financeServer()); err != nil {
		t.Fatal(err)
	}

	// Proposed: invisible to everyone, even the owning committee (fail closed).
	if got, _ := r.ListVisibleTools(ctx(), []string{"finance"}); len(got) != 0 {
		t.Fatalf("proposed server must be invisible, got %+v", got)
	}

	if err := r.SetServerLifecycle(ctx(), "finance.rules", store.LifecycleApproved); err != nil {
		t.Fatal(err)
	}

	// Approved: visible to a granted committee...
	fin, err := r.ListVisibleTools(ctx(), []string{"finance"})
	if err != nil {
		t.Fatal(err)
	}
	if len(fin) != 2 {
		t.Fatalf("finance should see 2 tools, got %d: %+v", len(fin), fin)
	}
	// ...and invisible to a committee with no grant.
	if got, _ := r.ListVisibleTools(ctx(), []string{"events"}); len(got) != 0 {
		t.Fatalf("events has no grant; must see nothing, got %+v", got)
	}

	// Disabling is a one-place kill switch even while approved.
	if err := r.SetServerEnabled(ctx(), "finance.rules", false); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.ListVisibleTools(ctx(), []string{"finance"}); len(got) != 0 {
		t.Fatalf("disabled server must be invisible, got %+v", got)
	}
}

func TestReRegisterPreservesLifecycle(t *testing.T) {
	reset(t)
	r := testRepo
	if _, err := r.RegisterServer(ctx(), financeServer()); err != nil {
		t.Fatal(err)
	}
	if err := r.SetServerLifecycle(ctx(), "finance.rules", store.LifecycleApproved); err != nil {
		t.Fatal(err)
	}

	// Re-register with a changed tool set; lifecycle must be preserved (a maintainer promoted it).
	in := financeServer()
	in.Description = "updated"
	in.Tools = []store.ToolInput{{Name: "evaluate", Scope: "finance.read", Impact: "read"}}
	s, err := r.RegisterServer(ctx(), in)
	if err != nil {
		t.Fatal(err)
	}
	if s.Lifecycle != store.LifecycleApproved {
		t.Fatalf("re-register must preserve lifecycle, got %s", s.Lifecycle)
	}
	if len(s.Tools) != 1 || s.Description != "updated" {
		t.Fatalf("re-register should replace tools and update metadata: %+v", s)
	}
}
