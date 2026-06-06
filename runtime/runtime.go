package runtime

import (
	"context"
	"errors"
)

// Runtime is the default AgentRuntime: it resolves a task to a RunSpec (recipe + model), runs it on
// an Engine, and coordinates the human-approval pause/resume. It has no gateway dependency — the
// Engine surfaces the gateway's approval id, and the Runtime just pauses until ResolveApproval.
type Runtime struct {
	engine     Engine
	models     ModelConfig
	recipesDir string
}

// Option configures a Runtime.
type Option func(*Runtime)

// WithModels sets the model strategy.
func WithModels(m ModelConfig) Option { return func(r *Runtime) { r.models = m } }

// WithRecipesDir sets where recipes are loaded from.
func WithRecipesDir(dir string) Option { return func(r *Runtime) { r.recipesDir = dir } }

// New builds a Runtime over an Engine.
func New(engine Engine, opts ...Option) *Runtime {
	r := &Runtime{engine: engine, recipesDir: "recipes"}
	for _, o := range opts {
		o(r)
	}
	return r
}

// SubmitTask resolves the task and starts it, returning a Task handle whose Events stream the run.
func (r *Runtime) SubmitTask(ctx context.Context, req TaskRequest) (*Task, error) {
	spec, err := r.buildSpec(req)
	if err != nil {
		return nil, err
	}
	t := &Task{
		events:    make(chan Event, 32),
		decisions: make(chan decision, 1),
		done:      make(chan struct{}),
	}
	go r.run(ctx, t, spec)
	return t, nil
}

// buildSpec resolves the recipe (or inline goal) and selects the model. A per-recipe model override
// wins; a hard task escalates; otherwise the default.
func (r *Runtime) buildSpec(req TaskRequest) (RunSpec, error) {
	spec := RunSpec{Identity: req.Identity, Committee: req.Committee, Params: req.Params}
	var recipeModel string
	switch {
	case req.RecipeID != "":
		rec, err := LoadRecipeByID(r.recipesDir, req.RecipeID)
		if err != nil {
			return RunSpec{}, err
		}
		spec.Goal = rec.Title
		spec.Instructions = rec.Instructions
		recipeModel = rec.Model
	case req.InlineGoal != "":
		spec.Goal = req.InlineGoal
	default:
		return RunSpec{}, errors.New("runtime: task needs a recipe_id or inline_goal")
	}
	spec.Model = r.models.Select(recipeModel, req.HardTask)
	return spec, nil
}

func (r *Runtime) run(ctx context.Context, t *Task, spec RunSpec) {
	defer close(t.done)
	defer close(t.events)

	hooks := Hooks{
		Emit: func(e Event) {
			select {
			case t.events <- e:
			case <-ctx.Done():
			}
		},
		Approve: func(ctx context.Context, tool, approvalID string) (bool, error) {
			select {
			case t.events <- Event{Kind: KindApprovalRequired, Tool: tool, ApprovalID: approvalID}:
			case <-ctx.Done():
				return false, ctx.Err()
			}
			for {
				select {
				case d := <-t.decisions:
					if d.approvalID != "" && d.approvalID != approvalID {
						continue // a decision for a different approval; keep waiting
					}
					return d.granted, nil
				case <-ctx.Done():
					return false, ctx.Err()
				}
			}
		},
	}

	res, err := r.engine.Run(ctx, spec, hooks)
	t.mu.Lock()
	t.result, t.err = res, err
	t.mu.Unlock()

	if err != nil {
		t.events <- Event{Kind: KindError, Text: err.Error()}
	} else {
		t.events <- Event{Kind: KindOutput, Text: res.Output}
	}
	t.events <- Event{Kind: KindDone}
}
