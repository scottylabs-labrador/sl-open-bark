# Hosting MCP Servers on Railway

The repeatable method for standing up a new capability on Railway and having the gateway use it.
This is the operational companion to the design doc's
[Section 7.6](./ScottyLabs-Agent-Platform-Design.md#76-hosting-mcp-servers-on-railway-the-method);
read that for the rationale, this for the steps.

There are two hosting paths, chosen by trust level. Both converge at the gateway: a server is only
usable once it is **registered, scoped, and approved**, and every call through it is audited. If a
server misbehaves, the gateway disables it in one place.

Before you start, work the [MCP server checklist](./mcp-server-checklist.md), copy the
[Go template](../mcp-servers/_template/scottylabs-mcp-template-go/) (see its
[README](../mcp-servers/_template/scottylabs-mcp-template-go/README.md)), and read the
[capabilities overview](../mcp-servers/README.md). The non-negotiable guardrails in
[`CLAUDE.md`](../CLAUDE.md) are binding.

> **Human-gated, by design.** Creating the Railway project and services, setting each service's
> Root Directory / Watch Path, adding real secrets, and promoting a server to `approved` require a
> human with the right accounts. They are intentionally not automated. See
> [`infra/README.md`](../infra/README.md).

## Transport and contract (both paths)

Every server speaks the same contract, so the gateway can front it without special-casing:

- **MCP over Streamable HTTP** at a single `/mcp` endpoint (the March 2025 MCP transport). This is
  what makes remote hosting behind Railway and the gateway work cleanly.
- **A `/healthz` check** that Railway and the registry poll, so the gateway can route around an
  unhealthy capability.
- **Stateless and idempotent.** A tool call is a pure function of its inputs plus the system it
  fronts. Durable state belongs in Postgres via the memory service, never inside the server.
- **Bearer auth.** The gateway presents `MCP_AUTH_TOKEN`; the server rejects anything else.
- **No secrets in code.** Config comes from the environment (Railway secrets).

The template ships all of this: `railway.json` declares the build, the start, the `/healthz` check,
and an `ON_FAILURE` restart policy; the `Dockerfile` produces a tiny static binary on a distroless
base; `manifest.yaml` is what the gateway registers.

## Path A: official server in the monorepo

For capabilities the org owns (Google, finance rules, events, memory), the server lives at
`mcp-servers/<name>/` in this monorepo and runs as **its own Railway service** inside the ScottyLabs
Railway project, reached by the gateway over Railway's private network. This is the most governed
path.

1. **Scaffold.** Copy `mcp-servers/_template/scottylabs-mcp-template-go/` to `mcp-servers/<name>/`.
   Implement your tools in the layered structure (pure rules in `internal/domain`, orchestration in
   `internal/service`, external systems behind interfaces in `internal/clients`, thin handlers in
   `internal/tools`), fill in `manifest.yaml` (tool names, `scope`, `impact`, `owner`,
   `allowed_committees`), and write tests. Default to read-only; mark anything irreversible
   `impact: high` so the gateway requires human approval.
2. **Config.** Keep the `railway.json` from the template at the server's root: Streamable HTTP on
   `$PORT`, the `/healthz` check, and the restart policy.
3. **PR.** Open a pull request. CI builds the image, runs `go vet` / `gofmt` / `go test`, validates
   `manifest.yaml` against [`manifest.schema.json`](../mcp-servers/manifest.schema.json), and checks
   scopes (`make ci`). Review requires **both** the owning committee and a platform maintainer,
   because a capability grants new power.
4. **One manual service step (maintainer, after merge).** A maintainer creates the Railway service
   **once**: point a new service at the monorepo, set its **Root Directory** to `mcp-servers/<name>`,
   set a matching **Watch Path**, and add the service's secrets. See *Why the manual step* below.
5. **Register and promote.** The gateway registers the server from its `manifest.yaml` in the
   `proposed` state. A maintainer promotes it to `approved` after a live check. It is now
   private-networked, governed, audited, and usable by the agent and by members' Claude clients per
   their roles.

After step 4, every push that touches `mcp-servers/<name>/**` auto-deploys **only that service**;
nothing else rebuilds.

## Path B: community server from the template repo

For experimental or member-owned capabilities, lower the barrier with the standalone
`scottylabs-mcp-template-go` template repo, which deploys cleanly from the repo root (no monorepo
Root-Directory step), then registers with the gateway by URL and token. It lands at a lower trust
tier.

1. **Use the template.** Click **"Use this template"** to create your repo. Implement tools, fill
   `manifest.yaml`, write tests. The template enforces the structure and standards.
2. **Deploy to Railway.** **New Project from your GitHub repo** — Railway reads `railway.json` and
   the `Dockerfile`. Add secrets (`MCP_AUTH_TOKEN`, any others) and deploy. The server comes up
   serving Streamable HTTP at `/mcp` behind a bearer token, with the `/healthz` check.
3. **Register it.** Submit a short PR to the platform repo's registry (or use the dashboard) with the
   server's public Railway URL, a scoped token, and its `manifest.yaml`. It lands as `proposed`,
   restricted to its owning committee, at a lower trust tier.
4. **Promote.** A maintainer reviews and promotes. The gateway now proxies it with the same auth,
   scoping, and audit as any official server; if it misbehaves, the gateway disables it in one place.

## Why the manual per-service step exists

Railway currently sets a service's **Root Directory** and config-file path through the dashboard or
API, not config-as-code. There is no committed file that wires a monorepo subdirectory to a service,
so a human does it once per server (Path A, step 4). It is **one-time per server**: once the Root
Directory and Watch Path are set, deploys are fully automatic — a push that touches that folder
deploys that service and nothing else. Path B sidesteps this entirely by deploying from the repo
root, which is why its barrier is lower.

## Choosing a path

| | Path A (official) | Path B (community) |
|---|---|---|
| Where it lives | `mcp-servers/<name>/` in the monorepo | Its own repo from the template |
| Railway deploy | A service in the ScottyLabs project, Root Directory set once | New Project from the repo root |
| Reached by the gateway | Railway private network | Public Railway URL + scoped token |
| Review | Owning committee **and** a platform maintainer | Maintainer review at registration |
| Trust tier | Highest | Lower (committee-scoped) |
| Lands as | `proposed` → `approved` after a live check | `proposed` → `approved` after a live check |

When in doubt, prefer Path A for anything the org will rely on; use Path B to prototype or for a
member-owned experiment.

## Both paths converge at the gateway

No matter where a server runs, it does nothing live until a maintainer promotes it from `proposed`
to `approved`, and the platform fails closed: an unknown or unapproved tool is not callable. Once
approved, the gateway enforces the server's per-tool scopes and `impact`, gates every `impact: high`
tool behind human approval, injects the downstream credentials, and logs every call (actor, tool,
redacted args, result, timing). This is what lets the org say yes to community contributions without
losing control.

## See also

- [Go template](../mcp-servers/_template/scottylabs-mcp-template-go/) and its
  [README](../mcp-servers/_template/scottylabs-mcp-template-go/README.md)
- [MCP server checklist](./mcp-server-checklist.md) — what a capability PR must include
- [Capabilities overview](../mcp-servers/README.md) and
  [`manifest.schema.json`](../mcp-servers/manifest.schema.json)
- [`infra/README.md`](../infra/README.md) — service topology and the human-gated steps
- [Design doc §7.6, §11.1, §11.3](./ScottyLabs-Agent-Platform-Design.md) — hosting method, topology,
  CI/CD
- [`CLAUDE.md`](../CLAUDE.md) — guardrails · [`BUILD-PLAYBOOK.md`](../BUILD-PLAYBOOK.md) — work packages
