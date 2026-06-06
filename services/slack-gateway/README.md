# services/slack-gateway/

The human front door: a Slack Events API app that takes intake, **acknowledges within Slack's
timeout**, runs work in the background through the runtime, and renders **approval blocks** wired to
runtime approvals (design §3.3, §9.4; Appendix E). Built in WP-07.

**Owner:** platform-team.

## Flow

1. `@mention` in a channel, or a DM → the gateway **verifies the Slack signature**, **acks 200
   immediately**, and processes in a background goroutine.
2. It posts "working on it" in-thread, submits a task to the **runtime** (`AgentRuntime`), and polls.
3. On a high-impact action it posts an **approval block** (Approve / Review / Cancel — design §9.4);
   **nothing runs until a human clicks**. Approve/Cancel resolve the runtime approval; Review leaves
   it pending.
4. On completion it posts the result in-thread.
5. `/fix-bug repo#issue` (slash command) enqueues to the Engineering Agent (WP-11, stub today).

External systems are behind interfaces (`Runtime`, `Poster`), so the logic is unit-tested with
fakes — intake, the run/approval/result flow, the interaction handler, **signature verification**,
and the Block Kit encoding. No live Slack in CI.

## Layout

```
internal/slack/         handler (events/interactions/commands) + blocks + signature verify + poster
internal/runtimeclient/ HTTP client for the runtime task API
internal/config/        typed env config
cmd/slack-gateway/      HTTP server: /slack/events, /slack/interactions, /slack/commands, /healthz
```

## Config (Appendix E)

Bot scopes: `app_mentions:read`, `chat:write`, `im:history`, `assistant:write`, `commands`. Use the
**HTTP Events API with request-signature verification** in production (Socket Mode is for local dev).

```bash
SLACK_SIGNING_SECRET=…   # empty disables signature checks — DEV ONLY
SLACK_BOT_TOKEN=…        # empty = log-only poster
SLACK_BOT_USER_ID=…
RUNTIME_URL=http://runtime.railway.internal:8080
RUNTIME_SERVICE_TOKEN=…
```

**Human-gated:** the Slack app + tokens. No secrets in code — config from the environment.

```bash
go test ./...
go run ./cmd/slack-gateway
```
