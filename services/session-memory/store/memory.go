package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// factCols selects a fact's columns. tags is bridged through JSON (array_to_json) so it scans
// reliably into a Go []string under the database/sql + pgx driver, which does not convert
// Postgres arrays directly.
const factCols = `id, scope_type, scope_id, key, value,
	COALESCE(array_to_json(tags)::text, '[]') AS tags, source, created_at, expires_at`

// encodeTags renders a tag slice as a JSON array string. Empty/nil becomes "[]" (never "null"),
// so the SQL side can always build a text[] from it.
func encodeTags(tags []string) (string, error) {
	if len(tags) == 0 {
		return "[]", nil
	}
	b, err := json.Marshal(tags)
	if err != nil {
		return "", fmt.Errorf("store: encode tags: %w", err)
	}
	return string(b), nil
}

func expiresArg(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}

func scanFact(row interface{ Scan(...any) error }) (Fact, error) {
	var (
		f        Fact
		tagsJSON string
		expires  sql.NullTime
	)
	if err := row.Scan(&f.ID, &f.ScopeType, &f.ScopeID, &f.Key, &f.Value,
		&tagsJSON, &f.Source, &f.CreatedAt, &expires); err != nil {
		return Fact{}, err
	}
	if err := json.Unmarshal([]byte(tagsJSON), &f.Tags); err != nil {
		return Fact{}, fmt.Errorf("store: decode tags: %w", err)
	}
	if f.Tags == nil {
		f.Tags = []string{}
	}
	if expires.Valid {
		t := expires.Time
		f.ExpiresAt = &t
	}
	return f, nil
}

// tagsToArraySQL is the SQL fragment that turns a JSON-array bind parameter ($n) into a text[].
func tagsToArraySQL(n int) string {
	return fmt.Sprintf(`COALESCE((SELECT array_agg(elem) FROM jsonb_array_elements_text($%d::jsonb) AS elem), '{}')`, n)
}

// WriteFact stores a durable fact in the given scope. Returns the stored row (with id and
// created_at). The scope_type must be one of user | committee | org.
func (r *Repository) WriteFact(ctx context.Context, in FactInput) (Fact, error) {
	if !validScope(in.ScopeType) {
		return Fact{}, fmt.Errorf("store: write fact: invalid scope_type %q", in.ScopeType)
	}
	tagsJSON, err := encodeTags(in.Tags)
	if err != nil {
		return Fact{}, err
	}
	q := `INSERT INTO memory_facts (scope_type, scope_id, key, value, tags, source, expires_at)
		VALUES ($1, $2, $3, $4, ` + tagsToArraySQL(5) + `, $6, $7)
		RETURNING ` + factCols
	row := r.db.QueryRowContext(ctx, q,
		in.ScopeType, in.ScopeID, in.Key, in.Value, tagsJSON, in.Source, expiresArg(in.ExpiresAt))
	f, err := scanFact(row)
	if err != nil {
		return Fact{}, fmt.Errorf("store: write fact: %w", err)
	}
	return f, nil
}

// SearchFacts returns facts in a single scope, most recent first. The query ALWAYS filters on both
// scope_type and scope_id, so results can never cross into another principal's scope. When Tags is
// set, only facts carrying at least one of those tags are returned. Expired facts are excluded
// unless IncludeExpired is set. Limit, when > 0, caps the result (top-k by recency).
func (r *Repository) SearchFacts(ctx context.Context, query FactQuery) ([]Fact, error) {
	if !validScope(query.ScopeType) {
		return nil, fmt.Errorf("store: search facts: invalid scope_type %q", query.ScopeType)
	}
	q := `SELECT ` + factCols + ` FROM memory_facts WHERE scope_type = $1 AND scope_id = $2`
	args := []any{query.ScopeType, query.ScopeID}

	if !query.IncludeExpired {
		q += ` AND (expires_at IS NULL OR expires_at > now())`
	}
	if len(query.Tags) > 0 {
		tagsJSON, err := encodeTags(query.Tags)
		if err != nil {
			return nil, err
		}
		q += ` AND tags && ` + tagsToArraySQL(len(args)+1)
		args = append(args, tagsJSON)
	}
	q += ` ORDER BY created_at DESC, key`
	if query.Limit > 0 {
		q += fmt.Sprintf(` LIMIT $%d`, len(args)+1)
		args = append(args, query.Limit)
	}

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("store: search facts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Fact
	for rows.Next() {
		f, err := scanFact(rows)
		if err != nil {
			return nil, fmt.Errorf("store: search facts: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// ForgetFact deletes a fact by id, reporting whether a row was removed. This is the "forget this"
// path.
func (r *Repository) ForgetFact(ctx context.Context, id string) (bool, error) {
	const q = `DELETE FROM memory_facts WHERE id = $1`
	res, err := r.db.ExecContext(ctx, q, id)
	if err != nil {
		return false, fmt.Errorf("store: forget fact: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ForgetFactsByKey deletes every fact with the given key within a single scope, returning the
// count removed. Scoped, so it can never delete another principal's facts.
func (r *Repository) ForgetFactsByKey(ctx context.Context, scopeType, scopeID, key string) (int64, error) {
	const q = `DELETE FROM memory_facts WHERE scope_type = $1 AND scope_id = $2 AND key = $3`
	res, err := r.db.ExecContext(ctx, q, scopeType, scopeID, key)
	if err != nil {
		return 0, fmt.Errorf("store: forget facts by key: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// DeleteExpiredFacts removes facts whose expires_at is at or before the given time, returning the
// count removed. Used by the retention job.
func (r *Repository) DeleteExpiredFacts(ctx context.Context, before time.Time) (int64, error) {
	const q = `DELETE FROM memory_facts WHERE expires_at IS NOT NULL AND expires_at <= $1`
	res, err := r.db.ExecContext(ctx, q, before)
	if err != nil {
		return 0, fmt.Errorf("store: delete expired facts: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
