# Contributing to the ScottyLabs Agent Platform

Welcome. This is the contributor's entry point. The platform is built to grow at its edges: you
extend it through pull requests, not by rewriting the core. Before you start, read the repo
guardrails in [`../CLAUDE.md`](../CLAUDE.md) — they are binding — and the security model in
[`./SECURITY.md`](./SECURITY.md). The full rationale lives in the
[design doc](./ScottyLabs-Agent-Platform-Design.md); this guide tells you what to do.

## Two contribution types — and only two

There are exactly two things you contribute, and they are deliberately different (design Section 7.1).

| | **Workflow** (recipe / skill) | **Capability** (MCP server) |
|---|---|---|
| Teaches the agent… | *how* to do a committee task | *what* it can touch |
| Grants new access? | **No** — composes tools that already exist behind the gateway | **Yes** — new external power enters the system here |
| What it is | a YAML file: instructions, the tools it needs, parameters | a small Go service exposing admin actions for one system |
| Review | light: committee owner | strict: owning committee **and** a platform maintainer |
| Effort | ship in an afternoon | a real service with tests and a security note |
| Lives in | `recipes/<committee>/` | `mcp-servers/<name>/` |

Keeping these separate is the whole point: a non-infrastructure member can safely contribute a
workflow in an afternoon, while changes that grant new power get the scrutiny they deserve. When in
doubt, prefer a workflow — most committee processes are workflows over capabilities that already
exist.

## Add a workflow (recipe)

A recipe is data, not code. It names itself, declares the gateway-fronted capabilities it needs,
declares its parameters, and gives the agent its instructions. It can call subrecipes (in
`recipes/shared/`) for reusable patterns like "draft, then confirm with a human, then act."

1. **Fork or branch.** Keep the PR to one recipe.
2. **Write `recipes/<committee>/<name>.yaml`** per the [recipe spec](./recipe-spec.md) (machine
   schema: [`../recipes/recipe.schema.json`](../recipes/recipe.schema.json); overview:
   [`../recipes/README.md`](../recipes/README.md)). Every capability you list under `extensions`
   must already exist in the registry and be permitted to your committee. Write instructions to
   **extract specific fields** from untrusted input, not to "do what this message says."
3. **List anything irreversible** under `response.require_human_approval_for` (for example
   `google.gmail.send`). The gateway gates it regardless of model output.
4. **Test locally** against the gateway with your own Claude Code or a local Goose, using a sandbox
   dataset.
5. **Open a PR.** CI lints the recipe schema and checks every declared capability exists and is
   allowed for your committee (`make validate`).
6. **A committee owner reviews and merges.** On merge the recipe is live for the deployed agent —
   **no runtime redeploy**, because recipes are read from the repo.

## Add a capability (MCP server)

A capability is a real service that touches real systems, so it is held to real engineering
standards (design Section 7.7). Start from the template — do not write a server from scratch.

1. **Copy the template:**
   [`../mcp-servers/_template/scottylabs-mcp-template-go/`](../mcp-servers/_template/scottylabs-mcp-template-go/)
   to `mcp-servers/<name>/` (its [README](../mcp-servers/_template/scottylabs-mcp-template-go/README.md)
   walks the layout). Change the module path in `go.mod`.
2. **Implement layered tools.** Business rules are pure functions in `internal/domain` (no I/O);
   orchestration is a service in `internal/service` that depends on injected interfaces; each
   external system is a client in `internal/clients` behind an interface; `internal/tools` are thin
   handlers that map inputs/outputs and call a service. Logic in pure functions; side effects at the
   edges.
3. **Fill `manifest.yaml`.** Every tool gets a `name`, a `scope`, and an `impact`
   (`read | write | high`) — validated against
   [`../mcp-servers/manifest.schema.json`](../mcp-servers/manifest.schema.json). **Default to
   read-only.** Mark anything irreversible (send, delete, book, money) `impact: high` so the gateway
   requires human approval. A tool with no scope is rejected (fail closed).
4. **Tests + a one-paragraph security note.** Unit-test the pure domain logic; test services with
   fake clients; no live calls in CI. The security note states what the server touches, the scopes
   it needs and why, and its failure modes.
5. **Open a PR.** CI builds the image, runs tests, and validates the manifest. Review requires the
   **owning committee and a platform maintainer**, because this grants new power.
6. **It deploys and registers as `proposed`.** On merge the server deploys on Railway and registers
   behind the gateway in `proposed` state, where it does nothing live. A maintainer promotes it to
   `approved` after a live behavior check.

Hosting details (Path A official monorepo server vs. Path B community template repo), the Railway
steps, and the registry handoff are in [`./mcp-hosting-on-railway.md`](./mcp-hosting-on-railway.md).
Run the full pre-submit list in [`./mcp-server-checklist.md`](./mcp-server-checklist.md). Servers
overview: [`../mcp-servers/README.md`](../mcp-servers/README.md).

## Local dev loop

This is a multi-module Go monorepo; tooling discovers every `go.mod` automatically, so adding a
module needs no tooling changes.

1. **Clone** the repo. You need **Go 1.25+** (the Go MCP SDK requires it).
2. **Make small, reviewable PRs:** one work package, one capability, or one recipe each.
3. **Run `make ci` before opening a PR.** It mirrors CI exactly:

   ```bash
   make ci        # fmt-check + vet + lint + test + validate
   make test      # go test every module
   make validate  # validate manifests + recipes against their schemas
   make lint      # golangci-lint (v2; optional locally, CI runs it)
   make help      # list all targets and discovered modules
   ```

4. **Fill in the PR template.** It carries the **Definition of Done** from
   [`../CLAUDE.md`](../CLAUDE.md): the change matches the design doc and its work package; new MCP
   tools declare `scope` + `impact` and irreversible tools are gated; tests are added and green
   (`gofmt`, `go vet`, `go test ./...` clean); no secrets committed; and for a capability, a
   one-paragraph security note. CI must be green before merge.

## Ownership and lifecycle

**Per-committee ownership.** `.github/CODEOWNERS` makes each committee responsible for its own
recipes and servers, which is how the catalog stays honest as members rotate (design Section 7.5).
Committees own `recipes/<committee>/` and their `mcp-servers/<name>/`; platform maintainers own the
core seams (`runtime/`, `gateway/`, `services/`, schemas, `_template/`).

**Versioning is git.** Recipes and servers are versioned in the repo; the registry tracks the live
version of each server.

**Deprecation is explicit.** A server moves through a lifecycle: `proposed` → `approved` →
`deprecated`. When deprecated, the gateway warns callers and the server is removed after a grace
period. Nothing disappears silently.

**The four extension points** (design Section 7.9), each with an owner and a review path:

- **Add a workflow** — drop a recipe in `recipes/<committee>/`, PR, merge. No new access. Lightest.
- **Add a capability** — a new MCP server (Path A or B). Grants new power; stricter review.
- **Add a committee** — a new `recipes/<committee>/` area, a CODEOWNERS entry, and a role mapping at
  the gateway. The platform was multi-committee from day one (design Section 8.3).
- **Add a surface or a model** — surfaces talk to the `AgentRuntime` interface and the model sits
  behind the OpenAI-compatible boundary, so each is a contained change.

## Where things live

| Path | Holds |
|---|---|
| `runtime/` | `AgentRuntime` interface, Goose config, `.goosehints` |
| `services/` | `slack-gateway`, `session-memory`, `engineering-agent` |
| `gateway/` | adopted MCP gateway config, policy, registry |
| `recipes/<committee>/` | **workflows**; `recipes/shared/` reusable subrecipes |
| `mcp-servers/<name>/` | **capabilities**; `mcp-servers/_template/` the starter |
| `skills/` | portable cross-runtime skill folders |
| `docs/` | this guide, [SECURITY](./SECURITY.md), [recipe-spec](./recipe-spec.md), the [checklist](./mcp-server-checklist.md), [Railway hosting](./mcp-hosting-on-railway.md) |
| `infra/` | Railway config, deploy scripts, CI validators ([`../infra/README.md`](../infra/README.md)) |

## Your first contribution

1. Read [`../CLAUDE.md`](../CLAUDE.md) and [`./SECURITY.md`](./SECURITY.md).
2. Decide: is this a **workflow** (procedure over existing tools) or a **capability** (new system to
   touch)? Prefer a workflow.
3. For a workflow, follow [the recipe spec](./recipe-spec.md); for a capability, copy
   [the template](../mcp-servers/_template/scottylabs-mcp-template-go/) and follow
   [the checklist](./mcp-server-checklist.md).
4. Run `make ci`, open one small PR, and fill in the Definition of Done.

## The guardrails, firmly

These are non-negotiable (design Section 2.3, Section 10; [`../CLAUDE.md`](../CLAUDE.md)):

- **Least privilege.** Every tool declares a `scope` and an `impact`. Default to read-only; justify
  anything more. A recipe can only use what its committee is allowed to use.
- **No secrets in code.** Config comes from the environment (Railway secrets). Never commit a real
  `.env`, key, or token.
- **Humans own irreversible actions.** Sending at scale, deletes, bookings, and anything touching
  money are gated behind a human at the gateway. **The agent never moves money. Ever.**
- **Untrusted input is data, not instructions.** Form text, emails, and issues are data; high-impact
  actions are gated regardless of model output.
- **Stateless and idempotent servers.** Durable state lives in Postgres via the memory service, not
  inside a server.
