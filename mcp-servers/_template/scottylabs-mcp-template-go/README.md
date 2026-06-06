# ScottyLabs MCP Server Template (Go)

A production-shaped starter for a ScottyLabs **MCP capability** in Go: a small service that exposes
admin actions for one system as MCP tools, deployable to Railway and usable by the agent and by
members' Claude clients through the shared MCP gateway.

Built on the official Go MCP SDK (`github.com/modelcontextprotocol/go-sdk`) with Streamable HTTP.
The example capability evaluates reimbursement requests against finance standards. Copy it, rename
the module, and replace the tools with your own.

## Why it is structured this way

A capability touches real systems, so it is built like a real service: layered, typed, and tested.
Logic lives in pure functions; side effects live at the edges. The layout is idiomatic Go
(`cmd/` for the binary, `internal/` for packages, interfaces defined at the consumer).

```
cmd/server/main.go        Composition root: read config, wire deps, register tools, serve HTTP.
internal/
  config/                 Typed settings from the environment (no magic globals).
  logging/                Structured slog logging + secret redaction.
  domain/                 Business rules as PURE FUNCTIONS + types. No I/O. Most logic lives here.
    rules_test.go         Table-driven unit tests (fast, no network).
  service/                Use-case orchestration. Defines the AuditSink interface it needs (DI).
    reimbursement_test.go Service test with a fake AuditSink.
  clients/                Concrete external clients (in-memory + HTTP audit). No import cycle.
  tools/                  THIN MCP handlers. Map inputs/outputs, call a service. No logic.
manifest.yaml             What the gateway registers: tools, scopes, impact, owner, committees.
Dockerfile                Multi-stage build -> tiny static binary on distroless (nonroot).
railway.json              Railway build + deploy config (health check at /healthz).
go.mod / go.sum           Module + pinned dependencies.
```

Principles: separation of concerns, dependency inversion (the service depends on an interface it
declares; `main` injects the concrete client), pure functions for decisions, side effects at the
edges, and a test for every rule. Go gives us a single static binary, fast cold start, and low
memory, which keeps an always-on Railway service cheap.

## Quickstart (local)

```bash
# Go 1.25+ (the SDK requires it; the toolchain auto-resolves via go.mod)
go mod download
go test ./...                 # fast unit tests, no network
go run ./cmd/server           # serves Streamable HTTP at http://localhost:8080/mcp
go vet ./... && gofmt -l .    # lint and format check (CI runs these)
```

Point a local Goose or your Claude Code/Desktop MCP config at `http://localhost:8080/mcp` to try
the tools.

## Build your own capability

1. Copy this repo and change the module path in `go.mod` (and the imports) to your server's name.
2. Replace `internal/domain` (types + pure-function rules) with your logic.
3. Wrap each external system (an API, a database) as a client in `internal/clients`, and declare the
   interface it satisfies in the consuming service (`internal/service`).
4. Put orchestration in a service struct with constructor injection.
5. Expose thin handlers in `internal/tools`. The tool `Description` is what the agent reads, so
   state what it does, its `scope`, and its `impact`.
6. Update `manifest.yaml`: every tool, its `scope`, and its `impact` (read | write | high). Mark
   anything irreversible as `impact: high` so the gateway requires human approval.
7. Add tests. Keep deterministic logic in `internal/domain` so it is trivial to test.

## Deploy to Railway

### Path A: official server in the platform monorepo (most governed)

1. Place the server at `mcp-servers/<name>/` in the `scottylabs-agent` monorepo and open a PR. CI
   runs `go vet`, `gofmt`, `go test`, builds the image, and validates the manifest; the owning
   committee and a platform maintainer review.
2. After merge, a maintainer creates the Railway service once: New Service from the monorepo, set
   **Root Directory** to `mcp-servers/<name>`, set a **Watch Path** to the same, add secrets.
3. The gateway registers the server from `manifest.yaml` as `proposed`; a maintainer promotes it to
   `approved` after a live check.

### Path B: community server from this template (lowest barrier)

1. Use this repo as a GitHub template ("Use this template").
2. Railway: New Project from your GitHub repo. Railway reads `railway.json` and the `Dockerfile`.
   Add secrets (`MCP_AUTH_TOKEN`, any others) and deploy. The server serves Streamable HTTP at
   `/mcp` with a `/healthz` check.
3. Register it with the gateway (PR to the registry or the dashboard) with the server's URL, a
   scoped token, and this `manifest.yaml`. It lands as `proposed` at a lower trust tier.
4. A maintainer reviews and promotes.

Either way, a server is only usable once registered, scoped, and approved, and every call through
the gateway is audited.

## Security checklist (must pass review)

- Default to read-only tools; mark every irreversible tool `impact: high`.
- No credentials in code. Secrets come from the environment (Railway secrets).
- Validate inputs (typed structs do this) and return clear tool errors, not internal details.
- Keep the server stateless and idempotent; durable state belongs in the platform's memory service.
- Redact secrets in logs (`logging.Redact`).

## A note on language

ScottyLabs writes its services in Go (single static binary, fast cold start, low memory, one
language across services). For these I/O-bound servers the language is not the latency bottleneck,
so this is a consistency and operability choice rather than a raw-speed one. See the design doc,
Section 7.8.
