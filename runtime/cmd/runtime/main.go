// Command runtime drives a single task through the AgentRuntime (Goose backed by OpenRouter),
// streaming its events. It is a thin CLI for local testing and for the Scheduler; the Slack Gateway
// uses the AgentRuntime interface directly. Config comes from the environment (see runtime/README).
//
// Usage:
//
//	runtime <recipe-id> [--committee finance] [--identity me@scottylabs.org] [key=value ...]
//	runtime --goal "free-form goal" [--hard]
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	runtime "github.com/scottylabs/scottylabs-agent/runtime"
)

func main() {
	cfg := runtime.LoadConfig()
	engine := runtime.NewGooseEngine(cfg)
	rt := runtime.New(engine, runtime.WithModels(cfg.Models()), runtime.WithRecipesDir(cfg.RecipesDir))

	req, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}

	task, err := rt.SubmitTask(context.Background(), req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "submit:", err)
		os.Exit(1)
	}

	for e := range task.Events() {
		switch e.Kind {
		case runtime.KindApprovalRequired:
			// This CLI does not approve; approvals come from a human in Slack. Deny so the run ends
			// safely rather than hanging.
			fmt.Printf("[approval-required] %s (%s) — denying in CLI; approve via Slack in production\n", e.Tool, e.ApprovalID)
			_ = task.ResolveApproval(e.ApprovalID, false, "cli")
		default:
			fmt.Printf("[%s] %s\n", e.Kind, e.Text)
		}
	}
	res, err := task.Result()
	if err != nil {
		fmt.Fprintln(os.Stderr, "result:", err)
		os.Exit(1)
	}
	fmt.Println("\n=== output ===")
	fmt.Println(res.Output)
}

func parseArgs(args []string) (runtime.TaskRequest, error) {
	req := runtime.TaskRequest{Params: map[string]string{}, Identity: "cli", Committee: ""}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--goal":
			i++
			if i >= len(args) {
				return req, fmt.Errorf("--goal needs a value")
			}
			req.InlineGoal = args[i]
		case a == "--committee":
			i++
			req.Committee = args[i]
		case a == "--identity":
			i++
			req.Identity = args[i]
		case a == "--hard":
			req.HardTask = true
		case strings.Contains(a, "="):
			kv := strings.SplitN(a, "=", 2)
			req.Params[kv[0]] = kv[1]
		default:
			req.RecipeID = a
		}
	}
	if req.RecipeID == "" && req.InlineGoal == "" {
		return req, fmt.Errorf("provide a recipe id or --goal")
	}
	return req, nil
}
