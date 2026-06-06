# Security checklist

The platform's safety invariants, with what enforces each and how it's verified. Most are checked
**automatically in CI** (`make checklist` → `slvalidate checklist`); the rest are reviewed per PR.
"The checklist passes" means the automated check is green and the manual items below are satisfied.

## Automated (CI: `make checklist`)

- [x] **Every tool declares a scope.** A tool with no scope is rejected (fail closed). — manifest
      schema + checklist.
- [x] **Every tool declares a valid impact** (`read | write | high`). — manifest schema + checklist.
- [x] **Irreversible tools are `impact: high`.** A tool whose name implies an irreversible action
      (`send`, `delete`, `submit`, `book`, `pay`, `transfer`, …) must be `high`, so the gateway gates
      it. — checklist heuristic.
- [x] **No committed secrets.** The tree is scanned for real OpenRouter / Slack / GitHub / AWS
      credentials and private keys (placeholders in `*.example` are ignored). — checklist secret scan.

## Enforced in code (with tests)

- [x] **HITL on high-impact actions.** The gateway requires a recorded, approved approval before an
      `impact: high` tool runs; a recipe or prompt cannot bypass it. — `gateway` policy tests.
- [x] **Fail-closed defaults.** Unknown tool → not callable; unrecognized caller → nothing; a
      proposed/disabled server → invisible; high-impact with no approval → does not execute. —
      `gateway`/`store` tests.
- [x] **Least privilege / scoping.** A caller sees and calls only tools its committee permits;
      effective tools = intersection of recipe and committee scope. — `gateway` registry tests.
- [x] **Rate limits.** Per-committee and global token-bucket rate limiting at the gateway; a runaway
      or abused caller is denied before any work (`ErrRateLimited`). — `gateway/internal/limits` +
      policy tests. Configure with `GATEWAY_RATE_COMMITTEE_PER_MIN` / `GATEWAY_RATE_GLOBAL_PER_MIN`.
- [x] **Everything audited.** Every gateway call and decision is written to `audit_log` with redacted
      args. — `gateway`/`store` tests.
- [x] **Secrets from the environment only.** No credential in code, recipes, or client configs;
      rotation runbook in [`SECURITY.md`](SECURITY.md).

## Manual review (per PR)

- [ ] **The agent never moves money**, and no new tool enables it (guardrail 3).
- [ ] **Egress allowlist** for the Engineering Agent sandbox (package registries + GitHub API only;
      block metadata `169.254.169.254` and RFC1918). — WP-11.
- [ ] **PII / FERPA.** No student records stored beyond what a task needs; logs redacted.
- [ ] **New downstream credential** is held only by the gateway and the one MCP server that needs it.

## Continuous hardening (tracked)

- **Per-task token budgets** and an **OpenRouter spend alarm** — the gateway rate limiter is the
  cost-control foundation; token accounting attaches once the runtime surfaces model token usage from
  Goose. Until then, the spend signal comes from the OpenRouter key's usage/limit endpoint.
- Structured logs flow to Railway and the audit tables from every service (`slog` JSON).

Run locally: `make checklist` (or `make ci`, which includes it).
