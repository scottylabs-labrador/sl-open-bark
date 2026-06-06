// Package runtimeclient submits scheduled recipes to the runtime service (WP-06).
package runtimeclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client calls the runtime service.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// New builds a client. An empty baseURL means "not configured".
func New(baseURL, token string) *Client {
	return &Client{baseURL: baseURL, token: token, http: &http.Client{Timeout: 20 * time.Second}}
}

// Configured reports whether a runtime URL is set.
func (c *Client) Configured() bool { return c.baseURL != "" }

// Submit starts a recipe task and returns its id.
func (c *Client) Submit(ctx context.Context, recipeID, committee string, params map[string]string) (string, error) {
	if !c.Configured() {
		return "", fmt.Errorf("runtime not configured")
	}
	body, _ := json.Marshal(map[string]any{"recipe_id": recipeID, "committee": committee, "params": params})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/tasks", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("runtime: status %d", resp.StatusCode)
	}
	var out struct {
		TaskID string `json:"task_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.TaskID, nil
}
