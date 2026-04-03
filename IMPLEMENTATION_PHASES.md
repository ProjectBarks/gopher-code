# Implementation Phases — Task Breakdown

This file is used by `run-phase.sh` to guide you through implementation. Each phase is self-contained and has clear deliverables.

---

## PHASE 1: Core Framework

**Duration**: 3 weeks  
**Effort**: 1,200 lines  
**Team Size**: 1 person (3 weeks), 2 people (2 weeks), 3+ people (1 week)

### Tasks

#### Task 1.1: Theme System (COMPLETE ✅)
- **File**: `pkg/ui/theme/`
- **Status**: ✅ Already implemented (6 files, 1,235 lines)
- **What**: Color palette, theme interface, 3 themes (dark/light/high-contrast)
- **Reference**: [Design System](md/GOPHER_UI_REDESIGN_PROPOSAL.md#blue-color-palette)

#### Task 1.2: Component Interface & Layout Engine (COMPLETE ✅)
- **Files to create**: 
  - `pkg/ui/core/component.go`
  - `pkg/ui/core/layout.go`
  - `pkg/ui/core/focus.go`
  - `pkg/ui/core/keymap.go`
- **Lines**: ~400
- **Status**: ✅ All files implemented, all tests passing
- **What**: Define Component interface, layout math, focus ring
- **Tests**: component_test.go, layout_test.go, focus_test.go
- **Checklist**:
  - [x] Component interface: SetSize, Focused, Init, Update, View
  - [x] Layout engine: vertical stack with width distribution
  - [x] Focus ring: Tab cycling, modal override
  - [x] All tests passing

#### Task 1.3: AppShell Container (COMPLETE ✅)
- **File**: `pkg/ui/app.go`
- **Lines**: 300
- **Status**: ✅ Implemented with all message types, focus routing, event bridge integration
- **What**: Top-level tea.Model, routes messages to children, manages focus
- **Tests**: app_test.go
- **Checklist**:
  - [x] struct with children (header, conversation, input, status)
  - [x] Init() spawns query goroutine
  - [x] Update() routes to focused child
  - [x] View() renders 4-part layout
  - [x] Tests for focus routing, modal stacking
  - [x] Integration with EventCallback bridge

#### Task 1.4: ConversationPane
- **Files**: `pkg/ui/components/conversation.go`
- **Lines**: 350
- **What**: Scrollable message history using Bubbles viewport
- **Tests**: conversation_test.go
- **Checklist**:
  - [ ] Embed viewport.Model from Bubbles
  - [ ] Message pre-rendering logic
  - [ ] StreamingText integration
  - [ ] Tests: empty, single message, many messages, scrolling
  - [ ] Auto-scroll to bottom on new message

#### Task 1.5: InputPane
- **Files**: `pkg/ui/components/input.go`
- **Lines**: 300
- **What**: Multi-line text input with history
- **Tests**: input_test.go
- **Checklist**:
  - [ ] Wrap Bubbles textinput.Model
  - [ ] History storage + Up/Down navigation
  - [ ] Multiline mode (\ to toggle)
  - [ ] SubmitMsg on Enter
  - [ ] Tests: empty input, history nav, multiline, submit

#### Task 1.6: StatusLine
- **Files**: `pkg/ui/components/statusline.go`
- **Lines**: 250
- **What**: Bottom bar with model, tokens, cost, mode
- **Tests**: statusline_test.go
- **Checklist**:
  - [ ] Display model name (from SessionState)
  - [ ] Display token count
  - [ ] Display mode (Idle, Streaming, ToolRunning)
  - [ ] React to StatusUpdateMsg
  - [ ] Tests: all modes, token updates

#### Task 1.7: EventCallback Bridge
- **Files**: `pkg/ui/bridge.go`
- **Lines**: 150
- **What**: Convert QueryEvent → tea.Msg, inject via program.Send()
- **Tests**: bridge_test.go
- **Checklist**:
  - [ ] Define message types for all QueryEvents
  - [ ] Create bridge function
  - [ ] Pass to query.Query() as onEvent
  - [ ] Tests: all QueryEvent types, no goroutine leaks

#### Task 1.8: Entry Point & CLI Integration
- **Files**: `internal/cli/tui_v2.go`
- **Lines**: 200
- **What**: RunTUIV2() function, wire up tea.Program
- **Checklist**:
  - [ ] Create tea.Program with AppModel
  - [ ] Handle Ctrl+C cleanup
  - [ ] Alternate screen mode (defer restore)
  - [ ] Call from main.go if GOPHER_NEW_UI env var set
  - [ ] Tests: startup, shutdown, signal handling

### Phase 1 Go/No-Go Criteria

Before proceeding to Phase 2:
- [ ] All 8 tasks complete
- [ ] `go test ./pkg/ui ./internal/cli -race` passes
- [ ] Binary builds: `go build -o gopher ./cmd/gopher-code`
- [ ] Can run: `GOPHER_NEW_UI=1 ./gopher -p "hello"`
- [ ] Output scrolls, input works, status bar updates
- [ ] Theme colors apply correctly

---

## PHASE 2: Content Rendering

**Duration**: 4 weeks  
**Effort**: 1,500 lines  
**Depends On**: Phase 1 ✅

### Tasks

#### Task 2.1: MessageBubble Renderer
- **File**: `pkg/ui/components/message_bubble.go`
- **Lines**: 400
- **What**: Render single message (user/assistant) with styling
- **Tests**: message_bubble_test.go
- **Checklist**:
  - [ ] Pure function: Render(msg *message.Message, width int) string
  - [ ] User messages: "> " prefix, dim styling
  - [ ] Assistant messages: iterate content blocks
  - [ ] Text block → render via Glamour
  - [ ] Tool use block → render via ToolCallDisplay
  - [ ] Tool result block → render via ToolResultDisplay
  - [ ] Metadata footer (tokens, cost)
  - [ ] Tests: all message types, word wrapping

#### Task 2.2: StreamingText Live Buffer
- **File**: `pkg/ui/components/streaming_text.go`
- **Lines**: 300
- **What**: Buffer text with blinking cursor during stream
- **Tests**: streaming_text_test.go
- **Checklist**:
  - [ ] AppendDelta(text string) adds to buffer
  - [ ] View() renders with optional cursor
  - [ ] Cursor blinks on Tick
  - [ ] Tests: text accumulation, cursor animation, reset on complete

#### Task 2.3: ToolCallDisplay
- **File**: `pkg/ui/components/tool_call.go`
- **Lines**: 300
- **What**: Show tool execution (pending/complete)
- **Tests**: tool_call_test.go
- **Checklist**:
  - [ ] Display tool name + icon
  - [ ] Spinner while pending
  - [ ] Result summary when complete
  - [ ] Tests: pending state, completion, error state

#### Task 2.4: ToolResultDisplay
- **File**: `pkg/ui/components/tool_result.go`
- **Lines**: 300
- **What**: Render tool output (text, JSON, error)
- **Tests**: tool_result_test.go
- **Checklist**:
  - [ ] Text results → formatted output
  - [ ] JSON results → pretty-printed
  - [ ] Error results → red, show error message
  - [ ] Tests: all result types

#### Task 2.5: CodeBlock with Syntax Highlighting
- **File**: `pkg/ui/components/code_block.go`
- **Lines**: 200
- **What**: Syntax highlight via chroma, line numbers
- **Tests**: code_block_test.go
- **Checklist**:
  - [ ] Detect language from code fence
  - [ ] Use chroma lexer for highlighting
  - [ ] Add line numbers
  - [ ] Handle long lines (wrap or scroll)
  - [ ] Tests: all languages, edge cases

#### Task 2.6: Integration - QueryEvent → Components
- **File**: Modify `pkg/ui/app.go`
- **Lines**: 150
- **What**: Wire QueryEvents to ConversationPane
- **Checklist**:
  - [ ] TextDeltaMsg → append to StreamingText
  - [ ] ToolUseStartMsg → add ToolCallDisplay
  - [ ] ToolResultMsg → update ToolResultDisplay
  - [ ] TurnCompleteMsg → finalize message, add to history
  - [ ] Tests: full conversation flow

### Phase 2 Go/No-Go Criteria

Before proceeding to Phase 3:
- [ ] All 6 tasks complete
- [ ] Conversations display user/assistant messages
- [ ] Streaming text updates real-time with cursor
- [ ] Tool calls show with status (pending → complete)
- [ ] Code blocks syntax-highlighted
- [ ] `go test -race ./pkg/ui` passes
- [ ] Full multi-turn conversation works

---

## PHASE 3: Interactive Components

**Duration**: 4 weeks  
**Effort**: 1,100 lines  
**Depends On**: Phase 2 ✅

### Tasks

#### Task 3.1: Modal/Overlay System
- **File**: `pkg/ui/layout/stack.go`
- **Lines**: 200
- **What**: Stack modals on top of main content
- **Tests**: stack_test.go
- **Checklist**:
  - [ ] Stack struct: main model + modal stack
  - [ ] Push() / Pop() modal
  - [ ] Update() routes to top modal, or main if none
  - [ ] View() renders main + backdrop + modal
  - [ ] Escape closes modal
  - [ ] Tests: modal lifecycle, focus override

#### Task 3.2: DiffViewer
- **File**: `pkg/ui/components/diff.go`
- **Lines**: 300
- **What**: Unified/side-by-side diffs with syntax
- **Tests**: diff_test.go
- **Checklist**:
  - [ ] Enhance existing pkg/cli/diff.go
  - [ ] Unified vs side-by-side mode toggle
  - [ ] Syntax highlighting per line
  - [ ] Scrollable with viewport
  - [ ] Tests: all diff formats, scrolling

#### Task 3.3: ErrorDisplay
- **File**: `pkg/ui/components/error.go`
- **Lines**: 200
- **What**: Classified errors with suggestions
- **Tests**: error_test.go
- **Checklist**:
  - [ ] Error classification (Type, Severity, Message)
  - [ ] Show suggestions
  - [ ] Color by severity (Red/Yellow/Blue)
  - [ ] Tests: all error classes

#### Task 3.4: TokenUsageDisplay
- **File**: `pkg/ui/components/tokens.go`
- **Lines**: 150
- **What**: Input/output/cache token tracking
- **Tests**: tokens_test.go
- **Checklist**:
  - [ ] Display input, output, cache tokens
  - [ ] Update on UsageMsg
  - [ ] Calculate cost if rates provided
  - [ ] Tests: token updates, cost calc

#### Task 3.5: ThinkingIndicator
- **File**: `pkg/ui/components/thinking.go`
- **Lines**: 150
- **What**: Extended thinking budget display
- **Tests**: thinking_test.go
- **Checklist**:
  - [ ] Show budget tokens
  - [ ] Progress bar for usage
  - [ ] Update on thinking events
  - [ ] Tests: budget tracking

#### Task 3.6: NotificationToast
- **File**: `pkg/ui/components/toast.go`
- **Lines**: 200
- **What**: Ephemeral success/error messages
- **Tests**: toast_test.go
- **Checklist**:
  - [ ] Queue multiple notifications
  - [ ] Auto-dismiss after delay
  - [ ] Color by type (success/error/info)
  - [ ] Tests: lifecycle, stacking

#### Task 3.7: DiffApprovalDialog
- **File**: `pkg/ui/components/diff_approval.go`
- **Lines**: 250
- **What**: Modal showing diff with approve/reject
- **Tests**: diff_approval_test.go
- **Checklist**:
  - [ ] Embed DiffViewer in modal
  - [ ] Approve/Reject buttons
  - [ ] Send response via channel
  - [ ] Tests: approval flow

#### Task 3.8: Permission Dialog Bridge (CRITICAL)
- **File**: `pkg/ui/permissions/bubbletea_policy.go`
- **Lines**: 200
- **What**: Async/sync bridge for permissions
- **Tests**: bubbletea_policy_test.go
- **Checklist**:
  - [ ] Implement PermissionPolicy interface
  - [ ] Use channel for communication
  - [ ] Timeout fallback (deny)
  - [ ] Show DiffApprovalDialog in modal
  - [ ] Tests: approval, timeout, deadlock scenarios

#### Task 3.9: Integration - Dialogs & Errors
- **File**: Modify `pkg/ui/app.go`
- **Lines**: 150
- **What**: Wire dialogs, errors, tokens to UI
- **Checklist**:
  - [ ] ErrorMsg → ErrorDisplay toast
  - [ ] PermissionRequestMsg → DiffApprovalDialog modal
  - [ ] UsageMsg → TokenUsageDisplay update
  - [ ] Tests: error flow, permission flow

### Phase 3 Go/No-Go Criteria

Before proceeding to Phase 4:
- [ ] All 9 tasks complete
- [ ] Permission dialogs appear and don't block
- [ ] Errors show classified messages
- [ ] Diffs display with approval
- [ ] Token tracking works
- [ ] No goroutine deadlocks or leaks
- [ ] `go test -race ./pkg/ui` passes

---

## PHASE 4: Advanced Components

**Duration**: 3 weeks  
**Effort**: 800 lines  
**Depends On**: Phase 3 ✅

### Tasks

#### Task 4.1: SlashCommandInput
- **File**: `pkg/ui/components/slash_input.go`
- **Lines**: 300
- **What**: Autocomplete for /command
- **Checklist**:
  - [ ] Detect "/" and trigger autocomplete
  - [ ] Suggest `/model`, `/session`, `/clear`, etc.
  - [ ] Fuzzy match commands
  - [ ] Tests: autocomplete matching

#### Task 4.2: SessionPicker
- **File**: `pkg/ui/components/session_picker.go`
- **Lines**: 300
- **What**: Fuzzy search + select prior sessions
- **Checklist**:
  - [ ] Load session list from disk
  - [ ] Fuzzy search sessions
  - [ ] Modal presentation
  - [ ] Load selected session
  - [ ] Tests: session loading

#### Task 4.3: Header
- **File**: `pkg/ui/components/header.go`
- **Lines**: 100
- **What**: Top bar with model/session/cwd
- **Checklist**:
  - [ ] Display model name
  - [ ] Display session name
  - [ ] Display current working directory
  - [ ] Update on context changes
  - [ ] Tests: header updates

#### Task 4.4: Command Dispatch & Handlers
- **File**: `pkg/ui/commands/handlers.go`
- **Lines**: 200
- **What**: Route slash commands to handlers
- **Checklist**:
  - [ ] `/model` → switch model, update SessionState
  - [ ] `/session` → show SessionPicker
  - [ ] `/clear` → clear ConversationPane
  - [ ] `/help` → show command list
  - [ ] Tests: all commands

#### Task 4.5: Integration - Slash Commands
- **File**: Modify `pkg/ui/components/input.go`
- **Lines**: 100
- **What**: Wire slash command input to handlers
- **Checklist**:
  - [ ] Detect "/" in input
  - [ ] Show autocomplete suggestions
  - [ ] Route to handler on Enter
  - [ ] Tests: command dispatch

### Phase 4 Go/No-Go Criteria

Before proceeding to Phase 5:
- [ ] All 5 tasks complete
- [ ] Slash commands autocomplete
- [ ] `/model`, `/session`, `/clear` work
- [ ] Header shows model, session, cwd
- [ ] Session history navigable
- [ ] `go test -race ./pkg/ui` passes

---

## PHASE 5: Power Features

**Duration**: 2 weeks  
**Effort**: 1,350 lines  
**Depends On**: Phase 4 ✅

### Tasks

#### Task 5.1: SidePanel
- **File**: `pkg/ui/components/side_panel.go`
- **Lines**: 300
- **What**: Session list, task list, file tree
- **Checklist**:
  - [ ] Toggle on/off
  - [ ] Session list view
  - [ ] Task list view
  - [ ] File tree view
  - [ ] Tests: panel rendering

#### Task 5.2: TabBar
- **File**: `pkg/ui/components/tabbar.go`
- **Lines**: 200
- **What**: Switch between conversations
- **Checklist**:
  - [ ] Tab titles
  - [ ] Highlight active tab
  - [ ] Click/key to switch
  - [ ] Add/close tabs
  - [ ] Tests: tab switching

#### Task 5.3: TreeView
- **File**: `pkg/ui/components/treeview.go`
- **Lines**: 350
- **What**: Hierarchical file/task display
- **Checklist**:
  - [ ] Tree node rendering
  - [ ] Expand/collapse nodes
  - [ ] Select node
  - [ ] Tests: all tree operations

#### Task 5.4: CommandPalette
- **File**: `pkg/ui/components/command_palette.go`
- **Lines**: 300
- **What**: Searchable command picker (Cmd+K)
- **Checklist**:
  - [ ] Modal presentation
  - [ ] Fuzzy search all commands
  - [ ] Execute selected command
  - [ ] Tests: search, execution

#### Task 5.5: AgentMessageRenderer
- **File**: `pkg/ui/components/agent_message.go`
- **Lines**: 200
- **What**: Color messages by agent identity
- **Checklist**:
  - [ ] Map agent ID → color
  - [ ] Apply color to message borders
  - [ ] Integration with MessageBubble
  - [ ] Tests: color mapping

#### Task 5.6: Responsive Layout
- **File**: Modify `pkg/ui/core/layout.go`
- **Lines**: 150
- **What**: Adapt to all terminal sizes
- **Checklist**:
  - [ ] Side panel collapses on narrow
  - [ ] TabBar collapses on small screens
  - [ ] All components responsive
  - [ ] Tests: all breakpoints

### Phase 5 Go/No-Go Criteria

Before shipping:
- [ ] All 6 tasks complete
- [ ] Side panels toggle
- [ ] Tabs switch conversations
- [ ] Command palette searchable
- [ ] Agent messages color-coded
- [ ] Layout responsive on all sizes
- [ ] `go test -race ./pkg/ui` passes
- [ ] Binary size acceptable
- [ ] Startup time <100ms

---

## Post-Implementation

### Code Quality
- [ ] All tests passing: `go test ./...`
- [ ] All tests pass with race detector: `go test -race ./...`
- [ ] No goroutine leaks
- [ ] Code coverage >80%
- [ ] Linting: `golangci-lint run`

### Documentation
- [ ] Component API docs
- [ ] Usage examples
- [ ] Architecture notes
- [ ] Troubleshooting guide

### Production Ready
- [ ] Default to new UI
- [ ] Old REPL available as fallback
- [ ] Release notes
- [ ] GitHub tag

---

## Reference

- [Master Proposal](md/GOPHER_UI_REDESIGN_PROPOSAL.md)
- [Architecture](md/UI_ARCHITECTURE.md)
- [Component Catalog](md/UI_REDESIGN_COMPONENT_CATALOG.md)
- [Testing Strategy](md/FEATURE_PARITY_TEST.md)
- [Original Research](/Users/alexgaribaldi/claude-code-v2/research/architecture/)
