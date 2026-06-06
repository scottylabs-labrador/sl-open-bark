// Command engineering-agent is the isolated coding-agent orchestrator (design 12). It receives
// /fix-bug tasks (enqueued by the Slack gateway), runs each in a throwaway sandbox, and opens a
// draft PR. It has NO access to the MCP gateway, Google, finance data, or any production secret —
// only a scoped GitHub identity and an allowlisted-egress sandbox.
package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"

	"github.com/scottylabs/scottylabs-agent/services/engineering-agent/internal/authz"
	"github.com/scottylabs/scottylabs-agent/services/engineering-agent/internal/config"
	"github.com/scottylabs/scottylabs-agent/services/engineering-agent/internal/githubapp"
	"github.com/scottylabs/scottylabs-agent/services/engineering-agent/internal/notify"
	"github.com/scottylabs/scottylabs-agent/services/engineering-agent/internal/orchestrator"
	"github.com/scottylabs/scottylabs-agent/services/engineering-agent/internal/sandbox"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()

	var notifier orchestrator.Notifier = notify.Log{}
	if cfg.SlackWebhookURL != "" {
		notifier = notify.NewWebhook(cfg.SlackWebhookURL)
	}
	allow := authz.New(cfg.Maintainers, cfg.Repos)
	orch := orchestrator.New(githubapp.New(cfg.GitHub), sandbox.NewRailway(), notifier, allow, cfg.SandboxTemplate, cfg.TimeLimit)

	logger.Info("engineering-agent: loaded", "maintainers", len(cfg.Maintainers), "repos", len(cfg.Repos))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("POST /fix-bug", func(w http.ResponseWriter, r *http.Request) {
		if cfg.ServiceToken == "" || r.Header.Get("Authorization") != "Bearer "+cfg.ServiceToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var task orchestrator.Task
		if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		// Run in the background; the result is posted to Slack via the notifier.
		go func() {
			res, err := orch.Run(context.Background(), task)
			if err != nil {
				logger.Error("engineering-agent: task failed", "repo", task.Repo, "issue", task.Issue, "err", err)
				return
			}
			logger.Info("engineering-agent: draft PR opened", "repo", task.Repo, "pr", res.PRURL)
		}()
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"accepted"}`))
	})

	addr := ":" + cfg.Port
	logger.Info("engineering-agent serving", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		logger.Error("engineering-agent: server exited", "err", err)
		os.Exit(1)
	}
}
