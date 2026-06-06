# runtime/

The **AgentRuntime** boundary and the Goose-backed implementation (design Sections 4.2, 5, 5.5).
This is the deliberately thin seam between every surface (Slack Gateway, Scheduler) and the agent
loop, so the platform is not hostage to one runtime's roadmap.

**Owner:** platform-team · **Built in:** WP-06.

## The contract

```go
type AgentRuntime interface {
    SubmitTask(ctx, TaskRequest) (*Task, error)
}
```

`SubmitTask` returns a `Task` whose `Events()` stream the run (`tool_call`, `approval_required`,
`output`, `done`). A high-impact tool surfaces an **`approval_required`** event (carrying the
gateway's approval id) and **pauses** until `task.ResolveApproval(id, granted, by)` is called;
`task.Result()` blocks for the final output + audit ref. Callers (Slack, Scheduler) depend only on
this interface.

## Design

```
agentruntime.go  the interface + Task handle (events, ResolveApproval, Result)
engine.go        the Engine interface + Hooks (Emit, Approve) — the agent loop is pluggable
runtime.go       the coordination impl: recipe -> model -> Engine, + approval pause/resume
model.go         model strategy: per-recipe override > escalate-hard > default (design 5.5)
recipe.go        load recipes/<committee>/<name>.yaml
goose.go         GooseEngine — the side-effecting edge (Goose + OpenRouter via env)
config.go        typed env config
.goosehints      static org guidance loaded every turn
cmd/runtime/     a thin CLI to run one task
```

The coordination, model selection, recipe loading, and **approval pause/resume** are pure and fully
unit-tested with a **fake Engine** — no Goose, no model, no network in CI. The `GooseEngine` is the
env-configured edge (it execs `goose` against OpenRouter and registers the gateway as an MCP
extension); its model/provider/gateway wiring is unit-tested, and it is exercised live in deployment
(WP-08).

## Model access (design 5.5)

```bash
GOOSE_PROVIDER=openrouter
GOOSE_MODEL=anthropic/claude-sonnet           # default; switch with no code change
GOOSE_ESCALATION_MODEL=anthropic/claude-opus  # hard tasks
OPENROUTER_API_KEY=sk-or-...                  # Railway secret only
```

A recipe may declare its own `model:` to pin a workflow's model. **Human-gated:** the OpenRouter key
and the `goose` binary are provisioned via the environment; no key is in code.

## Run

```bash
go test ./...                                 # fake-engine tests, no network
GOOSE_MODEL=anthropic/claude-opus go run ./cmd/runtime --goal "screen reimbursements"
go run ./cmd/runtime finance/screen-reimbursement --committee finance
```
