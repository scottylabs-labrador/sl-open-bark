# services/scheduler/

Recurring runs (design §4.2, §11): each minute (or each Railway-cron tick) the scheduler asks which
jobs are due and submits them to the runtime via the `AgentRuntime`. Built in WP-09.

**Owner:** platform-team.

## How it works

- A **job** is a standard 5-field cron `spec` (UTC, ≥5-min granularity), a `recipe`, its `committee`,
  and `params`. The built-in `DefaultSchedule` ships a **weekly leadership digest** (`shared/weekly-
  digest`, Mondays 13:00 UTC) and a **daily reimbursement screening** (`finance/screen-reimbursement`,
  daily 13:00 UTC). Override with a JSON `SCHEDULE_FILE`.
- `Due(now)` and `RunDue(now)` are pure/testable; submission is the only side effect. A failing job
  surfaces its error and never aborts the others.

## Modes

```bash
scheduler         # long-running: tick every minute, run due jobs, serve /healthz
scheduler run     # one-shot: run jobs due now and exit (for Railway cron)
```

Railway cron (min 5-minute granularity, UTC) can invoke `scheduler run`; or run it always-on and it
self-ticks. The digest recipe posts to Slack; the scheduler just triggers it. No secrets in code.

```bash
go test ./...
RUNTIME_URL=http://runtime.railway.internal:8080 go run ./cmd/scheduler
```
