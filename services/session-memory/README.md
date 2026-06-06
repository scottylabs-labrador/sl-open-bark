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

## What's next (WP-05)

Built on this store, WP-05 adds:

- A **Memory MCP** (`memory.write_fact`, `memory.search`, `memory.forget`) exposing the scoped
  repository to recipes like any other capability, backed by these tables.
- The per-turn **context-assembly pipeline** that budgets context in priority order (system +
  `.goosehints` + recipe + retrieved memory + rolling summary + recent turns + trimmed tool
  output) and updates the rolling summary and durable facts after each turn.

MCP servers stay stateless and idempotent; durable state lives here in Postgres, not in a server.
