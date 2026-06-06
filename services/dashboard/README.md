# services/dashboard/

The **maintainer dashboard + agent console** (design §6.3, §10.2, §11.4): a single Go service that
serves read views over the platform's Postgres state *and* a console that drives the agent runtime.

**Owner:** platform-team · **Built in:** WP-10.

A mission-control UI (amber-CRT-on-ink, monospace telemetry), compiled into the binary via
`go:embed` — no build step, single static binary, distroless image.

## What it does

- **Overview** — live telemetry: servers/tools, high-impact count, pending approvals, 24h activity,
  error/deny rate, avg latency; a fail-closed system-status panel; a recent-activity stream.
- **Registry** — every MCP server with its tools, scopes, impact, lifecycle, and committees.
  Expand a server; **promote `proposed → approved`**; enable/disable (the gateway kill switch).
- **Approvals** — pending high-impact actions, with **Approve / Deny** (writes the decision via the
  store, design §10.2).
- **Audit** — the live, color-coded audit log (actor · tool · result · latency).
- **Console** — submit a task (inline goal or recipe) to the runtime, **watch the agent's event
  stream** type out, and **approve high-impact actions in-line**.

## Layout

```
internal/server/        HTTP API + auth + embedded UI (server.go, api.go, web/)
internal/server/web/    index.html · app.css · app.js (the UI, embedded)
internal/runtimeclient/ thin client for the runtime task API (the console)
cmd/dashboard/          composition root
```

## Auth (maintainer-gated, design §10.2)

`DASHBOARD_TOKEN` gates the API; the UI shows a login. An **empty token runs in OPEN dev mode** (no
auth) for local development. Reads/writes go through the WP-01 store; the runtime is reached over the
private network via `RUNTIME_URL`.

## Run

```bash
export DATABASE_URL='postgres://…' RUNTIME_URL='http://runtime.railway.internal:8080'
go test ./...
go run ./cmd/dashboard          # serves the UI + API on :$PORT (open dev mode if no DASHBOARD_TOKEN)
```

Config from the environment (Railway secrets) — no secrets in code.
