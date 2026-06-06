# Finance Rules MCP

The deterministic **"rules as code"** server for reimbursement screening (design Section 9). It
exposes one read-only tool, `evaluate`, that takes a parsed request and returns a verdict
(`pass` / `fail` / `review`) with the specific failed standards and evidence. Encoding the
checks (caps, dates, required fields, receipt, event association) as real code rather than asking
the model to "judge" them makes the auto-return **auditable, consistent, and testable**. The model
parses messy input, decides the genuinely ambiguous `review` cases, and writes the explanation — it
never does arithmetic on caps and dates, and it never approves or rejects money.

## Tool

| Tool | Scope | Impact |
|---|---|---|
| `evaluate` | `finance.read` | read |

`fail` lists every broken standard (`eligible_category`, `itemized_receipt`, `event_association`,
`submission_deadline`, `category_cap`) with evidence. `review` flags an amount within the review
band just below a cap, for a human to decide.

## Standards as data (design §9.2)

The machine-checkable standards live in **[`internal/standards/reimbursement.json`](internal/standards/reimbursement.json)** — version-controlled,
finance-committee reviewed, and embedded into the binary (so a deploy always has them). Override
with a file via `FINANCE_STANDARDS_FILE`. The human-readable policy will live alongside the recipe
in `recipes/finance/standards/` (WP-08). Changing a cap or category is a reviewed PR, not a code
change to the rules.

## Structure

```
internal/domain/     types + PURE-FUNCTION rules (no I/O) + LoadStandards — the testable core
internal/standards/  the reviewed standards JSON, embedded
internal/service/    thin orchestration (holds the loaded standards)
internal/tools/      the thin evaluate handler
internal/config/     typed env config
cmd/server/          composition root: load standards, register the tool, serve /mcp + /healthz
```

## Run

```bash
go test ./...                 # the full Section 9 matrix + standards loading, no network
go vet ./... && gofmt -l .
go run ./cmd/server           # serves Streamable HTTP at /mcp
```

No secrets needed. Deploy and registration follow the standard Path A method
([`../../docs/mcp-hosting-on-railway.md`](../../docs/mcp-hosting-on-railway.md)); it lands
`proposed` and a maintainer promotes it to `approved` after a live check.
