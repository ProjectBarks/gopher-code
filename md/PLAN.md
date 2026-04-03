# Gopher vs Claude Code — Deep Feature Parity Audit

## Goal

This plan walks through every directory in the Claude Code TypeScript source, reads the actual implementation, compares it against the Go counterpart in Gopher, and **fixes what's wrong**:

1. **Feature parity** — every behavior in the TS code has a working equivalent in Go. If not, implement it.
2. **No bugs** — the Go implementation handles edge cases, error paths, and boundary conditions the same way. If not, fix it.
3. **Test coverage** — add test cases to `gopher/` to prove the fix works and prevent regressions.

**The output of each batch is code, not a report.** Find the gap → fix the Go code → add tests → move on.

**Source (TS):** `/Users/alexgaribaldi/claude-code-v2/research/claude-code-source-build/source/src/`
**Target (Go):** `/Users/alexgaribaldi/claude-code-v2/gopher/`
**Arch docs:** `/Users/alexgaribaldi/claude-code-v2/research/architchture/`
**Arch mapping:** `/Users/alexgaribaldi/claude-code-v2/gopher/md/refs/ARCHITECTURE_MAPPING.md`
**Notes dir:** `/Users/alexgaribaldi/claude-code-v2/gopher/md/` — write notes here if context would help the next iteration

## Per-Batch Process

For each batch:
1. Read every TS file in the listed directories
2. Read the corresponding Go files in Gopher
3. Compare behavior: control flow, error handling, edge cases, defaults
4. **Fix gaps in the Go code** — implement missing behavior, correct bugs, align edge cases
5. **Add tests** to validate the fixes compile and pass
6. **Write notes to `md/`** if anything would help the next iteration (e.g. cross-cutting concerns spotted, dependencies on a later batch, patterns to reuse). Name them `md/batch-NN-notes.md`.
7. Record what was fixed and what was already correct
8. Mark the batch checkbox when fully complete

---

## Batch 1 — Bootstrap & Entrypoints
- [x] `bootstrap/`
- [x] `entrypoints/`
- [x] `state/`
- [x] `cli/`
- [x] `screens/`

**Fixes applied:**
- `pkg/session/session.go`: Added ParentSessionID, OriginalCWD, ProjectRoot, TotalCostUSD, TotalAPIDuration, TotalToolDuration, TotalLinesAdded, TotalLinesRemoved, ModelUsage (per-model tracking), IsInteractive fields. Added AddCost(), AddLinesChanged(), RegenerateSessionID() methods.
- `pkg/query/query.go`: Fixed budget check — replaced hardcoded Sonnet pricing ($3/$15) with proper CalculateUSDCost() using model-specific rates and cache tokens. Added per-turn cost tracking via sess.AddCost().
- `internal/cli/repl.go`: Enhanced /cost to show USD cost. Enhanced /usage to show per-model breakdown, lines changed, and tracked TotalCostUSD. Enhanced /model to resolve aliases (haiku/sonnet/opus) and persist model changes to settings.json (matching TS onChangeAppState behavior). Extracted /doctor into comprehensive runDoctor() with 6 diagnostic sections (auth, tools, env, config, CLAUDE.md, session).
- `internal/cli/render.go`: Added ndjsonSafeStringify() to escape U+2028/U+2029 in stream-json output (matching TS cli/ndjsonSafeStringify.ts).
- `cmd/gopher-code/main.go`: Set OriginalCWD/ProjectRoot on session creation and resume. Set IsInteractive=true for TUI/REPL mode.

**Tests added:**
- `pkg/session/session_test.go`: TestNew_InitializesNewFields, TestAddCost, TestAddCost_MultipleModels, TestAddLinesChanged, TestRegenerateSessionID, TestSaveAndLoad_NewFields
- `internal/cli/render_test.go`: TestNdjsonSafeStringify (5 cases)

**Notes written:** `md/batch-01-notes.md`

---

## Batch 2 — Query Loop & Core Types
- [ ] `query/`
- [ ] `context/`
- [ ] `constants/`
- [ ] `types/`
- [ ] `schemas/`

**Fixes applied:**

**Tests added:**

**Notes written:**

---

## Batch 3 — File Tools
- [ ] `tools/FileReadTool/`
- [ ] `tools/FileWriteTool/`
- [ ] `tools/FileEditTool/`
- [ ] `tools/GlobTool/`
- [ ] `tools/GrepTool/`
- [ ] `tools/shared/`

**Fixes applied:**

**Tests added:**

**Notes written:**

---

## Batch 4 — Shell & Code Tools
- [ ] `tools/BashTool/`
- [ ] `tools/PowerShellTool/`
- [ ] `tools/REPLTool/`
- [ ] `tools/LSPTool/`
- [ ] `tools/NotebookEditTool/`

**Fixes applied:**

**Tests added:**

**Notes written:**

---

## Batch 5 — Agent & Team Tools
- [ ] `tools/AgentTool/`
- [ ] `tools/SendMessageTool/`
- [ ] `tools/TeamCreateTool/`
- [ ] `tools/TeamDeleteTool/`
- [ ] `tools/SkillTool/`
- [ ] `tools/ToolSearchTool/`

**Fixes applied:**

**Tests added:**

**Notes written:**

---

## Batch 6 — Task & Todo Tools
- [ ] `tools/TaskCreateTool/`
- [ ] `tools/TaskGetTool/`
- [ ] `tools/TaskListTool/`
- [ ] `tools/TaskOutputTool/`
- [ ] `tools/TaskStopTool/`
- [ ] `tools/TaskUpdateTool/`
- [ ] `tools/TodoWriteTool/`

**Fixes applied:**

**Tests added:**

**Notes written:**

---

## Batch 7 — Web & MCP Tools
- [ ] `tools/WebFetchTool/`
- [ ] `tools/WebSearchTool/`
- [ ] `tools/MCPTool/`
- [ ] `tools/McpAuthTool/`
- [ ] `tools/ListMcpResourcesTool/`
- [ ] `tools/ReadMcpResourceTool/`

**Fixes applied:**

**Tests added:**

**Notes written:**

---

## Batch 8 — Mode & Config Tools
- [ ] `tools/EnterPlanModeTool/`
- [ ] `tools/ExitPlanModeTool/`
- [ ] `tools/EnterWorktreeTool/`
- [ ] `tools/ExitWorktreeTool/`
- [ ] `tools/ConfigTool/`
- [ ] `tools/AskUserQuestionTool/`

**Fixes applied:**

**Tests added:**

**Notes written:**

---

## Batch 9 — Utility Tools
- [ ] `tools/SleepTool/`
- [ ] `tools/SyntheticOutputTool/`
- [ ] `tools/BriefTool/`
- [ ] `tools/RemoteTriggerTool/`
- [ ] `tools/ScheduleCronTool/`
- [ ] `tools/testing/`

**Fixes applied:**

**Tests added:**

**Notes written:**

---

## Batch 10 — Core Services
- [ ] `services/api/`
- [ ] `services/compact/`
- [ ] `services/mcp/`
- [ ] `services/oauth/`
- [ ] `services/tools/`

**Fixes applied:**

**Tests added:**

**Notes written:**

---

## Batch 11 — Memory & Dream Services
- [ ] `services/autoDream/`
- [ ] `services/extractMemories/`
- [ ] `services/SessionMemory/`
- [ ] `services/teamMemorySync/`
- [ ] `services/analytics/`

**Fixes applied:**

**Tests added:**

**Notes written:**

---

## Batch 12 — Auxiliary Services
- [ ] `services/AgentSummary/`
- [ ] `services/lsp/`
- [ ] `services/MagicDocs/`
- [ ] `services/plugins/`
- [ ] `services/policyLimits/`
- [ ] `services/PromptSuggestion/`

**Fixes applied:**

**Tests added:**

**Notes written:**

---

## Batch 13 — Remaining Services
- [ ] `services/remoteManagedSettings/`
- [ ] `services/settingsSync/`
- [ ] `services/tips/`
- [ ] `services/toolUseSummary/`

**Fixes applied:**

**Tests added:**

**Notes written:**

---

## Batch 14 — Permission & Settings Utils
- [ ] `utils/permissions/`
- [ ] `utils/settings/`
- [ ] `utils/hooks/`
- [ ] `utils/mcp/`
- [ ] `utils/memory/`

**Fixes applied:**

**Tests added:**

**Notes written:**

---

## Batch 15 — Shell & Model Utils
- [ ] `utils/bash/`
- [ ] `utils/shell/`
- [ ] `utils/powershell/`
- [ ] `utils/sandbox/`
- [ ] `utils/model/`
- [ ] `utils/messages/`

**Fixes applied:**

**Tests added:**

**Notes written:**

---

## Batch 16 — Git, Task & Plugin Utils
- [ ] `utils/git/`
- [ ] `utils/github/`
- [ ] `utils/todo/`
- [ ] `utils/task/`
- [ ] `utils/skills/`
- [ ] `utils/plugins/`

**Fixes applied:**

**Tests added:**

**Notes written:**

---

## Batch 17 — Storage & Swarm Utils
- [ ] `utils/secureStorage/`
- [ ] `utils/filePersistence/`
- [ ] `utils/processUserInput/`
- [ ] `utils/suggestions/`
- [ ] `utils/swarm/`
- [ ] `utils/telemetry/`

**Fixes applied:**

**Tests added:**

**Notes written:**

---

## Batch 18 — Platform & Remote Utils
- [ ] `utils/background/`
- [ ] `utils/claudeInChrome/`
- [ ] `utils/computerUse/`
- [ ] `utils/deepLink/`
- [ ] `utils/dxt/`
- [ ] `utils/nativeInstaller/`
- [ ] `utils/teleport/`
- [ ] `utils/ultraplan/`

**Fixes applied:**

**Tests added:**

**Notes written:**

---

## Batch 19 — Coordinator & Tasks Runtime
- [ ] `hooks/`
- [ ] `coordinator/`
- [ ] `tasks/`
- [ ] `buddy/`
- [ ] `assistant/`

**Fixes applied:**

**Tests added:**

**Notes written:**

---

## Batch 20 — Bridge & Remote
- [ ] `bridge/`
- [ ] `remote/`
- [ ] `server/`
- [ ] `upstreamproxy/`

**Fixes applied:**

**Tests added:**

**Notes written:**

---

## Batch 21 — TUI Core
- [ ] `ink/`
- [ ] `components/PromptInput/`
- [ ] `components/messages/`
- [ ] `components/Spinner/`
- [ ] `components/LogoV2/`

**Fixes applied:**

**Tests added:**

**Notes written:**

---

## Batch 22 — TUI Diff & Teams
- [ ] `components/diff/`
- [ ] `components/StructuredDiff/`
- [ ] `components/HighlightedCode/`
- [ ] `components/tasks/`
- [ ] `components/teams/`
- [ ] `components/agents/`

**Fixes applied:**

**Tests added:**

**Notes written:**

---

## Batch 23 — TUI Permissions & Settings
- [ ] `components/permissions/`
- [ ] `components/TrustDialog/`
- [ ] `components/sandbox/`
- [ ] `components/Settings/`
- [ ] `components/hooks/`

**Fixes applied:**

**Tests added:**

**Notes written:**

---

## Batch 24 — TUI MCP, Skills & Design
- [ ] `components/mcp/`
- [ ] `components/skills/`
- [ ] `components/grove/`
- [ ] `components/ui/`
- [ ] `components/design-system/`
- [ ] `components/wizard/`

**Fixes applied:**

**Tests added:**

**Notes written:**

---

## Batch 25 — TUI Misc Components
- [ ] `components/ClaudeCodeHint/`
- [ ] `components/CustomSelect/`
- [ ] `components/DesktopUpsell/`
- [ ] `components/FeedbackSurvey/`
- [ ] `components/HelpV2/`
- [ ] `components/LspRecommendation/`
- [ ] `components/ManagedSettingsSecurityDialog/`
- [ ] `components/Passes/`

**Fixes applied:**

**Tests added:**

**Notes written:**

---

## Batch 26 — Commands A-E
- [ ] `commands/add-dir/`
- [ ] `commands/agents/`
- [ ] `commands/branch/`
- [ ] `commands/bridge/`
- [ ] `commands/btw/`
- [ ] `commands/chrome/`
- [ ] `commands/clear/`
- [ ] `commands/color/`
- [ ] `commands/compact/`
- [ ] `commands/config/`

**Fixes applied:**

**Tests added:**

**Notes written:**

---

## Batch 27 — Commands C-E (cont)
- [ ] `commands/context/`
- [ ] `commands/copy/`
- [ ] `commands/cost/`
- [ ] `commands/desktop/`
- [ ] `commands/diff/`
- [ ] `commands/doctor/`
- [ ] `commands/effort/`
- [ ] `commands/exit/`
- [ ] `commands/export/`
- [ ] `commands/extra-usage/`

**Fixes applied:**

**Tests added:**

**Notes written:**

---

## Batch 28 — Commands F-L
- [ ] `commands/fast/`
- [ ] `commands/feedback/`
- [ ] `commands/files/`
- [ ] `commands/heapdump/`
- [ ] `commands/help/`
- [ ] `commands/hooks/`
- [ ] `commands/ide/`
- [ ] `commands/install-github-app/`
- [ ] `commands/install-slack-app/`
- [ ] `commands/keybindings/`

**Fixes applied:**

**Tests added:**

**Notes written:**

---

## Batch 29 — Commands L-P
- [ ] `commands/login/`
- [ ] `commands/logout/`
- [ ] `commands/mcp/`
- [ ] `commands/memory/`
- [ ] `commands/mobile/`
- [ ] `commands/model/`
- [ ] `commands/output-style/`
- [ ] `commands/passes/`
- [ ] `commands/permissions/`
- [ ] `commands/plan/`

**Fixes applied:**

**Tests added:**

**Notes written:**

---

## Batch 30 — Commands P-R
- [ ] `commands/plugin/`
- [ ] `commands/pr_comments/`
- [ ] `commands/privacy-settings/`
- [ ] `commands/rate-limit-options/`
- [ ] `commands/release-notes/`
- [ ] `commands/reload-plugins/`
- [ ] `commands/remote-env/`
- [ ] `commands/remote-setup/`
- [ ] `commands/rename/`
- [ ] `commands/resume/`

**Fixes applied:**

**Tests added:**

**Notes written:**

---

## Batch 31 — Commands R-S
- [ ] `commands/review/`
- [ ] `commands/rewind/`
- [ ] `commands/sandbox-toggle/`
- [ ] `commands/session/`
- [ ] `commands/skills/`
- [ ] `commands/stats/`
- [ ] `commands/status/`
- [ ] `commands/stickers/`
- [ ] `commands/tag/`
- [ ] `commands/tasks/`

**Fixes applied:**

**Tests added:**

**Notes written:**

---

## Batch 32 — Commands T-V
- [ ] `commands/terminalSetup/`
- [ ] `commands/theme/`
- [ ] `commands/thinkback/`
- [ ] `commands/thinkback-play/`
- [ ] `commands/upgrade/`
- [ ] `commands/usage/`
- [ ] `commands/vim/`
- [ ] `commands/voice/`

**Fixes applied:**

**Tests added:**

**Notes written:**

---

## Batch 33 — Plugins, Skills & Data
- [ ] `plugins/`
- [ ] `skills/`
- [ ] `memdir/`
- [ ] `migrations/`
- [ ] `keybindings/`

**Fixes applied:**

**Tests added:**

**Notes written:**

---

## Batch 34 — Native, Output & Extras
- [ ] `native-ts/`
- [ ] `outputStyles/`
- [ ] `moreright/`
- [ ] `vim/`
- [ ] `voice/`

**Fixes applied:**

**Tests added:**

**Notes written:**

---

# Summary

| Batch | Area | Status | Fixes | Tests | Notes |
|-------|------|--------|-------|-------|-------|
| 1 | Bootstrap & Entrypoints | [x] | 5 files | 2 test files (11 tests) | batch-01-notes.md |
| 2 | Query Loop & Core Types | [ ] | | | |
| 3 | File Tools | [ ] | | | |
| 4 | Shell & Code Tools | [ ] | | | |
| 5 | Agent & Team Tools | [ ] | | | |
| 6 | Task & Todo Tools | [ ] | | | |
| 7 | Web & MCP Tools | [ ] | | | |
| 8 | Mode & Config Tools | [ ] | | | |
| 9 | Utility Tools | [ ] | | | |
| 10 | Core Services | [ ] | | | |
| 11 | Memory & Dream Services | [ ] | | | |
| 12 | Auxiliary Services | [ ] | | | |
| 13 | Remaining Services | [ ] | | | |
| 14 | Permission & Settings Utils | [ ] | | | |
| 15 | Shell & Model Utils | [ ] | | | |
| 16 | Git, Task & Plugin Utils | [ ] | | | |
| 17 | Storage & Swarm Utils | [ ] | | | |
| 18 | Platform & Remote Utils | [ ] | | | |
| 19 | Coordinator & Tasks Runtime | [ ] | | | |
| 20 | Bridge & Remote | [ ] | | | |
| 21 | TUI Core | [ ] | | | |
| 22 | TUI Diff & Teams | [ ] | | | |
| 23 | TUI Permissions & Settings | [ ] | | | |
| 24 | TUI MCP, Skills & Design | [ ] | | | |
| 25 | TUI Misc Components | [ ] | | | |
| 26 | Commands A-E | [ ] | | | |
| 27 | Commands C-E (cont) | [ ] | | | |
| 28 | Commands F-L | [ ] | | | |
| 29 | Commands L-P | [ ] | | | |
| 30 | Commands P-R | [ ] | | | |
| 31 | Commands R-S | [ ] | | | |
| 32 | Commands T-V | [ ] | | | |
| 33 | Plugins, Skills & Data | [ ] | | | |
| 34 | Native, Output & Extras | [ ] | | | |
