# runtime/

The **AgentRuntime** boundary and the Goose configuration. This is the deliberately thin seam
between every surface (Slack, Scheduler) and the agent loop, so the platform is not hostage to one
runtime's roadmap (design Sections 4.2, 5).

**Owner:** platform-team · **Built in:** WP-06.

Will contain:

- The `AgentRuntime` interface (`submit_task`, `request_approval`, `result`) — the contract the
  Slack Gateway (WP-07) and Scheduler (WP-09) depend on, and nothing else.
- A Goose-backed implementation: headless Goose configured via env
  (`GOOSE_PROVIDER=openrouter`, `GOOSE_MODEL`, `OPENROUTER_API_KEY`), the MCP gateway registered as
  an extension, recipe loading from `recipes/`, and per-recipe model override.
- A default `.goosehints` with static org guidance (loaded every turn; see design Section 4.4).

No secrets in here — the OpenRouter key comes from the environment (Railway secrets).
