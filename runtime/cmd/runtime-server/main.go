// Command runtime-server runs the AgentRuntime as a long-running HTTP service (the API the Slack
// Gateway and Scheduler call). Config comes from the environment (design 5.5): GOOSE_PROVIDER,
// GOOSE_MODEL, OPENROUTER_API_KEY, the gateway URL/token, and RUNTIME_SERVICE_TOKEN. No secrets in
// code.
package main

import (
	"log/slog"
	"net/http"
	"os"

	runtime "github.com/scottylabs/scottylabs-agent/runtime"
	"github.com/scottylabs/scottylabs-agent/runtime/internal/server"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := runtime.LoadConfig()

	if cfg.OpenRouterAPIKey == "" {
		logger.Warn("runtime: OPENROUTER_API_KEY is empty; the service is healthy but tasks cannot call the model until it is set")
	}

	engine := runtime.NewGooseEngine(cfg)
	rt := runtime.New(engine, runtime.WithModels(cfg.Models()), runtime.WithRecipesDir(cfg.RecipesDir))

	token := os.Getenv("RUNTIME_SERVICE_TOKEN")
	if token == "" {
		logger.Warn("runtime: RUNTIME_SERVICE_TOKEN is empty; the task API will reject all callers")
	}
	srv := server.New(rt, token)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	logger.Info("runtime service serving", "addr", addr, "model", cfg.Model)
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		logger.Error("runtime: server exited", "err", err)
		os.Exit(1)
	}
}
