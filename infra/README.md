# infra/

Railway project configuration, deploy scripts, and the CI validators.

**Owner:** platform-team.

## Contents

- [`validate/`](validate/) — `slvalidate`, the Go tool CI uses to check every MCP server
  `manifest.yaml` and every recipe against their JSON Schemas (`make validate`). Its own Go module.
- [`deploy.sh`](deploy.sh) — documents the human-gated Railway deploy path (`make deploy`).

## Service topology (design Section 11.1)

One Railway project; services talk over Railway's private network:

| Service | Role |
|---|---|
| `slack-gateway` | Bolt app: receives Slack events, acks, enqueues jobs (public HTTPS) |
| `runtime` | Goose headless: runs recipes, backed by Claude via OpenRouter |
| `mcp-gateway` | Adopted OSS gateway + ScottyLabs policy layer |
| `mcp-<name>` | One service per capability (Google, finance-rules, memory, ...) |
| `postgres` | Managed Postgres: memory, audit, approvals, registry metadata |
| `scheduler` | Railway cron (min 5-minute granularity, UTC) |

## Human-gated steps (do not invent values)

These require a human with the right accounts and are intentionally **not** automated here:

- Creating the Railway project, services, and per-service Root Directory / Watch Path / secrets.
- Any real secret: OpenRouter API key, Google service-account JSON + delegation scopes, Slack app
  tokens, the GitHub App and its private key, the sandbox provider key.
- Promoting an MCP server from `proposed` to `approved` in the gateway registry.

Config comes from the environment (Railway secrets); never commit a real `.env`, key, or token.
