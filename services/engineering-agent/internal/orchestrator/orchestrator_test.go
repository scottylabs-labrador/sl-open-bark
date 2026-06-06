package orchestrator_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/scottylabs/scottylabs-agent/services/engineering-agent/internal/authz"
	"github.com/scottylabs/scottylabs-agent/services/engineering-agent/internal/orchestrator"
)

type fakeGitHub struct {
	minted, opened int
	failPR         bool
}

func (f *fakeGitHub) MintInstallToken(context.Context, string) (string, error) {
	f.minted++
	return "ghs_scoped_token", nil
}
func (f *fakeGitHub) OpenDraftPR(_ context.Context, repo, _, _, _ string) (string, error) {
	f.opened++
	if f.failPR {
		return "", errors.New("pr error")
	}
	return "https://github.com/" + repo + "/pull/1", nil
}

type fakeSandbox struct {
	created, destroyed int
	lastSpec           orchestrator.Spec
	failExec           bool
}

func (f *fakeSandbox) Create(_ context.Context, spec orchestrator.Spec) (string, error) {
	f.created++
	f.lastSpec = spec
	return "sb-1", nil
}
func (f *fakeSandbox) Exec(context.Context, string, string) (string, error) {
	if f.failExec {
		return "", errors.New("build failed in sandbox")
	}
	return "ok: 12 tests passed", nil
}
func (f *fakeSandbox) Destroy(context.Context, string) error { f.destroyed++; return nil }

type fakeNotifier struct{ posts []string }

func (f *fakeNotifier) Post(_ context.Context, _, text string) error {
	f.posts = append(f.posts, text)
	return nil
}

func newOrch(t *testing.T) (*orchestrator.Orchestrator, *fakeGitHub, *fakeSandbox, *fakeNotifier) {
	t.Helper()
	allow := authz.New([]string{"alice"}, []string{"scottylabs-labrador/site"})
	gh, sb, n := &fakeGitHub{}, &fakeSandbox{}, &fakeNotifier{}
	return orchestrator.New(gh, sb, n, allow, "dev-template", time.Minute), gh, sb, n
}

func authorizedTask() orchestrator.Task {
	return orchestrator.Task{Repo: "scottylabs-labrador/site", Issue: "123", IssueText: "tests fail on null input", Channel: "C1", Requester: "alice"}
}

func TestRunHappyPath(t *testing.T) {
	o, gh, sb, n := newOrch(t)
	res, err := o.Run(context.Background(), authorizedTask())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.PRURL, "/pull/1") {
		t.Fatalf("expected a draft PR url, got %q", res.PRURL)
	}
	if sb.created != 1 || sb.destroyed != 1 {
		t.Fatalf("sandbox should be created once and destroyed once: created=%d destroyed=%d", sb.created, sb.destroyed)
	}
	if gh.minted != 1 || gh.opened != 1 {
		t.Fatalf("expected one token mint and one draft PR: minted=%d opened=%d", gh.minted, gh.opened)
	}
	if len(n.posts) != 1 || !strings.Contains(n.posts[0], "/pull/1") {
		t.Fatalf("expected one Slack post with the PR link: %v", n.posts)
	}

	// Isolation: the sandbox env carries ONLY the scoped token + task — no production secret.
	allowedKeys := map[string]bool{"GITHUB_TOKEN": true, "REPO": true, "BRANCH": true, "ISSUE": true}
	for k := range sb.lastSpec.Env {
		if !allowedKeys[k] {
			t.Fatalf("sandbox env leaked a non-allowlisted key %q (possible production secret)", k)
		}
	}
	if len(sb.lastSpec.EgressAllowlist) == 0 {
		t.Fatal("the sandbox must be given an egress allowlist")
	}
	hasGitHub := false
	for _, h := range sb.lastSpec.EgressAllowlist {
		if h == "github.com" {
			hasGitHub = true
		}
	}
	if !hasGitHub {
		t.Fatal("egress allowlist should include github.com")
	}
}

func TestUnauthorizedNeverCreatesSandbox(t *testing.T) {
	o, gh, sb, _ := newOrch(t)
	if _, err := o.Run(context.Background(), orchestrator.Task{Repo: "scottylabs-labrador/site", Issue: "1", Requester: "eve"}); !errors.Is(err, orchestrator.ErrUnauthorized) {
		t.Fatalf("non-maintainer should be ErrUnauthorized, got %v", err)
	}
	if sb.created != 0 || gh.minted != 0 {
		t.Fatal("an unauthorized request must not mint a token or create a sandbox")
	}
	// A repo that is not opted in is also denied.
	if _, err := o.Run(context.Background(), orchestrator.Task{Repo: "scottylabs-labrador/secret", Issue: "1", Requester: "alice"}); !errors.Is(err, orchestrator.ErrUnauthorized) {
		t.Fatalf("non-opt-in repo should be denied, got %v", err)
	}
}

func TestSandboxDestroyedOnFailure(t *testing.T) {
	o, gh, sb, _ := newOrch(t)
	sb.failExec = true
	if _, err := o.Run(context.Background(), authorizedTask()); err == nil {
		t.Fatal("a sandbox failure should surface an error")
	}
	if sb.created != 1 || sb.destroyed != 1 {
		t.Fatalf("the sandbox must be destroyed even when the run fails: created=%d destroyed=%d", sb.created, sb.destroyed)
	}
	if gh.opened != 0 {
		t.Fatal("no PR should be opened when the sandbox run fails")
	}
}
