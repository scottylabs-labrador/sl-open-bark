# Google Workspace MCP

A capability server exposing **Calendar, Gmail, Drive, and Sheets** to the ScottyLabs agent,
read-first, acting as the agent's scottylabs.org identity via a scoped service account with
domain-wide delegation (design Sections 7.6, 8). Built on the Go MCP SDK; layered per the template.

## Tools

| Tool | Scope | Impact |
|---|---|---|
| `sheets_read` | `google.sheets.read` | read |
| `calendar_list` | `google.calendar.read` | read |
| `calendar_create_event` | `google.calendar.write` | write |
| `gmail_draft` | `google.gmail.draft` | write |
| `gmail_send` | `google.gmail.send` | **high** (irreversible — gated at the gateway) |
| `drive_read` | `google.drive.read` | read |

`gmail_send` is the only irreversible tool; it is `impact: high`, so the gateway requires a recorded
human approval before it runs. The agent never moves money; here it never sends without approval.

## Structure

```
internal/domain/    types + pure-function validation (no I/O), unit-tested
internal/service/   use-case orchestration; defines the Google interface it needs (DI)
internal/clients/   the concrete Google client (domain-wide delegation) — side effects at the edge
internal/tools/     thin MCP handlers
internal/config/    typed env config
cmd/server/         composition root: build client, register tools, serve /mcp + /healthz
```

Logic and mapping are tested against a **fake** Google client (`internal/service` tests) — no live
Google calls in CI. The real client is exercised end to end in WP-08.

## Identity and credentials (design §8.1)

The agent presents `agent@scottylabs.org`. A Google Cloud **service account with domain-wide
delegation** impersonates that one account, limited to the scopes in
`internal/clients/google.go` (Sheets read, Calendar events, Gmail compose+send, Drive read). A
Workspace admin configures the delegation; the scope list is reviewed whenever a tool is added.

> **Human-gated:** creating the service account, the `agent@scottylabs.org` account, and the
> domain-wide delegation (and providing the key) are human-gated steps. Provide the key via
> `GOOGLE_SA_JSON` / `GOOGLE_SA_JSON_FILE` and the subject via `GOOGLE_DELEGATED_SUBJECT`
> (env / Railway secrets). **No credentials in code.**

## Run

```bash
go test ./...                 # fast unit tests (fake client), no network
go vet ./... && gofmt -l .
# the server itself needs GOOGLE_SA_JSON(_FILE) + GOOGLE_DELEGATED_SUBJECT to start:
GOOGLE_SA_JSON_FILE=./sa.json GOOGLE_DELEGATED_SUBJECT=agent@scottylabs.org go run ./cmd/server
```

Deploy and registration follow the standard Path A method
([`../../docs/mcp-hosting-on-railway.md`](../../docs/mcp-hosting-on-railway.md)).
