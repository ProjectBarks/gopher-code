# Gopher UI Redesign - Integration Plan

**Status**: Delivered by Integration Strategist Agent  
**Date**: 2026-04-03  
**Phase**: Planning & Proposal

---

## Executive Summary

The integration plan outlines how to add a sophisticated Bubbletea-based UI to Gopher while maintaining 100% backward compatibility with the existing system. The key insight: **the query loop already has an EventCallback seam** that makes this integration elegant.

**Key Points:**
- **Primary seam**: `query.Query()` accepts `EventCallback func(QueryEvent)` — perfect integration point
- **All dependencies ready**: Bubbletea v2, Lipgloss v2, Bubbles v2, Huh v2 already in go.mod
- **Phased approach**: 5 phases from framework to polish
- **Risk mitigation**: Feature flags allow toggling between old/new UI
- **Effort**: Estimated 16-20 person-weeks for full implementation

---

## 1. Current Architecture Summary

### Entry Points
- **`cmd/gopher-code/main.go`** (582 lines) — CLI argument parsing, setup, calls either:
  - `query.Query()` for headless/print modes
  - `cli.RunREPL()` for interactive mode

- **`internal/cli/repl.go`** (541 lines) — Synchronous REPL loop:
  - `bufio.Scanner` reads user input
  - 40+ slash commands handled inline
  - Calls `query.Query()` with `PrintEvent` callback
  - Auto-saves session after each turn

### The Query Loop (The Key Integration Seam!)
**Location**: `pkg/query/query.go`

```go
func Query(ctx context.Context, sess *SessionState, prov ModelProvider, 
    registry *ToolRegistry, orch ToolOrchestrator, 
    onEvent EventCallback) error
```

The `onEvent` callback is **the integration point**. Currently called with:
- `QEventTextDelta` — Streaming text received
- `QEventToolUseStart` — Tool execution starting
- `QEventToolResult` — Tool result received
- `QEventTurnComplete` — Turn finished
- `QEventUsage` — Token usage reported

This is UI-agnostic. Perfect for Bubbletea integration via:
```go
onEvent := func(evt QueryEvent) {
    program.Send(evt)  // Inject into Bubbletea message loop
}
```

### Existing CLI Components
**`internal/cli/`** (1,200 lines total):
- `render.go` — `PrintEvent` callback, JSON output handlers
- `spinner.go` — Goroutine-based spinner
- `statusbar.go` — Task pills with mutex state
- `permission_dialog.go` — Huh forms for dialogs
- `diff.go` — ANSI diff renderer
- `markdown.go` — Glamour markdown rendering
- `tui.go` — Basic Bubbletea model (118 lines, mostly placeholder)

### Dependencies Already in go.mod
✅ `charm.land/bubbletea/v2 v2.0.2`  
✅ `charm.land/bubbles/v2 v2.1.0`  
✅ `charm.land/lipgloss/v2 v2.0.2`  
✅ `charm.land/huh/v2 v2.0.3`  
✅ `charm.land/glamour/v2 v2.0.0`  
✅ `github.com/charmbracelet/x/ansi v0.11.6`  

**No new dependencies needed.** Framework is ready.

---

## 2. Proposed File Structure

```
pkg/ui/                    ← New package: all UI code
  core/
    component.go           ← Component interface, BaseComponent
    layout.go              ← Flex, stack, grid layouts
    focus.go               ← Focus ring, tab order
    keymap.go              ← Keyboard bindings registry
  
  components/
    input.go               ← Multi-line text input
    output.go              ← Scrollable output viewport
    toolbar.go             ← Top status bar (model, session, mode)
    statusbar.go           ← Bottom status bar (tokens, cost, pills)
    toolpanel.go           ← Tool execution display
    markdown.go            ← Glamour viewport
    diff.go                ← Diff viewer
    table.go               ← Data table
    dialog.go              ← Modal dialogs
    spinner.go             ← Inline spinner
    toast.go               ← Transient notifications
  
  layout/
    split.go               ← Horizontal/vertical splits
    stack.go               ← Layered stack (main + modals)
    responsive.go          ← Width-aware breakpoints
  
  theme/
    theme.go               ← Theme struct, presets
    dark.go                ← Dark theme (BLUE primary)
    light.go               ← Light theme
    styles.go              ← Lipgloss style factories
  
  state/
    appstate.go            ← Global app state (page, modals)
    querystate.go          ← Active query state
    uistore.go             ← Observable state store
  
  app.go                   ← Top-level Bubbletea model

internal/cli/
  tui_v2.go               ← New entry point (replaces RunREPL)
  tui_old.go              ← Keep old REPL as fallback
  repl.go                 ← Keep as-is
  render.go               ← Keep as-is (for --print mode)
```

---

## 3. Data Flow Architecture

```
┌─────────────────────────────────────────────────────┐
│                  Bubbletea Program                    │
│  (main goroutine — owns terminal)                    │
│                                                       │
│  ┌──────────┐   ┌────────────┐   ┌──────────────┐   │
│  │  Input    │──>│  AppModel  │──>│  Output      │   │
│  │Component  │   │  Update()  │   │  Component   │   │
│  └──────────┘   └─────┬──────┘   └──────────────┘   │
│                        │                              │
│              (on Enter key)                           │
│                        ▼                              │
│              ┌─────────────────┐                     │
│              │ Start query in  │                     │
│              │ background      │                     │
│              │ goroutine       │                     │
│              └────────┬────────┘                     │
│                       │ (background goroutine)       │
│                       ▼                              │
│              ┌──────────────────────┐               │
│              │ query.Query(ctx,     │               │
│              │   ..., onEvent)      │               │
│              └────────┬─────────────┘               │
│                       │ onEvent := func(evt) {      │
│                       │   program.Send(evt)         │
│                       │ }                           │
│                       ▼                              │
│              [QueryEvent messages]                  │
│                       │                              │
│              ┌────────┴──────────┐                  │
│              ▼                   ▼                   │
│         TextDelta            ToolStart              │
│         ToolResult           TurnComplete           │
│         Usage                ...                    │
│                                                       │
│        ↑                                             │
│        │ program.Send(QueryEvent)                   │
│        │                                             │
│  ┌─────┴──────────────────────────┐                │
│  │ AppModel.Update(msg tea.Msg)   │                │
│  │ - Dispatch QueryEvent to       │                │
│  │   active component             │                │
│  │ - Update component state       │                │
│  └────────────────────────────────┘                │
│                                                       │
└─────────────────────────────────────────────────────┘
```

**Key insight**: Query loop runs on **background goroutine**, Bubbletea owns **main thread**. Communication via channels and `program.Send()`.

---

## 4. Integration Phases

### Phase 1: Core Framework (Weeks 1-4)
**Goal**: Build the component system, establish patterns

**Files to create**:
- `pkg/ui/core/component.go` — Component interface
- `pkg/ui/core/layout.go` — Layout engine
- `pkg/ui/core/focus.go` — Focus management
- `pkg/ui/theme/theme.go` — Theme system
- `pkg/ui/theme/styles.go` — Lipgloss factories
- `pkg/ui/app.go` — Skeleton AppModel

**Deliverables**:
- Component interface working
- Layout math correct
- Theme system operational
- Can render a simple component

**Risk**: Low (purely additive)

### Phase 2: Basic REPL in New UI (Weeks 5-8)
**Goal**: Replace `RunREPL()` with Bubbletea version

**Files to create**:
- `pkg/ui/components/input.go` — Text input
- `pkg/ui/components/output.go` — Output viewport
- `pkg/ui/components/statusbar.go` — Status line
- `pkg/ui/state/appstate.go` — App state
- `pkg/ui/state/querystate.go` — Query state
- `internal/cli/tui_v2.go` — Entry point

**Key integration**:
- Wire `EventCallback` → `program.Send(QueryEvent)`
- AppModel dispatches events to components
- Input handles slash commands

**Wire-up in main.go**:
```go
if os.Getenv("GOPHER_NEW_UI") != "" {
    cli.RunTUIV2(ctx, sess, prov, registry, ...)
} else {
    cli.RunREPL(ctx, sess, prov, registry, ...)
}
```

**Deliverables**:
- Fully functional REPL in Bubbletea
- Same slash commands
- All existing features working
- Old REPL still available as fallback

**Risk**: Medium (goroutine lifecycle, async/sync bridge)

### Phase 3: Interactive Components (Weeks 9-12)
**Goal**: Permission dialogs, tool progress, modals

**Files to create**:
- `pkg/ui/components/dialog.go` — Modal system
- `pkg/ui/components/toolpanel.go` — Tool display
- `pkg/ui/layout/stack.go` — Modal overlay stack

**Critical challenge**: Permission dialog async/sync bridge
- Current: `permissions.InteractivePolicy` → blocking Huh form
- New: Need channel-based `BubbleteePermissionPolicy`
- Orchestrator blocks on channel read
- UI sends response via channel when user confirms

**Deliverables**:
- Permission dialogs working
- Tool execution visible
- Modal system operational

**Risk**: High (goroutine synchronization)

### Phase 4: Advanced Components (Weeks 13-16)
**Goal**: Rich rendering (markdown, diffs, tables)

**Files to create**:
- `pkg/ui/components/markdown.go` — Glamour viewport
- `pkg/ui/components/diff.go` — Diff viewer
- `pkg/ui/components/table.go` — Data table
- `pkg/ui/layout/split.go` — Split panes

**Leverage**: Existing `internal/cli/markdown.go`, `internal/cli/diff.go`

**Deliverables**:
- Markdown rendered beautifully
- Diffs syntax-highlighted
- Tables interactive

**Risk**: Low (rendering improvements)

### Phase 5: Polish & Performance (Weeks 17-20)
**Goal**: Responsive layout, smooth interaction, session picker

**Deliverables**:
- Responsive to terminal width changes
- Session picker/switcher
- Settings page
- Keyboard shortcuts reference
- Performance optimizations

**Risk**: Low (refinement)

---

## 5. Module Responsibilities

### `pkg/ui/core/` — Framework Layer
- **Component interface**: `Init()`, `Update(tea.Msg)`, `View()`, `SetSize(w,h)`, `Focused() bool`
- **Layout engine**: Takes children + constraints, returns positions
- **Focus manager**: Tracks active component, Tab key cycling
- **Keymap registry**: Maps keys to actions, modal overrides

### `pkg/ui/components/` — Built Components
Each is a self-contained Bubbletea sub-model:
- **InputComponent** — Multi-line textarea with history
- **OutputComponent** — Scrollable viewport, auto-append on TextDelta
- **ToolPanel** — Shows tool name, spinner during execution, result summary
- **Dialog** — Modal for permissions, confirmations, input
- **StatusBar** — Token count, cost, background task pills

### `pkg/ui/state/` — State Management
- **AppState** — Current page (REPL, session picker), open modals
- **QueryState** — Is query running, streamed text, active tools, usage
- **UIStore** — Wraps state, can notify program of updates

### `pkg/ui/app.go` — Top-Level Coordinator
- Implements `tea.Model`
- Routes messages to active page/component
- Manages modal stack
- Holds references to `SessionState`, `ModelProvider`, `ToolRegistry`

### `internal/cli/tui_v2.go` — Entry Point
- Called from `main.go` instead of `RunREPL()`
- Creates `pkg/ui.AppModel`
- Starts `tea.NewProgram()`

---

## 6. Backward Compatibility

| Component | Strategy |
|---|---|
| `query.Query()` | **Unchanged** — EventCallback is the seam |
| `tools.ToolOrchestrator` | **Unchanged** — New permission policy implements same interface |
| `session.SessionState` | **Unchanged** — UI reads/writes same state |
| `--print` / `--query` | **Unchanged** — Never enter REPL path |
| `--output-format` | **Unchanged** — Use existing render.go |
| `RunREPL()` | **Kept** — Available via env var `GOPHER_NEW_UI` toggle |
| Slash commands | **Ported** — Same commands, dispatched from input component |

**Rollback Strategy**:
1. Default: old REPL
2. Env var `GOPHER_NEW_UI=1` → new UI
3. If issues, set `GOPHER_NEW_UI=0` → back to old
4. Phase 3+ can be toggled independently

---

## 7. Risk Assessment

| Risk | Severity | Mitigation |
|---|---|---|
| Goroutine leak in query→UI bridge | High | Context cancellation, integration tests |
| Permission dialog deadlock | High | Channel with timeout, fallback to deny |
| Terminal corruption on crash | Medium | Alternate screen mode, signal handler |
| Rendering artifacts | Medium | Test on multiple terminals |
| Slash command regression | Low | Port all tests, golden file tests |
| Performance with large outputs | Low | Viewport windowing (Bubbles handles) |

---

## 8. Effort Estimation

| Phase | Task | Effort |
|---|---|---|
| 1 | Core framework | 3 weeks |
| 2 | Basic REPL | 4 weeks |
| 3 | Interactive components | 4 weeks |
| 4 | Advanced components | 3 weeks |
| 5 | Polish & performance | 2 weeks |
| **Total** | | **16 weeks** |

**Timeline**:
- **1 person**: 4 months
- **2 people**: 2 months
- **3 people**: 6-7 weeks
- **4 people**: 5-6 weeks

---

## 9. Implementation Order

**Start with Phase 1** — zero risk, establishes patterns.

**Critical path**: Phase 2 (REPL replacement) → Phase 3 (permission bridge)

The permission dialog bridge is the hardest part. Solve it in Phase 3, then the rest flows naturally.

---

## 10. Key Files That Won't Change

✅ `pkg/query/query.go` — Agent loop unchanged  
✅ `pkg/tools/orchestrator.go` — Tool execution unchanged  
✅ `pkg/session/session.go` — Session state unchanged  
✅ `pkg/provider/anthropic.go` — API client unchanged  

Only the **UI layer** is being rebuilt. Everything else stays intact.

---

## Decision Points

**Before Phase 1**: 
- ✓ Is this approach acceptable? (EventCallback seam, phased rollout, backward compat)

**Before Phase 2**:
- ✓ Ready to commit to Bubbletea v2 as the framework?
- ✓ Accept that old REPL will be kept as fallback?

**Before Phase 3**:
- ✓ Accept the permission dialog async/sync bridge complexity?
- ✓ Resources committed to solve goroutine sync issues?

**Before Phase 4+**:
- ✓ Approved to add rich component support?

---

## Next Steps

1. Review this integration plan
2. Approve phased approach
3. Allocate resources (1-4 engineers)
4. Start Phase 1 (core framework)
5. Target: Complete framework by end of week 4

