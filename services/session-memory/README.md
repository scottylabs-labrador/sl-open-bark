# services/session-memory/

Statefulness and context engineering. This module owns the platform's **durable state** (design
Section 4.4): the Postgres schema, migrations, and the typed repository that every other component
uses to reach the database — plus, later, the Memory MCP and the context-assembly pipeline.

**Owner:** platform-team.

## What's here now (WP-01)

The persistence layer — the **only** way other components touch Postgres:

```
store/                     typed repository (package store), no business logic
  migrations/00001_init.sql  schema: users, committee_roles, sessions, turns, summaries,
                             memory_facts, approvals, audit_log (design §4.4), with up/down
  store.go migrate.go models.go identity.go sessions.go memory.go governance.go
  store_test.go            tests against a real Postgres (scoping, expiry, rollback, ...)
cmd/retention/             retention job stub: reap expired facts, age out old audit rows
```

The `store.Repository` exposes CRUD and **scoped** queries for each entity. Memory facts are scoped
by `(scope_type, scope_id)` where `scope_type ∈ {user, committee, org}`, and every read filters on
both — so one principal's context can never surface for another. Migrations run with
`store.Migrate(db)` (idempotent; safe on every start) and roll back with `store.Rollback(db)`.

### Run the tests

Tests need a disposable Postgres; without one they skip, so `make test` stays green anywhere. CI
provides a Postgres service.

```bash
# point at any throwaway database
export TEST_DATABASE_URL='postgres://user@localhost:5432/sl_test?sslmode=disable'
go test ./...
```

### Retention job

```bash
DATABASE_URL='postgres://…' AUDIT_RETENTION_DAYS=365 go run ./cmd/retention
```

`AUDIT_RETENTION_DAYS` is policy (the retention window is an open question for leadership; design
Section 14). No secrets in code — config comes from the environment.

## Statefulness and context engineering (WP-05)

Built on the store:

- **Memory MCP** (`cmd/memory-mcp`, `internal/memory`) — exposes scoped memory as MCP tools so
  recipes can use it like any other capability: `write_fact` (write), `search` (read, scoped),
  `forget` (write). Registry manifest at [`../../mcp-servers/memory/`](../../mcp-servers/memory/).
  Every search is scoped to a `(scope_type, scope_id)`, so one principal's facts never surface for
  another (enforced by the store).
- **Context-assembly pipeline** (`internal/assembly`) — assembles each turn's context in priority
  order (system + `.goosehints` + recipe + retrieved memory + rolling summary + recent turns +
  trimmed tool output), stopping before a **token budget** is hit and trimming the lowest-priority
  (tool-output) part rather than blowing the window. `Builder` pulls the scoped pieces from the
  store; `Assemble` is pure and deterministic. `PersistTurnResult` writes back the updated rolling
  summary and any new durable facts, so the agent gets smarter over time. The runtime (WP-06) drives
  it and supplies the model for summarization.

```
internal/memory/     the Memory MCP tools over the store      cmd/memory-mcp/  its server
internal/assembly/   budgeted, priority-ordered context assembly (pure) + the store-backed Builder
```

MCP servers stay stateless and idempotent; durable state lives here in Postgres, not in a server.
