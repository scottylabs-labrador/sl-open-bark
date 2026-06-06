package store

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

const migrationsDir = "migrations"

// Migrate applies all pending up migrations against db. It is idempotent — already-applied
// migrations are skipped — so it is safe to call on every service start.
func Migrate(db *sql.DB) error {
	goose.SetBaseFS(migrationsFS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("store: set dialect: %w", err)
	}
	if err := goose.Up(db, migrationsDir); err != nil {
		return fmt.Errorf("store: migrate up: %w", err)
	}
	return nil
}

// Rollback rolls every migration back down to a clean database. Used by tests to prove migrations
// roll back cleanly; in production, prefer a reviewed, targeted down migration over a full reset.
func Rollback(db *sql.DB) error {
	goose.SetBaseFS(migrationsFS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("store: set dialect: %w", err)
	}
	if err := goose.DownTo(db, migrationsDir, 0); err != nil {
		return fmt.Errorf("store: migrate down: %w", err)
	}
	return nil
}
