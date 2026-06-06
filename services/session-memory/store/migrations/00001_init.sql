-- The durable state model for the ScottyLabs Agent Platform (design Section 4.4): identity and
-- roles, sessions and conversation, scoped long-term memory, approvals, and the audit log. All
-- persistence goes through the typed repository in this package; this schema is its backing store.

-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Who the agent is talking to. Resolved once per request from the Slack identity.
CREATE TABLE users (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slack_id   text NOT NULL UNIQUE,
    name       text NOT NULL DEFAULT '',
    email      text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

-- A user's committee memberships and role, e.g. finance:member, events:lead. Drives RBAC.
CREATE TABLE committee_roles (
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    committee  text NOT NULL,
    role       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, committee)
);

-- An in-progress task and its Slack thread, so a multi-step task survives across messages.
CREATE TABLE sessions (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid REFERENCES users (id) ON DELETE CASCADE,
    channel    text NOT NULL DEFAULT '',
    thread_ts  text NOT NULL DEFAULT '',
    recipe     text NOT NULL DEFAULT '',
    status     text NOT NULL DEFAULT 'active',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_user_idx ON sessions (user_id);
CREATE INDEX sessions_thread_idx ON sessions (channel, thread_ts);

-- The raw conversation. Older turns are summarized; the last few are carried verbatim.
CREATE TABLE turns (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    session_id uuid NOT NULL REFERENCES sessions (id) ON DELETE CASCADE,
    role       text NOT NULL CHECK (role IN ('system', 'user', 'assistant', 'tool')),
    content    text NOT NULL DEFAULT '',
    tokens     integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX turns_session_idx ON turns (session_id, id);

-- A rolling summary of a thread, one row per session.
CREATE TABLE summaries (
    session_id uuid PRIMARY KEY REFERENCES sessions (id) ON DELETE CASCADE,
    summary    text NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Durable facts scoped to a user, committee, or org. Tagged, retrievable by recency, and with an
-- optional expiry for retention and a "forget" path. Scope is (scope_type, scope_id); a read must
-- always filter on both so one principal's context never leaks into another's.
CREATE TABLE memory_facts (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_type text NOT NULL CHECK (scope_type IN ('user', 'committee', 'org')),
    scope_id   text NOT NULL,
    key        text NOT NULL,
    value      text NOT NULL DEFAULT '',
    tags       text[] NOT NULL DEFAULT '{}',
    source     text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz
);
CREATE INDEX memory_facts_scope_idx ON memory_facts (scope_type, scope_id, created_at DESC);
CREATE INDEX memory_facts_tags_idx ON memory_facts USING gin (tags);
CREATE INDEX memory_facts_expiry_idx ON memory_facts (expires_at) WHERE expires_at IS NOT NULL;

-- A pending or decided human approval for a high-impact action. The gateway requires an approved
-- row before an impact:high tool runs; the decision is recorded here, never in model context.
CREATE TABLE approvals (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id    uuid REFERENCES sessions (id) ON DELETE CASCADE,
    tool          text NOT NULL,
    args_redacted jsonb NOT NULL DEFAULT '{}'::jsonb,
    status        text NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending', 'approved', 'denied', 'cancelled')),
    decided_by    text NOT NULL DEFAULT '',
    decided_at    timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX approvals_session_idx ON approvals (session_id);
CREATE INDEX approvals_status_idx ON approvals (status);

-- Every side effect, for trust and forensics: actor, tool, redacted args, result, and timing.
CREATE TABLE audit_log (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    actor         text NOT NULL DEFAULT '',
    tool          text NOT NULL DEFAULT '',
    args_redacted jsonb NOT NULL DEFAULT '{}'::jsonb,
    result        text NOT NULL DEFAULT '',
    latency_ms    integer NOT NULL DEFAULT 0,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_log_created_idx ON audit_log (created_at);
CREATE INDEX audit_log_actor_idx ON audit_log (actor);
CREATE INDEX audit_log_tool_idx ON audit_log (tool);

-- +goose Down
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS approvals;
DROP TABLE IF EXISTS memory_facts;
DROP TABLE IF EXISTS summaries;
DROP TABLE IF EXISTS turns;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS committee_roles;
DROP TABLE IF EXISTS users;
