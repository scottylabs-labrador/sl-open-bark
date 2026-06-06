// Package proxy is the downstream side of the gateway: it forwards an authorized tool call to the
// target MCP server over Streamable HTTP (a JSON-RPC tools/call), injecting that server's bearer
// credential. The policy decision has already been made by the time Call runs.
//
// This is a best-effort MCP client sufficient for the policy layer's contract; full streaming and
// session semantics are exercised end to end in WP-08.
package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/scottylabs/scottylabs-agent/services/session-memory/store"
)

// HTTPCaller calls downstream MCP servers over HTTP.
type HTTPCaller struct {
	token string
	http  *http.Client
}

// NewHTTPCaller builds a caller that presents token as the downstream bearer.
func NewHTTPCaller(token string) *HTTPCaller {
	return &HTTPCaller{token: token, http: &http.Client{Timeout: 30 * time.Second}}
}

type jsonRPCRequest struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      int            `json:"id"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params"`
}

type jsonRPCResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// Call invokes tool on server with args, returning the raw result payload.
func (c *HTTPCaller) Call(ctx context.Context, server store.Server, tool store.Tool, args json.RawMessage) (json.RawMessage, error) {
	if len(args) == 0 {
		args = json.RawMessage("{}")
	}
	body, err := json.Marshal(jsonRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{"name": tool.Name, "arguments": json.RawMessage(args)},
	})
	if err != nil {
		return nil, fmt.Errorf("proxy: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("proxy: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("proxy: call %s: %w", server.Name, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("proxy: %s returned %d", server.Name, resp.StatusCode)
	}

	var rpc jsonRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpc); err != nil {
		return nil, fmt.Errorf("proxy: decode response: %w", err)
	}
	if rpc.Error != nil {
		return nil, fmt.Errorf("proxy: tool error %d: %s", rpc.Error.Code, rpc.Error.Message)
	}
	return rpc.Result, nil
}
