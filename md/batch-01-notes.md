# Batch 1 Notes — Bootstrap & Entrypoints

## What was done

### SessionState expanded (pkg/session/session.go)
Added 10+ fields matching TS bootstrap/state.ts: ParentSessionID, OriginalCWD, ProjectRoot, cost/duration metrics, per-model usage tracking (ModelUsage map), IsInteractive. Added thread-safe AddCost() method for concurrent agent use.

### Budget check fixed (pkg/query/query.go)
The budget check previously used hardcoded Sonnet pricing ($3/$15). Now uses `provider.CalculateUSDCost()` with the actual model's pricing and accounts for cache creation/read tokens. Cost is tracked cumulatively in `sess.TotalCostUSD`.

### NDJSON safety (internal/cli/render.go)
Added `ndjsonSafeStringify()` matching TS `cli/ndjsonSafeStringify.ts`. Escapes U+2028/U+2029 in stream-json output to prevent line-splitting receivers from breaking.

### Doctor command expanded (internal/cli/repl.go)
Went from 5 basic checks to 6 full diagnostic sections: auth, tools (with versions and paths), environment (env var overrides), configuration (settings file paths and validation), CLAUDE.md files (with size warnings at >40k), and session info. Still missing vs TS: installation type detection, package manager detection, multiple installation detection, plugin/MCP parse errors, keybinding warnings, context usage warnings, sandbox config, auto-update status.

### Model command improved (internal/cli/repl.go)
/model now resolves aliases (haiku→claude-haiku-4-5-20251001, sonnet→claude-sonnet-4-6, opus→claude-opus-4-6) and persists changes to ~/.claude/settings.json (matching TS state/onChangeAppState.ts behavior).

## What's NOT done (deferred to later batches)

### SDK/Control Protocol (entrypoints/sdk/*)
The TS SDK has ~2,500 lines of Zod schemas for control protocol messages, hook events, and core types. These are for the Agent SDK programmatic API, not the interactive CLI. Relevant when Batch 10+ covers services/api and the SDK entrypoint.

### Launch Screen Animation (entrypoints/launchScreen.ts)
393-line ANSI animation with particle effects. Cosmetic. Could be added to internal/cli/ as a startup screen.

### Telemetry (bootstrap/state.ts telemetry fields)
~40 functions for OpenTelemetry counters, meters, tracers. Not present in Go. Would need a separate telemetry package.

### Store Pattern / Persistence Hooks (state/*)
TS uses a generic Store<T> with subscribe() and an onChange handler that persists settings changes. Go uses Bubbletea message passing which is architecturally different. The model persistence was extracted and added to /model, but the general pattern isn't replicated.

### CLI Transports (cli/transports/*)
SSE, WebSocket, hybrid transports for remote/bridge mode. Enterprise features. Relevant when Batch 20 covers bridge/remote.

### Full Doctor Parity (screens/Doctor.tsx)
TS Doctor has 17 check categories. Go now has 6. Missing: installation type/package manager detection, multiple installation detection, plugin/MCP errors, keybinding warnings, context usage warnings, sandbox config, version locks, auto-update status. Relevant when Batch 12-14 cover services and utils.

## Patterns noticed

1. **Model alias resolution** is duplicated in cmd/gopher-code/main.go and internal/cli/repl.go. Should be extracted to a shared helper (pkg/config or a new pkg/model package).

2. **Cost tracking** now flows: query.go → sess.AddCost() → sess.TotalCostUSD. Any tool that wants to add cost (e.g., web search pricing) should use the same path.

3. **SessionState.mu** (mutex) was added for AddCost() thread safety. Any new concurrent-access fields should use it too.
