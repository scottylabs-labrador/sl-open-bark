package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// nullable converts an empty string to a SQL NULL (used for the optional sessions.user_id FK).
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func scanSession(row interface{ Scan(...any) error }) (Session, error) {
	var s Session
	var userID sql.NullString
	if err := row.Scan(&s.ID, &userID, &s.Channel, &s.ThreadTS, &s.Recipe, &s.Status,
		&s.CreatedAt, &s.UpdatedAt); err != nil {
		return Session{}, err
	}
	s.UserID = userID.String
	return s, nil
}

const sessionCols = `id, user_id, channel, thread_ts, recipe, status, created_at, updated_at`

// CreateSession starts a new session for a Slack thread. userID may be empty if identity is not
// yet resolved.
func (r *Repository) CreateSession(ctx context.Context, userID, channel, threadTS, recipe string) (Session, error) {
	const q = `
		INSERT INTO sessions (user_id, channel, thread_ts, recipe)
		VALUES ($1, $2, $3, $4)
		RETURNING ` + sessionCols
	row := r.db.QueryRowContext(ctx, q, nullable(userID), channel, threadTS, recipe)
	s, err := scanSession(row)
	if err != nil {
		return Session{}, fmt.Errorf("store: create session: %w", err)
	}
	return s, nil
}

// GetSession returns a session by id, or ErrNotFound.
func (r *Repository) GetSession(ctx context.Context, id string) (Session, error) {
	const q = `SELECT ` + sessionCols + ` FROM sessions WHERE id = $1`
	s, err := scanSession(r.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("store: get session: %w", err)
	}
	return s, nil
}

// GetSessionByThread returns the most recent session for a Slack channel+thread, or ErrNotFound.
// Used to resume an in-progress task when a member replies in the same thread.
func (r *Repository) GetSessionByThread(ctx context.Context, channel, threadTS string) (Session, error) {
	const q = `SELECT ` + sessionCols + `
		FROM sessions WHERE channel = $1 AND thread_ts = $2
		ORDER BY created_at DESC LIMIT 1`
	s, err := scanSession(r.db.QueryRowContext(ctx, q, channel, threadTS))
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("store: get session by thread: %w", err)
	}
	return s, nil
}

// UpdateSessionStatus sets a session's status (e.g. "active", "awaiting_approval", "done") and
// bumps updated_at. Returns ErrNotFound if the session does not exist.
func (r *Repository) UpdateSessionStatus(ctx context.Context, id, status string) error {
	const q = `UPDATE sessions SET status = $2, updated_at = now() WHERE id = $1`
	res, err := r.db.ExecContext(ctx, q, id, status)
	if err != nil {
		return fmt.Errorf("store: update session status: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// AddTurn appends a message to a session's conversation.
func (r *Repository) AddTurn(ctx context.Context, sessionID, role, content string, tokens int) (Turn, error) {
	const q = `
		INSERT INTO turns (session_id, role, content, tokens)
		VALUES ($1, $2, $3, $4)
		RETURNING id, session_id, role, content, tokens, created_at`
	var t Turn
	err := r.db.QueryRowContext(ctx, q, sessionID, role, content, tokens).
		Scan(&t.ID, &t.SessionID, &t.Role, &t.Content, &t.Tokens, &t.CreatedAt)
	if err != nil {
		return Turn{}, fmt.Errorf("store: add turn: %w", err)
	}
	return t, nil
}

// ListTurns returns a session's turns in chronological order. If limit > 0 only the most recent
// limit turns are returned (still chronological), which is what the context pipeline wants for
// "the last N raw turns".
func (r *Repository) ListTurns(ctx context.Context, sessionID string, limit int) ([]Turn, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if limit > 0 {
		const q = `
			SELECT id, session_id, role, content, tokens, created_at FROM (
				SELECT id, session_id, role, content, tokens, created_at
				FROM turns WHERE session_id = $1 ORDER BY id DESC LIMIT $2
			) recent ORDER BY id ASC`
		rows, err = r.db.QueryContext(ctx, q, sessionID, limit)
	} else {
		const q = `
			SELECT id, session_id, role, content, tokens, created_at
			FROM turns WHERE session_id = $1 ORDER BY id ASC`
		rows, err = r.db.QueryContext(ctx, q, sessionID)
	}
	if err != nil {
		return nil, fmt.Errorf("store: list turns: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Turn
	for rows.Next() {
		var t Turn
		if err := rows.Scan(&t.ID, &t.SessionID, &t.Role, &t.Content, &t.Tokens, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("store: scan turn: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// UpsertSummary sets (or replaces) the rolling summary for a session.
func (r *Repository) UpsertSummary(ctx context.Context, sessionID, summary string) (Summary, error) {
	const q = `
		INSERT INTO summaries (session_id, summary, updated_at)
		VALUES ($1, $2, now())
		ON CONFLICT (session_id) DO UPDATE SET summary = EXCLUDED.summary, updated_at = now()
		RETURNING session_id, summary, updated_at`
	var s Summary
	err := r.db.QueryRowContext(ctx, q, sessionID, summary).
		Scan(&s.SessionID, &s.Summary, &s.UpdatedAt)
	if err != nil {
		return Summary{}, fmt.Errorf("store: upsert summary: %w", err)
	}
	return s, nil
}

// GetSummary returns a session's rolling summary, or ErrNotFound if none has been written.
func (r *Repository) GetSummary(ctx context.Context, sessionID string) (Summary, error) {
	const q = `SELECT session_id, summary, updated_at FROM summaries WHERE session_id = $1`
	var s Summary
	err := r.db.QueryRowContext(ctx, q, sessionID).Scan(&s.SessionID, &s.Summary, &s.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Summary{}, ErrNotFound
	}
	if err != nil {
		return Summary{}, fmt.Errorf("store: get summary: %w", err)
	}
	return s, nil
}
