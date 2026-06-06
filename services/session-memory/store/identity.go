package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// UpsertUser creates the user for a Slack id, or updates their name/email if already present.
// Identity is resolved once per request, so this is the common entry point.
func (r *Repository) UpsertUser(ctx context.Context, slackID, name, email string) (User, error) {
	const q = `
		INSERT INTO users (slack_id, name, email)
		VALUES ($1, $2, $3)
		ON CONFLICT (slack_id) DO UPDATE SET name = EXCLUDED.name, email = EXCLUDED.email
		RETURNING id, slack_id, name, email, created_at`
	var u User
	err := r.db.QueryRowContext(ctx, q, slackID, name, email).
		Scan(&u.ID, &u.SlackID, &u.Name, &u.Email, &u.CreatedAt)
	if err != nil {
		return User{}, fmt.Errorf("store: upsert user: %w", err)
	}
	return u, nil
}

// GetUserBySlackID returns the user for a Slack id, or ErrNotFound.
func (r *Repository) GetUserBySlackID(ctx context.Context, slackID string) (User, error) {
	const q = `SELECT id, slack_id, name, email, created_at FROM users WHERE slack_id = $1`
	var u User
	err := r.db.QueryRowContext(ctx, q, slackID).
		Scan(&u.ID, &u.SlackID, &u.Name, &u.Email, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("store: get user by slack id: %w", err)
	}
	return u, nil
}

// SetCommitteeRole grants or updates a user's role within a committee.
func (r *Repository) SetCommitteeRole(ctx context.Context, userID, committee, role string) error {
	const q = `
		INSERT INTO committee_roles (user_id, committee, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, committee) DO UPDATE SET role = EXCLUDED.role`
	if _, err := r.db.ExecContext(ctx, q, userID, committee, role); err != nil {
		return fmt.Errorf("store: set committee role: %w", err)
	}
	return nil
}

// ListCommitteeRoles returns all of a user's committee memberships, ordered by committee.
func (r *Repository) ListCommitteeRoles(ctx context.Context, userID string) ([]CommitteeRole, error) {
	const q = `
		SELECT user_id, committee, role, created_at
		FROM committee_roles WHERE user_id = $1 ORDER BY committee`
	rows, err := r.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("store: list committee roles: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []CommitteeRole
	for rows.Next() {
		var cr CommitteeRole
		if err := rows.Scan(&cr.UserID, &cr.Committee, &cr.Role, &cr.CreatedAt); err != nil {
			return nil, fmt.Errorf("store: scan committee role: %w", err)
		}
		out = append(out, cr)
	}
	return out, rows.Err()
}

// RemoveCommitteeRole revokes a user's membership in a committee. It reports whether a row was
// removed.
func (r *Repository) RemoveCommitteeRole(ctx context.Context, userID, committee string) (bool, error) {
	const q = `DELETE FROM committee_roles WHERE user_id = $1 AND committee = $2`
	res, err := r.db.ExecContext(ctx, q, userID, committee)
	if err != nil {
		return false, fmt.Errorf("store: remove committee role: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
