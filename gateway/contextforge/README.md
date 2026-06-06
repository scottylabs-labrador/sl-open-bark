# ContextForge (adopted gateway)

Per design §6.6 ScottyLabs **adopts** an open-source MCP gateway rather than building one. The
choice is **IBM ContextForge** (the MCP Gateway / context-forge project). This directory holds its
deployment configuration; the ScottyLabs **policy layer** that wraps it lives one level up in
[`../`](../).

## What ContextForge provides

- **OAuth 2.1 + PKCE** for human callers and signed **service credentials** for the deployed agent.
- **MCP transport** (Streamable HTTP) and a single endpoint for the agent and members' Claude.
- **Downstream proxying** to registered MCP servers, with credential injection.

## What the ScottyLabs policy layer adds (`../`)

The committee-role model ContextForge does not express off the shelf: per-tool **scope + committee
visibility**, **HITL gating** on `impact: high` (a recorded approval must exist before the tool
runs), the **registry** driven by each server's `manifest.yaml`, and **audit** to Postgres (WP-01).
ContextForge authenticates the caller and forwards the verified subject + roles; the policy service
makes the authorization decision and records the audit.

## Deploy (human-gated)

> Creating the Railway project/services and configuring the OAuth identity provider are
> **human-gated** steps (see [`../../infra/README.md`](../../infra/README.md)). Do not commit real
> client secrets or tokens — config comes from the environment (Railway secrets).

Outline (a maintainer performs this once):

1. Deploy ContextForge as its own Railway service (its container image), on the private network.
2. Configure its OAuth 2.1 provider (issuer, client, PKCE) for human callers, and a service
   credential for the agent.
3. Point ContextForge at the ScottyLabs policy service (`../cmd/gateway`) for authorization +
   registry, and set its downstream targets from the registry.
4. Set secrets as Railway secrets: the OAuth client secret, `GATEWAY_SERVICE_TOKEN`,
   `MCP_DOWNSTREAM_TOKEN`, `DATABASE_URL`.

Pin the adopted version and track upstream; contribute ScottyLabs-specific policy hooks back rather
than forking (design §6.6, §14).

## TODO (when the Railway project exists)

- [ ] Add the ContextForge service definition / image pin here.
- [ ] Document the exact OAuth provider config used.
- [ ] Wire ContextForge → policy service authorization callback.
