// Command memory-mcp is the Memory MCP server: it exposes the platform's scoped long-term memory
// (write_fact, search, forget) over MCP, backed by Postgres (WP-01), behind the gateway. Config
// comes from the environment (DATABASE_URL required); no secrets in code.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/scottylabs/scottylabs-agent/services/session-memory/internal/memory"
	"github.com/scottylabs/scottylabs-agent/services/session-memory/store"
)

func main() {
	setupLogging(os.Getenv("MCP_LOG_LEVEL"))

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		slog.Error("memory-mcp: DATABASE_URL is required")
		os.Exit(1)
	}

	ctx := context.Background()
	db, err := store.Open(ctx, dsn)
	if err != nil {
		slog.Error("memory-mcp: open database", "err", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()
	if err := store.Migrate(db); err != nil {
		slog.Error("memory-mcp: migrate", "err", err)
		os.Exit(1)
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "memory", Version: "0.1.0"}, nil)
	memory.Register(server, memory.NewHandlers(store.New(db)))
	mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)

	token := os.Getenv("MCP_AUTH_TOKEN")
	mux := http.NewServeMux()
	mux.Handle("/mcp", withAuth(token, mcpHandler))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	slog.Info("memory mcp serving", "addr", addr)
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
