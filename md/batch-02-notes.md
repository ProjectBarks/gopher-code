# Batch 2 Notes — Query Loop & Core Types

## What was done

### Missing beta header added (pkg/provider/betas.go)
Added `BetaTokenEfficientTools = "token-efficient-tools-2026-03-28"`. This was the only missing beta header constant out of 16 in the TS source.

### Cyber risk instruction added to system prompt (pkg/prompt/system.go)
The TS system prompt starts with `CYBER_RISK_INSTRUCTION` (from constants/cyberRiskInstruction.ts) — a Safeguards-team-owned instruction defining security boundaries. This was completely missing in Go. Also added the URL restriction ("NEVER generate or guess URLs").

### API duration tracking (pkg/query/query.go)
The `TotalAPIDuration` field was added to SessionState in Batch 1 but never populated. Now `time.Since(apiStart)` wraps the Stream() + stream consumption to track cumulative API call duration in milliseconds.

### Turn completion verbs (pkg/ui/components/spinner_verbs.go)
TS uses 8 past-tense verbs ("Baked", "Brewed", "Churned", etc.) for the completed spinner display. Go was using the static string "thought for Xs". Now uses a random turn completion verb matching TS behavior.

### NoContentMessage constant (pkg/message/message.go)
Added `NoContentMessage = "(no content)"` matching TS constants/messages.ts. Already used as a string literal in message_bubble.go.

## What's already at parity (no changes needed)

### query/tokenBudget.ts — FULLY PORTED
BudgetTracker, checkTokenBudget(), createBudgetTracker(), CompletionThreshold (0.9), DiminishingThreshold (500) — all match exactly in pkg/compact/autocompact.go. The nudge message text matches too.

### schemas/hooks.ts — FULLY PORTED
All 27 hook events, all 4 command types (command, prompt, agent, http), all fields match between TS and Go pkg/hooks/hooks.go. Only gap is Zod-style schema validation (URL format, timeout positive, shell enum) which is a nice-to-have.

### constants/ — 95% already matched
Beta headers (14/15 matched, now 15/15), all tool limits (6/6), all per-tool result limits (10/10), spinner verbs (188/188), effort figures (4/4), Bedrock extra params (3/3).

### context/ — React-specific, no Go equivalent needed
All 9 files are React context providers (FPS metrics, modals, overlays, notifications, voice, etc.). Bubbletea uses a fundamentally different architecture (message passing vs context propagation). The Go mailbox.go serves a different purpose (file-based inter-agent messaging) than TS mailbox.tsx (in-memory UI state queue).

## What's NOT done (deferred to later batches)

### query/stopHooks.ts — ~475 lines, only ~20 line stub in Go
The TS stop hooks system handles Stop, TaskCompleted, and TeammateIdle hooks with progress tracking, hook summary messages, and background fire-and-forget operations (memory extraction, auto-dream, prompt suggestion). The Go stub at query.go:348-368 handles the basic case only. Depends on:
- Hooks system (Batch 19)
- Memory extraction (Batch 11)
- Auto-dream (Batch 11)
- Team/agent system (Batch 5)

### query/config.ts — QueryConfig pattern
Captures feature gates once per query for pure reducer semantics. Design pattern for testability, not a behavioral gap. Would be useful when adding feature gates.

### query/deps.ts — Dependency injection
QueryDeps interface for test mocking (callModel, microcompact, autocompact, uuid). Go tests use different patterns. Nice-to-have.

### types/logs.ts — Rich transcript metadata
TS persists 18+ entry types (summaries, attribution, context collapse, PR links, worktree state). Go only has SessionMetadata (ID, model, CWD, timestamps). Relevant when Batch 11-13 cover services.

### types/permissions.ts — Rich permission rules
TS has 30+ types for rules, updates, sources, classifier, context. Go has 6 basic decision types. Relevant when Batch 14 covers permissions.

### types/command.ts — Unified command registry
TS has a rich Command discriminated union with hooks/plugins/context. Go has individual tools. Not needed for CLI mode.

## Patterns noticed

1. **API limits constants** (apiLimits.ts: image max size, PDF limits) are missing in Go. Will be needed for Batch 3 (File Tools) when image/PDF handling is audited.

2. **XML tag constants** (xml.ts) define tag names for task notifications, command messages, etc. These are used in system prompt message parsing and will be relevant for Batch 5-6 (Agent/Task tools).

3. **Output styles** (outputStyles.ts) define "Explanatory" and "Learning" styles with prompt templates. Not yet in Go. Would need a new pkg/outputstyle or similar when output style support is added.
