// Package runtimeclient is a thin HTTP client for the runtime service's task API (WP-06). The
// dashboard uses it to submit tasks, poll their event stream, and relay approval decisions.
package runtimeclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client calls the runtime service. A zero baseURL means "not configured" (the console is disabled).
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// New builds a client. baseURL like "http://runtime.railway.internal:8080".
func New(baseURL, token string) *Client {
	return &Client{baseURL: baseURL, token: token, http: &http.Client{Timeout: 30 * time.Second}}
}

// Configured reports whether a runtime URL is set.
func (c *Client) Configured() bool { return c.baseURL != "" }

// Do performs a request against the runtime and relays the raw body + status, so the dashboard can
// proxy the runtime's JSON straight through to the browser.
func (c *Client) Do(ctx context.Context, method, path string, body []byte) ([]byte, int, error) {
	if !c.Configured() {
		return nil, http.StatusServiceUnavailable, fmt.Errorf("runtime not configured")
	}
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, r)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, http.StatusBadGateway, err
	}
	defer func() { _ = resp.Body.Close() }()
	out, err := io.ReadAll(resp.Body)
	return out, resp.StatusCode, err
}
