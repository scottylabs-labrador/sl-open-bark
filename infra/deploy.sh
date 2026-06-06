#!/usr/bin/env bash
# infra/deploy.sh — deployment is HUMAN-GATED.
#
# Deploys happen on Railway, which sets each service's Root Directory, Watch Path, and secrets
# through its dashboard/API per service (one manual step per service; design Section 7.6, 11.3).
# There is no automated `make deploy` yet because creating the Railway project/services and adding
# real secrets is a human-gated step (see CLAUDE.md and docs/mcp-hosting-on-railway.md). This stub
# documents the path so the Makefile target exists and CI stays honest.
set -euo pipefail

cat <<'EOF'
Deploy is human-gated. To ship a service:

  1. (once per project) A maintainer creates the Railway project and managed Postgres, and adds
     secrets as Railway secrets — never in code (CLAUDE.md guardrail 1).
  2. (once per service) New Service from this monorepo; set Root Directory to the service folder
     (e.g. mcp-servers/<name>) and a matching Watch Path; add that service's secrets.
  3. After that, every push touching that folder auto-deploys only that service (Section 7.6).
  4. For an MCP server: the gateway registers it from manifest.yaml as `proposed`; a maintainer
     promotes it to `approved` after a live check.

See docs/mcp-hosting-on-railway.md for the full method.
EOF
