<!-- Keep PRs small and reviewable: one work package or one capability/recipe per PR. -->

## What changed

<!-- A short description of the change. -->

## Why

<!-- The work package (WP-NN) or issue this addresses, and the motivation. -->

## How it was tested

<!-- Commands run and results. CI must be green before merge. -->

## New tool scopes (capabilities only)

<!-- For a new/changed MCP server: list each new tool, its scope and impact, and why each scope is
     needed. Omit if no capability change. -->

---

### Definition of done (from CLAUDE.md)

- [ ] Matches the design doc and the assigned work package.
- [ ] New MCP tools declare `scope` + `impact` in `manifest.yaml`; irreversible tools are `impact: high` and gated.
- [ ] Tests added and green; `gofmt`, `go vet`, `go test ./...` clean (`make ci`).
- [ ] No secrets committed; config comes from the environment.
- [ ] For a capability: a one-paragraph security note (what it touches, failure modes).
