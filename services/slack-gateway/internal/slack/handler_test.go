package slack

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slack-go/slack"

	"github.com/scottylabs/scottylabs-agent/services/slack-gateway/internal/runtimeclient"
)

type fakeRT struct {
	submitted runtimeclient.SubmitBody
	snaps     []runtimeclient.Snapshot
	i         int
	approved  []string
}

func (f *fakeRT) Submit(_ context.Context, b runtimeclient.SubmitBody) (string, error) {
	f.submitted = b
	return "t1", nil
}
func (f *fakeRT) Get(_ context.Context, _ string) (runtimeclient.Snapshot, error) {
	s := f.snaps[min(f.i, len(f.snaps)-1)]
	f.i++
	return s, nil
}
func (f *fakeRT) Approve(_ context.Context, taskID, approvalID string, granted bool, by string) error {
	f.approved = append(f.approved, fmt.Sprintf("%s|%s|%v|%s", taskID, approvalID, granted, by))
	return nil
}

type fakePoster struct {
	mu   sync.Mutex
	msgs []Message
}

func (p *fakePoster) Post(_ context.Context, m Message) error {
	p.mu.Lock()
	p.msgs = append(p.msgs, m)
	p.mu.Unlock()
	return nil
}
func (p *fakePoster) all() []Message {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]Message{}, p.msgs...)
}

func newH(rt Runtime, p Poster) *Handler {
	return &Handler{rt: rt, poster: p, pollEvery: time.Millisecond, pollFor: 2 * time.Second}
}

func sectionText(m Message) string {
	var sb strings.Builder
	sb.WriteString(m.Fallback)
	for _, b := range m.Blocks {
		if s, ok := b.(*slack.SectionBlock); ok && s.Text != nil {
			sb.WriteString(" " + s.Text.Text)
		}
	}
	return sb.String()
}
func hasApprovalBlock(m Message) bool {
	for _, b := range m.Blocks {
		if _, ok := b.(*slack.ActionBlock); ok {
			return true
		}
	}
	return false
}

func TestRunTaskReportsResult(t *testing.T) {
	rt := &fakeRT{snaps: []runtimeclient.Snapshot{{Status: "done", Result: &runtimeclient.Result{Output: "screened 4 requests"}}}}
	fp := &fakePoster{}
	newH(rt, fp).runTask(context.Background(), "C1", "ts1", "slack:U1", "screen reimbursements")

	if rt.submitted.InlineGoal != "screen reimbursements" || rt.submitted.Identity != "slack:U1" {
		t.Fatalf("task not submitted correctly: %+v", rt.submitted)
	}
	msgs := fp.all()
	if len(msgs) < 2 {
		t.Fatalf("expected working + result messages, got %d", len(msgs))
	}
	if !strings.Contains(sectionText(msgs[0]), "working") {
		t.Fatalf("first message should acknowledge: %q", sectionText(msgs[0]))
	}
	if !strings.Contains(sectionText(msgs[len(msgs)-1]), "screened 4 requests") {
		t.Fatalf("last message should carry the output: %q", sectionText(msgs[len(msgs)-1]))
	}
	for _, m := range msgs {
		if m.Channel != "C1" || m.ThreadTS != "ts1" {
			t.Fatalf("message posted to wrong thread: %+v", m)
		}
	}
}

func TestRunTaskApprovalThenDone(t *testing.T) {
	pend := &runtimeclient.Pending{Tool: "google.workspace/gmail_send", ApprovalID: "a1"}
	rt := &fakeRT{snaps: []runtimeclient.Snapshot{
		{Status: "awaiting_approval", Pending: pend},
		{Status: "awaiting_approval", Pending: pend},
		{Status: "done", Result: &runtimeclient.Result{Output: "sent"}},
	}}
	fp := &fakePoster{}
	newH(rt, fp).runTask(context.Background(), "C1", "ts1", "slack:U1", "send the drafted returns")

	approvals, dones := 0, 0
	for _, m := range fp.all() {
		if hasApprovalBlock(m) {
			approvals++
		}
		if strings.Contains(m.Fallback, "Done") {
			dones++
		}
	}
	if approvals != 1 {
		t.Fatalf("approval block should post exactly once, got %d", approvals)
	}
	if dones != 1 {
		t.Fatalf("expected one done message, got %d", dones)
	}
}

func TestInteractionApprove(t *testing.T) {
	rt := &fakeRT{}
	fp := &fakePoster{}
	h := newH(rt, fp)

	cb := slack.InteractionCallback{
		Type: slack.InteractionTypeBlockActions,
		User: slack.User{ID: "U9"},
		ActionCallback: slack.ActionCallbacks{
			BlockActions: []*slack.BlockAction{{ActionID: ActionApprove, Value: "t1::a1"}},
		},
	}
	payload, _ := json.Marshal(cb)
	form := url.Values{"payload": {string(payload)}}.Encode()
	req := httptest.NewRequest("POST", "/slack/interactions", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.Interactions(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("interactions = %d", rr.Code)
	}
	if len(rt.approved) != 1 || rt.approved[0] != "t1|a1|true|slack:U9" {
		t.Fatalf("approve not recorded correctly: %v", rt.approved)
	}
}

func TestEventsURLVerification(t *testing.T) {
	h := newH(&fakeRT{}, &fakePoster{})
	body := `{"type":"url_verification","challenge":"chal-xyz"}`
	req := httptest.NewRequest("POST", "/slack/events", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.Events(rr, req)
	if rr.Body.String() != "chal-xyz" {
		t.Fatalf("challenge not echoed: %q", rr.Body.String())
	}
}

func TestSignatureVerification(t *testing.T) {
	secret := "8f742231b10e8888abcd99yyyzzz85a5"
	h := &Handler{rt: &fakeRT{}, poster: &fakePoster{}, signingSecret: secret, pollEvery: time.Millisecond, pollFor: time.Second}
	body := `{"type":"url_verification","challenge":"ok"}`
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("v0:" + ts + ":" + body))
	sig := "v0=" + hex.EncodeToString(mac.Sum(nil))

	good := httptest.NewRequest("POST", "/slack/events", strings.NewReader(body))
	good.Header.Set("X-Slack-Request-Timestamp", ts)
	good.Header.Set("X-Slack-Signature", sig)
	rr := httptest.NewRecorder()
	h.Events(rr, good)
	if rr.Code != http.StatusOK || rr.Body.String() != "ok" {
		t.Fatalf("valid signature should pass: %d %q", rr.Code, rr.Body.String())
	}

	bad := httptest.NewRequest("POST", "/slack/events", strings.NewReader(body))
	bad.Header.Set("X-Slack-Request-Timestamp", ts)
	bad.Header.Set("X-Slack-Signature", "v0=deadbeef")
	rr2 := httptest.NewRecorder()
	h.Events(rr2, bad)
	if rr2.Code != http.StatusUnauthorized {
		t.Fatalf("bad signature should be 401, got %d", rr2.Code)
	}
}

func TestCleanText(t *testing.T) {
	if got := cleanText("<@U123BOT> screen the reimbursements"); got != "screen the reimbursements" {
		t.Fatalf("cleanText = %q", got)
	}
}
