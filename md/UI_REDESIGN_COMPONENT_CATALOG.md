# Gopher Component Catalog & Implementation Roadmap

**Delivered by**: Component Designer Agent  
**Date**: 2026-04-03  
**Scope**: 25 critical components to match Claude Code UX with blue color scheme

---

## Executive Summary

- **Components needed**: 25 critical (Tier 1+2), 5 polish (Tier 3)
- **Estimated implementation**: 6,630 lines of Go
- **Reuse from Charm v2**: 15 components free (textinput, textarea, viewport, list, table, spinner, progress, etc.)
- **Result**: TUI layer grows from 1,197 → 7,800 lines (still lean vs Claude Code's 110K)
- **Build order**: 5 phases, each deployable independently

---

## Component Dependency Tree

```
Bubble Tea + Lip Gloss + Glamour + Huh (Framework)
    ↓
Theme System (palette, theme, colors)
    ↓
Base Components (StyledText, TextInput, Viewport, Spinner)
    ↓
Domain Components (MessageBubble, ToolDisplay, DiffViewer, MarkdownRenderer)
    ↓
Layout Components (AppShell, ConversationPane, InputPane, Header, StatusBar)
    ↓
Page-Level (REPL view, Session picker, Settings)
```

---

## The 25 Critical Components

### Tier 1: Absolute MVP (10 components) — 2,200 lines

| # | Component | Purpose | Lines | Status |
|---|-----------|---------|-------|--------|
| 1 | **AppShell** | Main layout coordinator | 300 | New |
| 2 | **ConversationPane** | Scrollable message history | 350 | New |
| 3 | **InputPane** | User input with history | 300 | New |
| 4 | **MessageBubble** | Render single message (user/assistant) | 400 | New |
| 5 | **StreamingText** | Buffer text with cursor during stream | 300 | New |
| 6 | **ToolCallDisplay** | Show tool execution (pending/complete) | 300 | New |
| 7 | **StatusLine** | Bottom bar (model, tokens, cost, mode) | 250 | Enhance |
| 8 | **PermissionDialog** | Allow/deny/always prompt | 150 | Exists |
| 9 | **Spinner** | Animated loading indicator | 100 | Exists |
| 10 | **MarkdownRenderer** | Glamour integration | 50 | Exists |

**Subtotal Tier 1**: 2,500 lines (300 already exist, 2,200 new)

### Tier 2: Claude Code Parity (10 components) — 1,980 lines

| # | Component | Purpose | Lines | Status |
|---|-----------|---------|-------|--------|
| 11 | **DiffViewer** | Unified/side-by-side diffs with colors | 300 | Enhance |
| 12 | **CodeBlock** | Syntax highlighting with line numbers | 200 | Enhance |
| 13 | **SlashCommandInput** | Autocomplete for /command | 300 | New |
| 14 | **DiffApprovalDialog** | Show diff, approve/reject | 250 | New |
| 15 | **ErrorDisplay** | Classified errors with suggestions | 200 | New |
| 16 | **TokenUsageDisplay** | Input/output/cache token tracking | 150 | New |
| 17 | **ThinkingIndicator** | Extended thinking budget display | 150 | New |
| 18 | **NotificationToast** | Ephemeral success/error messages | 200 | New |
| 19 | **SessionPicker** | Fuzzy search + select prior sessions | 300 | New |
| 20 | **Header** | Top bar with model/session/cwd | 100 | New |

**Subtotal Tier 2**: 2,150 lines (170 already exist, 1,980 new)

### Tier 3: Power Features (5 components) — 1,350 lines

| # | Component | Purpose | Lines | Status |
|---|-----------|---------|-------|--------|
| 21 | **SidePanel** | Session list, task list, file tree | 300 | New |
| 22 | **TabBar** | Switch between conversations | 200 | New |
| 23 | **TreeView** | Hierarchical file/task display | 350 | New |
| 24 | **CommandPalette** | Searchable command picker | 300 | New |
| 25 | **AgentMessageRenderer** | Color messages by agent identity | 200 | New |

**Subtotal Tier 3**: 1,350 lines (all new)

---

## Charm v2 Components We Get Free

These are **production-ready** and we just wrap them:

- `textinput` — single-line input with cursor
- `textarea` — multi-line editing
- `viewport` — scrollable pane (core to conversation display)
- `list` — filterable, scrollable list
- `table` — column-aligned data
- `spinner` — animated loading (multiple styles)
- `progress` — progress bar with percentage
- `paginator` — pagination control
- `help` — keybinding help display
- `cursor` — blinking cursor model
- `filepicker` — file system browser
- `timer` / `stopwatch` — time tracking
- `keys` — key event model
- `dropdown` — from huh v2

**Impact**: Saves ~5,000-8,000 lines of custom code. We're reusing 90% of what we need.

---

## Phase-by-Phase Implementation

### Phase 1: Foundation (Weeks 1-3) — 1,200 lines

**Components**:
- Theme system (palette, theme, colors) — 600 lines
- AppShell — 300 lines
- ConversationPane (wraps Bubbles viewport) — 350 lines
- InputPane (wraps Bubbles textinput) — 300 lines
- StatusLine (wraps Bubbles status) — 250 lines

**Deliverable**: A working TUI with input, scrollable output, theming, status bar

**Risk**: Low (all components independent, no complex interactions yet)

### Phase 2: Content (Weeks 4-7) — 1,500 lines

**Components**:
- MessageBubble — 400 lines
- StreamingText — 300 lines
- ToolCallDisplay — 300 lines
- ToolResultDisplay — 300 lines
- CodeBlock — 200 lines

**Deliverable**: Full conversation display with streaming, tool calls, code rendering

**Risk**: Low-Medium (streaming coordination between AppModel and goroutine)

### Phase 3: Status & Dialogs (Weeks 8-10) — 1,100 lines

**Components**:
- DiffViewer (enhance existing diff.go) — 300 lines
- ErrorDisplay — 200 lines
- TokenUsageDisplay — 150 lines
- ThinkingIndicator — 150 lines
- NotificationToast — 200 lines
- DiffApprovalDialog — 250 lines

**Deliverable**: Rich error handling, status displays, approval dialogs

**Risk**: Low (mostly rendering, few state transitions)

### Phase 4: Interactive (Weeks 11-13) — 800 lines

**Components**:
- SlashCommandInput — 300 lines
- SessionPicker — 300 lines
- Header (enhance) — 100 lines
- InputWithHistory — 200 lines

**Deliverable**: Full slash command system, session management

**Risk**: Medium (autocomplete, async completion, session discovery)

### Phase 5: Power Features (Weeks 14-16) — 1,350 lines

**Components**:
- SidePanel — 300 lines
- TabBar — 200 lines
- TreeView — 350 lines
- CommandPalette — 300 lines
- AgentMessageRenderer — 200 lines

**Deliverable**: Power-user features, agent-aware rendering

**Risk**: Low (all independent features)

---

## Component Deep Dives (Tier 1)

### 1. AppShell — The Container

```go
type AppShell struct {
    // Sub-models
    header       Header
    conversation ConversationPane
    input        InputPane
    statusLine   StatusLine
    modal        tea.Model // nil = no modal shown
    
    // State
    focus    FocusArea // Input, Conversation, Modal
    mode     AppMode   // Idle, Streaming, ToolRunning, PermissionPending
    width    int
    height   int
    theme    *Theme
}

// Handles: WindowSizeMsg, KeyMsg, all child messages
// Routing: WindowSize → resize all children
//          KeyMsg → route to focused child
//          Child messages → handle or propagate
// Rendering: Header | Conversation | InputPane | StatusLine (or modal overlay)
```

**Key insight**: AppShell is the "Coordinator Model" — it owns layout, focus, modals. Sub-components are dumb and only render what they're given.

### 2. ConversationPane — The Message History

```go
type ConversationPane struct {
    viewport  viewport.Model    // From Bubbles, handles scrolling
    messages  []RenderedMessage
    streaming *StreamingText    // non-nil during active streaming
    width     int
    theme     *Theme
}

type RenderedMessage struct {
    Role     string  // "user" or "assistant"
    Rendered string  // pre-rendered content (may be multi-line)
    Height   int     // how many lines this message consumes
}

// Handles: StreamDeltaMsg, AppendMessageMsg, ScrollMsg
// Rendering: Join messages with separators, render to viewport
```

**Key insight**: Messages are pre-rendered to strings before being added to viewport. This makes scrolling smooth (no re-rendering on every frame).

### 3. InputPane — User Text Entry

```go
type InputPane struct {
    textinput textinput.Model  // From Bubbles
    history   []string
    histIdx   int
    multiline bool
    theme     *Theme
}

// Handles: KeyMsg (Enter=submit, Up/Down=history, \=multiline)
// Emits: SubmitMsg{Text: string}
// Rendering: Shows prompt ("> "), renders input, shows history hint
```

**Key insight**: Wraps Bubbles textinput but adds history + multiline support.

### 4. MessageBubble — The Renderer

```go
type MessageBubble struct {
    theme *Theme
}

func (mb *MessageBubble) Render(msg message.Message, width int) string {
    // For user messages:
    //   "> " prefix, dim styling, word wrap to width
    // For assistant messages:
    //   Iterate content blocks:
    //     TextBlock → render via Glamour (markdown)
    //     ToolUseBlock → render via ToolCallDisplay
    //     ToolResultBlock → render via ToolResultDisplay
    //   Add metadata footer (tokens, cost)
    // All styled via theme colors
}
```

**Key insight**: Pure function (not a `tea.Model`). Takes data, returns string. No state.

### 5. StreamingText — The Live Buffer

```go
type StreamingText struct {
    buffer    strings.Builder
    cursor    rune         // '▌' blinking cursor
    width     int
    theme     *Theme
    cursorOn  bool         // toggled on Tick
}

func (st *StreamingText) AppendDelta(text string) {
    st.buffer.WriteString(text)
    // Trigger re-render of parent ConversationPane
}

func (st *StreamingText) View() string {
    content := st.buffer.String()
    if st.cursorOn {
        content += string(st.cursor)
    }
    return theme.Styles.StreamingText.Render(content)
}
```

**Key insight**: Accumulates text as it arrives, optionally shows blinking cursor. Removed when stream completes.

---

## Component Design Patterns

### Pattern 1: `tea.Model` for Stateful Components

Components with internal state that need to handle messages:

```go
type MyComponent struct {
    // State fields
    selected int
    items    []Item
    theme    *Theme
    
    // Optional child models
    subModel tea.Model
}

func (m *MyComponent) Init() tea.Cmd {
    // Initialize commands (e.g., fetch data)
    return nil
}

func (m *MyComponent) Update(msg tea.Msg) tea.Model {
    // Handle messages, update state
    return m
}

func (m *MyComponent) View() string {
    // Render to string
    return ""
}
```

**Used for**: Input, Dialogs, Lists, Any component with interactivity

### Pattern 2: Pure Renderer Functions

Components with no state, just render data:

```go
type MessageRenderer struct {
    theme *Theme
}

func (mr *MessageRenderer) Render(msg *message.Message, width int) string {
    // Deterministic: same input → same output
    // No state, no side effects
    return ""
}
```

**Used for**: MessageBubble, CodeBlock, ErrorDisplay, Any static renderer

### Pattern 3: Wrapper Components

Wrap Bubbles components and add custom behavior:

```go
type EnhancedTextInput struct {
    *textinput.Model        // Embed Bubbles model
    history    []string     // Add custom state
    histIdx    int
    theme      *Theme
}

func (eti *EnhancedTextInput) Update(msg tea.Msg) tea.Model {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.String() == "up" {
            // Custom: history navigation
            eti.SetValue(eti.history[eti.histIdx-1])
        } else {
            // Delegate to embedded model
            eti.Model, _ = eti.Model.Update(msg)
        }
    }
    return eti
}
```

**Used for**: SlashCommandInput, SessionPicker, Any enhancement to Bubbles

---

## Effort Estimation

| Phase | Components | Deliverable | Effort |
|-------|-----------|-------------|--------|
| 1 | 5 + Theme | Working TUI with input/output/theming | 1-2 weeks |
| 2 | 5 | Full message/tool/code rendering | 2-3 weeks |
| 3 | 6 | Dialogs, errors, status, approvals | 2-3 weeks |
| 4 | 4 | Slash commands, sessions, header | 2 weeks |
| 5 | 5 | Power features (panels, tabs, palette) | 2 weeks |
| **Total** | **30 components** | **Feature-complete TUI** | **9-13 weeks** |

**Timeline**:
- **1 person**: 2.5-3 months (includes testing, refinement)
- **2 people**: 6-7 weeks (parallel work on phases)
- **3 people**: 4-5 weeks (maximum parallelism)

---

## What NOT to Build (Because Charm Gives It Free)

✅ Don't build textinput — use Bubbles  
✅ Don't build viewport/scrolling — use Bubbles  
✅ Don't build list rendering — use Bubbles list  
✅ Don't build table layout — use Bubbles table  
✅ Don't build spinner animation — use Bubbles  
✅ Don't build markdown rendering — use Glamour  
✅ Don't build form components — use Huh  
✅ Don't build color palettes from scratch — use Lip Gloss  

**This focus on reuse is why we can do this in 6,630 lines instead of 20,000+.**

---

## Recommended Build Order (MVP First)

**Start with Phase 1** — it's self-contained and gives you a working REPL in Bubbletea immediately.

**Then Phase 2** — adds the "Claude Code look" (conversations with streaming, tool calls, code).

**Then Phases 3-5** — progressively add features until you match Claude Code's feature set.

**Each phase is deployable** — you can ship after Phase 2 if you want, add polish later.

---

## Key Design Decisions

1. ✅ **Favor `tea.Model` for stateful, `func Render()` for static** — clear separation
2. ✅ **Pass `*Theme` to all components** — no global theme, fully testable
3. ✅ **AppShell owns focus and modals** — single coordinator, not scattered
4. ✅ **Messages are pre-rendered** — smooth scrolling, no on-demand rendering
5. ✅ **Wrap Bubbles, don't rewrite** — leverage battle-tested code
6. ✅ **Width propagation via messages** — responsive to terminal resize
7. ✅ **No deeply nested hierarchies** — AppShell → 1-2 levels of children max

---

## Success Criteria for Each Phase

**Phase 1**: REPL works, input + output + theming + status bar all rendering correctly

**Phase 2**: Conversations show user/assistant messages, streaming text updates in real-time, tool calls display, code is syntax-highlighted

**Phase 3**: Errors show classified messages with suggestions, diffs display, permissions pop as dialogs, notifications toast

**Phase 4**: Slash commands autocomplete, /resume works, /model switching works, header shows session info

**Phase 5**: Side panels work, tabs switch views, command palette searchable, agent messages color-coded

