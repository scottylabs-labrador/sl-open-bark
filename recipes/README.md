# recipes/ — WORKFLOWS (the main contribution surface)

A **recipe** is a YAML workflow that teaches the agent *how* to do a committee task: the steps, the
standards to apply, the tone of the reply, which gateway-fronted tools to use, and what to hand to
a human. A recipe grants **no new access** — it composes capabilities that already exist behind the
gateway. This is the lightest extension point: drop a file, open a PR, merge (design Section 7).

```
recipes/
  finance/   events/   design/      # one folder per committee
  shared/                           # reusable subrecipes (e.g. draft-and-confirm)
```

## Add a workflow

1. Write `recipes/<committee>/<name>.yaml` per the schema in
   [`docs/recipe-spec.md`](../docs/recipe-spec.md) (machine schema:
   [`recipe.schema.json`](recipe.schema.json)).
2. Test it locally against the gateway with your own Claude Code or a local Goose.
3. Open a PR. CI lints the recipe schema and checks every declared capability exists in the
   registry and that your committee is allowed to use it (`make validate`).
4. A committee owner reviews and merges. On merge it is available to the deployed agent — no
   runtime redeploy (recipes are read from the repo).

Anything irreversible a recipe triggers must be listed under `response.require_human_approval_for`,
and the gateway gates it regardless of model output. The agent never sends money.
