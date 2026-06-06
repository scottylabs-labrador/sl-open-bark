package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Lifecycle states for a registered server.
const (
	LifecycleProposed   = "proposed"
	LifecycleApproved   = "approved"
	LifecycleDeprecated = "deprecated"
	LifecycleDisabled   = "disabled"
)

// ToolInput / ServerInput are the registry's view of a server's manifest.yaml.
type ToolInput struct {
	Name   string
	Scope  string
	Impact string
}

type ServerInput struct {
	Name        string
	Owner       string
	Description string
	Endpoint    string
	Transport   string
	Auth        string
	Tools       []ToolInput
	Committees  []string
}

// Tool is a registered tool.
type Tool struct {
	ID       string `json:"id"`
	ServerID string `json:"server_id"`
	Name     string `json:"name"`
	Scope    string `json:"scope"`
	Impact   string `json:"impact"`
}

// Server is a registered MCP server with its tools and committee grants.
type Server struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Owner       string    `json:"owner"`
	Description string    `json:"description"`
	Endpoint    string    `json:"endpoint"`
	Transport   string    `json:"transport"`
	Auth        string    `json:"auth"`
	Lifecycle   string    `json:"lifecycle"`
	Enabled     bool      `json:"enabled"`
	Health      string    `json:"health"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Tools       []Tool    `json:"tools"`
	Committees  []string  `json:"committees"`
}

// VisibleTool is a tool a caller is allowed to see, with the server it belongs to.
type VisibleTool struct {
	ServerName string `json:"server_name"`
	ServerID   string `json:"server_id"`
	ToolName   string `json:"tool_name"`
	Scope      string `json:"scope"`
	Impact     string `json:"impact"`
}

// ToolBinding resolves a (server, tool) pair to everything the gateway needs to authorize and
// proxy a call: the server's endpoint, lifecycle, and committee grants, plus the tool's impact.
type ToolBinding struct {
	Server Server
	Tool   Tool
}

// RegisterServer upserts a server and replaces its tools and committee grants atomically. On first
// registration the server lands in 'proposed' (it does nothing live until promoted); re-registering
// updates metadata, tools, and committees but preserves the current lifecycle and enabled flag.
func (r *Repository) RegisterServer(ctx context.Context, in ServerInput) (Server, error) {
	if in.Name == "" || in.Endpoint == "" {
		return Server{}, errors.New("store: register server: name and endpoint are required")
	}
	transport := in.Transport
	if transport == "" {
		transport = "streamable-http"
	}
	auth := in.Auth
	if auth == "" {
		auth = "bearer"
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Server{}, fmt.Errorf("store: register server: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const upsert = `
		INSERT INTO mcp_servers (name, owner, description, endpoint, transport, auth)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (name) DO UPDATE SET
			owner = EXCLUDED.owner, description = EXCLUDED.description,
			endpoint = EXCLUDED.endpoint, transport = EXCLUDED.transport,
			auth = EXCLUDED.auth, updated_at = now()
		RETURNING id`
	var serverID string
	if err := tx.QueryRowContext(ctx, upsert,
		in.Name, in.Owner, in.Description, in.Endpoint, transport, auth).Scan(&serverID); err != nil {
		return Server{}, fmt.Errorf("store: register server: upsert: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM mcp_tools WHERE server_id = $1`, serverID); err != nil {
		return Server{}, fmt.Errorf("store: register server: clear tools: %w", err)
	}
	for _, t := range in.Tools {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO mcp_tools (server_id, name, scope, impact) VALUES ($1, $2, $3, $4)`,
			serverID, t.Name, t.Scope, t.Impact); err != nil {
			return Server{}, fmt.Errorf("store: register server: insert tool %q: %w", t.Name, err)
		}
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM server_committees WHERE server_id = $1`, serverID); err != nil {
		return Server{}, fmt.Errorf("store: register server: clear committees: %w", err)
	}
	for _, c := range in.Committees {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO server_committees (server_id, committee) VALUES ($1, $2)
			 ON CONFLICT DO NOTHING`, serverID, c); err != nil {
			return Server{}, fmt.Errorf("store: register server: grant committee %q: %w", c, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return Server{}, fmt.Errorf("store: register server: commit: %w", err)
	}
	return r.GetServerByName(ctx, in.Name)
}

const serverCols = `id, name, owner, description, endpoint, transport, auth,
	lifecycle, enabled, health, created_at, updated_at`

func scanServer(row interface{ Scan(...any) error }) (Server, error) {
	var s Server
	if err := row.Scan(&s.ID, &s.Name, &s.Owner, &s.Description, &s.Endpoint, &s.Transport,
		&s.Auth, &s.Lifecycle, &s.Enabled, &s.Health, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return Server{}, err
	}
	return s, nil
}

// GetServerByName loads a server with its tools and committee grants, or ErrNotFound.
func (r *Repository) GetServerByName(ctx context.Context, name string) (Server, error) {
	s, err := scanServer(r.db.QueryRowContext(ctx,
		`SELECT `+serverCols+` FROM mcp_servers WHERE name = $1`, name))
	if errors.Is(err, sql.ErrNoRows) {
		return Server{}, ErrNotFound
	}
	if err != nil {
		return Server{}, fmt.Errorf("store: get server: %w", err)
	}
	if s.Tools, err = r.listTools(ctx, s.ID); err != nil {
		return Server{}, err
	}
	if s.Committees, err = r.listServerCommittees(ctx, s.ID); err != nil {
		return Server{}, err
	}
	return s, nil
}

// ListServers returns every registered server (with tools and committees), ordered by name. Used
// by the dashboard and the runtime's discovery.
func (r *Repository) ListServers(ctx context.Context) ([]Server, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+serverCols+` FROM mcp_servers ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("store: list servers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var servers []Server
	for rows.Next() {
		s, err := scanServer(rows)
		if err != nil {
			return nil, fmt.Errorf("store: scan server: %w", err)
		}
		servers = append(servers, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range servers {
		if servers[i].Tools, err = r.listTools(ctx, servers[i].ID); err != nil {
			return nil, err
		}
		if servers[i].Committees, err = r.listServerCommittees(ctx, servers[i].ID); err != nil {
			return nil, err
		}
	}
	return servers, nil
}

func (r *Repository) listTools(ctx context.Context, serverID string) ([]Tool, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, server_id, name, scope, impact FROM mcp_tools WHERE server_id = $1 ORDER BY name`, serverID)
	if err != nil {
		return nil, fmt.Errorf("store: list tools: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tools []Tool
	for rows.Next() {
		var t Tool
		if err := rows.Scan(&t.ID, &t.ServerID, &t.Name, &t.Scope, &t.Impact); err != nil {
			return nil, fmt.Errorf("store: scan tool: %w", err)
		}
		tools = append(tools, t)
	}
	return tools, rows.Err()
}

func (r *Repository) listServerCommittees(ctx context.Context, serverID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT committee FROM server_committees WHERE server_id = $1 ORDER BY committee`, serverID)
	if err != nil {
		return nil, fmt.Errorf("store: list server committees: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, fmt.Errorf("store: scan committee: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SetServerLifecycle promotes/demotes a server (proposed -> approved -> deprecated/disabled). This
// is the gate that makes a server live.
func (r *Repository) SetServerLifecycle(ctx context.Context, name, lifecycle string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE mcp_servers SET lifecycle = $2, updated_at = now() WHERE name = $1`, name, lifecycle)
	if err != nil {
		return fmt.Errorf("store: set lifecycle: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetServerEnabled enables or disables a server without changing its lifecycle (the gateway's
// one-place kill switch).
func (r *Repository) SetServerEnabled(ctx context.Context, name string, enabled bool) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE mcp_servers SET enabled = $2, updated_at = now() WHERE name = $1`, name, enabled)
	if err != nil {
		return fmt.Errorf("store: set enabled: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ResolveTool returns the server+tool for a (serverName, toolName), or ErrNotFound. The gateway
// uses it to authorize and route a call.
func (r *Repository) ResolveTool(ctx context.Context, serverName, toolName string) (ToolBinding, error) {
	s, err := r.GetServerByName(ctx, serverName)
	if err != nil {
		return ToolBinding{}, err
	}
	for _, t := range s.Tools {
		if t.Name == toolName {
			return ToolBinding{Server: s, Tool: t}, nil
		}
	}
	return ToolBinding{}, ErrNotFound
}

// ListVisibleTools returns the tools a caller in the given committees may see: tools whose server
// is approved AND enabled AND granted to at least one of the caller's committees. This is the
// discovery surface — a caller never even lists tools its role does not permit.
func (r *Repository) ListVisibleTools(ctx context.Context, committees []string) ([]VisibleTool, error) {
	if len(committees) == 0 {
		return nil, nil
	}
	committeesJSON, err := json.Marshal(committees)
	if err != nil {
		return nil, fmt.Errorf("store: encode committees: %w", err)
	}
	const q = `
		SELECT DISTINCT s.name, s.id, t.name, t.scope, t.impact
		FROM mcp_tools t
		JOIN mcp_servers s ON s.id = t.server_id
		JOIN server_committees sc ON sc.server_id = s.id
		WHERE s.lifecycle = 'approved' AND s.enabled
		  AND sc.committee IN (SELECT value FROM jsonb_array_elements_text($1::jsonb))
		ORDER BY s.name, t.name`
	rows, err := r.db.QueryContext(ctx, q, string(committeesJSON))
	if err != nil {
		return nil, fmt.Errorf("store: list visible tools: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []VisibleTool
	for rows.Next() {
		var vt VisibleTool
		if err := rows.Scan(&vt.ServerName, &vt.ServerID, &vt.ToolName, &vt.Scope, &vt.Impact); err != nil {
			return nil, fmt.Errorf("store: scan visible tool: %w", err)
		}
		out = append(out, vt)
	}
	return out, rows.Err()
}
