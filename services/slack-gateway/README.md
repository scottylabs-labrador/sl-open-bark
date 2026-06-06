# services/slack-gateway/

The human front door: a Slack **Bolt** app that takes intake, acknowledges within Slack's timeout,
runs work in the background through the `AgentRuntime` interface (WP-06), and renders approval
blocks (design Sections 3.3, 9.4; Appendix E).

**Owner:** platform-team · **Built in:** WP-07.

Will contain: event handlers (`app_mention`, DMs, the assistant panel), fast `ack()` then a
background job, result + approval-block posting to the originating thread, approval records written
through the gateway, and a `/fix-bug` slash command that enqueues to the Engineering Agent (WP-11).

Slack scopes (Appendix E): `app_mentions:read`, `chat:write`, `im:history`, `assistant:write`,
`commands`. Production uses the HTTP Events API with request-signature verification; Socket Mode is
for local development only. No tokens in code — they come from Railway secrets.
