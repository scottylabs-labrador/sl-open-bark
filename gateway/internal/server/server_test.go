package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/scottylabs/scottylabs-agent/gateway/internal/policy"
	"github.com/scottylabs/scottylabs-agent/gateway/internal/server"
	"github.com/scottylabs/scottylabs-agent/services/session-memory/store"
)

// minimal in-memory Store + ToolCaller, enough to exercise the HTTP contract.
type fakeStore struct{ approvals []store.Approval }

var googleServer = store.Server{
	Name: "google", ID: "srv", Lifecycle: store.LifecycleApproved, Enabled: true,
	Committees: []string{"finance"},
	Tools: []store.Tool{
		{Name: "sheets_read", Scope: "google.read", Impact: "read"},
		{Name: "gmail_send", Scope: "google.send", Impact: "high"},
	},
}

func (f *fakeStore) RegisterServer(context.Context, store.ServerInput) (store.Server, error) {
	return googleServer, nil
}
func (f *fakeStore) ListVisibleTools(_ context.Context, committees []string) ([]store.VisibleTool, error) {
	for _, c := range committees {
		if c == "finance" {
			return []store.VisibleTool{
				{ServerName: "google", ToolName: "sheets_read", Scope: "google.read", Impact: "read"},
				{ServerName: "google", ToolName: "gmail_send", Scope: "google.send", Impact: "high"},
			}, nil
		}
	}
	return nil, nil
}
func (f *fakeStore) ResolveTool(_ context.Context, serverName, toolName string) (store.ToolBinding, error) {
	if serverName != "google" {
		return store.ToolBinding{}, store.ErrNotFound
	}
	for _, t := range googleServer.Tools {
		if t.Name == toolName {
			return store.ToolBinding{Server: googleServer, Tool: t}, nil
		}
	}
	return store.ToolBinding{}, store.ErrNotFound
}
func (f *fakeStore) CreateApproval(_ context.Context, sessionID, tool string, args json.RawMessage) (store.Approval, error) {
	a := store.Approval{ID: "appr-1", SessionID: sessionID, Tool: tool, Status: "pending", ArgsRedacted: args}
	f.approvals = append(f.approvals, a)
	return a, nil
}
func (f *fakeStore) ListApprovalsBySession(_ context.Context, sessionID string) ([]store.Approval, error) {
	return f.approvals, nil
}
func (f *fakeStore) WriteAudit(_ context.Context, in store.AuditInput) (store.AuditEntry, error) {
	return store.AuditEntry{ID: 1, Tool: in.Tool, Result: in.Result}, nil
}

type okCaller struct{}

func (okCaller) Call(context.Context, store.Server, store.Tool, json.RawMessage) (json.RawMessage, error) {
	return json.RawMessage(`{"ok":true}`), nil
}

const token = "svc"

func newServer() http.Handler {
	gw := policy.New(&fakeStore{}, okCaller{})
	return server.New(gw, token, server.HeaderIdentity).Handler()
}

func do(t *testing.T, h http.Handler, method, path, committees, bodyStr string, withAuth bool) *httptest.ResponseRecorder {
	t.Helper()
	var body *strings.Reader
	if bodyStr != "" {
		body = strings.NewReader(bodyStr)
	} else {
		body = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, body)
	if withAuth {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if committees != "" {
		req.Header.Set("X-ScottyLabs-Subject", "alice@scottylabs.org")
		req.Header.Set("X-ScottyLabs-Committees", committees)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestHealthz(t *testing.T) {
	rr := do(t, newServer(), http.MethodGet, "/healthz", "", "", false)
	if rr.Code != http.StatusOK {
		t.Fatalf("healthz = %d", rr.Code)
	}
}

func TestAuthRequired(t *testing.T) {
	rr := do(t, newServer(), http.MethodGet, "/tools", "finance", "", false)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("missing bearer should be 401, got %d", rr.Code)
	}
}

func TestToolsListScoped(t *testing.T) {
	h := newServer()
	rr := do(t, h, http.MethodGet, "/tools", "finance", "", true)
	if rr.Code != http.StatusOK {
		t.Fatalf("tools = %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "sheets_read") {
		t.Fatalf("expected tools in body: %s", rr.Body.String())
	}
	// A committee with no grant sees an empty list.
	rr = do(t, h, http.MethodGet, "/tools", "events", "", true)
	var resp struct {
		Tools []store.VisibleTool `json:"tools"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Tools) != 0 {
		t.Fatalf("events should see no tools, got %d", len(resp.Tools))
	}
}

func TestCallPaths(t *testing.T) {
	h := newServer()

	// Authorized read -> 200.
	rr := do(t, h, http.MethodPost, "/call", "finance", `{"server":"google","tool":"sheets_read"}`, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("authorized call = %d (%s)", rr.Code, rr.Body.String())
	}

	// Unauthorized committee -> 403.
	rr = do(t, h, http.MethodPost, "/call", "events", `{"server":"google","tool":"sheets_read"}`, true)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("unauthorized call should be 403, got %d", rr.Code)
	}

	// Unknown tool -> 404.
	rr = do(t, h, http.MethodPost, "/call", "finance", `{"server":"google","tool":"nope"}`, true)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("unknown tool should be 404, got %d", rr.Code)
	}

	// High-impact without approval -> 202 approval_required.
	rr = do(t, h, http.MethodPost, "/call", "finance", `{"server":"google","tool":"gmail_send","session_id":"s1"}`, true)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("high-impact should be 202, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "approval_required") {
		t.Fatalf("expected approval_required: %s", rr.Body.String())
	}
}
