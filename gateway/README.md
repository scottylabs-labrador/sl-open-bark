# gateway/

The **capability bus**: an adopted open-source MCP gateway plus the ScottyLabs policy layer and
registry. One governed endpoint fronts every MCP server with auth, per-tool and per-committee scope
enforcement, human-in-the-loop gating on `impact: high`, discovery, and full audit (design Section
6, Section 10).

**Owner:** platform-team · **Built in:** WP-02 (depends on WP-01's repository).

Will contain: the adopted gateway config (for example ContextForge or the agentic-community
gateway), the policy layer (committee-role resolution, scope enforcement, HITL gating), and the
registry that loads servers from each `manifest.yaml` and tracks lifecycle `proposed -> approved`.

**Gateway API contract** (depended on by the runtime and members' Claude):
`register(manifest)`, `list_tools(identity)`, `call(tool, args, identity) -> result`.

Safe defaults (Section 10.3): fail closed. Unknown tool → not callable. Unrecognized caller →
nothing. A capability with no declared scope is rejected by CI. A high-impact action with no
approval row does not execute. New servers land in `proposed` and do nothing live until promoted.
