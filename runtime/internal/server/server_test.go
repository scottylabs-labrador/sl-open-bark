package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	runtime "github.com/scottylabs/scottylabs-agent/runtime"
	"github.com/scottylabs/scottylabs-agent/runtime/internal/server"
)

// fakeEngine pauses for approval, then outputs — exercising the full service flow.
type fakeEngine struct{}

func (fakeEngine) Run(ctx context.Context, _ runtime.RunSpec, hooks runtime.Hooks) (runtime.Result, error) {
	hooks.Emit(runtime.Event{Kind: runtime.KindToolCall, Tool: "google.workspace/gmail_send"})
	granted, err := hooks.Approve(ctx, "google.workspace/gmail_send", "appr-1")
	if err != nil {
		return runtime.Result{}, err
	}
	if !granted {
		return runtime.Result{}, errors.New("denied")
	}
	return runtime.Result{Output: "sent", AuditRef: "a1"}, nil
}

const token = "tok"

func newServer() http.Handler {
	rt := runtime.New(fakeEngine{})
	return server.New(rt, token).Handler()
}

func req(t *testing.T, h http.Handler, method, path, body string, auth bool) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if auth {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, r)
	return rr
}

// pollUntil polls GET /tasks/{id} until cond(snapshot) or timeout.
func pollUntil(t *testing.T, h http.Handler, id string, cond func(map[string]any) bool) map[string]any {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		rr := req(t, h, http.MethodGet, "/tasks/"+id, "", true)
		var snap map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &snap)
		if cond(snap) {
			return snap
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for task condition")
	return nil
}

func TestHealthzAndAuth(t *testing.T) {
	h := newServer()
	if rr := req(t, h, http.MethodGet, "/healthz", "", false); rr.Code != http.StatusOK {
		t.Fatalf("healthz = %d", rr.Code)
	}
	if rr := req(t, h, http.MethodPost, "/tasks", `{"inline_goal":"x"}`, false); rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated submit should be 401, got %d", rr.Code)
	}
}

func TestSubmitApproveFlow(t *testing.T) {
	h := newServer()

	// Submit.
	rr := req(t, h, http.MethodPost, "/tasks", `{"inline_goal":"send the returns","identity":"alice","committee":"finance"}`, true)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("submit = %d (%s)", rr.Code, rr.Body.String())
	}
	var sub map[string]string
	_ = json.Unmarshal(rr.Body.Bytes(), &sub)
	id := sub["task_id"]
	if id == "" {
		t.Fatal("no task_id returned")
	}

	// Poll until the high-impact tool is awaiting approval.
	snap := pollUntil(t, h, id, func(s map[string]any) bool { return s["status"] == "awaiting_approval" })
	pending, _ := snap["pending_approval"].(map[string]any)
	if pending == nil || pending["approval_id"] != "appr-1" {
		t.Fatalf("expected pending approval appr-1, got %v", snap["pending_approval"])
	}

	// Approve.
	ar := req(t, h, http.MethodPost, "/tasks/"+id+"/approve", `{"approval_id":"appr-1","granted":true,"decided_by":"lead@scottylabs.org"}`, true)
	if ar.Code != http.StatusOK {
		t.Fatalf("approve = %d (%s)", ar.Code, ar.Body.String())
	}

	// Poll until done with the output.
	done := pollUntil(t, h, id, func(s map[string]any) bool { return s["status"] == "done" })
	result, _ := done["result"].(map[string]any)
	if result == nil || result["output"] != "sent" {
		t.Fatalf("expected result output 'sent', got %v", done["result"])
	}
}

func TestUnknownTask(t *testing.T) {
	h := newServer()
	if rr := req(t, h, http.MethodGet, "/tasks/nope", "", true); rr.Code != http.StatusNotFound {
		t.Fatalf("unknown task should be 404, got %d", rr.Code)
	}
}
