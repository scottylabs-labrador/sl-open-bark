// Package runtimeclient is the Slack gateway's HTTP client for the runtime task API (WP-06). The
// gateway acks Slack fast, then drives the agent through this client in the background.
package runtimeclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// SubmitBody starts a task.
type SubmitBody struct {
	InlineGoal string `json:"inline_goal,omitempty"`
	RecipeID   string `json:"recipe_id,omitempty"`
	Identity   string `json:"identity,omitempty"`
	Committee  string `json:"committee,omitempty"`
	HardTask   bool   `json:"hard_task,omitempty"`
}

// Event is one streamed event.
type Event struct {
	Kind       string `json:"kind"`
	Text       string `json:"text"`
	Tool       string `json:"tool"`
	ApprovalID string `json:"approval_id"`
}

// Pending is a high-impact action awaiting approval.
type Pending struct {
	Tool       string `json:"tool"`
	ApprovalID string `json:"approval_id"`
}

// Result is the finished output.
type Result struct {
	Output   string `json:"output"`
	AuditRef string `json:"audit_ref"`
}

// Snapshot is the runtime's task view.
type Snapshot struct {
	Status  string   `json:"status"`
	Events  []Event  `json:"events"`
	Pending *Pending `json:"pending_approval"`
	Result  *Result  `json:"result"`
}

// Client calls the runtime service.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// New builds a client. An empty baseURL means "not configured".
func New(baseURL, token string) *Client {
	return &Client{baseURL: baseURL, token: token, http: &http.Client{Timeout: 30 * time.Second}}
}

// Configured reports whether a runtime URL is set.
func (c *Client) Configured() bool { return c.baseURL != "" }

// Submit starts a task and returns its id.
func (c *Client) Submit(ctx context.Context, body SubmitBody) (string, error) {
	var out struct {
		TaskID string `json:"task_id"`
		Error  string `json:"error"`
	}
	if err := c.do(ctx, http.MethodPost, "/tasks", body, &out); err != nil {
		return "", err
	}
	if out.TaskID == "" {
		return "", fmt.Errorf("runtime: %s", out.Error)
	}
	return out.TaskID, nil
}

// Get fetches a task snapshot.
func (c *Client) Get(ctx context.Context, taskID string) (Snapshot, error) {
	var s Snapshot
	err := c.do(ctx, http.MethodGet, "/tasks/"+taskID, nil, &s)
	return s, err
}

// Approve resolves a pending approval on a task.
func (c *Client) Approve(ctx context.Context, taskID, approvalID string, granted bool, by string) error {
	body := map[string]any{"approval_id": approvalID, "granted": granted, "decided_by": by}
	return c.do(ctx, http.MethodPost, "/tasks/"+taskID+"/approve", body, nil)
}

func (c *Client) do(ctx context.Context, method, path string, in, out any) error {
	if !c.Configured() {
		return fmt.Errorf("runtime not configured")
	}
	body := bytes.NewReader(nil)
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("runtime: status %d", resp.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
