// Package orchestrator drives a coding task in a throwaway sandbox and opens a draft PR (design
// 12.2). It is an isolated trust domain: it touches only a scoped GitHub identity and an
// allowlisted-egress sandbox — never the MCP gateway, Google, finance data, or any production
// secret. External systems are behind interfaces so the flow is testable, and the sandbox is always
// destroyed.
package orchestrator

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/scottylabs/scottylabs-agent/services/engineering-agent/internal/authz"
	"github.com/scottylabs/scottylabs-agent/services/engineering-agent/internal/egress"
)

// GitHub is the least-privilege GitHub App surface the orchestrator needs (design 12.3): mint a
// short-lived per-repo token and open a DRAFT PR. There is deliberately no merge method — the app
// can only ever propose.
type GitHub interface {
	MintInstallToken(ctx context.Context, repo string) (token string, err error)
	OpenDraftPR(ctx context.Context, repo, branch, title, body string) (url string, err error)
}

// Spec configures a sandbox: a fast-start template, the egress allowlist, a time ceiling, and a
// minimal env. The env carries ONLY the scoped GitHub token and the task — never a production
// secret.
type Spec struct {
	Template        string
	EgressAllowlist []string
	TimeLimit       time.Duration
	Env             map[string]string
}

// Sandbox is a throwaway, allowlisted-egress execution environment (design 12.4).
type Sandbox interface {
	Create(ctx context.Context, spec Spec) (id string, err error)
	Exec(ctx context.Context, id, script string) (output string, err error)
	Destroy(ctx context.Context, id string) error
}

// Notifier posts the result back to Slack.
type Notifier interface {
	Post(ctx context.Context, channel, text string) error
}

// Task is one coding request from /fix-bug.
type Task struct {
	Repo      string // owner/repo
	Issue     string // issue number or short id
	IssueText string // the (untrusted) issue body — treated as data, not instructions
	Channel   string // Slack channel to report back to
	Requester string // who triggered it
}

// Result is a completed task.
type Result struct {
	PRURL     string
	Branch    string
	Summary   string
	SandboxID string
}

// Orchestrator runs coding tasks.
type Orchestrator struct {
	gh        GitHub
	sb        Sandbox
	notify    Notifier
	allow     *authz.Allowlist
	template  string
	timeLimit time.Duration
}

// New builds an Orchestrator.
func New(gh GitHub, sb Sandbox, notify Notifier, allow *authz.Allowlist, template string, timeLimit time.Duration) *Orchestrator {
	if timeLimit <= 0 {
		timeLimit = 20 * time.Minute
	}
	return &Orchestrator{gh: gh, sb: sb, notify: notify, allow: allow, template: template, timeLimit: timeLimit}
}

var branchSanitize = regexp.MustCompile(`[^a-z0-9._-]+`)

// ErrUnauthorized is returned when the requester or repo is not allowlisted.
var ErrUnauthorized = fmt.Errorf("engineering-agent: requester or repo not allowlisted")

// Run executes the task: authorize, mint a scoped token, create a sandbox, run the coding agent,
// push a branch, open a DRAFT PR, report to Slack, and ALWAYS destroy the sandbox. The sandbox holds
// no production credentials and only the allowlisted egress.
func (o *Orchestrator) Run(ctx context.Context, task Task) (Result, error) {
	if !o.allow.CanTrigger(task.Requester, task.Repo) {
		return Result{}, ErrUnauthorized
	}

	token, err := o.gh.MintInstallToken(ctx, task.Repo)
	if err != nil {
		return Result{}, fmt.Errorf("engineering-agent: mint token: %w", err)
	}

	branch := "agent/fix-" + branchSanitize.ReplaceAllString(strings.ToLower(task.Issue), "-")
	spec := Spec{
		Template:        o.template,
		EgressAllowlist: egress.AllowedHosts(),
		TimeLimit:       o.timeLimit,
		// ONLY the scoped token + task data. No gateway, Google, finance, or production secret.
		Env: map[string]string{
			"GITHUB_TOKEN": token,
			"REPO":         task.Repo,
			"BRANCH":       branch,
			"ISSUE":        task.Issue,
		},
	}

	id, err := o.sb.Create(ctx, spec)
	if err != nil {
		return Result{}, fmt.Errorf("engineering-agent: create sandbox: %w", err)
	}
	// The sandbox is ALWAYS destroyed, even if a later step fails.
	defer func() { _ = o.sb.Destroy(context.WithoutCancel(ctx), id) }()

	// Inside the sandbox: clone, run the headless coding agent (reproduce → fix → test → push a
	// branch). The issue text is data, not instructions; the agent cannot reach any production
	// system from here even if manipulated. The test summary is the script's output.
	summary, err := o.sb.Exec(ctx, id, codingScript)
	if err != nil {
		o.report(ctx, task.Channel, fmt.Sprintf(":x: Coding task for %s#%s failed in the sandbox.\n%s", task.Repo, task.Issue, trunc(err.Error(), 400)))
		return Result{SandboxID: id}, fmt.Errorf("engineering-agent: sandbox run: %w", err)
	}

	title := fmt.Sprintf("agent: fix %s (#%s)", task.Issue, task.Issue)
	body := fmt.Sprintf("Automated draft PR from the ScottyLabs Engineering Agent for issue #%s.\n\n_Requested by %s. Tests:_\n```\n%s\n```\n\nThis is a draft for human review — the app cannot merge.", task.Issue, task.Requester, trunc(summary, 2000))
	prURL, err := o.gh.OpenDraftPR(ctx, task.Repo, branch, title, body)
	if err != nil {
		return Result{SandboxID: id, Branch: branch, Summary: summary}, fmt.Errorf("engineering-agent: open draft PR: %w", err)
	}

	o.report(ctx, task.Channel, fmt.Sprintf(":sparkles: Draft PR ready for %s#%s: %s\nTests:\n```\n%s\n```", task.Repo, task.Issue, prURL, trunc(summary, 1200)))
	return Result{PRURL: prURL, Branch: branch, Summary: summary, SandboxID: id}, nil
}

func (o *Orchestrator) report(ctx context.Context, channel, text string) {
	if o.notify != nil && channel != "" {
		_ = o.notify.Post(ctx, channel, text)
	}
}

const codingScript = `set -euo pipefail
git clone --depth 50 "https://x-access-token:${GITHUB_TOKEN}@github.com/${REPO}.git" /work
cd /work && git checkout -b "${BRANCH}"
# Run the headless coding agent (Goose/Claude Code) on the issue: reproduce, fix, add tests.
run-coding-agent --issue "${ISSUE}" --repo "${REPO}"
# Run the project's tests and capture a summary.
make test 2>&1 | tail -40 || go test ./... 2>&1 | tail -40 || npm test 2>&1 | tail -40
git push origin "${BRANCH}"`

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
