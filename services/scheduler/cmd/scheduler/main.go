// Command scheduler runs recurring recipes via the runtime (design 4.2, 11). Two modes:
//
//	scheduler        long-running: tick every minute and run due jobs; serves /healthz
//	scheduler run    one-shot: run jobs due now and exit (for Railway cron, min 5-min granularity)
//
// Config from the environment: RUNTIME_URL, RUNTIME_SERVICE_TOKEN, SCHEDULE_FILE, PORT.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/scottylabs/scottylabs-agent/services/scheduler/internal/config"
	"github.com/scottylabs/scottylabs-agent/services/scheduler/internal/runtimeclient"
	"github.com/scottylabs/scottylabs-agent/services/scheduler/internal/scheduler"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()

	jobs, err := cfg.Jobs()
	if err != nil {
		logger.Error("scheduler: load jobs", "err", err)
		os.Exit(1)
	}
	rt := runtimeclient.New(cfg.RuntimeURL, cfg.RuntimeToken)
	if !rt.Configured() {
		logger.Warn("scheduler: RUNTIME_URL empty — due jobs will fail to submit")
	}
	s := scheduler.New(jobs, rt)
	logger.Info("scheduler: loaded", "jobs", len(jobs))

	if len(os.Args) > 1 && os.Args[1] == "run" {
		runOnce(context.Background(), s, logger)
		return
	}
	serve(s, cfg.Port, logger)
}

func runOnce(ctx context.Context, s *scheduler.Scheduler, logger *slog.Logger) {
	results, err := s.RunDue(ctx, time.Now().UTC())
	if err != nil {
		logger.Error("scheduler: run due", "err", err)
		os.Exit(1)
	}
	report(results, logger)
}

func serve(s *scheduler.Scheduler, port string, logger *slog.Logger) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	go func() {
		addr := ":" + port
		logger.Info("scheduler serving health", "addr", addr)
		if err := http.ListenAndServe(addr, mux); err != nil {
			logger.Error("scheduler: health server exited", "err", err)
		}
	}()

	// Align to the top of the next minute, then tick every minute.
	time.Sleep(time.Until(time.Now().Truncate(time.Minute).Add(time.Minute)))
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for now := range tick.C {
		results, err := s.RunDue(context.Background(), now.UTC())
		if err != nil {
			logger.Error("scheduler: run due", "err", err)
			continue
		}
		report(results, logger)
	}
}

func report(results []scheduler.RunResult, logger *slog.Logger) {
	for _, r := range results {
		if r.Err != "" {
			logger.Error("scheduler: job failed", "job", r.Job, "err", r.Err)
		} else {
			logger.Info("scheduler: job submitted", "job", r.Job, "task", r.TaskID)
		}
	}
}
