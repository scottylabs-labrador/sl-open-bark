# mcp-servers/ — CAPABILITIES (one folder per server)

A **capability** is a small MCP server that extends *what* the agent can touch: admin actions for
one system (read form responses, create a calendar event, check a design's contrast). A capability
is where new external power enters the system, so it gets stricter review and explicit scopes
(design Section 7.4, 7.6).

```
mcp-servers/
  _template/        scottylabs-mcp-template-go — copy this to start a new server
  finance-rules/    design-standards/   events-booking/   memory/   ...
  manifest.schema.json    # the contract every server's manifest.yaml is validated against
```

Each server is its **own Go module** so it deploys independently to its own Railway service
(Section 7.6, Path A/B). The `Makefile` and CI discover every `go.mod` automatically.

## Add a capability

1. Copy `_template/scottylabs-mcp-template-go/` to `mcp-servers/<name>/`. Implement your tools in
   the layered structure, wrap each external system as a client behind an interface, and keep
   business rules as pure functions in `internal/domain`.
2. Fill `manifest.yaml`: every tool's `name`, `scope`, and `impact` (`read | write | high`). Mark
   anything irreversible `impact: high` so the gateway requires human approval. Validated by
   [`manifest.schema.json`](manifest.schema.json) (`make validate`).
3. Add tests (fakes for I/O via injected interfaces; no live calls in CI) and a one-paragraph
   security note. Open a PR — the owning committee **and** a platform maintainer review.
4. On merge it deploys on Railway and registers behind the gateway as `proposed`; a maintainer
   promotes it to `approved` after a live behavior check.

See [`docs/mcp-server-checklist.md`](../docs/mcp-server-checklist.md) and
[`docs/mcp-hosting-on-railway.md`](../docs/mcp-hosting-on-railway.md). Default to read-only. No
credentials in code. Stateless and idempotent — durable state belongs in the memory service.
