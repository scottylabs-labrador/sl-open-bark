// Command dashboard serves the maintainer dashboard + agent console (design 6.3, 10.2, 11.4): read
// views over the platform's Postgres state plus a console that drives the runtime. Config comes from
// the environment (DATABASE_URL required; DASHBOARD_TOKEN gates access; RUNTIME_URL enables the
// console). No secrets in code.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/scottylabs/scottylabs-agent/services/dashboard/internal/runtimeclient"
	"github.com/scottylabs/scottylabs-agent/services/dashboard/internal/server"
	"github.com/scottylabs/scottylabs-agent/services/session-memory/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		logger.Error("dashboard: DATABASE_URL is required")
		os.Exit(1)
	}
	ctx := context.Background()
	db, err := store.Open(ctx, dsn)
	if err != nil {
		logger.Error("dashboard: open database", "err", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()
	if err := store.Migrate(db); err != nil {
		logger.Error("dashboard: migrate", "err", err)
		os.Exit(1)
	}

	rt := runtimeclient.New(os.Getenv("RUNTIME_URL"), os.Getenv("RUNTIME_SERVICE_TOKEN"))
	token := os.Getenv("DASHBOARD_TOKEN")
	if token == "" {
		logger.Warn("dashboard: DASHBOARD_TOKEN is empty — running in OPEN dev mode (no auth)")
	}
	srv := server.New(store.New(db), rt, token)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	logger.Info("dashboard serving", "addr", addr, "runtime", rt.Configured())
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		logger.Error("dashboard: server exited", "err", err)
		os.Exit(1)
	}
}
