# gateway/

The **capability bus**: the single governed endpoint that fronts every MCP server with auth,
per-tool and per-committee scope enforcement, human-in-the-loop gating on `impact: high`, discovery,
and full audit (design Section 6). It is what makes capabilities contributable *and* safe.

**Owner:** platform-team · **Built in:** WP-02 (depends on WP-01's repository).

## Adopt + a thin policy layer

Per design §6.6 we **adopt** an open-source gateway rather than build one, and wrap it with a thin
ScottyLabs policy layer only where our committee-role model needs something off-the-shelf does not
express. The adopted gateway is **IBM ContextForge**.

```
┌─────────┐   OAuth 2.1 / PKCE (humans)        ┌──────────────┐   committee-scope + HITL + audit
│ callers │ ───────────────────────────────▶  │ ContextForge │ ─────────────────────────────────┐
│ agent / │   service bearer (agent)           │  (adopted)   │                                   ▼
│ Claude  │                                    └──────────────┘                          ┌──────────────────┐
└─────────┘                                                                              │ ScottyLabs policy │
                                                                                         │  (this Go module) │
                                                                                         └────────┬─────────┘
                                                                                                  │ store (WP-01)
                                                                                                  ▼  Postgres: registry,
                                                                                            approvals, audit_log
```

- **ContextForge** provides OAuth 2.1 + PKCE for humans, signed service creds for the agent, MCP
  transport, downstream proxying, and credential injection. We do not fork it.
- **This module** is the policy layer ContextForge does not express off the shelf: it resolves a
  caller to committee roles, enforces per-tool scope + committee visibility, **gates `impact: high`
  behind a recorded human approval**, and audits every call — all backed by WP-01's `store`.

## The contract (depended on by the runtime and members' Claude)

- `register(manifest)` — load a server from its `manifest.yaml` into the registry (lands `proposed`).
- `list_tools(identity)` — only tools on **approved, enabled** servers granted to the caller's
  committees. A caller never even lists tools its role does not permit.
- `call(tool, args, identity)` — resolve → authorize → (if `impact: high`) require an **approved**
  approval, else record a pending one and return `approval_required` → proxy downstream → audit.

Exposed over HTTP by `cmd/gateway`: `GET /healthz`, `GET /tools`, `POST /call` (service bearer;
ContextForge fronts human OAuth and sets the verified `X-ScottyLabs-Subject` / `-Committees`).

## Layout

```
internal/policy/    the enforcement engine (Register / ListTools / Call) + audit + HITL + redaction
internal/manifest/  load mcp-servers/**/manifest.yaml into the registry (fail-closed)
internal/server/    thin HTTP handlers + bearer auth + identity-from-headers
internal/proxy/     downstream MCP-over-HTTP caller (injects the server's credential)
internal/config/    typed env config
cmd/gateway/        composition root: `gateway` (serve) and `gateway sync` (register manifests)
contextforge/       the adopted gateway's deployment config (human-gated)
```

## Run

```bash
export DATABASE_URL='postgres://…' GATEWAY_SERVICE_TOKEN='…' REPO_ROOT="$(git rev-parse --show-toplevel)"
go run ./cmd/gateway sync     # register every manifest into the registry (proposed)
go run ./cmd/gateway          # serve the policy API on :$PORT
go test ./...                 # policy/registry/server tests (no DB needed for these)
```

Promotion `proposed → approved` is a maintainer action (a `store` call / dashboard, WP-10).

## Safe defaults (fail closed)

Unknown tool → not callable. Unrecognized caller → nothing. A tool with no scope → rejected at
load. A high-impact action with no approval → does not execute. New servers land `proposed` and do
nothing live until promoted (design §10.3).

## Deploy (human-gated)

Creating the Railway services and the ContextForge OAuth configuration is human-gated. This module
ships a `Dockerfile` (build context = repo root, because of the cross-module `replace` to
`../services/session-memory`) and `railway.json`; ContextForge adoption notes are in
[`contextforge/`](contextforge/). Secrets come from the environment — never commit a real token.
