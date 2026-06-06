package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestChecklistCatchesProblems(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "mcp-servers/bad/manifest.yaml"), `name: bad.server
tools:
  - name: send_email
    scope: bad.write
    impact: write
  - name: read_thing
    impact: read
`)
	// A committed secret (assembled at runtime so this source file holds no literal secret).
	write(t, filepath.Join(dir, "leaked.conf"), "OPENROUTER_API_KEY=sk-or-v1-"+strings.Repeat("a", 52))

	problems, err := RunChecklist(dir)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(problems, "\n")
	if !strings.Contains(joined, "send_email") || !strings.Contains(joined, "must be high") {
		t.Fatalf("should flag the irreversible write tool: %v", problems)
	}
	if !strings.Contains(joined, "no scope") {
		t.Fatalf("should flag the scopeless tool: %v", problems)
	}
	if !strings.Contains(joined, "COMMITTED SECRET") {
		t.Fatalf("should flag the committed secret: %v", problems)
	}
}

func TestChecklistCleanPasses(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "mcp-servers/good/manifest.yaml"), `name: good.server
tools:
  - name: gmail_send
    scope: google.send
    impact: high
  - name: read_x
    scope: google.read
    impact: read
`)
	write(t, filepath.Join(dir, ".env.example"), "OPENROUTER_API_KEY=sk-or-v1-"+strings.Repeat("b", 52)) // .example is skipped

	problems, err := RunChecklist(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Fatalf("a clean tree should pass, got: %v", problems)
	}
}
