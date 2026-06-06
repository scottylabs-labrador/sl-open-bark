-- The MCP gateway's registry (design Section 6.3): the catalog of registered servers, the tools
-- each exposes with their scope and impact, and which committees may use them. Backed by Postgres
-- like all other state, reached through the typed repository. Lifecycle gates what is live:
-- a server does nothing until a maintainer promotes it from 'proposed' to 'approved'.

-- +goose Up
CREATE TABLE mcp_servers (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name        text NOT NULL UNIQUE,
    owner       text NOT NULL,
    description text NOT NULL DEFAULT '',
    endpoint    text NOT NULL,
    transport   text NOT NULL DEFAULT 'streamable-http',
    auth        text NOT NULL DEFAULT 'bearer',
    lifecycle   text NOT NULL DEFAULT 'proposed'
                CHECK (lifecycle IN ('proposed', 'approved', 'deprecated', 'disabled')),
    enabled     boolean NOT NULL DEFAULT true,
    health      text NOT NULL DEFAULT 'unknown',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE mcp_tools (
    id        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id uuid NOT NULL REFERENCES mcp_servers (id) ON DELETE CASCADE,
    name      text NOT NULL,
    scope     text NOT NULL,
    impact    text NOT NULL CHECK (impact IN ('read', 'write', 'high')),
    UNIQUE (server_id, name)
);
CREATE INDEX mcp_tools_server_idx ON mcp_tools (server_id);
CREATE INDEX mcp_tools_name_idx ON mcp_tools (name);

-- Which committees a server is available to. The gateway intersects this with the caller's roles.
CREATE TABLE server_committees (
    server_id uuid NOT NULL REFERENCES mcp_servers (id) ON DELETE CASCADE,
    committee text NOT NULL,
    PRIMARY KEY (server_id, committee)
);
CREATE INDEX server_committees_committee_idx ON server_committees (committee);

-- +goose Down
DROP TABLE IF EXISTS server_committees;
DROP TABLE IF EXISTS mcp_tools;
DROP TABLE IF EXISTS mcp_servers;
