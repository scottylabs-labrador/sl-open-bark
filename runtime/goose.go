package runtime

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// GooseEngine runs the agent loop with headless Goose, configured for OpenRouter via the
// environment (design 5, 5.5). It is the side-effecting edge: it requires the `goose` binary and an
// OpenRouter key (both provided via the environment), so it is exercised live in deployment, not in
// CI. The model/provider/gateway wiring it builds (gooseEnv) is unit-tested.
type GooseEngine struct {
	cfg        Config
	binaryName string
}

// NewGooseEngine builds the engine from config.
func NewGooseEngine(cfg Config) *GooseEngine {
	return &GooseEngine{cfg: cfg, binaryName: "goose"}
}

// gooseEnv builds the environment Goose runs under. Switching GOOSE_MODEL (or a per-recipe model,
// carried in spec.Model) changes the model with no code change — that selection happened upstream
// in ModelConfig.Select; here it is passed straight through to Goose.
func (g *GooseEngine) gooseEnv(spec RunSpec) []string {
	env := []string{
		"GOOSE_PROVIDER=" + g.cfg.Provider,
		"GOOSE_MODEL=" + spec.Model,
	}
	if g.cfg.OpenRouterAPIKey != "" {
		env = append(env, "OPENROUTER_API_KEY="+g.cfg.OpenRouterAPIKey)
	}
	// The gateway is registered as an MCP extension (Streamable HTTP); Goose calls all tools through
	// it. The extension config is read from these by the generated Goose config.
	if g.cfg.GatewayURL != "" {
		env = append(env, "SCOTTYLABS_GATEWAY_URL="+g.cfg.GatewayURL)
	}
	if g.cfg.GatewayToken != "" {
		env = append(env, "SCOTTYLABS_GATEWAY_TOKEN="+g.cfg.GatewayToken)
	}
	return env
}

// Run executes the task with Goose. The exact CLI/extension wiring is finalized against the
// installed goose version (the binary is provisioned via the environment); approval interception
// (the gateway returns approval_required for an impact:high tool, which is surfaced via
// hooks.Approve) is the documented integration point completed end to end in WP-08.
func (g *GooseEngine) Run(ctx context.Context, spec RunSpec, hooks Hooks) (Result, error) {
	if g.cfg.OpenRouterAPIKey == "" {
		return Result{}, errors.New("runtime: GooseEngine needs OPENROUTER_API_KEY (set via the environment; see runtime/README.md)")
	}

	prompt := spec.Goal
	if spec.Instructions != "" {
		prompt = spec.Instructions
	}

	cmd := exec.CommandContext(ctx, g.binaryName, "run", "--text", prompt)
	cmd.Env = append([]string{}, g.gooseEnv(spec)...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Result{}, fmt.Errorf("runtime: goose stdout: %w", err)
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return Result{}, fmt.Errorf("runtime: start goose (is the binary installed?): %w", err)
	}

	var last strings.Builder
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		hooks.Emit(Event{Kind: KindStatus, Text: line})
		last.WriteString(line)
		last.WriteString("\n")
	}
	if err := cmd.Wait(); err != nil {
		return Result{}, fmt.Errorf("runtime: goose run failed: %w", err)
	}
	return Result{Output: strings.TrimSpace(last.String())}, nil
}
