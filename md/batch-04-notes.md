# Batch 4 Notes — Shell & Code Tools

## What was done

### BashTool: Exit code semantics fix (BEHAVIORAL BUG)
Go was returning `IsError=true` for non-zero exit codes. TS returns `is_error: false` (success) with "Exit code N" appended to the output text. The model needs to see the output + exit code to understand what happened — wrapping it as a tool error loses context. Only interrupts (Ctrl+C) set `is_error: true` in TS.

### BashTool: User's shell instead of /bin/sh
Go was using `sh -c` which gives a minimal POSIX shell. TS uses the user's configured shell (from `$SHELL`), typically bash or zsh. This matters because:
- Profile/rc files aren't loaded with /bin/sh
- Shell-specific features (arrays, brace expansion, etc.) don't work
- PATH from user's profile may be missing

Now uses `getUserShell()` which checks `$SHELL`, falls back to `bash`, then `/bin/sh`.

### BashTool: Output truncation
TS truncates output at 30K chars (configurable via `BASH_MAX_OUTPUT_LENGTH` env var, max 150K) with a `"... [N lines truncated] ..."` message. Go had no truncation — model could receive megabytes of output, wasting context tokens. Now matches TS behavior exactly.

### BashTool: stripEmptyLines
TS strips leading and trailing empty lines from command output. Added `stripEmptyLines()` to match.

## What's NOT done (deferred)

### BashTool: Background tasks
TS has full `run_in_background` implementation with task tracking, auto-backgrounding after 15s, and output file persistence. Go parses the parameter but doesn't implement it. This needs the task system (Batch 6) and output file persistence.

### BashTool: Safety checks
TS has comprehensive safety: AST-based shell parsing (`bashSecurity.ts`, `sedValidation.ts`), sleep/infinite-loop detection, destructive command warnings, mode validation, path validation. Go has minimal `IsReadOnlyCommand()` check. Security layer covered by Batch 14 (permissions).

### BashTool: Progress reporting
TS sends real-time progress updates during long-running commands. Go executes synchronously with no progress. Would need goroutine-based output streaming.

### PowerShellTool
TS version has Windows-specific features: edition detection (5.1 vs Core), CLM detection, UTF-16 LE encoding, registry paths, Windows-specific destructive command warnings. Go has a basic PowerShell executor. These are platform-specific concerns.

### REPLTool
TS "REPL mode" is an ANT-internal feature that hides primitive tools (FileRead, FileEdit, Bash, etc.) and forces Claude to batch operations through a VM context. This is fundamentally different from Go's REPLTool which is a language interpreter executor. They solve different problems.

### LSPTool
TS has full LSP server protocol integration (9 operations: definition, references, hover, symbols, call hierarchy, etc.) with server lifecycle management. Go uses shell commands (go vet, tsc, py_compile) and regex-based symbol extraction. Bridging this gap requires a full LSP client implementation.

### NotebookEditTool
Both support replace/insert/delete. TS has:
- Cell-ID addressing (by UUID or "cell-N" format) vs Go's 0-based index
- Read-before-edit enforcement via readFileState
- Cell ID generation for nbformat 4.5+
- Execution state management (reset execution_count on edit)
Minor gaps, core operations work the same.

## Patterns noticed

1. **Tool error vs command error**: TS distinguishes between tool errors (IsError=true, like invalid input or timeout) and command errors (success output with exit code appended). The model gets full context for command errors. This pattern should be verified in PowerShellTool and REPLTool too.

2. **Output truncation pattern**: The `formatOutput()` → `getMaxOutputLength()` → env var pattern is reusable. Other tools that produce large output (Agent, TaskOutput) should use the same truncation approach.

3. **Shell provider architecture**: TS has a sophisticated shell provider system (bashProvider.ts, powershellProvider.ts) with shell detection, profile snapshotting, and extglob management. Go directly calls the shell binary. If shell compatibility issues arise, consider adding shell detection logic.
