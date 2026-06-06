// Package clients holds concrete implementations of external collaborators. They satisfy the
// interfaces defined by their consumers (for example service.AuditSink) structurally, so this
// package does not import the service package and there is no import cycle.
package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// InMemoryAudit keeps events in memory and logs them. Good default for local dev and tests.
type InMemoryAudit struct {
	Events []map[string]any
}

func NewInMemoryAudit() *InMemoryAudit { return &InMemoryAudit{} }

func (a *InMemoryAudit) Record(_ context.Context, actor, action string, payload map[string]any) error {
	event := map[string]any{"actor": actor, "action": action}
	for k, v := range payload {
		event[k] = v
	}
	a.Events = append(a.Events, event)
	slog.Info("audit", "actor", actor, "action", action)
	return nil
}

// HTTPAudit posts events to an external audit endpoint. Audit is best-effort: a failure is
// logged but never breaks a tool call.
type HTTPAudit struct {
	URL  string
	HTTP *http.Client
}

func NewHTTPAudit(url string) *HTTPAudit {
	return &HTTPAudit{URL: url, HTTP: &http.Client{Timeout: 5 * time.Second}}
}

func (a *HTTPAudit) Record(ctx context.Context, actor, action string, payload map[string]any) error {
	body := map[string]any{"actor": actor, "action": action}
	for k, v := range payload {
		body[k] = v
	}
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.URL, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.HTTP.Do(req)
	if err != nil {
		slog.Warn("audit post failed", "err", err)
		return nil
	}
	defer func() { _ = resp.Body.Close() }()
	return nil
}
