# ScottyLabs Reimbursement Standards

The human-readable policy (design §9.2). The **machine-checkable** version is the data the Finance
Rules MCP loads ([`mcp-servers/finance-rules/internal/standards/reimbursement.json`](../../../mcp-servers/finance-rules/internal/standards/reimbursement.json));
the two must stay in sync. The model parses messy form input and explains; the **code decides**
caps and dates, so the auto-return is auditable, consistent, and testable.

A request is **returned (fail)** if it breaks any standard below, **flagged for a human (review)**
if the amount is borderline, and **passes** otherwise. The agent never approves or rejects money —
it screens, drafts, and recommends; a human decides and sends.

## Standards

1. **Eligible category.** The expense must be one of: `travel`, `food`, `supplies`, `swag`,
   `printing`. Anything else is returned.
2. **Itemized receipt.** An itemized receipt must be attached. Missing receipt → returned.
3. **Event association.** The expense must be tied to a ScottyLabs event. No association → returned.
4. **Submission deadline.** Submit within **30 days** of purchase. Later → returned.
5. **Category cap (USD).** The amount must not exceed the category cap:

   | Category | Cap |
   |---|---|
   | travel | $500 |
   | food | $250 |
   | supplies | $300 |
   | swag | $400 |
   | printing | $150 |

   Over the cap → returned, naming the cap and the amount.

## Review band

An amount **within 10% just below** its category cap (e.g. $470 on the $500 travel cap) is **not**
auto-passed — it goes to a human with a one-line recommendation. Edge cases are never auto-decided.

## Appeal

Every auto-returned request's email includes:
> If you believe this was returned in error, reply here and a finance member will review.

That reply routes to a **human finance member, not the agent** (design §9.5), keeping a person
accountable for the final word on anyone's money.

## Changing a standard

Edit both this file and `reimbursement.json` in the same PR, with finance-committee review. The
Finance Rules MCP loads the JSON; CI's rule tests (`mcp-servers/finance-rules`) must stay green.
