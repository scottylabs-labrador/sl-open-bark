// Command server is the Finance Rules MCP's composition root: load the reviewed standards (embedded,
// or overridden by a file), construct the evaluator, register the tool, and serve MCP over
// Streamable HTTP behind Railway and the gateway.
package main

import (
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/scottylabs/scottylabs-agent/mcp-servers/finance-rules/internal/config"
	"github.com/scottylabs/scottylabs-agent/mcp-servers/finance-rules/internal/domain"
	"github.com/scottylabs/scottylabs-agent/mcp-servers/finance-rules/internal/service"
	"github.com/scottylabs/scottylabs-agent/mcp-servers/finance-rules/internal/standards"
	"github.com/scottylabs/scottylabs-agent/mcp-servers/finance-rules/internal/tools"
)

func main() {
	cfg := config.Load()
	setupLogging(cfg.LogLevel)

	raw := standards.Default()
	if cfg.StandardsFile != "" {
		b, err := os.ReadFile(cfg.StandardsFile)
		if err != nil {
			slog.Error("finance-rules: read FINANCE_STANDARDS_FILE", "err", err)
			os.Exit(1)
		}
		raw = b
	}
	std, err := domain.LoadStandards(raw)
	if err != nil {
		slog.Error("finance-rules: invalid standards", "err", err)
		os.Exit(1)
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "finance.rules", Version: "0.1.0"}, nil)
	tools.Register(server, service.New(std))
	mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)

	mux := http.NewServeMux()
	mux.Handle("/mcp", withAuth(cfg.AuthToken, mcpHandler))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	addr := ":" + cfg.Port
	slog.Info("finance-rules mcp serving", "addr", addr, "categories", len(std.EligibleCategories))
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
