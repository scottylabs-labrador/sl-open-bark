# services/session-memory/

Statefulness and context engineering: the Session/Memory service plus the **Memory MCP** that
exposes scoped memory to recipes like any other capability (design Section 4.4).

**Owner:** platform-team · **Built in:** WP-05 (depends on WP-01's repository and WP-02's gateway).

Will contain:

- A Memory MCP (`memory.write_fact`, `memory.search`, `memory.forget`) backed by Postgres via
  WP-01's repository — scoped by `user | committee | org` so one person's context never leaks into
  another's.
- The per-turn context-assembly pipeline that budgets context in priority order (system +
  `.goosehints` + recipe + retrieved memory + rolling summary + recent turns + trimmed tool
  output), and updates the rolling summary and durable facts after each turn.

MCP servers stay stateless and idempotent; durable state lives in Postgres, not in the server.
