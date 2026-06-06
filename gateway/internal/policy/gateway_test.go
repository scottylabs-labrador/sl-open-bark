package policy_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/scottylabs/scottylabs-agent/gateway/internal/policy"
	"github.com/scottylabs/scottylabs-agent/services/session-memory/store"
)

// fakeStore implements policy.Store in memory, mirroring the real registry/approval/audit semantics
// for the tested paths. It lets the policy logic be tested without a database.
type fakeStore struct {
	servers   map[string]store.Server
	approvals []store.Approval
	audits    []store.AuditEntry
	auditSeq  int64
	apprSeq   int
}

func newFakeStore() *fakeStore { return &fakeStore{servers: map[string]store.Server{}} }

func (f *fakeStore) RegisterServer(_ context.Context, in store.ServerInput) (store.Server, error) {
	s := store.Server{
		ID: "srv-" + in.Name, Name: in.Name, Owner: in.Owner, Endpoint: in.Endpoint,
		Lifecycle: store.LifecycleProposed, Enabled: true, Committees: in.Committees,
	}
	for i, t := range in.Tools {
		s.Tools = append(s.Tools, store.Tool{
			ID: fmt.Sprintf("%s-%d", in.Name, i), ServerID: s.ID,
			Name: t.Name, Scope: t.Scope, Impact: t.Impact,
		})
	}
	f.servers[in.Name] = s
	return s, nil
}

func (f *fakeStore) approve(name string) {
	s := f.servers[name]
	s.Lifecycle = store.LifecycleApproved
	f.servers[name] = s
}

func (f *fakeStore) ListVisibleTools(_ context.Context, committees []string) ([]store.VisibleTool, error) {
	var out []store.VisibleTool
	for _, s := range f.servers {
		if s.Lifecycle != store.LifecycleApproved || !s.Enabled || !overlap(s.Committees, committees) {
			continue
		}
		for _, t := range s.Tools {
			out = append(out, store.VisibleTool{
				ServerName: s.Name, ServerID: s.ID, ToolName: t.Name, Scope: t.Scope, Impact: t.Impact,
			})
		}
	}
	return out, nil
}

func (f *fakeStore) ResolveTool(_ context.Context, serverName, toolName string) (store.ToolBinding, error) {
	s, ok := f.servers[serverName]
	if !ok {
		return store.ToolBinding{}, store.ErrNotFound
	}
	for _, t := range s.Tools {
		if t.Name == toolName {
			return store.ToolBinding{Server: s, Tool: t}, nil
		}
	}
	return store.ToolBinding{}, store.ErrNotFound
}

func (f *fakeStore) CreateApproval(_ context.Context, sessionID, tool string, args json.RawMessage) (store.Approval, error) {
	f.apprSeq++
	a := store.Approval{
		ID: fmt.Sprintf("appr-%d", f.apprSeq), SessionID: sessionID, Tool: tool,
		ArgsRedacted: args, Status: "pending",
	}
	f.approvals = append(f.approvals, a)
	return a, nil
}

func (f *fakeStore) ListApprovalsBySession(_ context.Context, sessionID string) ([]store.Approval, error) {
	var out []store.Approval
	for _, a := range f.approvals {
		if a.SessionID == sessionID {
			out = append(out, a)
		}
	}
	return out, nil
}

func (f *fakeStore) decide(id, status string) {
	for i := range f.approvals {
		if f.approvals[i].ID == id {
			f.approvals[i].Status = status
		}
	}
}

func (f *fakeStore) WriteAudit(_ context.Context, in store.AuditInput) (store.AuditEntry, error) {
	f.auditSeq++
	e := store.AuditEntry{
		ID: f.auditSeq, Actor: in.Actor, Tool: in.Tool,
		ArgsRedacted: in.ArgsRedacted, Result: in.Result, LatencyMS: in.LatencyMS,
	}
	f.audits = append(f.audits, e)
	return e, nil
}

func (f *fakeStore) lastAudit() store.AuditEntry { return f.audits[len(f.audits)-1] }

func overlap(a, b []string) bool {
	for _, x := range a {
		for _, y := range b {
			if x == y {
				return true
			}
		}
	}
	return false
}

type fakeCaller struct {
	calls  int
	output json.RawMessage
	err    error
}

func (c *fakeCaller) Call(_ context.Context, _ store.Server, _ store.Tool, _ json.RawMessage) (json.RawMessage, error) {
	c.calls++
	return c.output, c.err
}

// setup registers a "google" server (committee finance) with a read tool and a high-impact tool,
// approved by default.
func setup(t *testing.T) (*policy.Gateway, *fakeStore, *fakeCaller) {
	t.Helper()
	fs := newFakeStore()
	if _, err := fs.RegisterServer(context.Background(), store.ServerInput{
		Name: "google", Owner: "platform-team", Endpoint: "http://google.internal/mcp",
		Tools: []store.ToolInput{
			{Name: "sheets_read", Scope: "google.read", Impact: "read"},
			{Name: "gmail_send", Scope: "google.send", Impact: "high"},
		},
		Committees: []string{"finance"},
	}); err != nil {
		t.Fatal(err)
	}
	fs.approve("google")
	caller := &fakeCaller{output: json.RawMessage(`{"ok":true}`)}
	return policy.New(fs, caller), fs, caller
}

func financeID() policy.Identity {
	return policy.Identity{Subject: "alice@scottylabs.org", Committees: []string{"finance"}}
}
func eventsID() policy.Identity {
	return policy.Identity{Subject: "bob@scottylabs.org", Committees: []string{"events"}}
}

func TestListToolsScoped(t *testing.T) {
	gw, _, _ := setup(t)
	fin, err := gw.ListTools(context.Background(), financeID())
	if err != nil {
		t.Fatal(err)
	}
	if len(fin) != 2 {
		t.Fatalf("finance should see 2 tools, got %d", len(fin))
	}
	ev, _ := gw.ListTools(context.Background(), eventsID())
	if len(ev) != 0 {
		t.Fatalf("events should see 0 tools, got %d", len(ev))
	}
}

func TestCallAllowedReadTool(t *testing.T) {
	gw, fs, caller := setup(t)
	res, err := gw.Call(context.Background(), policy.CallRequest{
		Identity: financeID(), ServerName: "google", ToolName: "sheets_read",
		Args: json.RawMessage(`{"range":"A1:B2"}`),
	})
	if err != nil {
		t.Fatalf("authorized read should succeed: %v", err)
	}
	if caller.calls != 1 {
		t.Fatalf("downstream should be called once, got %d", caller.calls)
	}
	if string(res.Output) != `{"ok":true}` {
		t.Fatalf("output not returned: %s", res.Output)
	}
	if fs.lastAudit().Result != "ok" || fs.lastAudit().Tool != "google/sheets_read" {
		t.Fatalf("audit wrong: %+v", fs.lastAudit())
	}
}

func TestCallDeniedForUnauthorizedCommittee(t *testing.T) {
	gw, fs, caller := setup(t)
	_, err := gw.Call(context.Background(), policy.CallRequest{
		Identity: eventsID(), ServerName: "google", ToolName: "sheets_read",
	})
	if !errors.Is(err, policy.ErrUnauthorized) {
		t.Fatalf("want ErrUnauthorized, got %v", err)
	}
	if caller.calls != 0 {
		t.Fatal("downstream must not be called on denial")
	}
	if !strings.HasPrefix(fs.lastAudit().Result, "denied") {
		t.Fatalf("denial should be audited, got %q", fs.lastAudit().Result)
	}
}

func TestCallUnknownTool(t *testing.T) {
	gw, _, caller := setup(t)
	_, err := gw.Call(context.Background(), policy.CallRequest{
		Identity: financeID(), ServerName: "google", ToolName: "nope",
	})
	if !errors.Is(err, policy.ErrUnknownTool) {
		t.Fatalf("want ErrUnknownTool, got %v", err)
	}
	if caller.calls != 0 {
		t.Fatal("downstream must not be called for unknown tool")
	}
}

func TestProposedServerNotCallable(t *testing.T) {
	gw, fs, _ := setup(t)
	// Demote google back to proposed: fail closed even for a granted committee.
	s := fs.servers["google"]
	s.Lifecycle = store.LifecycleProposed
	fs.servers["google"] = s
	_, err := gw.Call(context.Background(), policy.CallRequest{
		Identity: financeID(), ServerName: "google", ToolName: "sheets_read",
	})
	if !errors.Is(err, policy.ErrUnauthorized) {
		t.Fatalf("proposed server must be uncallable, got %v", err)
	}
}

func TestHighImpactGatedUntilApproved(t *testing.T) {
	gw, fs, caller := setup(t)
	req := policy.CallRequest{
		Identity: financeID(), SessionID: "s1", ServerName: "google", ToolName: "gmail_send",
		Args: json.RawMessage(`{"to":"x@cmu.edu"}`),
	}

	// First call: blocked, a pending approval is recorded, downstream not called.
	_, err := gw.Call(context.Background(), req)
	var are *policy.ApprovalRequiredError
	if !errors.As(err, &are) {
		t.Fatalf("want ApprovalRequiredError, got %v", err)
	}
	if caller.calls != 0 {
		t.Fatal("high-impact tool must not run before approval")
	}
	if len(fs.approvals) != 1 || fs.approvals[0].Status != "pending" {
		t.Fatalf("a pending approval should exist: %+v", fs.approvals)
	}
	if fs.lastAudit().Result != "gated: awaiting approval" {
		t.Fatalf("gating should be audited, got %q", fs.lastAudit().Result)
	}

	// A human approves; the same call now proceeds.
	fs.decide(are.ApprovalID, "approved")
	res, err := gw.Call(context.Background(), req)
	if err != nil {
		t.Fatalf("approved high-impact call should proceed: %v", err)
	}
	if caller.calls != 1 {
		t.Fatalf("downstream should run once after approval, got %d", caller.calls)
	}
	if res.AuditID == 0 {
		t.Fatal("successful call should be audited")
	}
}

func TestAuditArgsRedacted(t *testing.T) {
	gw, fs, _ := setup(t)
	_, err := gw.Call(context.Background(), policy.CallRequest{
		Identity: financeID(), ServerName: "google", ToolName: "sheets_read",
		Args: json.RawMessage(`{"token":"super-secret","range":"A1"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	var logged map[string]any
	if err := json.Unmarshal(fs.lastAudit().ArgsRedacted, &logged); err != nil {
		t.Fatal(err)
	}
	if logged["token"] != "***" {
		t.Fatalf("token should be redacted in audit, got %v", logged["token"])
	}
	if logged["range"] != "A1" {
		t.Fatalf("non-sensitive field should survive, got %v", logged["range"])
	}
}
