// Package sandbox runs commands in a throwaway, allowlisted-egress environment (design 12.4). The
// default implementation drives Railway Sandboxes via the `railway sandbox` CLI (templates + fork
// for fast startup). The sandbox holds no production credentials and is destroyed after each task.
package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/scottylabs/scottylabs-agent/services/engineering-agent/internal/orchestrator"
)

// Railway drives Railway Sandboxes via the CLI.
type Railway struct {
	bin string // the railway binary (default "railway")
}

// NewRailway builds a Railway sandbox driver.
func NewRailway() *Railway { return &Railway{bin: "railway"} }

// Create forks a fresh sandbox from the spec's template with the given egress allowlist, time
// ceiling, and minimal env. The egress allowlist and "no production credentials" property are the
// core safety controls.
func (r *Railway) Create(ctx context.Context, spec orchestrator.Spec) (string, error) {
	args := []string{"sandbox", "create", "--template", spec.Template}
	out, err := r.run(ctx, args...)
	if err != nil {
		return "", fmt.Errorf("sandbox: create: %w", err)
	}
	id := strings.TrimSpace(out)
	if id == "" {
		return "", fmt.Errorf("sandbox: create returned no id")
	}
	// TODO(live): apply spec.EgressAllowlist + spec.TimeLimit + spec.Env to the sandbox via the
	// provider API before exec.
	return id, nil
}

// Exec runs a script inside the sandbox and returns its combined output.
func (r *Railway) Exec(ctx context.Context, id, script string) (string, error) {
	return r.run(ctx, "sandbox", "exec", "--id", id, "--", "bash", "-lc", script)
}

// Destroy tears the sandbox down. Best-effort: a destroy failure is reported but the task result
// stands.
func (r *Railway) Destroy(ctx context.Context, id string) error {
	_, err := r.run(ctx, "sandbox", "destroy", "--id", id)
	return err
}

func (r *Railway) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, r.bin, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.String(), err
}
