# recipes/finance/

The **v1 reference workflow** (design Section 9): screen reimbursement requests from Slack, draft
returns for clear failures, route edge cases to a human, and **send only after human approval**.

## Files

- [`screen-reimbursement.yaml`](screen-reimbursement.yaml) — the workflow: read the sheet
  (`google.workspace.sheets_read`) → evaluate each row (`finance.rules.evaluate`) → draft returns
  for failures (`google.workspace.gmail_draft`, **draft only**) → recommend on edge cases → post a
  Slack summary + approval → send only after approval.
- [`standards/reimbursement-standards.md`](standards/reimbursement-standards.md) — the human-readable
  policy (the machine version is the Finance Rules MCP's `reimbursement.json`).
- [`../shared/draft-and-confirm.yaml`](../shared/draft-and-confirm.yaml) — the reusable
  draft-then-confirm-then-act subrecipe.

## The end-to-end path (and how each guarantee is enforced)

```
Slack @mention → slack-gateway (ack fast, background)
  → runtime (AgentRuntime) loads screen-reimbursement
    → gateway → google.workspace.sheets_read   (read responses)
    → gateway → finance.rules.evaluate          (deterministic pass/fail/review — code decides)
    → gateway → google.workspace.gmail_draft     (draft returns; NEVER send)
  → slack-gateway posts summary + approval block (Approve / Review / Cancel)
  → human approves → gateway → google.workspace.gmail_send  (impact:high, gated)
  → every call written to audit_log
```

| Guarantee | Enforced by | Verified in |
|---|---|---|
| Correct pass/fail/review splits | `finance.rules.evaluate` (rules as code) | `mcp-servers/finance-rules` `TestSeededSheetSplits` (the §9 seeded sheet → 3 pass / 5 fail / 2 review, each fail names its standard) |
| Nothing sends before approval | gateway gating on `impact: high` + `response.require_human_approval_for` | `gateway` policy tests (high-impact blocks until an approved row exists) |
| Edge cases never auto-decided | `review` verdict → human recommendation, never a send | `TestSeededSheetSplits` (reviews carry notes; no auto-decision) |
| Every step audited | gateway audit on every call | `gateway`/`store` audit tests |
| Appeal routes to a human | the return email's appeal line → a finance member, not the agent | the recipe instructions + standards doc (§9.5) |

## Status

The recipe, subrecipe, standards, and the deterministic split test ship here and validate in CI
(`make validate`). The fully-live run (Slack → runtime → Goose → gateway → Google) additionally needs
ContextForge wired and the Google service-account key (human-gated) — the live wiring finalized
alongside the runtime.
