// Package server exposes the AgentRuntime as a long-running HTTP service — the API the Slack
// Gateway and Scheduler call. It is poll-based: submit a task, read its accumulated events and any
// pending approval, post an approval decision, read the result. The agent loop runs in the
// background; this layer is thin and tracks per-task state.
package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sync"

	runtime "github.com/scottylabs/scottylabs-agent/runtime"
)

// Manager tracks running tasks by id and drains each task's event stream into a snapshot the HTTP
// handlers serve.
type Manager struct {
	rt    runtime.AgentRuntime
	mu    sync.Mutex
	tasks map[string]*taskState
}

// NewManager builds a Manager over an AgentRuntime.
func NewManager(rt runtime.AgentRuntime) *Manager {
	return &Manager{rt: rt, tasks: map[string]*taskState{}}
}

type pendingApproval struct {
	Tool       string `json:"tool"`
	ApprovalID string `json:"approval_id"`
}

type taskState struct {
	mu      sync.Mutex
	task    *runtime.Task
	events  []runtime.Event
	pending *pendingApproval
	status  string // running | awaiting_approval | done | error
	result  runtime.Result
	errMsg  string
}

func newID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// Submit starts a task and returns its id. Events are drained in the background so a poller never
// blocks the agent loop.
func (m *Manager) Submit(req runtime.TaskRequest) (string, error) {
	task, err := m.rt.SubmitTask(context.Background(), req)
	if err != nil {
		return "", err
	}
	ts := &taskState{task: task, status: "running"}
	id := newID()

	m.mu.Lock()
	m.tasks[id] = ts
	m.mu.Unlock()

	go m.drain(ts)
	return id, nil
}

func (m *Manager) drain(ts *taskState) {
	for e := range ts.task.Events() {
		ts.mu.Lock()
		ts.events = append(ts.events, e)
		switch e.Kind {
		case runtime.KindApprovalRequired:
			ts.status = "awaiting_approval"
			ts.pending = &pendingApproval{Tool: e.Tool, ApprovalID: e.ApprovalID}
		case runtime.KindError:
			ts.errMsg = e.Text
		}
		ts.mu.Unlock()
	}
	res, err := ts.task.Result()
	ts.mu.Lock()
	ts.result = res
	if err != nil {
		ts.status = "error"
		if ts.errMsg == "" {
			ts.errMsg = err.Error()
		}
	} else {
		ts.status = "done"
	}
	ts.pending = nil
	ts.mu.Unlock()
}

func (m *Manager) get(id string) (*taskState, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ts, ok := m.tasks[id]
	return ts, ok
}

// snapshot is the JSON view of a task.
type snapshot struct {
	Status  string           `json:"status"`
	Events  []runtime.Event  `json:"events"`
	Pending *pendingApproval `json:"pending_approval,omitempty"`
	Result  *runtime.Result  `json:"result,omitempty"`
	Error   string           `json:"error,omitempty"`
}

func (ts *taskState) snapshot() snapshot {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	s := snapshot{Status: ts.status, Events: append([]runtime.Event{}, ts.events...), Pending: ts.pending, Error: ts.errMsg}
	if ts.status == "done" {
		r := ts.result
		s.Result = &r
	}
	return s
}

// Server wraps the Manager with HTTP routes and bearer auth.
type Server struct {
	mgr   *Manager
	token string
}

// New builds the HTTP server. token is the bearer callers must present.
func New(rt runtime.AgentRuntime, token string) *Server {
	return &Server{mgr: NewManager(rt), token: token}
}

// Handler returns the runtime service routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.Handle("POST /tasks", s.auth(http.HandlerFunc(s.handleSubmit)))
	mux.Handle("GET /tasks/{id}", s.auth(http.HandlerFunc(s.handleGet)))
	mux.Handle("POST /tasks/{id}/approve", s.auth(http.HandlerFunc(s.handleApprove)))
	return mux
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.token == "" || r.Header.Get("Authorization") != "Bearer "+s.token {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleSubmit(w http.ResponseWriter, r *http.Request) {
	var req runtime.TaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	id, err := s.mgr.Submit(req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"task_id": id})
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	ts, ok := s.mgr.get(r.PathValue("id"))
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such task"})
		return
	}
	writeJSON(w, http.StatusOK, ts.snapshot())
}

type approveBody struct {
	ApprovalID string `json:"approval_id"`
	Granted    bool   `json:"granted"`
	DecidedBy  string `json:"decided_by"`
}

func (s *Server) handleApprove(w http.ResponseWriter, r *http.Request) {
	ts, ok := s.mgr.get(r.PathValue("id"))
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such task"})
		return
	}
	var body approveBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	if err := ts.task.ResolveApproval(body.ApprovalID, body.Granted, body.DecidedBy); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	ts.mu.Lock()
	if ts.status == "awaiting_approval" {
		ts.status = "running"
		ts.pending = nil
	}
	ts.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"status": "resolved"})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
