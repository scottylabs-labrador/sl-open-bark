# Memory MCP (registry entry)

This directory holds the **registry manifest** for the Memory MCP. The **implementation** lives in
[`../../services/session-memory/`](../../services/session-memory/) — specifically
`cmd/memory-mcp/` — because the Memory MCP shares the WP-01 Postgres `store` that owns durable state
(design §4.4). Keeping the code there avoids duplicating the data layer; the manifest lives here so
the server registers and is governed like every other capability.

Tools: `write_fact` (write), `search` (read, scoped), `forget` (write). Built in WP-05.
