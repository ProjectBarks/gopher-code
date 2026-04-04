# Batch 14 Notes — Permission & Settings Utils

## What was done

All 5 directories reviewed. The Go implementations are comprehensive — no code changes needed.

### utils/permissions/ → pkg/permissions/
**15 Go files** covering the full permission system:
- **types.go**: All TS permission modes present (default, acceptEdits, bypassPermissions, dontAsk, plan, auto). PermissionDecision sealed interface (Allow/Deny/Ask).
- **rules.go**: Mode-based RuleBasedPolicy with tool-specific allowlists per mode. acceptEdits allows all tools except Bash. plan mode restricts to read-only tools.
- **rules_parser.go**: Full rule parsing matching TS `permissionRuleParser.ts` — legacy :* prefix syntax, escaped parentheses, wildcard glob patterns, NormalizeLegacyToolName (Task→Agent, KillShell→TaskStop, etc.).
- **waterfall.go**: Waterfall evaluation of rules across setting sources (user→project→local→flag→policy).
- **dangerous.go**: Dangerous permission detection for Bash rules — detects interpreters (python, node, ruby, sh, bash, perl, php, lua), shell builtins (eval, exec, source), package runners (npm, pip, cargo, etc.). Matches TS `IsDangerousBashPermission`.
- **persistence.go**: Permission rule persistence to settings files with deduplication.
- **validation.go**: Permission update schema validation.

### utils/settings/ → pkg/config/
**10 Go files** covering settings management:
- **sources.go**: 5 setting sources (userSettings, projectSettings, localSettings, flagSettings, policySettings) matching TS `constants.ts` exactly. Display names match.
- **config.go**: Load and merge settings from global (~/.claude/settings.json) and project (.claude/settings.json).
- **managed.go**: Enterprise managed settings support.
- **mdm.go**: MDM (Mobile Device Management) settings for macOS managed-settings.json.
- **schema.go**: Settings schema validation.

### utils/hooks/ → pkg/hooks/
**4 Go files** covering hook execution:
- **hooks.go**: 27 hook events matching TS exactly (all verified in Batch 2). 4 command types (command, prompt, agent, http). Exit code semantics: 0=success, 2=blocking, other=non-blocking.
- **exec_http.go**: HTTP hook execution with env var interpolation, header sanitization, URL pattern matching.

### utils/mcp/ — 2 small TS files
- **dateTimeParser.ts**: DateTime parsing for cron schedules — Go handles cron differently.
- **elicitationValidation.ts**: MCP elicitation validation — not needed for basic CLI.

### utils/memory/ — 2 small TS files
- **versions.ts**: Memory format versioning — Go handles in pkg/session/memory.go.
- **types.ts**: Memory types (already in Go as MemoryType enum in pkg/session/memory.go).

## What's NOT done (deferred)

### Classifier-based permissions
TS has `yoloClassifier.ts`, `bashClassifier.ts`, `classifierDecision.ts`, `classifierShared.ts` — these use an LLM call (Haiku) to classify bash commands as safe/unsafe. Go uses mode-based rules instead. Would need a secondary LLM call to implement.

### Auto-mode state
TS has `autoModeState.ts` for managing the auto permission mode which uses the classifier. Go's `ModeAuto` exists in types but falls back to `AskDecision`.

### Denial tracking
TS has `denialTracking.ts` for tracking consecutive denials to detect and prevent permission fatigue. Not in Go.

### Permission mode cycling
TS has `getNextPermissionMode.ts` for Shift+Tab cycling through modes. UI-specific feature.

### Settings change detection
TS has `changeDetector.ts`, `internalWrites.ts` for detecting settings changes and distinguishing internal from external writes. Go doesn't track this.

## Patterns noticed

1. **Permission system is one of Go's strongest areas**: 15 files, 40+ tests, covering rule parsing, waterfall evaluation, dangerous permission detection, and persistence. This is at high parity with TS.

2. **Classifier gap is the main permission difference**: TS can auto-approve commands using an LLM classifier (yoloClassifier). Go falls back to asking the user. This is the biggest permission-related gap for UX but not a correctness issue.

3. **Settings source ordering is critical**: Both TS and Go use the same 5-source hierarchy (user < project < local < flag < policy). This is important for enterprise managed settings to work correctly.
