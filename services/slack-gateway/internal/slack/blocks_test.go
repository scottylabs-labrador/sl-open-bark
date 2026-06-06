package slack

import (
	"testing"

	"github.com/slack-go/slack"
)

func TestValueEncoding(t *testing.T) {
	if got := encodeValue("t1", "a1"); got != "t1::a1" {
		t.Fatalf("encode = %q", got)
	}
	tID, aID := decodeValue("t1::a1")
	if tID != "t1" || aID != "a1" {
		t.Fatalf("decode = %q, %q", tID, aID)
	}
}

func TestApprovalBlocksButtons(t *testing.T) {
	blocks := approvalBlocks("task9", "appr9", "google.workspace/gmail_send", `{"to":"***"}`)
	var action *slack.ActionBlock
	for _, b := range blocks {
		if ab, ok := b.(*slack.ActionBlock); ok {
			action = ab
		}
	}
	if action == nil {
		t.Fatal("approval blocks must include an actions block")
	}
	ids := map[string]string{}
	for _, el := range action.Elements.ElementSet {
		btn, ok := el.(*slack.ButtonBlockElement)
		if !ok {
			continue
		}
		ids[btn.ActionID] = btn.Value
	}
	for _, want := range []string{ActionApprove, ActionReview, ActionCancel} {
		if ids[want] != "task9::appr9" {
			t.Fatalf("button %s value = %q, want task9::appr9", want, ids[want])
		}
	}
}
