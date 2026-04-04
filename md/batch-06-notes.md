# Batch 6 Notes — Task & Todo Tools

## What was done

### TaskUpdate: Metadata null deletion
TS TaskUpdateTool handles metadata merging with null-deletion semantics: setting a key to `null` removes it from metadata. Go was just setting the key to `nil` in the map instead of deleting it. Fixed to match TS behavior exactly.

### Task ID format: "task_N" → "N"
TS uses numeric string IDs ("1", "2", "3") created by `String(highestId + 1)` in utils/tasks.ts:297. Go was using "task_1", "task_2" format. Changed to match TS for session compatibility.

### TodoWriteTool: Already at parity
Go TodoWriteTool matches TS behavior:
- Replace-entire-list semantics (not append)
- Status validation (pending/in_progress/done)
- Shared state between TodoWriteTool and TodoReadTool
- TS TodoWriteTool is the legacy system (disabled when isTodoV2Enabled), while TaskCreate/Update/etc. are the new system

## What's NOT done (deferred)

### Task persistence
TS persists tasks to disk (~/.claude/tasks/{list-id}/{task-id}.json) with file locking for concurrent access. Go uses in-memory TaskStore. Tasks don't survive process restart.

### Task hooks
TS fires TaskCreated and TaskCompleted hooks with team/agent context. Go doesn't fire any hooks.

### Task completion hooks and verification nudge
TS TaskUpdateTool fires executeTaskCompletedHooks when marking tasks complete. It also checks if a verification step was needed and nudges the model. Go has none of this.

### Auto-owner assignment
TS auto-sets the task owner to the current agent name when marking a task as in_progress in swarm mode. Go doesn't do this.

### TodoWriteTool: Verification nudge
TS TodoWriteTool has a verification nudge feature — when all items are completed and none was a verification step, it reminds the model. This is feature-gated behind VERIFICATION_AGENT.

## Patterns noticed

1. **TS has two task systems**: TodoWriteTool (legacy, disabled by isTodoV2Enabled) and TaskCreate/Update/etc. (new "v2" system). Go has both. The new system is the one that matters for parity.

2. **File-based vs in-memory storage**: All TS task/team tools use file-based persistence. Go uses in-memory stores for both tasks and teams. This means teams and tasks are session-scoped in Go (lost on restart). File persistence would need to be added for multi-session/multi-agent scenarios.

3. **Task output truncation**: The TS TaskOutputTool uses the same output truncation as BashTool (configurable max length). Go's TaskOutputTool stores output in metadata without truncation. The batch-04 note about output truncation being reusable applies here too.
