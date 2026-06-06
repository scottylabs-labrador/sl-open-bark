// Command gateway is the ScottyLabs MCP gateway policy layer. It fronts the adopted MCP gateway
// (ContextForge), enforcing committee-role scopes, HITL gating on impact:high, and audit, backed
// by the platform's Postgres store (WP-01).
//
// Usage:
//
//	gateway          serve the policy HTTP API (/healthz, /tools, /call)
//	gateway sync     register all mcp-servers/**/manifest.yaml into the registry (lands proposed)
//
// Config from the environment (no secrets in code): DATABASE_URL, GATEWAY_SERVICE_TOKEN,
// MCP_DOWNSTREAM_TOKEN, CONTEXTFORGE_URL, REPO_ROOT, PORT.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/scottylabs/scottylabs-agent/gateway/internal/config"
	"github.com/scottylabs/scottylabs-agent/gateway/internal/limits"
	"github.com/scottylabs/scottylabs-agent/gateway/internal/manifest"
	"github.com/scottylabs/scottylabs-agent/gateway/internal/policy"
	"github.com/scottylabs/scottylabs-agent/gateway/internal/proxy"
	"github.com/scottylabs/scottylabs-agent/gateway/internal/server"
	"github.com/scottylabs/scottylabs-agent/services/session-memory/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()

	if cfg.DatabaseURL == "" {
		logger.Error("gateway: DATABASE_URL is required")
		os.Exit(1)
	}

	ctx := context.Background()
	db, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("gateway: open database", "err", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()
	if err := store.Migrate(db); err != nil {
		logger.Error("gateway: migrate", "err", err)
		os.Exit(1)
	}

	repo := store.New(db)
	limiter := limits.NewLimiter(cfg.RateCommitteePerMin, cfg.RateCommitteePerMin, cfg.RateGlobalPerMin, cfg.RateGlobalPerMin)
	gw := policy.New(repo, proxy.NewHTTPCaller(cfg.DownstreamToken), policy.WithLimiter(limiter))
	logger.Info("gateway: rate limits", "committee_per_min", cfg.RateCommitteePerMin, "global_per_min", cfg.RateGlobalPerMin)

	if len(os.Args) > 1 && os.Args[1] == "sync" {
		if err := syncRegistry(ctx, gw, cfg.RepoRoot, logger); err != nil {
			logger.Error("gateway: sync", "err", err)
			os.Exit(1)
		}
		return
	}

	if cfg.ServiceToken == "" {
		logger.Warn("gateway: GATEWAY_SERVICE_TOKEN is empty; the policy API will reject all callers")
	}
	srv := server.New(gw, cfg.ServiceToken, server.HeaderIdentity)

	addr := ":" + cfg.Port
	logger.Info("gateway: serving policy API", "addr", addr)
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		logger.Error("gateway: server exited", "err", err)
		os.Exit(1)
	}
}

// syncRegistry loads every manifest in the monorepo and registers it (lands 'proposed'; a
// maintainer promotes to 'approved' after a live check).
func syncRegistry(ctx context.Context, gw *policy.Gateway, root string, logger *slog.Logger) error {
	servers, err := manifest.LoadDir(root)
	if err != nil {
		return err
	}
	for _, in := range servers {
		s, err := gw.Register(ctx, in)
		if err != nil {
			return err
		}
		logger.Info("registered", "server", s.Name, "tools", len(s.Tools), "lifecycle", s.Lifecycle)
	}
	logger.Info("sync complete", "servers", len(servers))
	return nil
}
