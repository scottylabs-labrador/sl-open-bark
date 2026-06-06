package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/scottylabs/scottylabs-agent/services/dashboard/internal/server"
	"github.com/scottylabs/scottylabs-agent/services/session-memory/store"
)

type fakeStore struct {
	servers  []store.Server
	audit    []store.AuditEntry
	pending  []store.Approval
	decided  string
	lifeName string
}

func (f *fakeStore) ListServers(context.Context) ([]store.Server, error) { return f.servers, nil }
func (f *fakeStore) SetServerLifecycle(_ context.Context, name, _ string) error {
	f.lifeName = name
	return nil
}
func (f *fakeStore) SetServerEnabled(context.Context, string, bool) error { return nil }
func (f *fakeStore) ListAudit(context.Context, int) ([]store.AuditEntry, error) {
	return f.audit, nil
}
func (f *fakeStore) ListPendingApprovals(context.Context) ([]store.Approval, error) {
	return f.pending, nil
}
func (f *fakeStore) DecideApproval(_ context.Context, id, status, _ string) (store.Approval, error) {
	f.decided = id + ":" + status
	return store.Approval{ID: id, Status: status}, nil
}

type fakeRuntime struct {
	configured bool
	lastPath   string
}

func (f *fakeRuntime) Configured() bool { return f.configured }
func (f *fakeRuntime) Do(_ context.Context, _, path string, _ []byte) ([]byte, int, error) {
	f.lastPath = path
	return []byte(`{"task_id":"t1"}`), 202, nil
}

func newStore() *fakeStore {
	return &fakeStore{
		servers: []store.Server{{
			Name: "finance.rules", Owner: "finance-committee", Lifecycle: "approved", Enabled: true,
			Tools:      []store.Tool{{Name: "evaluate", Scope: "finance.read", Impact: "read"}},
			Committees: []string{"finance"},
		}},
		audit:   []store.AuditEntry{{Actor: "agent", Tool: "finance.rules/evaluate", Result: "ok", LatencyMS: 12, CreatedAt: time.Now()}},
		pending: []store.Approval{{ID: "a1", Tool: "google.workspace/gmail_send", Status: "pending", CreatedAt: time.Now()}},
	}
}

func do(t *testing.T, h http.Handler, method, path, body string, cookie string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if cookie != "" {
		r.AddCookie(&http.Cookie{Name: "sl_dash", Value: cookie})
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, r)
	return rr
}

func TestAuthGate(t *testing.T) {
	h := server.New(newStore(), &fakeRuntime{}, "secret").Handler()
	if rr := do(t, h, "GET", "/api/stats", "", ""); rr.Code != http.StatusUnauthorized {
		t.Fatalf("no cookie should be 401, got %d", rr.Code)
	}
	if rr := do(t, h, "GET", "/api/stats", "", "secret"); rr.Code != http.StatusOK {
		t.Fatalf("valid cookie should be 200, got %d", rr.Code)
	}
	// Dev mode (empty token) is open.
	hd := server.New(newStore(), &fakeRuntime{}, "").Handler()
	if rr := do(t, hd, "GET", "/api/stats", "", ""); rr.Code != http.StatusOK {
		t.Fatalf("dev mode should be open, got %d", rr.Code)
	}
}

func TestStats(t *testing.T) {
	h := server.New(newStore(), &fakeRuntime{configured: true}, "").Handler()
	rr := do(t, h, "GET", "/api/stats", "", "")
	var s map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &s)
	if s["servers_total"].(float64) != 1 || s["tools_total"].(float64) != 1 || s["pending_approvals"].(float64) != 1 {
		t.Fatalf("unexpected stats: %v", s)
	}
	if s["runtime_available"] != true {
		t.Fatal("runtime_available should be true")
	}
}

func TestRegistryAndDecide(t *testing.T) {
	fs := newStore()
	h := server.New(fs, &fakeRuntime{}, "").Handler()

	rr := do(t, h, "GET", "/api/registry", "", "")
	if !strings.Contains(rr.Body.String(), "finance.rules") {
		t.Fatalf("registry missing server: %s", rr.Body.String())
	}
	if rr := do(t, h, "POST", "/api/registry/finance.rules/lifecycle", `{"lifecycle":"approved"}`, ""); rr.Code != http.StatusOK || fs.lifeName != "finance.rules" {
		t.Fatalf("lifecycle update failed: %d %q", rr.Code, fs.lifeName)
	}
	if rr := do(t, h, "POST", "/api/approvals/a1/decide", `{"status":"approved","decided_by":"me"}`, ""); rr.Code != http.StatusOK || fs.decided != "a1:approved" {
		t.Fatalf("decide failed: %d %q", rr.Code, fs.decided)
	}
}

func TestRuntimeProxy(t *testing.T) {
	fr := &fakeRuntime{configured: true}
	h := server.New(newStore(), fr, "").Handler()
	rr := do(t, h, "POST", "/api/tasks", `{"inline_goal":"hi"}`, "")
	if rr.Code != 202 || !strings.Contains(rr.Body.String(), "t1") || fr.lastPath != "/tasks" {
		t.Fatalf("proxy failed: %d %s path=%s", rr.Code, rr.Body.String(), fr.lastPath)
	}

	// When the runtime is not configured, the console returns 503.
	h2 := server.New(newStore(), &fakeRuntime{configured: false}, "").Handler()
	if rr := do(t, h2, "POST", "/api/tasks", `{"inline_goal":"hi"}`, ""); rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured runtime should be 503, got %d", rr.Code)
	}
}

func TestHealthz(t *testing.T) {
	h := server.New(newStore(), &fakeRuntime{}, "x").Handler()
	if rr := do(t, h, "GET", "/healthz", "", ""); rr.Code != http.StatusOK {
		t.Fatalf("healthz = %d", rr.Code)
	}
}
