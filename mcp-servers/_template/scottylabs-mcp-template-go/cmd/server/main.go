// Command server is the composition root: it reads config, builds the clients, constructs the
// service, registers the tools, and serves MCP over Streamable HTTP (a single /mcp endpoint) so
// it runs behind Railway and the gateway.
package main

import (
	"log/slog"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/scottylabs/scottylabs-mcp-example/internal/clients"
	"github.com/scottylabs/scottylabs-mcp-example/internal/config"
	"github.com/scottylabs/scottylabs-mcp-example/internal/domain"
	"github.com/scottylabs/scottylabs-mcp-example/internal/logging"
	"github.com/scottylabs/scottylabs-mcp-example/internal/service"
	"github.com/scottylabs/scottylabs-mcp-example/internal/tools"
)

func main() {
	cfg := config.Load()
	logging.Setup(cfg.LogLevel)

	// Choose the concrete audit client here (the only place that decides). The service depends
	// only on the service.AuditSink interface.
	var audit service.AuditSink
	if cfg.AuditURL != "" {
		audit = clients.NewHTTPAudit(cfg.AuditURL)
	} else {
		audit = clients.NewInMemoryAudit()
	}

	svc := service.New(domain.DefaultStandards(), audit)

	server := mcp.NewServer(&mcp.Implementation{Name: "scottylabs.example", Version: "0.1.0"}, nil)
	tools.Register(server, svc)

	mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)

	mux := http.NewServeMux()
	mux.Handle("/mcp", withAuth(cfg.AuthToken, mcpHandler))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	addr := ":" + cfg.Port
	slog.Info("starting mcp server", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("server exited", "err", err)
	}
}

// withAuth enforces a bearer token only when one is configured. The platform gateway is the real
// auth boundary; this is defense in depth for any direct call.
func withAuth(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token != "" && r.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
