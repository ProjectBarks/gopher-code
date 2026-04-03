# Batch 9 Notes — Utility Tools

## What was done

### CronCreateTool: Schema and behavior parity
Added `durable` parameter to CronCreate schema. TS has two persistence modes:
- `durable: false` (default): In-memory only, session-scoped. Dies when session ends.
- `durable: true`: Persisted to `.claude/scheduled_tasks.json`, survives restarts.

Also added:
- `MaxCronJobs = 50` limit with validation (TS rejects if >= 50 jobs exist)
- `DefaultMaxAgeDays = 7` constant for auto-expiry note
- `Recurring` and `Durable` fields to `CronEntry` struct
- Result messages matching TS format exactly (recurring vs one-shot, session-only vs durable, auto-expiry)

### Other tools reviewed — no changes needed
- **SleepTool**: `duration_ms` parameter, max 300000ms (5 min), context cancellation. TS defines SleepTool as a primitive tool (only prompt.ts exists, no dedicated implementation file). Go implementation matches.
- **SyntheticOutputTool**: Passthrough tool returning `text` as-is. Used for injecting synthetic messages. Go matches TS.
- **BriefTool**: Go has send/receive actions for briefing messages. TS BriefTool is more complex (uploads briefs to server for sharing). Go stub is functional.
- **RemoteTriggerTool**: Placeholder returning "not configured". TS triggers remote CCR agent executions. Both are stubs when remote is not set up.
- **tools/testing/**: No TS source files found in this directory.

## What's NOT done (deferred)

### CronCreate: Cron expression validation
TS validates cron expressions via `parseCronExpression()` and checks that the expression matches a calendar date in the next year via `nextCronRunMs()`. Go stores the cron expression without validation. Proper cron parsing would need a cron library.

### CronCreate: Durable persistence
TS writes durable tasks to `.claude/scheduled_tasks.json` via `addCronTask()`. Go stores all tasks in memory. The `durable` parameter is accepted but the actual file persistence is not implemented.

### CronCreate: Human-readable schedule
TS converts cron expressions to human-readable text via `cronToHuman()` (e.g., "every 5 minutes"). Go uses the raw cron expression in output.

### BriefTool: Upload functionality
TS BriefTool has `upload.ts` that uploads briefs to a server for cross-session sharing. Go just echoes the message locally.

### RemoteTriggerTool: Remote execution
TS triggers remote CCR agent executions with detailed scheduling. Go is a placeholder.

## Patterns noticed

1. **Cron tools pattern**: TS has CronCreate, CronDelete, CronList as separate tool files in a ScheduleCronTool directory. Go has them in a single cron.go with a shared CronStore. Both approaches work.

2. **SleepTool is a primitive**: TS doesn't have a dedicated SleepTool.ts — it's likely defined as a built-in primitive. The prompt.ts just provides the description and guidance. This means the actual sleep logic is in the query loop or tool infrastructure, not in a tool-specific file.

3. **tools/testing/ is empty**: This directory had no TS files. It may have been used for test utilities that were moved or deleted.
