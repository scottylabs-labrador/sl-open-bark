// Command server is the Google Workspace MCP's composition root: read config, build the Google
// client (domain-wide delegation), construct the service, register the tools, and serve MCP over
// Streamable HTTP behind Railway and the gateway.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/scottylabs/scottylabs-agent/mcp-servers/google-workspace/internal/clients"
	"github.com/scottylabs/scottylabs-agent/mcp-servers/google-workspace/internal/config"
	"github.com/scottylabs/scottylabs-agent/mcp-servers/google-workspace/internal/service"
	"github.com/scottylabs/scottylabs-agent/mcp-servers/google-workspace/internal/tools"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	setupLogging(cfg.LogLevel)

	if !cfg.HasGoogleCredentials() {
		slog.Error("google-workspace: GOOGLE_SA_JSON(_FILE) and GOOGLE_DELEGATED_SUBJECT are required")
		os.Exit(1)
	}

	ctx := context.Background()
	google, err := clients.NewGoogle(ctx, cfg.GoogleSAJSON, cfg.DelegatedSubject)
	if err != nil {
		slog.Error("google-workspace: build client", "err", err)
		os.Exit(1)
	}
	svc := service.New(google)

	server := mcp.NewServer(&mcp.Implementation{Name: "google.workspace", Version: "0.1.0"}, nil)
	tools.Register(server, svc)
	mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)

	mux := http.NewServeMux()
	mux.Handle("/mcp", withAuth(cfg.AuthToken, mcpHandler))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	addr := ":" + cfg.Port
	slog.Info("google-workspace mcp serving", "addr", addr, "subject", cfg.DelegatedSubject)
	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("server exited", "err", err)
		os.Exit(1)
	}
}

func withAuth(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token != "" && r.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func setupLogging(level string) {
	var l slog.Level
	switch strings.ToLower(level) {
	case "debug":
		l = slog.LevelDebug
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: l})))
}
