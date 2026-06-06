package runtime

import (
	"context"
	"strings"
	"testing"
)

func TestGooseEnvWiring(t *testing.T) {
	g := NewGooseEngine(Config{
		Provider: "openrouter", OpenRouterAPIKey: "sk-or-test",
		GatewayURL: "http://gateway.railway.internal:8080/mcp", GatewayToken: "tok",
	})
	env := strings.Join(g.gooseEnv(RunSpec{Model: "anthropic/claude-opus"}), "\n")
	for _, want := range []string{
		"GOOSE_PROVIDER=openrouter",
		"GOOSE_MODEL=anthropic/claude-opus",
		"OPENROUTER_API_KEY=sk-or-test",
		"SCOTTYLABS_GATEWAY_URL=http://gateway.railway.internal:8080/mcp",
	} {
		if !strings.Contains(env, want) {
			t.Fatalf("goose env missing %q:\n%s", want, env)
		}
	}
}

func TestGooseRunNeedsKey(t *testing.T) {
	g := NewGooseEngine(Config{Provider: "openrouter"})
	_, err := g.Run(context.Background(), RunSpec{Goal: "x"}, Hooks{Emit: func(Event) {}})
	if err == nil {
		t.Fatal("GooseEngine.Run without an OpenRouter key should error")
	}
}
