# Batch 11 Notes — Memory & Dream Services

## What was done

### All constants and data structures verified at parity

**AutoDream** (`pkg/session/autodream.go`):
- `SessionScanIntervalMs = 10 * 60 * 1000` matches TS `SESSION_SCAN_INTERVAL_MS`
- `DefaultMinHours = 24`, `DefaultMinSessions = 5` match TS `DEFAULTS`
- `ShouldTriggerAutoDream` correctly implements the 3-gate logic: time → scan throttle → session count
- `ReadLastConsolidatedAt`/`RecordConsolidation` manage `.last-consolidation` timestamp file

**Memory** (`pkg/session/memory.go`):
- 4 memory types (user/feedback/project/reference) match TS `memoryTypes.ts`
- `EntrypointName = "MEMORY.md"`, `MaxEntrypointLines = 200`, `MaxEntrypointBytes = 25_000` all match
- `ParseMemoryFile` correctly handles YAML frontmatter with `---` delimiters
- `FormatMemoryFile` produces the correct frontmatter+body format
- `TruncateEntrypointContent` handles line truncation, byte truncation, and compound truncation with correct warning messages

## What's NOT done (deferred)

### extractMemories service
TS runs a **forked LLM agent** after each turn to extract memories from the conversation. It:
- Detects if the main agent already wrote memories (skips if so)
- Builds a specialized extraction prompt with the memory taxonomy
- Runs with limited tools (Read, Grep, Glob, read-only Bash, Edit, Write for memory dir only)
- Uses a 2-turn strategy: parallel reads first, then parallel writes
This requires the forked agent infrastructure (Batch 5 deferred agent forking).

### SessionMemory service
TS manages per-session memory during compaction — saves session-specific context that might be lost during compaction. This hooks into the compact system to preserve important state. Depends on compact integration (Batch 10).

### teamMemorySync service
TS watches team memory files for changes and syncs them across team members. Includes:
- File system watcher for `.claude/teams/*/memory/`
- Secret scanner to prevent accidental credential leaks
- Types for memory sync protocol
Depends on team infrastructure (Batch 5 deferred).

### analytics service
TS has full analytics infrastructure:
- GrowthBook feature flags (A/B testing, kill switches)
- Datadog integration
- First-party event logging via OpenTelemetry exporters
- Sink for buffered event sending
These are platform-specific infrastructure concerns not relevant to a Go CLI.

## Patterns noticed

1. **Memory system is well-ported**: The core memory data structures and file management in Go are faithful to TS. The gap is in the *background services* that use these data structures (extractMemories, autoDream execution, team sync).

2. **autoDream is a trigger system**: Go has the trigger logic (ShouldTriggerAutoDream) but not the execution side (running the consolidation prompt via forked agent). The trigger itself is correctly implemented.

3. **analytics/GrowthBook is the biggest gap**: Many TS features are gated behind GrowthBook feature flags (isAutoDreamEnabled, various thresholds). Go hardcodes the defaults, which is correct behavior when GrowthBook is not available. If Go ever needs dynamic feature flags, a simple env-var-based system would suffice.
