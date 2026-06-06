// Package runtime is the thin AgentRuntime boundary (design Sections 4.2, 5): the seam every
// surface (Slack Gateway, Scheduler) depends on, so the platform is not hostage to one runtime's
// roadmap. The agent loop itself runs in an Engine (Goose is the default implementation); this
// package coordinates recipe loading, per-task model selection, and human-approval pause/resume,
// and is fully testable with a fake Engine.
package runtime

import (
	"context"
	"errors"
	"sync"
)

// TaskRequest describes a unit of work. Either RecipeID (a workflow to load) or InlineGoal (an
// ad-hoc goal) is set. Identity and Committee scope the work; HardTask hints that the model should
// escalate to a stronger model.
type TaskRequest struct {
	RecipeID   string
	InlineGoal string
	Params     map[string]string
	Identity   string
	Committee  string
	HardTask   bool
}

// EventKind enumerates the events a task streams.
type EventKind string

const (
	KindStatus           EventKind = "status"
	KindToolCall         EventKind = "tool_call"
	KindApprovalRequired EventKind = "approval_required"
	KindOutput           EventKind = "output"
	KindError            EventKind = "error"
	KindDone             EventKind = "done"
)

// Event is one item in a task's event stream.
type Event struct {
	Kind       EventKind `json:"kind"`
	Text       string    `json:"text,omitempty"`
	Tool       string    `json:"tool,omitempty"`        // tool_call / approval_required
	ApprovalID string    `json:"approval_id,omitempty"` // approval_required
}

// Result is a finished task's output plus a reference into the audit log.
type Result struct {
	Output   string `json:"output"`
	AuditRef string `json:"audit_ref,omitempty"`
}

// AgentRuntime is the contract callers depend on. submit_task returns a Task whose Events stream
// the run; high-impact actions surface an approval_required event and pause until ResolveApproval.
type AgentRuntime interface {
	SubmitTask(ctx context.Context, req TaskRequest) (*Task, error)
}

// Task is a running task handle.
type Task struct {
	events    chan Event
	decisions chan decision
	done      chan struct{}

	mu     sync.Mutex
	result Result
	err    error
}

type decision struct {
	approvalID string
	granted    bool
	decidedBy  string
}

// Events returns the task's event stream. It is closed when the task finishes.
func (t *Task) Events() <-chan Event { return t.events }

// ResolveApproval delivers a human decision for a pending approval, resuming (or aborting) the
// task. It is safe to call from another goroutine. Returns an error if the task already finished.
func (t *Task) ResolveApproval(approvalID string, granted bool, decidedBy string) error {
	select {
	case t.decisions <- decision{approvalID: approvalID, granted: granted, decidedBy: decidedBy}:
		return nil
	case <-t.done:
		return errors.New("runtime: task already finished")
	}
}

// Result blocks until the task finishes and returns its output (or error).
func (t *Task) Result() (Result, error) {
	<-t.done
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.result, t.err
}
