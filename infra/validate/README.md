# slvalidate

The CI gate that validates ScottyLabs **registry manifests** and **recipes** against their JSON
Schemas (the registry and workflow contracts, design Section 7). Its own Go module so it builds and
runs independently like every other component.

## What it checks

- Every `mcp-servers/**/manifest.yaml` against
  [`../../mcp-servers/manifest.schema.json`](../../mcp-servers/manifest.schema.json) — name, owner,
  endpoint, and that **every tool declares a `scope` and an `impact` (read | write | high)**. A tool
  with no scope is rejected (fail closed, Section 10.3).
- Every `recipes/**/*.yaml` against
  [`../../recipes/recipe.schema.json`](../../recipes/recipe.schema.json) — version, title,
  description, owner, instructions, and the shapes of `parameters`, `extensions`, and `response`.

## Usage

```bash
go run . manifests <repo-root>   # validate manifests
go run . recipes   <repo-root>   # validate recipes
go run . all       <repo-root>   # both

# from the repo root, via the Makefile:
make validate
```

Exits non-zero if any file is invalid, printing the offending file and the schema violation. With
no servers or recipes present yet, it reports "no files to validate" and passes.

## Not yet checked (future work)

The design also wants CI to confirm each recipe's declared capability **exists in the registry** and
is **permitted to the owning committee** (Section 7.3). That needs the gateway registry, so it lands
with WP-02; this tool does the schema half today.
