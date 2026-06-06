// Command retention is the data-retention job (a stub for WP-01): it deletes expired memory facts
// and ages out audit rows past the retention window, then exits. Run it on a schedule (WP-09).
//
// Config comes from the environment (no secrets in code):
//
//	DATABASE_URL          Postgres DSN (required)
//	AUDIT_RETENTION_DAYS  how long to keep audit rows (default 365; policy — leadership decides)
package main

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/scottylabs/scottylabs-agent/services/session-memory/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		logger.Error("retention: DATABASE_URL is required")
		os.Exit(1)
	}
	retentionDays := envInt("AUDIT_RETENTION_DAYS", 365)

	ctx := context.Background()
	db, err := store.Open(ctx, dsn)
	if err != nil {
		logger.Error("retention: open database", "err", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	repo := store.New(db)
	now := time.Now()

	facts, err := repo.DeleteExpiredFacts(ctx, now)
	if err != nil {
		logger.Error("retention: delete expired facts", "err", err)
		os.Exit(1)
	}

	cutoff := now.AddDate(0, 0, -retentionDays)
	audit, err := repo.AgeOutAudit(ctx, cutoff)
	if err != nil {
		logger.Error("retention: age out audit", "err", err)
		os.Exit(1)
	}

	logger.Info("retention complete",
		"expired_facts_deleted", facts,
		"audit_rows_aged_out", audit,
		"audit_retention_days", retentionDays,
		"audit_cutoff", cutoff.Format(time.RFC3339))
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
