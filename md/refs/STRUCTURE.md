# Gopher Code Project Structure

**Goal**: Mirror Claude Code's functionality while using idiomatic Go organization.

---

## Root Files

```
gopher/
├── README.md                    # User-facing guide
├── STATUS.md                    # Completion status ← START HERE
├── FEATURE_PARITY.md            # Feature matrix vs Claude Code
├── ARCHITECTURE_MAPPING.md      # TypeScript → Go equivalence
├── IMPLEMENTATION_COMPLETE.md   # Phase summary
├── STRUCTURE.md                 # This file
├── go.mod                       # Module definition + dependencies
├── go.sum                       # Dependency checksums
└── deps.go                      # Dependency documentation
```

---

## `cmd/gopher/`

**CLI entry point and main executable**

```
cmd/gopher/
└── main.go
    ├── Flag parsing (--model, --print, --resume, --cwd, --verbose, --output)
    ├── Config loading (~/.claude/settings.json)
    ├── Session discovery/initialization
    ├── REPL startup (Bubbletea)
    └── One-shot query mode
```

**Run with**: `go build ./cmd/gopher && ./gopher`

---

## `pkg/` — Core Subsystems

### `pkg/message/`
**Message types and normalization** — [Maps to `src/types/Message.ts`]

```
message/
├── message.go        # Role, ContentBlock, Message types
├── normalize.go      # Remove trailing spaces, merge text blocks
├── helpers.go        # Extract text, find tool calls
└── message_test.go
```

### `pkg/provider/`
**Anthropic API client with SSE streaming** — [Maps to `src/services/api/`]

```
provider/
├── provider.go       # ModelProvider interface
├── request.go        # ModelRequest, ToolDefinition assembly
├── response.go       # StreamEvent types (message_start, delta, etc.)
├── anthropic.go      # HTTP client, exponential backoff
├── stream.go         # SSE parsing, channel conversion
├── errors.go         # Error classification (context_too_long, 429, etc.)
├── cost.go           # Token cost calculation
├── retry.go          # Retry policy (exponential backoff)
├── anthropic_test.go
└── ...
```

### `pkg/session/`
**Session state and persistence** — [Maps to `src/services/SessionMemory/`]

```
session/
├── session.go        # SessionState: messages, metadata, usage
├── persist.go        # Load/save to ~/.claude/sessions/
├── discover.go       # Find sessions by ID, title, CWD
├── resume.go         # Restore prior session
├── teams.go          # Team coordination
├── planapproval.go   # Plan mode state tracking
└── ...
```

### `pkg/query/`
**Query loop orchestration** — [Maps to `src/query/`] — **THE HEART**

```
query/
├── query.go          # Core loop: build request → stream → tool execute → loop
├── model.go          # ModelRequest assembly, tool definition export
├── errors.go         # AgentError, error classification
├── events.go         # QueryEvent types (TextDelta, ToolUseStart, etc.)
├── helpers.go        # Helper functions (shouldCompact, classifyError)
├── l1_basic_loop_test.go   # L1: 10 tests (basic workflow)
├── l2_error_recovery_test.go # L2: 8 tests (retry, compact)
├── l3_budget_compact_test.go # L3: 7 tests (auto-compaction)
└── l4_lifecycle_test.go      # L4: 6 tests (persistence, hooks)
```

### `pkg/compact/`
**Token budget and context compaction** — [Maps to `src/services/compact/`]

```
compact/
├── budget.go         # TokenBudget: token counting, threshold
├── compact.go        # Compact session (drop old messages)
├── microcompact.go   # Truncate large tool results >10KB
├── prompt.go         # Build compact message sequence
├── tokenbudget_test.go
├── compact_parity_test.go
└── ...
```

### `pkg/tools/`
**40+ built-in tools** — [Maps to `src/tools/`]

```
tools/
├── tool.go           # Tool interface: Name, Description, Execute
├── context.go        # ToolContext (CWD, permissions, environment)
├── registry.go       # ToolRegistry: register, lookup tools
├── orchestrator.go   # ToolOrchestrator: parallel/sequential execution
├── defaults.go       # RegisterDefaults() — register all 40+ tools
│
├── [Core Tools]
├── bash.go           # BashTool: execute shell commands
├── fileread.go       # FileReadTool: read with line numbers
├── filewrite.go      # FileWriteTool: atomic write
├── fileedit.go       # FileEditTool: find/replace
├── glob.go           # GlobTool: recursive patterns
├── grep.go           # GrepTool: ripgrep or fallback
│
├── [Web Tools]
├── webfetch.go       # WebFetchTool: fetch + HTML→markdown
├── websearch.go      # WebSearchTool: DuckDuckGo/Brave
│
├── [Code Tools]
├── lsp.go            # LSPTool: definition, references, hover
├── notebook.go       # NotebookEditTool: Jupyter cell edit
├── pdf.go            # PDFTool: read PDF with page ranges
│
├── [Agent & Team]
├── agent.go          # AgentTool: spawn subagents
├── teamtools.go      # TeamCreate, TeamDelete
├── sendmessage.go    # SendMessageTool: peer messaging
│
├── [Task Management]
├── tasks.go          # TaskCreate, TaskList, TaskGet, TaskUpdate, TaskStop
│
├── [Configuration]
├── configtool.go     # ConfigTool: read/write settings.json
├── skill.go          # SkillTool: load and execute skills
├── cron.go           # CronCreate, RemoteTrigger: scheduling
│
├── [Sandbox & Planning]
├── worktree.go       # EnterWorktree, ExitWorktree
├── planmode.go       # EnterPlanMode, ExitPlanMode
│
├── [MCP]
├── mcpauth.go        # MCPAuthTool
├── mcpresources.go   # MCPResources, MCPTools
│
├── [User Interaction]
├── askuser.go        # AskUserQuestionTool: interactive prompts
│
├── [Utilities]
├── brief.go          # BriefTool
├── sleep.go          # SleepTool
├── synthetic.go      # SyntheticOutputTool
├── listdir.go        # ListDirTool
├── pathextract.go    # Path extraction helper
├── shellparse.go     # Shell AST security analysis
├── todo.go           # Memory/todo utilities
│
├── [Platform-Specific]
├── powershell.go     # PowerShell (Windows)
│
└── [Tests]
└── *_test.go         # 30+ tool-specific tests
```

### `pkg/permissions/`
**Permission evaluation and rules** — [Maps to `src/utils/permissions/`]

```
permissions/
├── types.go          # PermissionMode, PermissionRequest, Decision
├── policy.go         # PermissionPolicy interface
├── rules.go          # RuleBasedPolicy implementation
├── scope.go          # Scope matching (tool, action, pattern)
└── permissions_test.go
```

### `pkg/prompt/`
**System prompt assembly** — [Maps to `src/services/compact/prompt.ts`]

```
prompt/
├── prompt.go         # Assemble system prompt from components
├── memory.go         # Load CLAUDE.md (project + global)
└── prompt_test.go
```

### `pkg/config/`
**Configuration management** — [Maps to `src/services/settingsSync/`]

```
config/
├── settings.go       # Load ~/.claude/settings.json
├── schema.go         # JSON schema validation
├── model.go          # Model definitions, aliases, costs
├── keybindings.go    # Keybinding customization
└── config_test.go
```

### `pkg/hooks/`
**Lifecycle hooks (27 events)** — [Maps to `src/commands/hooks/`]

```
hooks/
├── hooks.go          # Hook registration, firing
├── executor.go       # Shell/HTTP executor
├── events.go         # 27 lifecycle event types
└── hooks_test.go
```

### `pkg/mcp/`
**Model Context Protocol client** — [Maps to `src/services/mcp/`]

```
mcp/
├── client.go         # MCP client interface
├── stdio/            # Stdio transport (local MCPs)
├── sse/              # SSE transport (HTTP MCPs)
├── websocket/        # WebSocket transport
├── discovery.go      # Tool discovery + caching
└── mcp_test.go
```

### `pkg/skills/`
**Skill loader and execution** — [Maps to `src/skills/`]

```
skills/
├── loader.go         # Load skills from filesystem
├── frontmatter.go    # Parse YAML frontmatter
├── skill.go          # Skill type definition
└── skills_test.go
```

### `pkg/auth/`
**Authentication and credentials** — [Maps to `src/services/oauth/`]

```
auth/
├── auth.go           # Auth provider interface
├── oauth.go          # OAuth token management
├── keyring.go        # OS credential storage (macOS Keychain, Linux SecretService)
└── auth_test.go
```

---

## `internal/` — Internal Infrastructure

### `internal/cli/`
**Bubble Tea TUI and REPL** — [Maps to `src/cli/` + `src/ink/`]

```
cli/
├── tui.go            # Bubbletea Elm-architecture model
├── repl.go           # REPL loop, command dispatch
├── render.go         # Output rendering (streaming text, tool results)
├── spinner.go        # Animated spinners
├── statusbar.go      # Real-time status display (tokens, cost, model)
├── markdown.go       # Glamour markdown rendering
├── diff.go           # Unified diff rendering
├── permission_dialog.go  # Huh-based permission prompts
└── ...
```

### `internal/testharness/`
**Golden file testing framework** — [Maps to `claw-code-rust/crates/core/tests/harness/`]

```
testharness/
├── scripted_provider.go   # Mock LLM (play back pre-recorded events)
├── spy_tool.go            # Mock tool (record calls, return configured responses)
├── event_collector.go     # Capture QueryEvent emissions for assertions
├── builders.go            # Test scenario builders
│   ├── MakeSession()
│   ├── MakeTextTurn()
│   ├── MakeToolTurn()
│   ├── MakeMultiToolTurn()
│   └── ...
└── golden.go              # Golden file loader
```

---

## `testdata/`
**Parity test fixtures**

```
testdata/
├── api_contracts/         # Golden API response files
├── golden_sessions/       # Prior session transcripts
├── tool_schemas.json      # Tool definition reference
├── query_loop_constants.json
└── ...
```

---

## `assets/`
**Branding and images**

```
assets/
└── go-gopher-pixel-art.png
```

---

## `old/`
**Archived planning documents**

```
old/
├── agent-team-plan.md.bak       # Original agent team implementation plan
├── go-library-stack.md.bak      # Original dependency mapping
└── clever-orbiting-melody.md.bak # Original concept document
```

---

## Configuration Files

### `~/.claude/`
**User configuration directory** (read by gopher at startup)

```
~/.claude/
├── settings.json          # Main config (model, permissions, hooks, MCP, etc.)
├── keybindings.json       # Custom keybindings
├── mcp.json               # MCP client configs (stdio, SSE, WebSocket)
├── CLAUDE.md              # Global project memory
├── sessions/              # Session history
│   ├── session-1.json
│   ├── session-2.json
│   └── ...
├── memory/                # Persistent memory (from MemoryTool)
├── skills/                # Custom skills (.md files with frontmatter)
├── agents/                # Custom agent definitions
├── hooks/                 # Hook scripts (shell commands)
└── teams/                 # Team coordination (shared task lists)
```

### Project Root
```
<project>/
├── .claude/               # Project-specific Claude Code config
│   ├── settings.json      # Project overrides for settings
│   ├── mcp.json           # Project-specific MCPs
│   └── CLAUDE.md          # Project memory (auto-loaded)
└── CLAUDE.md              # Alternative project memory location
```

---

## How It Maps to Claude Code

| Claude Code (TS) | Gopher Code (Go) | Type |
|---|---|---|
| `src/types/` | `pkg/message/` | Types |
| `src/query/` | `pkg/query/` | Core loop |
| `src/services/api/` | `pkg/provider/` | API |
| `src/services/SessionMemory/` | `pkg/session/` | Persistence |
| `src/services/compact/` | `pkg/compact/` | Budgeting |
| `src/utils/permissions/` | `pkg/permissions/` | Permissions |
| `src/tools/` | `pkg/tools/` | Tools (40+) |
| `src/commands/hooks/` | `pkg/hooks/` | Hooks |
| `src/services/mcp/` | `pkg/mcp/` | MCP |
| `src/skills/` | `pkg/skills/` | Skills |
| `src/cli/` | `cmd/gopher/` + `internal/cli/` | CLI |
| `src/ink/` | `internal/cli/` + Bubbletea | TUI |

---

## Reading Order for Contributors

1. **STATUS.md** — Understand what's complete
2. **README.md** — User perspective
3. **FEATURE_PARITY.md** — Feature matrix
4. **ARCHITECTURE_MAPPING.md** — How TS maps to Go
5. **IMPLEMENTATION_COMPLETE.md** — Phase breakdown
6. **This file** — File organization
7. **Code**:
   - Start with `pkg/query/query.go` (the heart)
   - Then `pkg/tools/` (40+ tool implementations)
   - Then `internal/cli/` (TUI rendering)
   - Then subsystems as needed

---

## Build & Test

```bash
# Build the binary
go build -o gopher ./cmd/gopher

# Run all tests
go test -race ./...

# Run specific test level
go test -run L1 ./pkg/query

# Update golden files
go test ./... -update

# Cross-compile
GOOS=linux GOARCH=arm64 go build -o gopher-linux-arm64 ./cmd/gopher
```

---

## Maintenance Notes

- **No Node.js** — Pure Go, single binary
- **No npm/package.json** — Only go.mod/go.sum
- **No build scripts** — `go build` is all you need
- **No platform-specific builds** — Cross-compile with GOOS/GOARCH
- **No version bumps needed** — All dependencies pinned in go.sum

---

**This structure makes Gopher immediately recognizable to anyone familiar with Claude Code while being idiomatic Go.**
