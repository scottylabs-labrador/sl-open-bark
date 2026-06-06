# ScottyLabs Agent Platform

A shared AI teammate for ScottyLabs: reachable in **Slack**, acting through its own scottylabs.org
Google identity, extensible by members through **recipes** (workflows) and **MCP servers**
(capabilities), with per-user memory and an Engineering Agent that opens PRs from a sandbox. The
runtime is **Goose**; the model is served through **OpenRouter**; hosting is **Railway**.

- **Design:** [`docs/ScottyLabs-Agent-Platform-Design.md`](docs/ScottyLabs-Agent-Platform-Design.md)
- **Conventions & guardrails (read first):** [`CLAUDE.md`](CLAUDE.md)
- **Build plan (work packages):** [`BUILD-PLAYBOOK.md`](BUILD-PLAYBOOK.md)
- **Contributing:** [`docs/CONTRIBUTING.md`](docs/CONTRIBUTING.md) ·
  **Security:** [`docs/SECURITY.md`](docs/SECURITY.md)

## Architecture in one screen

Surfaces (Slack, members' Claude clients, an internal dashboard) reach an **orchestration** layer
(Slack Gateway, the Goose runtime behind an `AgentRuntime` interface, a Session/Memory service, a
Scheduler), which calls tools through one **MCP Gateway + Registry** that fronts many MCP servers
(Google, finance rules, memory, events, design, community). State lives in **Postgres** (memory,
audit, approvals, registry). The **Engineering Agent** is an isolated trust domain (GitHub App +
Railway Sandbox) that opens PRs only.

## Non-negotiable guardrails

1. No secrets in code — config comes from the environment (Railway secrets).
2. Least privilege — every tool declares `scope` + `impact`; default read-only.
3. Humans own irreversible actions; **the agent never moves money**.
4. MCP servers are stateless and idempotent; durable state goes through Postgres.
5. Untrusted input (form text, email, issues) is data, not instructions.
6. Everything is audited.

## Repository layout

```
runtime/        AgentRuntime interface + Goose config + .goosehints   (WP-06)
services/       slack-gateway (WP-07), session-memory (WP-05), engineering-agent (WP-11)
gateway/        adopted MCP gateway config + policy + registry          (WP-02)
recipes/        WORKFLOWS, one folder per committee + shared/           (recipe.schema.json)
mcp-servers/    CAPABILITIES, one folder per server + _template/        (manifest.schema.json)
skills/         portable cross-runtime skill folders
docs/           design doc, CONTRIBUTING, SECURITY, recipe-spec, checklists
infra/          Railway config, deploy scripts, CI validators (slvalidate)
.github/        CODEOWNERS + CI workflows
```

This is a **multi-module** Go monorepo — each server/service is its own Go module so it deploys
independently. Tooling discovers every `go.mod` automatically.

## Quickstart

Requires Go 1.25+ (the Go MCP SDK needs it). `golangci-lint` v2 is optional locally (CI runs it).

```bash
make ci          # fmt-check + vet + lint + test + validate — mirrors CI
make test        # go test every module
make validate    # validate manifests + recipes against their schemas
make help        # list all targets and discovered modules
```

## Extending the platform

- **Add a workflow:** drop a recipe in `recipes/<committee>/`, PR, merge. No new access.
  See [`docs/recipe-spec.md`](docs/recipe-spec.md).
- **Add a capability:** copy `mcp-servers/_template/`, implement tools, declare scopes/impact in
  `manifest.yaml`, PR (committee + maintainer review).
  See [`docs/mcp-server-checklist.md`](docs/mcp-server-checklist.md) and
  [`docs/mcp-hosting-on-railway.md`](docs/mcp-hosting-on-railway.md).
