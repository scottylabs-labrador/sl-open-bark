package slack

import (
	"context"
	"log/slog"

	"github.com/slack-go/slack"
)

// SlackPoster posts to Slack via the Web API (chat.postMessage).
type SlackPoster struct{ api *slack.Client }

// NewSlackPoster builds a poster from a bot token.
func NewSlackPoster(botToken string) *SlackPoster { return &SlackPoster{api: slack.New(botToken)} }

// Post sends a message (optionally threaded, with blocks) to a channel.
func (p *SlackPoster) Post(ctx context.Context, m Message) error {
	opts := []slack.MsgOption{slack.MsgOptionText(m.Fallback, false)}
	if len(m.Blocks) > 0 {
		opts = append(opts, slack.MsgOptionBlocks(m.Blocks...))
	}
	if m.ThreadTS != "" {
		opts = append(opts, slack.MsgOptionTS(m.ThreadTS))
	}
	_, _, err := p.api.PostMessageContext(ctx, m.Channel, opts...)
	return err
}

// LogPoster logs instead of posting — used when no Slack bot token is configured (dev mode).
type LogPoster struct{}

// Post logs the fallback text.
func (LogPoster) Post(_ context.Context, m Message) error {
	slog.Info("slack post (dev)", "channel", m.Channel, "thread", m.ThreadTS, "text", m.Fallback, "blocks", len(m.Blocks))
	return nil
}
