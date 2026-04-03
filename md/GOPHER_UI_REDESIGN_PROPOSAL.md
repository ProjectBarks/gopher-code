# Gopher UI Redesign Proposal

> Comprehensive plan for achieving visual and UX parity with Claude Code's terminal UI,
> translating the Ink/React component model to Go's Bubble Tea framework.
>
> **Supporting documents**: `UI_ARCHITECTURE.md` (detailed architecture), `pkg/ui/theme/` (implemented code)

---

## 1. Executive Summary

### What We're Building

A complete UI layer rewrite of Gopher Code's terminal interface (`internal/cli/`) to match
Claude Code's rich Ink/React-based TUI. The current Gopher CLI uses basic `fmt.Print` ANSI
output with a synchronous `bufio.Scanner` REPL. Claude Code uses a sophisticated component
hierarchy: a design system with themed primitives (ThemedText, ThemedBox, Pane, Dialog,
Tabs, Divider, ProgressBar), a reactive message list with virtual scrolling, structured
diff rendering, shimmer animations, permission dialogs, spinner states, and a status line
with context-aware information density.

### Scope

We are targeting **visual and functional equivalence** with Claude Code v2.1.88. This means:

- **Full**: Theme system (dark, light, high-contrast, dark-ansi, light-ansi, auto-detect)
- **Full**: Design system primitives (ThemedText, Pane, Dialog, Divider, Tabs, ProgressBar)
- **Full**: Message rendering (streaming markdown, tool use indicators, structured diffs)
- **Full**: Spinner with shimmer animation and verb display
- **Full**: Status line (model, context %, CWD, permissions, session name)
- **Full**: Permission dialog (allow/deny/always with keybindings)
- **Full**: Prompt input (multi-line, history, paste, mode indicators)
- **Partial**: Virtual message list (Bubble Tea viewport, not React virtual list)
- **Deferred**: Voice mode, Grove, Chrome integration, desktop handoff

### Already Implemented

The design-system-expert has built the foundation theme system at `pkg/ui/theme/`:

| File | Lines | Description |
|------|-------|-------------|
| `palette.go` | 161 | 80+ hex color constants (blues, cyans, indigos, grays, semantic, agent) |
| `theme.go` | 212 | Theme interface, global registry, `SetTheme()`/`Current()`, runtime switching |
| `colors.go` | 130 | `ColorScheme` struct with 40+ semantic color roles |
| `dark.go` | 237 | Navy+cyan dark theme with full lipgloss style methods |
| `light.go` | 237 | White+blue light theme with full lipgloss style methods |
| `highcontrast.go` | 258 | WCAG AAA accessibility theme (bold+underline for color-blind) |

This means **Phase 1 theme work is ~60% complete**. Remaining: ANSI-16 fallbacks,
daltonized variants, terminal auto-detection, bridging Claude Code's 89-key Theme to
Gopher's 40-key ColorScheme.

### Key Integration Insight

The `query.Query()` function's `EventCallback` parameter is the **clean integration seam**.
The query loop is completely UI-agnostic -- all rendering flows through the callback. The
new Bubble Tea UI replaces `PrintEvent` with `program.Send(evt)`, injecting query events
into the Bubble Tea message loop. No changes to `pkg/query/`, `pkg/tools/`, or
`pkg/provider/` are needed.

### Effort Estimate

- **Total**: 40 person-weeks
- **Timeline**: 40 weeks (1 person), 21 weeks (2 people), 14 weeks (3 people)

### Success Metrics

1. Screenshot parity: side-by-side comparison shows equivalent visual output
2. All themes render correctly on macOS Terminal, iTerm2, Alacritty, kitty
3. Spinner animation maintains 12+ FPS without CPU spikes
4. Permission dialogs match the allow/deny/always flow exactly
5. Structured diffs show word-level highlighting with correct colors
6. Status line updates reactively as context window fills
7. Zero regressions in `--print` / `--output-format` / `--query` modes

---

## 2. Architecture Overview

### 2.1 Current State (Gopher)

**Entry point:** `cmd/gopher-code/main.go` (582 lines)
- Parses ~40 flags, builds provider/registry/session
- Three execution paths: `--print` mode, `--query` one-shot, interactive `cli.RunREPL()`

**REPL:** `internal/cli/repl.go` (541 lines)
- Synchronous `bufio.Scanner` loop, ~40 slash commands in a switch block
- Calls `query.Query(ctx, sess, prov, registry, orchestrator, PrintEvent)` per user input
- Auto-saves session after each exchange

**Rendering:** `internal/cli/render.go`
- `PrintEvent` callback: switch on 5 event types, hardcoded ANSI escape codes
- `PlainTextCallback`, `StreamJSONCallback`, `JSONCollector` (for `--output-format`)

**Other CLI files:**
- `spinner.go` -- Goroutine-based ticker spinner (hardcoded cyan, 80ms)
- `statusbar.go` -- Thread-safe task pills with mutex (hardcoded gray)
- `permission_dialog.go` -- Huh forms blocking on stdin
- `diff.go` -- Line-by-line diff with hardcoded ANSI (not unified, not word-level)
- `markdown.go` -- Glamour wrapper (2 functions)
- `tui.go` -- Skeleton Bubble Tea model (118 lines, not wired to REPL)
- `completions.go` -- Shell completion scripts

**Problems:**
- Colors are hardcoded ANSI escape codes scattered across files
- No theme system -- everything assumes dark terminal
- No component abstraction -- rendering is procedural printf
- `tui.go` Bubble Tea model is a placeholder (not connected to query loop)
- `repl.go` uses blocking `bufio.Scanner`, not Bubble Tea messages
- No reactive layout -- terminal resize does nothing useful
- No structured diff (line-by-line, not word-level)
- Spinner is a standalone goroutine, not integrated with Bubble Tea

### 2.2 Target State (Claude Code TS Reference)

Claude Code's UI has 150+ components. Key architectural elements:

```
src/utils/theme.ts           -- Theme type (89 color keys), 6 theme definitions
src/components/design-system/ -- 16 primitives (ThemedText, Pane, Dialog, Tabs, etc.)
src/screens/REPL.tsx          -- Main REPL screen (~2000 lines)
src/components/Spinner.tsx    -- Shimmer animation, verb display, teammate tree
src/components/StatusLine.tsx -- Context %, model, CWD, permissions, ANSI output
src/components/Messages.tsx   -- Virtual message list with grouping/collapsing
src/components/PromptInput/   -- Multi-file input component (20+ files)
src/components/StructuredDiff.tsx -- NAPI Rust syntax-highlighted diffs
src/components/permissions/   -- Permission request flow
```

### 2.3 Proposed Go Architecture

The architecture follows a **4-layer** model (~28 files, 5,000-6,500 new lines):

```
pkg/ui/
├── theme/                       *** ALREADY IMPLEMENTED (6 files, 1,235 lines) ***
│   palette.go         -- 80+ color constants
│   theme.go           -- Theme interface, registry, SetTheme/Current
│   colors.go          -- ColorScheme struct (40+ semantic roles)
│   dark.go            -- Dark theme with lipgloss styles
│   light.go           -- Light theme with lipgloss styles
│   highcontrast.go    -- Accessibility theme
│   dark_ansi.go       -- [TODO] ANSI-16 dark fallback
│   light_ansi.go      -- [TODO] ANSI-16 light fallback
│   resolve.go         -- [TODO] Terminal auto-detection
│
├── core/                        *** Layer 1: Abstractions (800-1,000 lines) ***
│   component.go       -- Component interface (extends tea.Model + SetSize)
│   focus.go           -- Focusable + Identifiable interfaces
│   keymap.go          -- Key binding registry, context-aware keymaps
│   styles.go          -- Lipgloss style factories per component type
│   eventbus.go        -- Typed tea.Msg definitions (StreamTextMsg, ToolStartMsg, etc.)
│   viewport.go        -- Enhanced viewport with virtual scrolling
│
├── containers/                  *** Layer 2: Layout (500-700 lines) ***
│   stack.go           -- Layered stack (main + overlays/modals)
│   overlay.go         -- Modal overlay that captures all input
│   split.go           -- Horizontal/vertical split panes
│   scrollview.go      -- Scrollable content wrapper
│
├── components/                  *** Layer 3: UI Components (2,500-3,500 lines) ***
│   messagelist.go     -- Virtual-scrolled message list (only render visible)
│   messagebubble.go   -- User/assistant message with role headers
│   streambuffer.go    -- Streaming text with delta append + blinking cursor
│   toolcallcard.go    -- Expandable tool call card (name, params, status)
│   permissiondialog.go -- Channel-based permission dialog (modal)
│   inputarea.go       -- Multi-line text input with history (wraps Bubbles textarea)
│   statusbar.go       -- Bottom bar: model, tokens, cost, mode pills
│   spinner.go         -- Inline spinner (wraps Bubbles spinner)
│   diffview.go        -- Syntax-highlighted unified diff viewer
│   codeblock.go       -- Chroma-highlighted code with line numbers
│   markdown.go        -- Glamour-rendered markdown viewport
│   taskprogress.go    -- Task progress indicators
│   toast.go           -- Ephemeral notification toasts with TTL
│
├── pages/                       *** Layer 4: Full-screen pages (600-900 lines) ***
│   replpage.go        -- Main REPL page (input + conversation + status)
│   welcomepage.go     -- Startup welcome with logo + status
│   sessionpicker.go   -- Filterable session list for /resume
│
├── app.go                       *** Root Bubble Tea model, page router ***
└── bridge.go                    *** QueryEvent → tea.Msg adapter (300-400 lines) ***

internal/cli/
  tui_v2.go          -- New entry point: creates pkg/ui app, wires to session/query
  tui.go             -- Keep as legacy fallback (rename later)
  repl.go            -- Keep as-is (fallback for --no-tui / --print mode)
  render.go          -- Keep as-is (used by --print and --output-format modes)
```

### 2.4 Core Abstractions

**Component interface** -- extends `tea.Model` with `SetSize(width, height int)`. Separate
`Focusable` and `Identifiable` interfaces for optional capabilities. Two patterns:
stateful `tea.Model` for interactive components, pure `Render(data, width) string` for
display-only renderers.

**EventBus** -- NOT a custom system. Uses Bubble Tea's native `tea.Msg` pipeline. Typed
messages defined in `core/eventbus.go`:
- `StreamTextMsg` -- text delta from query
- `ToolStartMsg` / `ToolResultMsg` -- tool lifecycle
- `PermissionRequestMsg` -- with `ResponseCh chan bool`
- `QueryDoneMsg` -- turn complete
- `ToastMsg` -- ephemeral notification

**State management** -- component-local ownership (no global store, no Redux pattern):
- Parents pass data down via struct fields
- Children communicate up via `tea.Cmd` returns
- Cross-cutting updates flow through root as typed `tea.Msg`
- Query loop runs in goroutine, sends messages via `program.Send()`

### 2.5 Rendering Strategy

- **Lipgloss v2 for ALL styling** -- zero raw ANSI codes in new UI
- **Tick-based stream batching** at 60fps (16ms flush interval) to avoid per-character re-renders
- **Virtual scrolling** in MessageList -- only render visible messages
- **Cached markdown rendering** per MessageBubble (recompute only on resize)
- **Lazy syntax highlighting** -- Chroma highlighting only on expand, not on initial render

### 2.6 Charm Library Reuse

| Category | Approach | Components |
|----------|----------|------------|
| **Reused directly** (6) | Thin wrapper adapters | viewport, textarea, spinner, key bindings, help, list |
| **Custom implementations** (6) | Built from scratch | virtual scrolling, overlay/modal, permission dialog, stream batching, expandable tool cards, toast |

### 2.7 Key Architectural Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Rendering framework | Bubble Tea + Lip Gloss | Already in go.mod (v2); Go-idiomatic; active ecosystem |
| Integration seam | `EventCallback` on `query.Query()` | Clean boundary; query loop is already UI-agnostic |
| UI thread model | Bubble Tea owns main thread; query on goroutine | Standard Bubble Tea pattern; `program.Send()` bridges |
| Theme storage | Go structs with string color values | Matches TS Theme type; lipgloss resolves to terminal capability |
| Component model | `tea.Model` + `SetSize()` interface | Composable; supports focus/identity optional interfaces |
| Permission bridge | Channel-based `BubbleteePermissionPolicy` | Blocks orchestrator goroutine; dialog on main thread |
| Diff rendering | Pure Go (go-diff + chroma) | No NAPI/Rust dependency; good enough for terminal |
| Markdown | Glamour (already imported) | Theme-aware styles via custom style JSON |
| State management | Component-local, no global store | Bubble Tea pattern; parents pass down, children cmd up |
| Terminal detection | `$COLORTERM`, `$COLORFGBG`, OSC 11 | Same signals as Claude Code for auto-theme |
| Rollback | `GOPHER_NEW_UI=1` env var + `--no-tui` flag | Zero-risk deployment; old REPL unchanged |
| Migration | Additive -- `pkg/ui/` alongside `internal/cli/` | Old code untouched until new TUI fully wired |

---

## 3. Data Flow Architecture

### 3.1 Query-to-UI Bridge

```
User Input
  --> Bubble Tea Update()
    --> Dispatch to input component
      --> If query: spawn goroutine calling query.Query() with program.Send callback
        --> QueryEvent messages flow back into Update()
          --> QEventTextDelta   --> append to output viewport
          --> QEventToolUseStart --> show tool panel with spinner
          --> QEventToolResult   --> show result summary, collapse panel
          --> QEventTurnComplete --> re-enable input, update status bar
          --> QEventUsage        --> update token/cost display
```

### 3.2 Permission Bridge (Critical Path)

Current flow: `permissions.InteractivePolicy.Check()` blocks, calls `ShowPermissionDialog()` (Huh form on stdin).

New flow with channel-based `BubbleteePermissionPolicy`:
1. `Check()` sends `PermissionRequestMsg` to program, blocks on response channel
2. `AppModel.Update()` receives msg, shows dialog overlay
3. User responds via dialog keybindings (Enter/Esc/a)
4. Response sent back via channel, unblocks `Check()`
5. Orchestrator goroutine continues with allow/deny decision

**Deadlock mitigation**: Channel with 30-second timeout; fallback to deny on timeout.

### 3.3 Backward Compatibility Matrix

| Component | Strategy | Risk |
|-----------|----------|------|
| `query.Query()` | Unchanged -- EventCallback is the seam | None |
| `tools.ToolOrchestrator` | Unchanged -- new permission policy implements same interface | None |
| `session.SessionState` | Unchanged -- UI reads/writes same state | None |
| `--print` / `--query` modes | Unchanged -- never enter REPL path | None |
| `--output-format json/stream-json` | Unchanged -- use existing render.go callbacks | None |
| `RunREPL()` | Kept as fallback via `--no-tui` flag | None |
| `PrintEvent` callback | Kept as-is for legacy mode | None |

---

## 4. Component Catalog

Claude Code has ~389 components. We need 25 critical components (covering 80% of the UX)
plus the theme infrastructure. Two component patterns are used:

- **Stateful `tea.Model`**: For interactive components (`Init`/`Update`/`View` lifecycle)
- **Pure render functions**: For display-only renderers (`Render(data, width) string`)

### 4.0 Component Dependency Tree

```
Level 0: Framework (Bubble Tea + Lip Gloss + Glamour + Huh + Chroma)
Level 1: Theme System (palette.go, theme.go, colors.go)        *** DONE ***
Level 2: Base Components (StyledText, TextInput, Viewport, Spinner, ModalOverlay)
Level 3: Domain Components (MessageBubble, CodeBlock, DiffViewer, ToolCallDisplay,
         SlashCommandInput, StatusLine, PermissionDialog)
Level 4: Layout Components (ConversationPane, InputPane, SidePanel, Header, StatusBar)
Level 5: Page-Level (AppShell, SessionPicker, SettingsView)
```

### 4.1 Theme System (pkg/ui/theme/) -- PARTIALLY COMPLETE

| Component | File | Status | Description |
|-----------|------|--------|-------------|
| Theme interface | `theme.go` | DONE | Interface, registry, `SetTheme()`/`Current()` |
| Palette | `palette.go` | DONE | 80+ hex color constants |
| ColorScheme | `colors.go` | DONE | 40+ semantic color roles |
| Dark theme | `dark.go` | DONE | Navy+cyan, all lipgloss styles |
| Light theme | `light.go` | DONE | White+blue, all lipgloss styles |
| High contrast | `highcontrast.go` | DONE | WCAG AAA, bold+underline |
| Dark ANSI | `dark_ansi.go` | TODO | 16-color ANSI fallback |
| Light ANSI | `light_ansi.go` | TODO | 16-color ANSI fallback |
| Auto-detect | `resolve.go` | TODO | Terminal capability detection |

### 4.2 Tier 1 -- MVP (10 components, ~2,500 lines, ~300 already exist)

These 10 components deliver a working Bubble Tea REPL that replaces the current
`bufio.Scanner` loop. This is the minimum viable TUI.

| # | Component | Type | Lines | Status | Description |
|---|-----------|------|-------|--------|-------------|
| 1 | **AppShell** | `tea.Model` | 300 | TODO | Top-level model; owns focus/mode/resize; routes messages to sub-models |
| 2 | **ConversationPane** | `tea.Model` | 350 | TODO | Wraps Bubbles viewport; holds rendered messages + active StreamingText |
| 3 | **InputPane** | `tea.Model` | 300 | TODO | Wraps Bubbles textinput; adds history (up/down), multiline (`\`), prefill |
| 4 | **MessageBubble** | Pure render | 400 | TODO | `Render(msg, width) string`; user/assistant messages with role headers |
| 5 | **StreamingText** | `tea.Model` | 300 | TODO | String buffer with `AppendDelta`/`Finish`, blinking cursor |
| 6 | **ToolCallDisplay** | Pure render | 300 | TODO | Tool name, params, pending/complete status indicator |
| 7 | **StatusLine** | `tea.Model` | 250 | TODO | Bottom bar: model name, token counts, cost, mode indicator |
| 8 | **PermissionDialog** | `tea.Model` | 150 | EXISTS | Needs upgrade from Huh form to Bubble Tea modal |
| 9 | **Spinner** | `tea.Model` | 100 | EXISTS | Needs migration from goroutine to Bubbles spinner |
| 10 | **MarkdownRenderer** | Pure render | 50 | EXISTS | Glamour wrapper; needs theme style integration |

**Design decisions for Tier 1:**
- **Theme by pointer**: All components receive `*Theme` at construction, no globals in render path
- **Width propagation**: AppShell receives `WindowSizeMsg`, calculates sub-widths, sends resize messages down
- **Focus management**: AppShell owns focus enum (`Input`/`Conversation`/`Modal`), routes `KeyMsg` to focused child
- **Modal pattern**: AppShell holds optional `modal tea.Model` that captures all input when non-nil
- **Bubbles wrapping**: We wrap Bubbles v2 components (viewport, textinput, spinner) with thin adapters, not reimplementing

### 4.3 Tier 2 -- Claude Code Parity (10 components, ~2,150 lines)

These bring visual parity with Claude Code's core UX: syntax highlighting, enhanced diffs,
slash command completion, error handling, and token usage display.

| # | Component | Type | Lines | Description |
|---|-----------|------|-------|-------------|
| 11 | **DiffViewer** | Pure render | 300 | Enhanced unified diff with hunk headers, side-by-side option |
| 12 | **CodeBlock** | Pure render | 200 | Chroma syntax highlighting with line numbers, language detection |
| 13 | **SlashCommandInput** | `tea.Model` | 300 | Prefix matching, autocomplete dropdown, Tab completion |
| 14 | **DiffApprovalDialog** | `tea.Model` | 250 | Modal showing diff, approve/reject/edit choices |
| 15 | **ErrorDisplay** | Pure render | 200 | Classified error rendering with recovery suggestions |
| 16 | **TokenUsageDisplay** | Pure render | 150 | Compact and full views of input/output/cache/cost |
| 17 | **ThinkingIndicator** | `tea.Model` | 150 | Extended thinking budget display with elapsed time |
| 18 | **NotificationToast** | `tea.Model` | 200 | Ephemeral success/error/info messages with TTL auto-dismiss |
| 19 | **SessionPicker** | `tea.Model` | 300 | Filterable session list with fuzzy search |
| 20 | **Header** | Pure render | 100 | Model name, CWD, session ID, compact/full modes |

### 4.4 Tier 3 -- Power Features (5 components, ~1,350 lines)

These are advanced features that complete the experience for power users.

| # | Component | Type | Lines | Description |
|---|-----------|------|-------|-------------|
| 21 | **SidePanel** | `tea.Model` | 300 | Collapsible side panel for sessions/tasks/files |
| 22 | **TabBar** | `tea.Model` | 200 | Switch between conversations/views |
| 23 | **TreeView** | `tea.Model` | 350 | Expandable tree for files, tasks, dependencies |
| 24 | **CommandPalette** | `tea.Model` | 300 | Ctrl+K style fuzzy command picker |
| 25 | **AgentMessageRenderer** | Pure render | 200 | Color-coded messages by agent identity |

### 4.5 Category Coverage (Claude Code's ~389 components mapped to Go)

| Category | CC Components | Go Custom Lines | Charm Coverage |
|----------|--------------|----------------|----------------|
| Core Primitives (text, box, border) | ~40 | ~600 | Lip Gloss covers 90% |
| Form & Input (text, select, confirm) | ~35 | ~1,170 | Bubbles textinput/textarea, Huh forms |
| Dialog & Modal (permission, confirm) | ~25 | ~827 | Huh v2 (3 dialogs already built) |
| Data Display (messages, code, diff) | ~55 | ~2,200 | Glamour, Chroma, Bubbles table/list |
| Status & Progress (spinner, progress) | ~30 | ~670 | Bubbles spinner/progress |
| Layout (app shell, panes, tabs) | ~35 | ~1,700 | All custom |
| Rich Content (streaming markdown) | ~40 | ~1,800 | All custom |
| Interactive (fuzzy, command palette) | ~30 | ~1,000 | Bubbles list partial |

### 4.6 Free from Bubbles v2 (saving ~5,000-8,000 lines)

These 15 production components from Bubbles v2 are wrapped, not reimplemented:
`textinput`, `textarea`, `viewport`, `list`, `table`, `spinner`, `progress`,
`paginator`, `help`, `cursor`, `filepicker`, `timer`, `stopwatch`

### 4.7 Lines of Code Summary

| Layer | New Lines | Existing |
|-------|-----------|----------|
| Framework/Theme infrastructure | 600 | 1,235 (done) |
| Tier 1 MVP (10 components) | 2,200 | 300 (exists) |
| Tier 2 Parity (10 components) | 2,150 | -- |
| Tier 3 Power (5 components) | 1,350 | -- |
| Component infrastructure (messages, interfaces) | 500 | -- |
| **Total new code** | **6,800** | |
| **Existing code retained** | | **1,535** |
| **Final TUI layer** | **~8,335 lines** | |

---

## 5. Design System Specification

### 5.1 Color Architecture

The Gopher theme system uses a three-layer approach:

```
Layer 1: Palette (pkg/ui/theme/palette.go)
  Raw hex constants: Blue900, Cyan300, Gray100, Red400, etc.
  80+ named colors organized by hue and shade

Layer 2: ColorScheme (pkg/ui/theme/colors.go)
  Semantic roles: TextPrimary, Success, DiffAdded, ToolName, etc.
  40+ named roles that map to palette constants per theme

Layer 3: Theme Styles (dark.go, light.go, etc.)
  lipgloss.Style methods: TextPrimary(), ToolCallHeader(), DiffAdded(), etc.
  Pre-composed styles with foreground, background, bold, underline
```

**Brand identity shift**: Gopher replaces Claude Code's orange brand color entirely with
blue/cyan. Primary action = AccentBlue (`#0087ff`), accent/active = Cyan300 (`#00d7ff`).

**Usage pattern** (from `pkg/ui/theme/`):
```go
c := theme.C()                                    // Access ColorScheme
header := theme.S().ToolCallHeader().Render("Bash") // Pre-built lipgloss styles
box := theme.S().BoxFocused().Width(60).Render(content)
theme.SetTheme(theme.ThemeHighContrast)            // Runtime switch (thread-safe)
```

**Gap with Claude Code**: Claude Code's `Theme` type has 89 color keys including shimmer
variants, plan mode, prompt border, Claude brand, and rainbow colors. Gopher's `ColorScheme`
currently has 40 roles. The bridge work (Phase 1 remaining) adds the missing keys:

| Missing from Gopher | Claude Code Key | Action |
|---------------------|-----------------|--------|
| Shimmer colors | `claudeShimmer`, `permissionShimmer`, etc. | Add shimmer variants to ColorScheme |
| Brand color | `claude` (orange) | Add `Brand` role |
| Plan mode | `planMode` (teal) | Add `PlanMode` role |
| Prompt border | `promptBorder`, `promptBorderShimmer` | Add `PromptBorder` roles |
| User message bg | `userMessageBackground` | Add `UserMessageBg` role |
| Fast mode | `fastMode`, `fastModeShimmer` | Add `FastMode` roles |
| Rate limit | `rate_limit_fill`, `rate_limit_empty` | Add `RateLimitFill`/`Empty` |
| Rainbow | 14 rainbow keys | Defer to Phase 5 |

### 5.2 Typography

| Role | Style | Color Key | Example |
|------|-------|-----------|---------|
| Assistant text | Normal | `TextPrimary` | Regular response text |
| User input echo | Bold | `TextPrimary` | `> user message here` |
| Tool name | Bold | `ToolName` (Cyan300 dark / Blue400 light) | `Read file.go` |
| Tool result | Normal | `Success` / `Error` | `Done (1.2s)` / `Error: not found` |
| Inactive text | Normal | `TextSecondary` | Dimmed timestamps, hints |
| Status bar | Normal | `StatusBarFg` on `StatusBarBg` | `claude-haiku-4-5 | 45% | /project` |
| Error message | Bold | `Error` | `Error: API rate limit` |
| Warning | Normal | `Warning` | `Approaching context limit` |

### 5.3 Spacing Constants

From `pkg/ui/theme/theme.go`:

```go
DefaultSpacing = Spacing{PadH: 1, PadV: 0, MarginH: 0, MarginV: 0, Gap: 1}
CompactSpacing = Spacing{PadH: 1, PadV: 0, MarginH: 0, MarginV: 0, Gap: 0}
```

| Element | Padding | Margin | Source |
|---------|---------|--------|--------|
| Pane | 2 chars horizontal | 1 line above (divider) | Pane.tsx: `paddingX={2}` |
| Box | 1 char horizontal | 0 | `DefaultSpacing.PadH` |
| Status bar | 1 char horizontal, compact | 0 | `CompactSpacing.PadH` |
| Message row | 0 | 1 blank line between | MessageRow.tsx |
| Tool use block | 2 chars indent | 0 | Indented under assistant |
| Prompt | 2 chars indent (`> `) | 1 blank line above | Standard REPL prompt |

### 5.4 Icons & Symbols

| Symbol | Unicode | Usage |
|--------|---------|-------|
| Spinner frames | `---...---` (braille dots) | Loading animation |
| Success | `✓` | Tool completed |
| Error | `✗` | Tool failed |
| Warning | `⚠` | Context warning |
| Info | `ℹ` | Informational |
| Arrow right | `▶` | Tool execution |
| Divider | `─` | Horizontal rules |
| Progress blocks | `▏▎▍▌▋▊▉█` | Progress bar |
| Bullet | `•` | List items |

---

## 6. Integration Plan (Phased)

### Phase 1: Theme + AppShell + Infrastructure (Weeks 1-3) -- PARTIALLY COMPLETE

**Goal**: Theme system complete, AppShell and core infrastructure in place.
**Components**: Theme remaining + AppShell + core infra (~1,200 lines new).

**Already Done:**
- `pkg/ui/theme/` -- 6 files, 3 themes, 1,235 lines, full lipgloss integration

**Remaining Deliverables:**
1. `dark_ansi.go` + `light_ansi.go` -- ANSI-16 fallback themes
2. `resolve.go` -- Terminal auto-detection (`$COLORTERM`, `$COLORFGBG`)
3. Add missing ColorScheme roles (shimmer, brand, plan mode, prompt border)
4. `pkg/ui/core/component.go` -- Component interface and lifecycle
5. `pkg/ui/core/keymap.go` -- Key binding registry
6. `pkg/ui/app.go` -- AppShell (top-level `tea.Model`, focus/mode/resize routing)
7. `pkg/ui/components/input.go` -- InputPane (wraps Bubbles textinput, history, multiline)
8. `pkg/ui/components/output.go` -- ConversationPane (wraps Bubbles viewport, streaming)
9. Migrate `internal/cli/render.go` from hardcoded ANSI to `theme.S()` styles
10. `internal/cli/tui_v2.go` -- New entry point

**Risk**: Zero (purely additive, no existing code changes except color values).

**Success Criteria:**
- All themes render without panic; `go test ./pkg/ui/theme/...` passes
- ANSI-16 fallback works when `$COLORTERM` is unset
- AppShell renders empty shell with input + conversation panes

### Phase 2: Tier 1 MVP -- Working REPL (Weeks 4-7) -- MEDIUM RISK

**Goal**: Complete Tier 1 (10 components). Bubble Tea REPL replaces `bufio.Scanner`.
**Components**: MessageBubble + StreamingText + ToolCallDisplay + StatusLine + Spinner (~1,500 lines new).

**Deliverables:**
1. `MessageBubble` -- Pure renderer for user/assistant messages with role headers
2. `StreamingText` -- `tea.Model` with `AppendDelta`/`Finish`, blinking cursor
3. `ToolCallDisplay` -- Pure renderer for tool name, params, pending/complete status
4. `StatusLine` -- Bottom bar: model name, token counts, cost, mode indicator
5. Spinner upgrade -- Migrate from goroutine to Bubbles spinner with `tea.Tick`
6. PermissionDialog upgrade -- Migrate from Huh to Bubble Tea modal
7. Wire `query.Query()` on background goroutine with `program.Send()` callback
8. `pkg/ui/state/appstate.go` + `querystate.go` -- State management

**Feature flag**: `GOPHER_NEW_UI=1` env var enables new UI (default off).

**Critical path**: The goroutine bridge between query loop and Bubble Tea main thread.
Query events sent via `program.Send(evt)` which is thread-safe.

**Success Criteria:**
- REPL no longer uses `bufio.Scanner` (in new UI mode)
- Spinner renders inline via `tea.Tick` (not goroutine stderr)
- Status line shows model + context % + CWD
- Terminal resize triggers re-render
- Ctrl+C cancels query without exiting
- All 5 QueryEvent types render correctly

### Phase 3: Tier 2 -- Dialogs + Rich Rendering (Weeks 8-11) -- HIGH RISK

**Goal**: Permission bridge, structured diffs, code highlighting, error handling.
**Components**: Tier 2 components 11-20 (~2,150 lines new).

**Deliverables:**
1. `DiffViewer` -- Enhanced unified diff with hunk headers
2. `CodeBlock` -- Chroma syntax highlighting with line numbers
3. `SlashCommandInput` -- Prefix matching, autocomplete, Tab completion
4. `DiffApprovalDialog` -- Modal showing diff, approve/reject/edit
5. `ErrorDisplay` -- Classified error rendering with recovery suggestions
6. `TokenUsageDisplay` -- Compact and full views of input/output/cache/cost
7. `ThinkingIndicator` -- Extended thinking budget display
8. `NotificationToast` -- Ephemeral messages with TTL auto-dismiss
9. `SessionPicker` -- Filterable session list with fuzzy search
10. `Header` -- Model name, CWD, session ID, compact/full modes
11. Channel-based `BubbleteePermissionPolicy`

**Critical path**: Permission dialog synchronization. The orchestrator goroutine
blocks waiting for user permission. The dialog renders on the Bubble Tea main thread.
A channel bridges the two:

```go
type BubbleteePermissionPolicy struct {
    program *tea.Program
}

func (p *BubbleteePermissionPolicy) Check(tool, args string) (bool, error) {
    respCh := make(chan bool, 1)
    p.program.Send(PermissionRequestMsg{Tool: tool, Args: args, Response: respCh})
    select {
    case allowed := <-respCh:
        return allowed, nil
    case <-time.After(30 * time.Second):
        return false, nil // Timeout -> deny
    }
}
```

**Success Criteria:**
- Permission dialogs work without deadlock
- FileEdit tool shows word-level diffs with add/remove colors
- Slash commands show autocomplete dropdown
- Token usage displays compact in status bar, full via `/usage`

### Phase 4: Tier 3 -- Power Features (Weeks 12-15) -- LOW RISK

**Goal**: Complete Tier 3, adding advanced navigation and agent support.
**Components**: Tier 3 components 21-25 (~1,350 lines new).

**Deliverables:**
1. `SidePanel` -- Collapsible side panel for sessions/tasks/files
2. `TabBar` -- Switch between conversations/views
3. `TreeView` -- Expandable tree for files, tasks, dependencies
4. `CommandPalette` -- Ctrl+K style fuzzy command picker
5. `AgentMessageRenderer` -- Color-coded messages by agent identity
6. `pkg/ui/layout/responsive.go` -- Width-aware breakpoints

**Success Criteria:**
- Ctrl+K opens command palette with fuzzy search
- Agent messages show distinct colors per teammate
- Side panel collapses on narrow terminals (< 100 cols)

### Phase 5: Polish & Default Flip (Weeks 16-20) -- LOW RISK

**Goal**: Visual polish, performance, edge cases. Flip `GOPHER_NEW_UI` to default-on.

**Deliverables:**
1. Logo display at startup
2. Theme picker dialog
3. Performance optimization (minimize re-renders, viewport windowing)
4. Edge cases (narrow terminal, no color, screen readers)
5. Flip default: `GOPHER_NEW_UI` defaults to enabled
6. `--no-tui` flag as permanent escape hatch
7. Integration testing (screenshot comparison)
8. Documentation

**Success Criteria:**
- Side-by-side screenshot comparison with Claude Code
- No visible flicker on 80-column terminal
- CPU usage under 5% during spinner animation
- All 40+ slash commands render with themed output
- Zero regressions in `--print` / `--query` modes

---

## 7. Testing & Validation Strategy

### 7.1 Unit Tests

| Area | Test File | What It Tests |
|------|-----------|---------------|
| Theme | `theme/theme_test.go` | All ColorScheme fields non-empty for each theme |
| Theme | `theme/resolve_test.go` | Terminal detection, fallback chain |
| Components | `components/*_test.go` | Render output matches expected ANSI |
| Divider | `components/divider_test.go` | Width, title centering, color |
| ProgressBar | `components/progressbar_test.go` | Ratio to block char mapping |
| Spinner | `components/spinner_test.go` | Frame progression |
| StatusBar | `components/statusbar_test.go` | Token formatting, context % |
| Diff | `components/diff_test.go` | Hunk parsing, word-level |
| Permission | Channel-based policy | Timeout, allow, deny flows |

### 7.2 Golden File Tests

Golden file testing compares rendered output against committed reference files:

```go
func TestDarkThemeToolCall(t *testing.T) {
    th := theme.Current() // dark by default
    output := components.RenderToolCall(th, "Read", "file.go")
    golden.Assert(t, output, "testdata/dark_tool_call.golden")
}
```

Golden files stored in `pkg/ui/testdata/`:
- `dark_tool_call.golden`
- `light_tool_result.golden`
- `dark_diff_hunk.golden`
- `statusbar_80col.golden`
- `permission_dialog.golden`

### 7.3 Integration Tests

| Test | Description |
|------|-------------|
| Theme round-trip | Load theme -> render component -> verify ANSI codes match |
| Query bridge | Spawn mock query -> verify events arrive in Bubble Tea model |
| Permission flow | Send PermissionRequestMsg -> respond -> verify unblock |
| Resize handling | Set 40-col -> render -> set 120-col -> verify reflow |
| Color fallback | Set `TERM=xterm` -> verify ANSI-16 used |
| Spinner perf | Run spinner 1000 frames, measure CPU |

### 7.4 Parity Tests

Extend the existing `pkg/tools/*_parity_test.go` pattern to UI:

```go
func TestColorSchemeCompleteness(t *testing.T) {
    // Verify no ColorScheme field is empty string in any theme
    for _, name := range theme.ListThemes() {
        theme.SetTheme(name)
        c := theme.C()
        v := reflect.ValueOf(c)
        for i := 0; i < v.NumField(); i++ {
            field := v.Type().Field(i)
            val := v.Field(i).String()
            assert.NotEmpty(t, val, "theme %s: ColorScheme.%s is empty", name, field.Name)
        }
    }
}
```

### 7.5 Slash Command Parity

Every existing slash command in `repl.go` must work identically in the new TUI:

| Command | Old Implementation | New Implementation |
|---------|-------------------|-------------------|
| `/help` | `printHelp()` | Themed Pane with tab navigation |
| `/cost` | `fmt.Printf` | Status bar update |
| `/model` | `fmt.Printf` | Fuzzy picker dialog |
| `/theme` | `fmt.Printf` | Theme picker dialog |
| `/diff` | `exec.Command("git")` | Themed diff viewer |
| `/compact` | `query.CompactSession` | Same + visual feedback |
| All others | Direct printf | Themed output |

---

## 8. Risk Assessment

### 8.1 Technical Risks

| Risk | Severity | Likelihood | Mitigation |
|------|----------|-----------|------------|
| Permission dialog deadlock | High | Medium | Channel with 30s timeout; fallback to deny |
| Goroutine leak in query-to-UI bridge | High | Low | Context cancellation, deferred cleanup |
| Terminal state corruption on crash | Medium | Low | `tea.Program` cleanup + SIGTERM handler |
| Bubble Tea performance with long conversations | Medium | Medium | Viewport windowing; only render visible |
| Color inconsistency across terminals | Medium | Medium | ANSI fallback themes; test on 4+ terminals |
| Lip Gloss doesn't support all ANSI features | Low | Low | Raw ANSI escape for unsupported features |
| Large output performance | Low | Low | Bubbles viewport handles windowing |

### 8.2 Scope Risks

| Risk | Severity | Likelihood | Mitigation |
|------|----------|-----------|------------|
| Claude Code UI changes during development | Medium | High | Pin to v2.1.88 reference; catch up in Phase 5 |
| Shimmer animation requires custom terminal code | Medium | Medium | Degrade to static color on limited terminals |
| Slash command parity regression | Low | Medium | Golden file tests for every command |

### 8.3 Rollback Strategy

1. `GOPHER_NEW_UI=1` env var enables new UI (default off during Phases 1-4)
2. `--no-tui` flag forces old REPL even after default flip in Phase 5
3. Old `repl.go`, `render.go`, and all existing callbacks remain **unchanged throughout**
4. Each phase is independently deployable/revertable
5. Phase 3 can fall back to Phase 2's simpler permission flow (Huh forms)
6. If new TUI has critical bugs, revert to old REPL with zero code changes

---

## 9. Effort Estimation

### 9.1 Breakdown by Phase

| Area | Lines | Person-Weeks | Phase |
|------|-------|-------------|-------|
| Theme remaining (ANSI, auto-detect, bridge roles) | ~400 | 2 | 1 |
| Core framework (component.go, keymap.go) | ~300 | 1 | 1 |
| AppShell + InputPane + ConversationPane | ~950 | 3 | 1 |
| Migrate existing CLI to theme-aware + tui_v2.go | ~200 | 2 | 1 |
| **Phase 1 subtotal** | **~1,850** | **8** | |
| MessageBubble + StreamingText + ToolCallDisplay | ~1,000 | 4 | 2 |
| StatusLine + Spinner upgrade | ~350 | 2 | 2 |
| PermissionDialog upgrade + state management | ~450 | 2 | 2 |
| Query bridge (program.Send callback) | ~200 | 2 | 2 |
| **Phase 2 subtotal** | **~2,000** | **10** | |
| DiffViewer + CodeBlock + DiffApprovalDialog | ~750 | 3 | 3 |
| SlashCommandInput + ErrorDisplay + Header | ~700 | 2 | 3 |
| Permission bridge (BubbleteePermissionPolicy) | ~200 | 2 | 3 |
| TokenUsageDisplay + ThinkingIndicator + Toast | ~500 | 2 | 3 |
| SessionPicker | ~300 | 1 | 3 |
| **Phase 3 subtotal** | **~2,450** | **10** | |
| SidePanel + TabBar + TreeView | ~850 | 3 | 4 |
| CommandPalette + AgentMessageRenderer | ~500 | 2 | 4 |
| Responsive layout | ~200 | 1 | 4 |
| **Phase 4 subtotal** | **~1,550** | **6** | |
| Logo, theme picker, perf optimization | ~400 | 2 | 5 |
| Edge cases + default flip + testing | ~300 | 2 | 5 |
| Documentation | -- | 2 | 5 |
| **Phase 5 subtotal** | **~700** | **6** | |
| | | | |
| **TOTAL** | **~8,550 new** | **40** | |

### 9.2 Timeline Scenarios

| Team Size | Duration | Notes |
|-----------|----------|-------|
| 1 person | 40 weeks (~10 months) | Sequential; high risk of burnout |
| 2 people | 21 weeks (~5 months) | Person A: Phase 1-2; Person B: Phase 3-4 prep |
| 3 people | 14 weeks (~3.5 months) | Optimal; theme/components/integration parallel |
| 4 people | 11 weeks (~2.75 months) | Diminishing returns on UI coordination |

---

## 10. Decision Points

### 10.1 Critical Questions

1. **Replace Huh or wrap it?** Current `permission_dialog.go` uses Huh forms. Should we replace Huh entirely with custom Bubble Tea dialogs, or keep Huh as fallback?
   - **Recommendation**: Keep Huh as `--no-tui` fallback. Replace with custom dialogs in new TUI.

2. **Theme location: `pkg/ui/theme/` or `internal/cli/theme/`?**
   - **Decision made**: `pkg/ui/theme/` (already implemented there). This allows other packages to use theme colors.

3. **When to flip the default?** When should `GOPHER_NEW_UI` default to enabled?
   - **Recommendation**: Phase 5, after all slash commands and permission flows are verified.

4. **Shimmer animation: worth the complexity?** Claude Code's spinner uses color interpolation.
   - **Recommendation**: Implement solid-color spinner first (Phase 2). Add shimmer in Phase 5.

5. **go-diff vs custom diff?** The existing `diff.go` is naive line-by-line.
   - **Recommendation**: `go-diff` for hunk computation, custom rendering via theme styles.

### 10.2 Trade-offs

| Trade-off | Option A | Option B | Recommendation |
|-----------|----------|----------|----------------|
| Scope vs. timeline | Full parity (20 weeks) | Core parity (12 weeks) | Full -- the UI is the product |
| Custom vs. library | Build all components custom | Use Bubbles (viewport, textinput, spinner) | Mix: custom theme, Bubbles for standard widgets |
| Exact pixel parity vs. Go-idiomatic | Match TS output byte-for-byte | Adapt to Go terminal idioms | Match visual feel, not exact bytes |
| Risk vs. rollback | Ship incrementally with feature flag | Big-bang replacement | Feature flag (already decided) |

### 10.3 Go/No-Go Criteria per Phase

| Phase | Go | No-Go |
|-------|-----|-------|
| 1 -> 2 | All themes render, existing tests pass, ANSI fallback works | Color rendering broken on any major terminal |
| 2 -> 3 | Bubble Tea REPL works for basic query-response cycle | Spinner causes CPU spikes; input drops keystrokes |
| 3 -> 4 | Permission dialog works without deadlock | Goroutine leak detected; permission timeout > 1% of requests |
| 4 -> 5 | Structured diffs render correctly, slash commands work | Diff rendering crashes on large files |
| 5 -> Ship | Screenshot parity, perf targets met, all tests pass | Any regression in existing `--print`/`--query` functionality |

---

## 11. Success Criteria

### 11.1 Production Readiness Checklist

- [ ] All themes render with all ColorScheme fields populated
- [ ] Theme auto-detection works (dark/light terminal)
- [ ] ANSI-16 fallback works on basic terminals
- [ ] Spinner animation smooth (12+ FPS, < 5% CPU)
- [ ] Status line shows model, context %, CWD, permission mode
- [ ] Prompt input supports multi-line, history, paste
- [ ] Permission dialog matches allow/deny/always flow (no deadlocks)
- [ ] Tool use shows tool name in brand color with spinner
- [ ] Tool results truncate at 200 chars with expand
- [ ] Structured diffs with word-level highlighting
- [ ] Markdown renders with theme-appropriate colors
- [ ] Terminal resize triggers proper re-render
- [ ] Ctrl+C cancels query, double Ctrl+C exits
- [ ] No visible flicker on 80x24 terminal
- [ ] Works on macOS Terminal, iTerm2, Alacritty, kitty
- [ ] All existing parity tests still pass
- [ ] No regression in `--print`, `--query`, `--output-format` modes
- [ ] `--no-tui` flag correctly falls back to old REPL
- [ ] All 40+ slash commands work in new TUI

### 11.2 Performance Targets

| Metric | Target | Measurement |
|--------|--------|-------------|
| Spinner FPS | >= 12 FPS | `tea.Tick` interval 80ms |
| Render latency | < 16ms per frame | `pprof` profiling |
| CPU during idle | < 1% | `top` measurement |
| CPU during spinner | < 5% | `top` measurement |
| Memory baseline | < 50MB | `runtime.MemStats` |
| Startup time | < 200ms to first prompt | `time gopher-code --version` |
| Permission timeout | < 0.1% of requests | Logging in production |
| Resize response | < 100ms | Visual observation |

---

## 12. Conclusion

**READY TO IMPLEMENT: YES**

The Gopher Code backend (query loop, tools, sessions, permissions) is fully implemented with
comprehensive parity tests. The UI layer is the primary remaining gap. This proposal defines
a clear, phased path from the current minimal CLI to full Claude Code visual parity.

**Work has already begun**: The theme system at `pkg/ui/theme/` (6 files, 3 themes, full
lipgloss integration) provides a solid foundation. Phase 1 is ~60% complete.

The integration strategy is low-risk by design: the `EventCallback` seam means zero changes
to the query loop, tools, or provider. The `GOPHER_NEW_UI` feature flag and `--no-tui`
fallback provide safe rollback at every phase.

**ESTIMATED EFFORT: 40 person-weeks**

**TIMELINE: 40 weeks (1 person), 21 weeks (2 people), 14 weeks (3 people)**

**NEXT STEPS:**

1. **Immediate (this week)** -- Complete Phase 1 foundation:
   - Add `dark_ansi.go` and `light_ansi.go` to `pkg/ui/theme/`
   - Add `resolve.go` for terminal auto-detection
   - Add `theme_test.go` verifying all ColorScheme fields non-empty across all themes
   - Add missing ColorScheme roles (shimmer, brand, plan mode, prompt border)

2. **Week 2**:
   - Build `pkg/ui/core/component.go` and `pkg/ui/core/keymap.go`
   - Build `pkg/ui/components/divider.go` and `progressbar.go`
   - Migrate `internal/cli/render.go`, `diff.go`, `spinner.go` to theme-aware

3. **Week 3-4** (Phase 2 start):
   - Build `pkg/ui/app.go` -- Root Bubble Tea model
   - Build `pkg/ui/components/input.go` -- Multi-line input with history
   - Build `internal/cli/tui_v2.go` -- Wire query.Query() with program.Send()
   - Build `pkg/ui/components/statusbar.go` with context % display

4. **Week 5-7**:
   - Build `pkg/ui/components/output.go` -- Streaming output viewport
   - Build `pkg/ui/components/spinner.go` -- Inline Bubbles spinner
   - `GOPHER_NEW_UI=1` available for testing
