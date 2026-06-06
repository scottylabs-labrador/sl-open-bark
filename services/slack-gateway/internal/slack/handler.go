// Package slack is the Slack gateway: the human front door (design 3.3, 9.4; Appendix E). It
// verifies Slack request signatures, acknowledges within Slack's timeout, runs work in the
// background through the runtime, and renders approval blocks wired to runtime approvals. External
// systems (Slack posting, the runtime) are behind interfaces so the logic is testable.
package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"

	"github.com/scottylabs/scottylabs-agent/services/slack-gateway/internal/runtimeclient"
)

// Runtime is the slice of the runtime API the gateway needs (defined at the consumer).
type Runtime interface {
	Submit(ctx context.Context, body runtimeclient.SubmitBody) (string, error)
	Get(ctx context.Context, taskID string) (runtimeclient.Snapshot, error)
	Approve(ctx context.Context, taskID, approvalID string, granted bool, by string) error
}

// Message is one outbound Slack post.
type Message struct {
	Channel  string
	ThreadTS string
	Fallback string
	Blocks   []slack.Block
}

// Poster posts a message to Slack. The real implementation wraps slack.Client; tests use a fake.
type Poster interface {
	Post(ctx context.Context, m Message) error
}

// Handler routes Slack events, interactions, and slash commands.
type Handler struct {
	rt            Runtime
	poster        Poster
	signingSecret string
	botUserID     string
	pollEvery     time.Duration
	pollFor       time.Duration
}

// New builds a Handler.
func New(rt Runtime, poster Poster, signingSecret, botUserID string) *Handler {
	return &Handler{rt: rt, poster: poster, signingSecret: signingSecret, botUserID: botUserID,
		pollEvery: 900 * time.Millisecond, pollFor: 100 * time.Second}
}

// verify reads and (unless dev mode) verifies the Slack request signature, returning the raw body.
func (h *Handler) verify(r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	if h.signingSecret == "" {
		return body, nil // dev mode: signature checks disabled
	}
	sv, err := slack.NewSecretsVerifier(r.Header, h.signingSecret)
	if err != nil {
		return nil, err
	}
	if _, err := sv.Write(body); err != nil {
		return nil, err
	}
	if err := sv.Ensure(); err != nil {
		return nil, err
	}
	return body, nil
}

// Events handles the Slack Events API: URL verification, app mentions, and DMs. It acks fast and
// processes in the background (Appendix E).
func (h *Handler) Events(w http.ResponseWriter, r *http.Request) {
	body, err := h.verify(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	ev, err := slackevents.ParseEvent(body, slackevents.OptionNoVerifyToken())
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if ev.Type == slackevents.URLVerification {
		var c slackevents.ChallengeResponse
		_ = json.Unmarshal(body, &c)
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(c.Challenge))
		return
	}
	w.WriteHeader(http.StatusOK) // fast ack; work happens in the background
	if ev.Type != slackevents.CallbackEvent {
		return
	}
	switch e := ev.InnerEvent.Data.(type) {
	case *slackevents.AppMentionEvent:
		if e.User == h.botUserID {
			return
		}
		go h.runTask(context.Background(), e.Channel, firstNonEmpty(e.ThreadTimeStamp, e.TimeStamp), identity(e.User), cleanText(e.Text))
	case *slackevents.MessageEvent:
		if e.BotID != "" || e.User == h.botUserID || e.SubType != "" || e.ChannelType != "im" {
			return
		}
		go h.runTask(context.Background(), e.Channel, firstNonEmpty(e.ThreadTimeStamp, e.TimeStamp), identity(e.User), cleanText(e.Text))
	}
}

// runTask submits a task and reports progress, approvals, and the result back to the thread.
func (h *Handler) runTask(ctx context.Context, channel, threadTS, identity, text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	h.post(ctx, channel, threadTS, "Got it — working on it.", statusBlocks("Got it — working on it…"))

	taskID, err := h.rt.Submit(ctx, runtimeclient.SubmitBody{InlineGoal: text, Identity: identity})
	if err != nil {
		h.post(ctx, channel, threadTS, "Could not start the task.", errorBlocks(err.Error()))
		return
	}

	posted := map[string]bool{}
	deadline := time.Now().Add(h.pollFor)
	for time.Now().Before(deadline) {
		snap, err := h.rt.Get(ctx, taskID)
		if err == nil {
			if snap.Pending != nil && !posted[snap.Pending.ApprovalID] {
				posted[snap.Pending.ApprovalID] = true
				h.post(ctx, channel, threadTS, "Approval required.", approvalBlocks(taskID, snap.Pending.ApprovalID, snap.Pending.Tool, ""))
			}
			switch snap.Status {
			case "done":
				out := ""
				if snap.Result != nil {
					out = snap.Result.Output
				}
				h.post(ctx, channel, threadTS, "Done.", resultBlocks(out))
				return
			case "error":
				h.post(ctx, channel, threadTS, "Task failed.", errorBlocks(lastError(snap)))
				return
			}
		}
		time.Sleep(h.pollEvery)
	}
	h.post(ctx, channel, threadTS, "Task timed out.", errorBlocks("task timed out before completing"))
}

// Interactions handles approval button clicks (design 9.4).
func (h *Handler) Interactions(w http.ResponseWriter, r *http.Request) {
	body, err := h.verify(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	vals, _ := url.ParseQuery(string(body))
	var cb slack.InteractionCallback
	if err := json.Unmarshal([]byte(vals.Get("payload")), &cb); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
	if cb.Type != slack.InteractionTypeBlockActions {
		return
	}
	for _, a := range cb.ActionCallback.BlockActions {
		taskID, approvalID := decodeValue(a.Value)
		by := identity(cb.User.ID)
		ch, ts := cb.Channel.ID, cb.Message.Timestamp
		switch a.ActionID {
		case ActionApprove:
			_ = h.rt.Approve(context.Background(), taskID, approvalID, true, by)
			h.post(context.Background(), ch, ts, "Approved.", statusBlocks(fmt.Sprintf("Approved by <@%s> — resuming.", cb.User.ID)))
		case ActionCancel:
			_ = h.rt.Approve(context.Background(), taskID, approvalID, false, by)
			h.post(context.Background(), ch, ts, "Cancelled.", statusBlocks(fmt.Sprintf(":no_entry: Cancelled by <@%s>.", cb.User.ID)))
		case ActionReview:
			h.post(context.Background(), ch, ts, "Left for review.", statusBlocks(fmt.Sprintf("Left pending for review by <@%s>.", cb.User.ID)))
		}
	}
}

// Command handles slash commands. /fix-bug enqueues to the Engineering Agent (WP-11, stub).
func (h *Handler) Command(w http.ResponseWriter, r *http.Request) {
	body, err := h.verify(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	vals, _ := url.ParseQuery(string(body))
	cmd, text := vals.Get("command"), vals.Get("text")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"response_type": "ephemeral",
		"text":          fmt.Sprintf(":wrench: `%s %s` enqueued to the Engineering Agent. You'll get a draft PR link here when it's ready. (WP-11)", cmd, text),
	})
}

func (h *Handler) post(ctx context.Context, channel, threadTS, fallback string, blocks []slack.Block) {
	_ = h.poster.Post(ctx, Message{Channel: channel, ThreadTS: threadTS, Fallback: fallback, Blocks: blocks})
}

var mentionRE = regexp.MustCompile(`<@[A-Z0-9]+>`)

func cleanText(s string) string { return strings.TrimSpace(mentionRE.ReplaceAllString(s, "")) }
func identity(slackUserID string) string {
	if slackUserID == "" {
		return "slack:unknown"
	}
	return "slack:" + slackUserID
}
func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
func lastError(s runtimeclient.Snapshot) string {
	for i := len(s.Events) - 1; i >= 0; i-- {
		if s.Events[i].Kind == "error" {
			return s.Events[i].Text
		}
	}
	return "task failed"
}
