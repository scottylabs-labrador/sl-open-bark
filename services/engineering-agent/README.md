# services/engineering-agent/

From `/fix-bug repo#issue`, run a headless coding agent in a **throwaway sandbox** and open a
**draft PR** on the Labrador org for human review (design §12; Appendix H). Built in WP-11.

> ### Isolated trust domain
> This subsystem has **no** access to the MCP gateway, the Google account, finance data, or any
> production secret. Its only outward powers are a **scoped GitHub App** identity (read code, open
> PRs — never merge) and **allowlisted internet** (fetch packages). Keeping this boundary is the core
> safety property — it imports nothing from `gateway`, `store`, Google, or finance.

## Flow

`/fix-bug` (from the Slack gateway) → authorize (maintainer + opt-in repo) → mint a short-lived,
repo-scoped GitHub token → create a sandbox (template + fork, **default-deny egress allowlist**, time
ceiling, **no production credentials**) → clone, run the headless coding agent (reproduce → fix →
test), push a branch → open a **draft** PR → post the link + test summary to Slack → **destroy the
sandbox** (always). The app cannot merge (branch protection requires human review).

## What's tested (with fakes; no live GitHub/sandbox)

- **Egress allowlist** (`internal/egress`): allow GitHub + package registries + their subdomains;
  block `169.254.169.254`, RFC1918, loopback, link-local, `*.internal`/`*.local`, and raw IPs.
- **Maintainer/repo allowlist** (`internal/authz`): default-deny; both must be allowlisted.
- **Orchestrator** (`internal/orchestrator`): the full flow — sandbox created **and destroyed**
  (even on failure), a **draft** PR opened, Slack notified, the sandbox env carries **only** the
  scoped token + task (no production secret), and an egress allowlist is supplied.

## Human-gated (provision, then set via env)

The GitHub App + private key, the sandbox provider (Railway Sandboxes), the Slack webhook, and the
maintainer/repo allowlists. The live GitHub token minting / PR creation and the sandbox CLI calls are
finalized against the provisioned providers. No secrets in code.
