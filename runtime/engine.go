package runtime

import "context"

// Engine runs the actual agent loop for one task. Goose is the default implementation (goose.go);
// tests inject a fake. The Engine reasons with the model, calls tools through the gateway, and uses
// the Hooks to stream progress and to pause for human approval on a high-impact action.
type Engine interface {
	Run(ctx context.Context, spec RunSpec, hooks Hooks) (Result, error)
}

// RunSpec is the resolved plan for a task: the goal/instructions, the chosen model, and the caller
// scope. Gateway connection details live in the Engine implementation (from the environment).
type RunSpec struct {
	Goal         string
	Instructions string
	Model        string
	Identity     string
	Committee    string
	Params       map[string]string
}

// Hooks let the Engine communicate with the runtime without knowing about channels or approvals.
type Hooks struct {
	// Emit streams a progress event to the caller.
	Emit func(Event)

	// Approve is called when the Engine hits a high-impact tool that the gateway gated: the gateway
	// returned an approval id, which the Engine passes here. Approve blocks until a human decides
	// and returns whether to proceed. The recording of the decision in the gateway is the caller's
	// responsibility (it happens before ResolveApproval is delivered).
	Approve func(ctx context.Context, tool, approvalID string) (granted bool, err error)
}
