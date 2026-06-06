# Security

The security posture of the ScottyLabs Agent Platform, as a living document. It expands the design
doc's [Section 10](./ScottyLabs-Agent-Platform-Design.md#10-security-safety-and-guardrails) into the
operational specifics maintainers need: the threat model, the controls that answer each threat, the
platform's safe defaults, a secret-rotation runbook, and how to report a vulnerability.

This document never contradicts the six guardrails in [`CLAUDE.md`](../CLAUDE.md); it implements
them. The most load-bearing one, in one line: **the agent never moves money, ever.**

Security hardening is continuous work. [`BUILD-PLAYBOOK.md`](../BUILD-PLAYBOOK.md) tracks it as
**WP-12**, which hardens and expands the controls and the runbook below.

## Threat model

The realistic risks for a shared, Slack-reachable agent acting under a real Google identity
([design 10.1](./ScottyLabs-Agent-Platform-Design.md#101-threat-model)):

- **Prompt injection** — a form response, email, or GitHub issue tries to redirect the agent into
  actions the requester never asked for.
- **Over-permissioned tools** — a capability that can do more than its workflow needs, widening the
  blast radius of any single mistake or compromise.
- **Data exposure** — student PII in forms and mail leaks through logs, storage, or an
  over-broad tool.
- **Irreversible actions in error** — the wrong return email sent, a delete, a booking, or
  anything touching money executed on a bad parse.
- **Credential leakage** — a service-account key, token, or API key ends up in code, a recipe, a
  log line, or a contributor's machine.
- **Cost runaway** — a looping or abused agent burns tokens and money without a ceiling.

## Controls

Each control maps to one or more threats above and is described in
[design 10.2](./ScottyLabs-Agent-Platform-Design.md#102-controls).

### Human-in-the-loop on irreversible actions
*(answers: irreversible actions in error)*

Sending mail at scale, deletes, submitting bookings, and anything touching money require an explicit
human approval recorded at the gateway. The gateway enforces this from a tool's declared
`impact: high`, so a recipe or a prompt **cannot** bypass it. Approval state lives in Postgres, not
in the model's context. The agent never moves money under any circumstance — this is guardrail 3 and
it is not configurable.

### Least privilege and per-tool / per-committee scoping
*(answers: over-permissioned tools, prompt injection, data exposure)*

Every tool declares a `scope` and an `impact` in its `manifest.yaml`
([schema](../mcp-servers/manifest.schema.json), [servers overview](../mcp-servers/README.md)), and
defaults to read-only. The gateway exposes to a caller only the tools their role permits. A
deployed agent's **effective tools are the intersection of the recipe's declared capabilities and
the requesting committee's allowed scopes** ([design 8.3](./ScottyLabs-Agent-Platform-Design.md#83-per-committee-access-and-rbac)),
so a finance task literally cannot see events write tools, even if prompted to. Tools a caller
cannot see cannot be invoked or reasoned about, which also shrinks the injection surface.

### Prompt-injection containment
*(answers: prompt injection)*

Untrusted content — form text, inbound email, GitHub issues — is **data, not instructions**
(guardrail 5). Three layers contain it:

- Recipes extract specific named fields rather than "do what this message says."
- Deterministic checks live in code (for example the Finance Rules MCP's `finance.rules.evaluate`),
  not in the model's discretion.
- High-impact actions are gated at the gateway **regardless of model output** — no phrasing, by the
  agent or by a malicious input, can promote an action past its `impact` gate.

### Data minimization and PII / FERPA handling
*(answers: data exposure)*

The platform stores **metadata and audit records, not bulk copies** of Slack or Google data. It
pulls data at use time and keeps only what an audit needs. Sensitive fields are redacted in logs,
and access to PII-bearing tools is role-scoped. For a US university student org, treat student
records as potentially **FERPA-relevant**: avoid storing them beyond what a task requires, and when
in doubt, ask a human and log less.

### Auditability
*(answers: all threats — it is how we detect and reconstruct them)*

Every gateway call and every approval is logged with **actor, tool, redacted arguments, result
status, and timing** (guardrail 6), queryable from the internal dashboard. Side effects are routed
through the gateway so they cannot escape the audit trail. This is both a security control and the
basis for trust with leadership.

### Cost and rate controls
*(answers: cost runaway)*

Per-committee and global rate limits at the gateway, a hard ceiling on concurrent sessions (Goose
already caps parallel workers), per-task token budgets, and a monthly spend alarm on the OpenRouter
account. A runaway loop hits a wall quickly. WP-12 owns the budgets and the spend alarm.

### Secrets hygiene
*(answers: credential leakage)*

Credentials live **only in Railway secrets** and are injected as environment variables; downstream
tokens are held only by the gateway and the one MCP server that needs them (guardrail 1,
[design 8.2](./ScottyLabs-Agent-Platform-Design.md#82-secrets-and-credentials)). No credential is
ever placed in a recipe ([recipe schema](../recipes/recipe.schema.json)), a client config, or a
contributor's machine. CI and code review reject anything that looks like a committed secret. See
the [rotation runbook](#secret-rotation-runbook) below.

## Safe defaults

The platform **fails closed** ([design 10.3](./ScottyLabs-Agent-Platform-Design.md#103-safe-defaults)).
When the answer is uncertain, the answer is "no":

- An **unknown tool is not callable**.
- An **unrecognized caller gets nothing** — no tools, no data.
- A **capability with no declared scope is rejected by CI** (`make validate`).
- A **high-impact action with no recorded approval does not execute**.
- **New capabilities land in `proposed`** and do nothing live until a maintainer promotes them.

You can confirm these locally before opening a PR:

```bash
make validate   # manifests + recipes against their schemas; rejects missing scopes
make ci         # fmt-check + vet + lint + test + validate — mirrors CI
```

## Secret-rotation runbook

A concrete starter. WP-12 hardens and expands this; until then, follow it manually and record each
rotation in the audit trail. The platform holds these secrets, all stored in Railway secrets only:

| Secret | Held by | Rotate at |
|---|---|---|
| OpenRouter API key | runtime | OpenRouter dashboard |
| Google service-account JSON key + delegation scopes | mcp-google (gateway-held downstream) | Google Cloud Console / Workspace admin |
| Slack app tokens (bot, signing secret, app-level) | slack-gateway | Slack app config |
| GitHub App private key | engineering-agent (isolated trust domain) | GitHub App settings |
| Sandbox provider key | engineering-agent | Railway Sandbox / provider console |
| `MCP_AUTH_TOKEN` (per MCP server) | each MCP server + gateway | generate a new token |
| Postgres credentials | every service that connects | Railway managed Postgres |

The delegation scopes are reviewed whenever a new Google capability is added
([design 8.1](./ScottyLabs-Agent-Platform-Design.md#81-the-agents-own-google-identity)).

### Rotation steps

For any secret above, rotate one secret at a time and verify before moving on:

1. **Rotate in the provider.** Generate the new value (or new key pair) in the provider's console.
   Where the provider supports overlapping validity, keep the old value live until step 4 succeeds.
2. **Update the Railway secret.** Set the new value on the affected service in Railway. Never edit a
   recipe, code, or a local `.env` — secrets live only in Railway.
3. **Redeploy / restart the affected service** so it picks up the new environment variable. Restart
   only the services that hold that secret (see the table); avoid a platform-wide restart.
4. **Verify.** Exercise a real call that uses the credential and confirm success in the
   [audit trail](#auditability) — for example a Google read for the service-account key, a Slack ack
   for the Slack tokens, a gateway tool call for an `MCP_AUTH_TOKEN`. Watch for auth errors.
5. **Revoke the old value** in the provider once the new value is confirmed working.

If a secret is suspected leaked, treat it as an incident: revoke first (step 5 before the overlap
window closes), then rotate, accepting the brief outage. Capture what leaked and where in the audit
trail, and check the logs for any use of the leaked value.

## Reporting a vulnerability

We practice responsible disclosure. If you find a security issue in the platform:

- **Email the platform team at security@scottylabs.org.**
  > Placeholder — a human maintainer must confirm this alias exists and routes to the platform team
  > before relying on it.
- **Do not open a public GitHub issue or PR** for anything sensitive (credential exposure, PII
  leakage, an injection that bypasses a high-impact gate, a way to make the agent move money).
  Public issues are fine for non-sensitive hardening suggestions.
- Include: what you found, the affected component, reproduction steps, and the impact you observed.
  Do not include live secrets or real student PII in your report — redact or reference them.

We will acknowledge the report, work a fix, and credit you if you wish. Please give us a reasonable
window to remediate before any public disclosure.

## See also

- Guardrails (binding): [`../CLAUDE.md`](../CLAUDE.md)
- Design doc, full security section: [`./ScottyLabs-Agent-Platform-Design.md`](./ScottyLabs-Agent-Platform-Design.md#10-security-safety-and-guardrails)
- Hardening work package: [`../BUILD-PLAYBOOK.md`](../BUILD-PLAYBOOK.md) (WP-12)
- Contributing: [`./CONTRIBUTING.md`](./CONTRIBUTING.md) ·
  Recipes: [`./recipe-spec.md`](./recipe-spec.md) ·
  MCP servers: [`./mcp-server-checklist.md`](./mcp-server-checklist.md) ·
  Hosting: [`./mcp-hosting-on-railway.md`](./mcp-hosting-on-railway.md)
- Schemas: [recipe](../recipes/recipe.schema.json) · [manifest](../mcp-servers/manifest.schema.json)
- Infra (Railway config, validators): [`../infra/README.md`](../infra/README.md)
