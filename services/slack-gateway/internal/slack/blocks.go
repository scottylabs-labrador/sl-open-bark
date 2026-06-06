package slack

import (
	"fmt"
	"strings"

	"github.com/slack-go/slack"
)

// Action ids on the approval buttons. The Slack interaction payload carries the id (the decision)
// and the button value (taskID::approvalID).
const (
	ActionApprove = "sl_approve"
	ActionReview  = "sl_review"
	ActionCancel  = "sl_cancel"
)

const valueSep = "::"

func encodeValue(taskID, approvalID string) string { return taskID + valueSep + approvalID }

func decodeValue(v string) (taskID, approvalID string) {
	parts := strings.SplitN(v, valueSep, 2)
	if len(parts) != 2 {
		return v, ""
	}
	return parts[0], parts[1]
}

func mrkdwn(s string) *slack.TextBlockObject {
	return slack.NewTextBlockObject(slack.MarkdownType, s, false, false)
}

// statusBlocks is the fast "working on it" acknowledgement posted in-thread.
func statusBlocks(text string) []slack.Block {
	return []slack.Block{slack.NewSectionBlock(mrkdwn(":robot_face: "+text), nil, nil)}
}

// resultBlocks renders a finished task's output.
func resultBlocks(output string) []slack.Block {
	if strings.TrimSpace(output) == "" {
		output = "_(no output)_"
	}
	return []slack.Block{
		slack.NewSectionBlock(mrkdwn(":white_check_mark: *Done.*\n"+truncate(output, 2800)), nil, nil),
		slack.NewContextBlock("", mrkdwn("ScottyLabs Agent · audited")),
	}
}

// errorBlocks renders a failed task.
func errorBlocks(msg string) []slack.Block {
	return []slack.Block{slack.NewSectionBlock(mrkdwn(":x: *Task failed:* "+truncate(msg, 2800)), nil, nil)}
}

// approvalBlocks renders a high-impact action awaiting a human, with Approve / Review / Cancel
// (design 9.4). Nothing runs until a human acts; the button value carries the task + approval ids.
func approvalBlocks(taskID, approvalID, tool, argsJSON string) []slack.Block {
	val := encodeValue(taskID, approvalID)
	header := slack.NewSectionBlock(mrkdwn(fmt.Sprintf(
		":warning: *Approval required* — high-impact action\n*Tool:* `%s`", tool)), nil, nil)
	args := slack.NewSectionBlock(mrkdwn("*Arguments (redacted):*\n```"+truncate(argsJSON, 1200)+"```"), nil, nil)
	approve := slack.NewButtonBlockElement(ActionApprove, val, slack.NewTextBlockObject(slack.PlainTextType, "Approve", false, false)).WithStyle(slack.StylePrimary)
	review := slack.NewButtonBlockElement(ActionReview, val, slack.NewTextBlockObject(slack.PlainTextType, "Review", false, false))
	cancel := slack.NewButtonBlockElement(ActionCancel, val, slack.NewTextBlockObject(slack.PlainTextType, "Cancel", false, false)).WithStyle(slack.StyleDanger)
	actions := slack.NewActionBlock("sl_approval", approve, review, cancel)
	return []slack.Block{header, args, actions}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
