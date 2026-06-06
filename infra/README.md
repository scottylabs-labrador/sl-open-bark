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

## Monorepo deploy method (Railway CLI)

Railway's `railway up` uploads the **git root** as the build context, so each monorepo service uses
a **repo-root-context Dockerfile** (paths prefixed with the service's subdir) selected via the
`RAILWAY_DOCKERFILE_PATH` service variable — no manual dashboard Root Directory step needed. Per
service:

```bash
railway link --project OpenBark --environment production    # once
railway add -d postgres                                     # once: managed Postgres
railway add -s <name>
railway variables --set "RAILWAY_DOCKERFILE_PATH=<subdir>/Dockerfile" -s <name>
railway variables --set 'DATABASE_URL=${{Postgres.DATABASE_URL}}' -s <name>   # services that need it
railway up -s <name> -c                                     # build + deploy from repo root
```

Seed the registry once the gateway + Postgres are up: `DATABASE_URL=<Postgres public URL> \
REPO_ROOT=. go run ./cmd/gateway sync` (registers every manifest as `proposed`).

### Live on OpenBark (production)

| Service | State |
|---|---|
| `Postgres` | provisioned; schema migrated; registry seeded (4 servers `proposed`) |
| `gateway` | live — policy API on `:8080` (service-token auth; ContextForge OAuth front is TODO) |
| `finance-rules` | live — `finance.rules/evaluate` |
| `memory` | live — Memory MCP (`memory.railway.internal`) |
| `google-workspace` | registered `proposed`, **not deployed** (needs the Google service-account key) |

Remaining to make Phase 0 fully live: deploy ContextForge (OAuth), provide the Google SA key + deploy
`google-workspace`, and promote servers `proposed → approved` (a maintainer action).
