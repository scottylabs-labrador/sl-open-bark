package runtime_test

import (
	"context"
	"errors"
	"testing"

	rt "github.com/scottylabs/scottylabs-agent/runtime"
)

// fakeEngine simulates the agent loop: it records the spec it was given, emits a tool call, and —
// when highImpact — pauses for approval before producing output. No real Goose or model.
type fakeEngine struct {
	gotSpec     rt.RunSpec
	highImpact  bool
	approveTool string
	approvalID  string
}

func (f *fakeEngine) Run(ctx context.Context, spec rt.RunSpec, hooks rt.Hooks) (rt.Result, error) {
	f.gotSpec = spec
	hooks.Emit(rt.Event{Kind: rt.KindToolCall, Tool: "finance.rules/evaluate"})
	if f.highImpact {
		granted, err := hooks.Approve(ctx, f.approveTool, f.approvalID)
		if err != nil {
			return rt.Result{}, err
		}
		if !granted {
			return rt.Result{}, errors.New("denied by human")
		}
	}
	return rt.Result{Output: "done: " + spec.Goal, AuditRef: "audit-1"}, nil
}

func drain(t *rt.Task) []rt.Event {
	var out []rt.Event
	for e := range t.Events() {
		out = append(out, e)
	}
	return out
}

func kinds(events []rt.Event) []rt.EventKind {
	var ks []rt.EventKind
	for _, e := range events {
		ks = append(ks, e.Kind)
	}
	return ks
}

func contains(ks []rt.EventKind, want rt.EventKind) bool {
	for _, k := range ks {
		if k == want {
			return true
		}
	}
	return false
}

// A trivial recipe runs end to end against a (stub) tool and produces output.
func TestSubmitTaskRunsRecipe(t *testing.T) {
	fe := &fakeEngine{}
	r := rt.New(fe, rt.WithModels(rt.ModelConfig{Default: "anthropic/claude-sonnet"}), rt.WithRecipesDir("testdata"))

	task, err := r.SubmitTask(context.Background(), rt.TaskRequest{
		RecipeID: "screen-reimbursement", Identity: "alice@scottylabs.org", Committee: "finance",
	})
	if err != nil {
		t.Fatal(err)
	}
	events := drain(task)
	res, err := task.Result()
	if err != nil {
		t.Fatal(err)
	}
	if !contains(kinds(events), rt.KindToolCall) || !contains(kinds(events), rt.KindOutput) || !contains(kinds(events), rt.KindDone) {
		t.Fatalf("expected tool_call, output, done; got %v", kinds(events))
	}
	if res.Output != "done: Screen reimbursement requests" {
		t.Fatalf("unexpected output: %q", res.Output)
	}
	if fe.gotSpec.Model != "anthropic/claude-sonnet" {
		t.Fatalf("default model not applied: %q", fe.gotSpec.Model)
	}
	if fe.gotSpec.Instructions == "" {
		t.Fatal("recipe instructions were not loaded")
	}
}

// A high-impact tool surfaces approval_required and only proceeds after the human grants it.
func TestHighImpactPausesForApproval(t *testing.T) {
	fe := &fakeEngine{highImpact: true, approveTool: "google.workspace/gmail_send", approvalID: "appr-1"}
	r := rt.New(fe, rt.WithRecipesDir("testdata"))
	task, err := r.SubmitTask(context.Background(), rt.TaskRequest{InlineGoal: "send the drafted returns"})
	if err != nil {
		t.Fatal(err)
	}

	sawApproval := false
	outputBeforeApproval := false
	for e := range task.Events() {
		switch e.Kind {
		case rt.KindApprovalRequired:
			sawApproval = true
			if e.Tool != "google.workspace/gmail_send" || e.ApprovalID != "appr-1" {
				t.Fatalf("approval event missing tool/id: %+v", e)
			}
			if err := task.ResolveApproval(e.ApprovalID, true, "lead@scottylabs.org"); err != nil {
				t.Fatal(err)
			}
		case rt.KindOutput:
			if !sawApproval {
				outputBeforeApproval = true
			}
		}
	}
	res, err := task.Result()
	if err != nil {
		t.Fatalf("granted task should succeed: %v", err)
	}
	if !sawApproval {
		t.Fatal("high-impact tool did not request approval")
	}
	if outputBeforeApproval {
		t.Fatal("output was produced before the approval was granted")
	}
	if res.Output == "" {
		t.Fatal("expected output after approval")
	}
}

// Denying the approval aborts the task with an error and no output.
func TestHighImpactDenied(t *testing.T) {
	fe := &fakeEngine{highImpact: true, approveTool: "google.workspace/gmail_send", approvalID: "appr-2"}
	r := rt.New(fe, rt.WithRecipesDir("testdata"))
	task, _ := r.SubmitTask(context.Background(), rt.TaskRequest{InlineGoal: "send"})

	for e := range task.Events() {
		if e.Kind == rt.KindApprovalRequired {
			_ = task.ResolveApproval(e.ApprovalID, false, "lead@scottylabs.org")
		}
		if e.Kind == rt.KindOutput {
			t.Fatal("a denied task must not produce output")
		}
	}
	if _, err := task.Result(); err == nil {
		t.Fatal("denied task should error")
	}
}

// A per-recipe model override wins over the default.
func TestRecipeModelOverride(t *testing.T) {
	fe := &fakeEngine{}
	r := rt.New(fe, rt.WithModels(rt.ModelConfig{Default: "anthropic/claude-sonnet"}), rt.WithRecipesDir("testdata"))
	task, err := r.SubmitTask(context.Background(), rt.TaskRequest{RecipeID: "with-model"})
	if err != nil {
		t.Fatal(err)
	}
	drain(task)
	if fe.gotSpec.Model != "anthropic/claude-opus" {
		t.Fatalf("recipe model override not applied: %q", fe.gotSpec.Model)
	}
}

func TestSubmitTaskRequiresRecipeOrGoal(t *testing.T) {
	r := rt.New(&fakeEngine{})
	if _, err := r.SubmitTask(context.Background(), rt.TaskRequest{}); err == nil {
		t.Fatal("a task with neither recipe nor goal should error")
	}
}
