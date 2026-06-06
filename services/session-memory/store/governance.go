package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// defaultJSON returns a non-empty JSON object for an empty/nil payload, so jsonb columns are never
// fed an empty byte string.
func defaultJSON(b json.RawMessage) []byte {
	if len(b) == 0 {
		return []byte("{}")
	}
	return b
}

const approvalCols = `id, session_id, tool, args_redacted, status, decided_by, decided_at, created_at`

func scanApproval(row interface{ Scan(...any) error }) (Approval, error) {
	var (
		a         Approval
		sessionID sql.NullString
		args      []byte
		decidedAt sql.NullTime
	)
	if err := row.Scan(&a.ID, &sessionID, &a.Tool, &args, &a.Status, &a.DecidedBy,
		&decidedAt, &a.CreatedAt); err != nil {
		return Approval{}, err
	}
	a.SessionID = sessionID.String
	a.ArgsRedacted = json.RawMessage(args)
	if decidedAt.Valid {
		t := decidedAt.Time
		a.DecidedAt = &t
	}
	return a, nil
}

// CreateApproval records a pending approval request for a high-impact action. sessionID may be
// empty. The gateway requires an "approved" row before an impact:high tool runs.
func (r *Repository) CreateApproval(ctx context.Context, sessionID, tool string, argsRedacted json.RawMessage) (Approval, error) {
	const q = `INSERT INTO approvals (session_id, tool, args_redacted)
		VALUES ($1, $2, $3) RETURNING ` + approvalCols
	a, err := scanApproval(r.db.QueryRowContext(ctx, q, nullable(sessionID), tool, defaultJSON(argsRedacted)))
	if err != nil {
		return Approval{}, fmt.Errorf("store: create approval: %w", err)
	}
	return a, nil
}

// GetApproval returns an approval by id, or ErrNotFound.
func (r *Repository) GetApproval(ctx context.Context, id string) (Approval, error) {
	const q = `SELECT ` + approvalCols + ` FROM approvals WHERE id = $1`
	a, err := scanApproval(r.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Approval{}, ErrNotFound
	}
	if err != nil {
		return Approval{}, fmt.Errorf("store: get approval: %w", err)
	}
	return a, nil
}

// DecideApproval records a human decision (status approved | denied | cancelled) and who made it.
// Returns the updated row, or ErrNotFound.
func (r *Repository) DecideApproval(ctx context.Context, id, status, decidedBy string) (Approval, error) {
	const q = `UPDATE approvals SET status = $2, decided_by = $3, decided_at = now()
		WHERE id = $1 RETURNING ` + approvalCols
	a, err := scanApproval(r.db.QueryRowContext(ctx, q, id, status, decidedBy))
	if errors.Is(err, sql.ErrNoRows) {
		return Approval{}, ErrNotFound
	}
	if err != nil {
		return Approval{}, fmt.Errorf("store: decide approval: %w", err)
	}
	return a, nil
}

// ListApprovalsBySession returns a session's approvals, newest first.
func (r *Repository) ListApprovalsBySession(ctx context.Context, sessionID string) ([]Approval, error) {
	const q = `SELECT ` + approvalCols + ` FROM approvals WHERE session_id = $1 ORDER BY created_at DESC`
	return r.queryApprovals(ctx, q, sessionID)
}

// ListPendingApprovals returns all approvals still awaiting a decision, oldest first (the queue the
// dashboard works through).
func (r *Repository) ListPendingApprovals(ctx context.Context) ([]Approval, error) {
	const q = `SELECT ` + approvalCols + ` FROM approvals WHERE status = 'pending' ORDER BY created_at ASC`
	return r.queryApprovals(ctx, q)
}

func (r *Repository) queryApprovals(ctx context.Context, q string, args ...any) ([]Approval, error) {
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list approvals: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Approval
	for rows.Next() {
		a, err := scanApproval(rows)
		if err != nil {
			return nil, fmt.Errorf("store: scan approval: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

const auditCols = `id, actor, tool, args_redacted, result, latency_ms, created_at`

func scanAudit(row interface{ Scan(...any) error }) (AuditEntry, error) {
	var (
		e    AuditEntry
		args []byte
	)
	if err := row.Scan(&e.ID, &e.Actor, &e.Tool, &args, &e.Result, &e.LatencyMS, &e.CreatedAt); err != nil {
		return AuditEntry{}, err
	}
	e.ArgsRedacted = json.RawMessage(args)
	return e, nil
}

// WriteAudit appends one entry to the audit log. Every side effect should be routed through here
// (or the service that calls it) so nothing escapes the trail.
func (r *Repository) WriteAudit(ctx context.Context, in AuditInput) (AuditEntry, error) {
	const q = `INSERT INTO audit_log (actor, tool, args_redacted, result, latency_ms)
		VALUES ($1, $2, $3, $4, $5) RETURNING ` + auditCols
	e, err := scanAudit(r.db.QueryRowContext(ctx, q,
		in.Actor, in.Tool, defaultJSON(in.ArgsRedacted), in.Result, in.LatencyMS))
	if err != nil {
		return AuditEntry{}, fmt.Errorf("store: write audit: %w", err)
	}
	return e, nil
}

// ListAudit returns the most recent audit entries, newest first. limit <= 0 returns a default of
// 100.
func (r *Repository) ListAudit(ctx context.Context, limit int) ([]AuditEntry, error) {
	if limit <= 0 {
		limit = 100
	}
	const q = `SELECT ` + auditCols + ` FROM audit_log ORDER BY id DESC LIMIT $1`
	rows, err := r.db.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("store: list audit: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []AuditEntry
	for rows.Next() {
		e, err := scanAudit(rows)
		if err != nil {
			return nil, fmt.Errorf("store: scan audit: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// AgeOutAudit deletes audit entries older than the cutoff, returning the count removed. The
// retention window is policy (an open question for leadership); the caller supplies the cutoff.
func (r *Repository) AgeOutAudit(ctx context.Context, before time.Time) (int64, error) {
	const q = `DELETE FROM audit_log WHERE created_at < $1`
	res, err := r.db.ExecContext(ctx, q, before)
	if err != nil {
		return 0, fmt.Errorf("store: age out audit: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
