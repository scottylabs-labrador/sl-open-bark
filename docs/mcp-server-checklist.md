# MCP server checklist

The bar a new MCP **capability** PR must clear, plus the engineering standards behind it. A
capability is where new external power enters the platform, so it gets stricter review and explicit
scopes than a recipe. This page is the operational companion to design [Section 7.4][s74] (manifest
+ contribution flow), [Section 7.7][s77] (engineering standards), and [Appendix D][appd].

Read this alongside the repo guardrails in [`../CLAUDE.md`](../CLAUDE.md), the starter
[`../mcp-servers/_template/scottylabs-mcp-template-go/`][tmpl] (and [its README][tmplreadme]), the
[`../mcp-servers/manifest.schema.json`][schema] contract, and the hosting walkthrough in
[`./mcp-hosting-on-railway.md`](./mcp-hosting-on-railway.md).

## The PR checklist

Copy these checkboxes into your PR description and tick each one. The owning committee **and** a
platform maintainer both review (see [Appendix D][appd]).

- [ ] **Built from the template.** The server is copied from
      [`../mcp-servers/_template/scottylabs-mcp-template-go/`][tmpl] into `mcp-servers/<name>/`, with
      the module path renamed. It keeps the layered structure ([standards](#engineering-standards-section-77) below).
- [ ] **`manifest.yaml` declares every tool.** Each tool lists `name`, `scope`, and `impact`
      (`read | write | high`). A tool with no declared scope is rejected (fail closed).
- [ ] **Manifest validates.** `make validate` passes against
      [`../mcp-servers/manifest.schema.json`][schema] (CI runs this).
- [ ] **Default to read-only.** Tools are `impact: read` unless they must do more; `write` only for
      reversible side effects.
- [ ] **Every irreversible tool is `impact: high`.** Sends, deletes, bookings, and anything touching
      money are `high` so the gateway gates them behind human approval. The agent never moves money.
- [ ] **A test suite that runs in CI.** Fakes for all I/O via injected interfaces; **no live calls,
      no network** in CI. `go test ./...` is green.
- [ ] **No credentials in code.** Config and secrets come from the environment (Railway secrets);
      no real `.env`, key, or token is committed.
- [ ] **Stateless and idempotent.** No durable state in the server; persistent state lives in the
      platform memory service (Postgres), not in process.
- [ ] **A one-paragraph security note.** What the server touches, its failure modes, and why each
      scope is needed. (See [`./SECURITY.md`](./SECURITY.md).)
- [ ] **An owning committee + a platform maintainer.** Set `owner` in the manifest and list
      `allowed_committees`; both review the PR. CODEOWNERS routes it.
- [ ] **`gofmt` and `go vet` clean.** `make ci` mirrors the full pipeline (format check, vet, lint,
      tests, schema validation).

**Lifecycle.** A merged server lands as `lifecycle: proposed` and does nothing live until a platform
maintainer promotes it to `approved` after a live behavior check. Until then it is registered but
inert. See [hosting on Railway](./mcp-hosting-on-railway.md) for the deploy-and-register steps.

```bash
make validate   # validate every manifest.yaml against the schema
make test       # run all module test suites
make ci         # the full gate CI runs: fmt-check, vet, lint, test, validate
make help       # list available targets
```

## Engineering standards (Section 7.7)

A capability touches real systems, so it is built like a real service: layered, typed, and tested.
Logic lives in pure functions; side effects live at the edges. The template ships this structure —
follow it rather than reinventing it ([Section 7.7][s77], and the [template README][tmplreadme]).

The layered design, from the outside in:

- **Tools layer (thin handlers).** MCP handlers do input/output mapping only — **no business
  logic**. They validate inputs via typed structs (the SDK builds each tool's schema from them),
  call a service, and return a typed result. The tool `Description` is what the agent reads: state
  what it does, its `scope`, and its `impact`.
- **Service layer (orchestration).** A service struct orchestrates one use case. It **defines the
  interfaces it needs** and receives concrete collaborators by **dependency injection** (passed in
  by `main`, never constructed inline), so it is unit-testable with fakes.
- **Domain layer (pure functions).** Business rules are pure functions with **no I/O** —
  deterministic and trivially testable. Most logic lives here ("is this reimbursement over the
  category cap" is a pure function).
- **Clients layer (external systems behind interfaces).** Each external system (an API, a database)
  is a client that satisfies the interface its consuming service declared, so services depend on the
  abstraction and tests inject a fake. No import cycle.

Cross-cutting standards the template enforces:

- **Typed config from the environment.** A typed settings loader, no magic globals; secrets from
  Railway, not from code.
- **Structured logging with secrets redacted.** Use the template's `logging` helpers; never log a
  token or credential.
- **Clear tool errors.** Return actionable tool errors — never leak stack traces or internal
  details to the caller.
- **Small, single-responsibility functions** with descriptive names and short modules.
- **A test for every rule.** Fast unit tests for the pure domain logic, service tests with fake
  clients, and a transport smoke test. CI (`gofmt`, `go vet`, `go test ./...`) must be green before
  merge.

> ScottyLabs services are Go using the official Go MCP SDK; TypeScript is allowed only for a server
> that shares code with a TypeScript frontend ([`../CLAUDE.md`](../CLAUDE.md), [Section 7.8][s78]).

## Manifest shape

The registry contract. The gateway reads each server's `manifest.yaml` to register it, enforce
per-tool `scope` and `impact`, gate `impact: high` behind human approval, and decide which
committees may use it. The schema of record is [`../mcp-servers/manifest.schema.json`][schema];
`make validate` checks your manifest against it.

Fields:

| Field | Required | Notes |
|---|---|---|
| `name` | yes | Dotted server id, e.g. `events.booking`. |
| `owner` | yes | Committee + maintainer responsible, e.g. `events-committee`. |
| `description` | yes | One or two sentences on what the server does. |
| `endpoint` | yes | Where the gateway reaches it; **must end in `/mcp`**. Path A: `http://<name>.railway.internal:8080/mcp`; Path B: the public Railway URL. |
| `transport` | — | `streamable-http` (the only value; default). |
| `auth` | — | `bearer` (gateway presents `MCP_AUTH_TOKEN`) or `none`. Default `bearer`. |
| `tools[]` | yes | Each: `name`, `scope`, `impact`. A tool with no scope is rejected. |
| `allowed_committees` | — | Committees whose roles may use the server; intersected with per-tool scope. |
| `lifecycle` | — | `proposed` → `approved` (→ `deprecated` → `disabled`). New servers default to `proposed`. |

`impact` values: `read` = no external side effect; `write` = reversible side effect; `high` =
irreversible (send, delete, book, money) and **gated behind human approval at the gateway**.

Canonical example (mirrors the [schema][schema] and the [template manifest][tmpl]):

```yaml
# mcp-servers/events-booking/manifest.yaml
name: events.booking
owner: events-committee
description: Create and manage ScottyLabs event bookings.
endpoint: http://events-booking.railway.internal:8080/mcp
transport: streamable-http
auth: bearer
tools:
  - name: check_availability
    scope: events.read
    impact: read
  - name: create_booking_draft
    scope: events.write
    impact: write
  - name: submit_booking
    scope: events.write
    impact: high            # irreversible -> gateway requires human approval
allowed_committees: [events, leadership]
lifecycle: proposed         # promoted to approved after a live behavior check
```

## See also

- Repo guardrails: [`../CLAUDE.md`](../CLAUDE.md)
- Manifest schema: [`../mcp-servers/manifest.schema.json`][schema]
- Starter template: [`../mcp-servers/_template/scottylabs-mcp-template-go/`][tmpl] · [README][tmplreadme]
- Hosting + registration: [`./mcp-hosting-on-railway.md`](./mcp-hosting-on-railway.md)
- Security model and the security note: [`./SECURITY.md`](./SECURITY.md)
- Design doc: [`./ScottyLabs-Agent-Platform-Design.md`](./ScottyLabs-Agent-Platform-Design.md)

[s74]: ./ScottyLabs-Agent-Platform-Design.md#74-contributing-a-capability-mcp-server
[s77]: ./ScottyLabs-Agent-Platform-Design.md#77-software-engineering-standards-for-mcp-servers
[s78]: ./ScottyLabs-Agent-Platform-Design.md#78-language-choice-and-performance
[appd]: ./ScottyLabs-Agent-Platform-Design.md#appendix-d-mcp-server-contribution-checklist
[schema]: ../mcp-servers/manifest.schema.json
[tmpl]: ../mcp-servers/_template/scottylabs-mcp-template-go/
[tmplreadme]: ../mcp-servers/_template/scottylabs-mcp-template-go/README.md
