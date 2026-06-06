// Package notify posts the Engineering Agent's results to Slack. It uses a single-channel incoming
// webhook (low privilege — not a broad bot token), keeping the isolated subsystem's outward reach
// minimal.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Webhook posts to a Slack incoming-webhook URL.
type Webhook struct {
	url  string
	http *http.Client
}

// NewWebhook builds a webhook notifier.
func NewWebhook(url string) *Webhook {
	return &Webhook{url: url, http: &http.Client{Timeout: 10 * time.Second}}
}

// Post sends text to the webhook's channel (the channel arg is informational; the webhook is bound
// to one channel).
func (w *Webhook) Post(ctx context.Context, _ /*channel*/, text string) error {
	body, _ := json.Marshal(map[string]string{"text": text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := w.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("notify: webhook status %d", resp.StatusCode)
	}
	return nil
}

// Log logs the result instead of posting — used when no webhook is configured.
type Log struct{}

// Post logs the text.
func (Log) Post(_ context.Context, channel, text string) error {
	slog.Info("engineering-agent result (no webhook)", "channel", channel, "text", text)
	return nil
}
