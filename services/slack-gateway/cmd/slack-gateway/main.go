// Command slack-gateway is the human front door: a Slack Events API app that acks fast and runs work
// in the background through the runtime (design 3.3, 9.4; Appendix E). Config from the environment.
package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/scottylabs/scottylabs-agent/services/slack-gateway/internal/config"
	"github.com/scottylabs/scottylabs-agent/services/slack-gateway/internal/runtimeclient"
	slackgw "github.com/scottylabs/scottylabs-agent/services/slack-gateway/internal/slack"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()

	if cfg.SigningSecret == "" {
		logger.Warn("slack-gateway: SLACK_SIGNING_SECRET empty — request signatures are NOT verified (dev only)")
	}
	var poster slackgw.Poster = slackgw.LogPoster{}
	if cfg.BotToken != "" {
		poster = slackgw.NewSlackPoster(cfg.BotToken)
	} else {
		logger.Warn("slack-gateway: SLACK_BOT_TOKEN empty — posting to logs only")
	}

	rt := runtimeclient.New(cfg.RuntimeURL, cfg.RuntimeToken)
	h := slackgw.New(rt, poster, cfg.SigningSecret, cfg.BotUserID)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /slack/events", h.Events)
	mux.HandleFunc("POST /slack/interactions", h.Interactions)
	mux.HandleFunc("POST /slack/commands", h.Command)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	addr := ":" + cfg.Port
	logger.Info("slack-gateway serving", "addr", addr, "runtime", rt.Configured())
	if err := http.ListenAndServe(addr, mux); err != nil {
		logger.Error("slack-gateway: server exited", "err", err)
		os.Exit(1)
	}
}
