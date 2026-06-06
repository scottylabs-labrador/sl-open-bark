# services/engineering-agent/

The coding-agent orchestrator: from `/fix-bug repo#issue`, run a headless coding agent in a
throwaway Railway Sandbox and open a **draft** PR on the Labrador org for human review (design
Section 12; Appendix H).

**Owner:** platform-team · **Built in:** WP-11.

> **Isolated trust domain.** This subsystem has **no** access to the MCP gateway, the Google
> account, finance data, or any production secret. Its only outward powers are a scoped GitHub App
> identity (read code, open PRs — never merge) and allowlisted internet (fetch packages). Keeping
> this boundary is the core safety property; do not add a dependency that crosses it.

Will contain: a least-privilege GitHub App (Contents r/w, Pull requests write; opt-in repos) with
short-lived per-repo installation tokens; an orchestrator that creates a sandbox, clones, runs the
agent (reproduce → fix → test), pushes a branch, opens a draft PR, posts results to Slack, and
destroys the sandbox; a default-deny egress allowlist (package registries + GitHub API only; block
metadata `169.254.169.254` and RFC1918); per-task time/cost ceilings; and a maintainer allowlist.
