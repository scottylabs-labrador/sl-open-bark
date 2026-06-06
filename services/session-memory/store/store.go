// Package store is the typed data-access layer for the ScottyLabs Agent Platform's durable state
// (design Section 4.4). It is the ONLY way other components touch Postgres: the gateway, the
// runtime, and the Session/Memory service depend on these repository functions, not on raw SQL.
//
// There is no business logic here — methods are CRUD plus scoped queries. Decisions (what to
// remember, when to gate, how to budget context) live in the services that call this package.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // register the "pgx" database/sql driver
)

// ErrNotFound is returned by Get-style methods when no row matches. Callers can use errors.Is.
var ErrNotFound = errors.New("store: not found")

// Scope types for memory_facts. A fact is scoped to exactly one of these.
const (
	ScopeUser      = "user"
	ScopeCommittee = "committee"
	ScopeOrg       = "org"
)

// Open opens a database/sql pool against the given Postgres DSN (e.g.
// "postgres://user:pass@host:5432/db?sslmode=disable") and verifies connectivity. The caller owns
// the returned *sql.DB and must Close it.
func Open(ctx context.Context, dsn string) (*sql.DB, error) {
	if dsn == "" {
		return nil, errors.New("store: empty database DSN")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	return db, nil
}

// Repository is the data-access object. Construct it with New and inject it where persistence is
// needed; consumers should depend on a narrow interface they define themselves (Go idiom).
type Repository struct {
	db *sql.DB
}

// New wraps an open *sql.DB in a Repository.
func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// validScope reports whether t is a known memory-fact scope type.
func validScope(t string) bool {
	return t == ScopeUser || t == ScopeCommittee || t == ScopeOrg
}
