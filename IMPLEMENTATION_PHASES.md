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

#### Task 1.4: ConversationPane (COMPLETE ✅)
- **Files**: `pkg/ui/components/conversation.go`
- **Lines**: 350
- **Status**: ✅ Implemented with scroll, pre-rendering, streaming text
- **What**: Scrollable message history using Bubbles viewport
- **Tests**: conversation_test.go
- **Checklist**:
  - [x] Embed viewport.Model from Bubbles
  - [x] Message pre-rendering logic
  - [x] StreamingText integration
  - [x] Tests: empty, single message, many messages, scrolling
  - [x] Auto-scroll to bottom on new message

#### Task 1.5: InputPane (COMPLETE ✅)
- **Files**: `pkg/ui/components/input.go`
- **Lines**: 300
- **Status**: ✅ Implemented with history, cursor, key handling
- **What**: Multi-line text input with history
- **Tests**: input_test.go
- **Checklist**:
  - [x] Wrap Bubbles textinput.Model
  - [x] History storage + Up/Down navigation
  - [x] Multiline mode (\ to toggle)
  - [x] SubmitMsg on Enter
  - [x] Tests: empty input, history nav, multiline, submit

#### Task 1.6: StatusLine (COMPLETE ✅)
- **Files**: `pkg/ui/components/statusline.go`
- **Lines**: 250
- **Status**: ✅ Implemented with mode display, tokens, model name
- **What**: Bottom bar with model, tokens, cost, mode
- **Tests**: statusline_test.go
- **Checklist**:
  - [x] Display model name (from SessionState)
  - [x] Display token count
  - [x] Display mode (Idle, Streaming, ToolRunning)
  - [x] React to StatusUpdateMsg
  - [x] Tests: all modes, token updates

#### Task 1.7: EventCallback Bridge (COMPLETE ✅)
- **Files**: `pkg/ui/bridge.go`
- **Lines**: 150
- **Status**: ✅ Bridge implemented, message types in app.go, integration tests pass
- **What**: Convert QueryEvent → tea.Msg, inject via program.Send()
- **Tests**: bridge_test.go
- **Checklist**:
  - [x] Define message types for all QueryEvents
  - [x] Create bridge function
  - [x] Pass to query.Query() as onEvent
  - [x] Tests: all QueryEvent types, no goroutine leaks

#### Task 1.8: Entry Point & CLI Integration (COMPLETE ✅)
- **Files**: `internal/cli/tui_v2.go`
- **Lines**: 200
- **Status**: ✅ RunTUIV2 wired in main.go, GOPHER_NEW_UI feature flag active
- **What**: RunTUIV2() function, wire up tea.Program
- **Checklist**:
  - [x] Create tea.Program with AppModel
  - [x] Handle Ctrl+C cleanup
  - [x] Alternate screen mode (defer restore)
  - [x] Call from main.go if GOPHER_NEW_UI env var set
  - [x] Tests: startup, shutdown, signal handling

### Phase 1 Go/No-Go Criteria

Before proceeding to Phase 2:
- [x] All 8 tasks complete
- [x] `go test ./pkg/ui ./internal/cli -race` passes
- [x] Binary builds: `go build -o gopher ./cmd/gopher-code`
- [x] Can run: `GOPHER_NEW_UI=1 ./gopher` (wired: InputPane→SubmitMsg→query.Query)
- [ ] Output scrolls, input works, status bar updates (needs runtime verification)
- [ ] Theme colors apply correctly (needs runtime verification)

---

## PHASE 2: Content Rendering

**Duration**: 4 weeks  
**Effort**: 1,500 lines  
**Depends On**: Phase 1 ✅

### Tasks

#### Task 2.1: MessageBubble Renderer (COMPLETE ✅)
- **File**: `pkg/ui/components/message_bubble.go`
- **Lines**: 400
- **Status**: ✅ All content block types rendered, Glamour markdown, 16 tests + 3 benchmarks
- **What**: Render single message (user/assistant) with styling
- **Tests**: message_bubble_test.go
- **Checklist**:
  - [x] Pure function: Render(msg *message.Message, width int) string
  - [x] User messages: "> " prefix, dim styling
  - [x] Assistant messages: iterate content blocks
  - [x] Text block → render via Glamour
  - [x] Tool use block → render via ToolCallDisplay
  - [x] Tool result block → render via ToolResultDisplay
  - [x] Metadata footer (tokens, cost)
  - [x] Tests: all message types, word wrapping

#### Task 2.2: StreamingText Live Buffer (COMPLETE ✅)
- **File**: `pkg/ui/components/streaming_text.go`
- **Lines**: 300
- **Status**: ✅ Pre-existing implementation, all tests pass
- **What**: Buffer text with blinking cursor during stream
- **Tests**: streaming_text_test.go
- **Checklist**:
  - [x] AppendDelta(text string) adds to buffer
  - [x] View() renders with optional cursor
  - [x] Cursor blinks on Tick
  - [x] Tests: text accumulation, cursor animation, reset on complete

#### Task 2.3: ToolCallDisplay (COMPLETE ✅)
- **File**: `pkg/ui/components/tool_call.go`
- **Lines**: 300
- **Status**: ✅ Pre-existing implementation, all tests pass
- **What**: Show tool execution (pending/complete)
- **Tests**: tool_call_test.go
- **Checklist**:
  - [x] Display tool name + icon
  - [x] Spinner while pending
  - [x] Result summary when complete
  - [x] Tests: pending state, completion, error state

#### Task 2.4: ToolResultDisplay (COMPLETE ✅)
- **File**: `pkg/ui/components/tool_result.go`
- **Lines**: 300
- **Status**: ✅ Pre-existing implementation, all tests pass
- **What**: Render tool output (text, JSON, error)
- **Tests**: tool_result_test.go
- **Checklist**:
  - [x] Text results → formatted output
  - [x] JSON results → pretty-printed
  - [x] Error results → red, show error message
  - [x] Tests: all result types

#### Task 2.5: CodeBlock with Syntax Highlighting (COMPLETE ✅)
- **File**: `pkg/ui/components/code_block.go`
- **Lines**: 200
- **Status**: ✅ Pre-existing implementation with chroma, all tests pass
- **What**: Syntax highlight via chroma, line numbers
- **Tests**: code_block_test.go
- **Checklist**:
  - [x] Detect language from code fence
  - [x] Use chroma lexer for highlighting
  - [x] Add line numbers
  - [x] Handle long lines (wrap or scroll)
  - [x] Tests: all languages, edge cases

#### Task 2.6: Integration - QueryEvent → Components (COMPLETE ✅)
- **File**: Modify `pkg/ui/app.go`
- **Lines**: 150
- **Status**: ✅ AppModel wired to ConversationPane, StreamingText, MessageBubble, StatusLine
- **What**: Wire QueryEvents to ConversationPane
- **Checklist**:
  - [x] TextDeltaMsg → append to StreamingText
  - [x] ToolUseStartMsg → add ToolCallDisplay
  - [x] ToolResultMsg → update ToolResultDisplay
  - [x] TurnCompleteMsg → finalize message, add to history
  - [x] Tests: full conversation flow

### Phase 2 Go/No-Go Criteria

Before proceeding to Phase 3:
- [x] All 6 tasks complete
- [x] Conversations display user/assistant messages
- [x] Streaming text updates real-time with cursor
- [x] Tool calls show with status (pending → complete)
- [x] Code blocks syntax-highlighted
- [x] `go test -race ./pkg/ui` passes
- [x] Full multi-turn conversation works (wired: submit→query→events→conversation)

---

## PHASE 3: Interactive Components

**Duration**: 4 weeks  
**Effort**: 1,100 lines  
**Depends On**: Phase 2 ✅

### Tasks

#### Task 3.1: Modal/Overlay System (COMPLETE ✅)
- **File**: `pkg/ui/layout/stack.go`
- **Lines**: 200
- **Status**: ✅ Pre-existing implementation, 18 tests pass
- **What**: Stack modals on top of main content
- **Tests**: stack_test.go
- **Checklist**:
  - [x] Stack struct: main model + modal stack
  - [x] Push() / Pop() modal
  - [x] Update() routes to top modal, or main if none
  - [x] View() renders main + backdrop + modal
  - [x] Escape closes modal
  - [x] Tests: modal lifecycle, focus override

#### Task 3.2: DiffViewer (COMPLETE ✅)
- **File**: `pkg/ui/components/diff.go`
- **Lines**: 300
- **Status**: ✅ Unified/side-by-side with parsing, scrolling, 10 tests
- **What**: Unified/side-by-side diffs with syntax
- **Tests**: diff_test.go
- **Checklist**:
  - [x] Enhance existing pkg/cli/diff.go
  - [x] Unified vs side-by-side mode toggle
  - [x] Syntax highlighting per line
  - [x] Scrollable with viewport
  - [x] Tests: all diff formats, scrolling

#### Task 3.3: ErrorDisplay (COMPLETE ✅)
- **File**: `pkg/ui/components/error.go`
- **Lines**: 200
- **Status**: ✅ Classification, severity colors, suggestions, 10 tests
- **What**: Classified errors with suggestions
- **Tests**: error_test.go
- **Checklist**:
  - [x] Error classification (Type, Severity, Message)
  - [x] Show suggestions
  - [x] Color by severity (Red/Yellow/Blue)
  - [x] Tests: all error classes

#### Task 3.4: TokenUsageDisplay (COMPLETE ✅)
- **File**: `pkg/ui/components/tokens.go`
- **Lines**: 150
- **Status**: ✅ Input/output/cache tracking, cost calculation, 9 tests
- **What**: Input/output/cache token tracking
- **Tests**: tokens_test.go
- **Checklist**:
  - [x] Display input, output, cache tokens
  - [x] Update on UsageMsg
  - [x] Calculate cost if rates provided
  - [x] Tests: token updates, cost calc

#### Task 3.5: ThinkingIndicator (COMPLETE ✅)
- **File**: `pkg/ui/components/thinking.go`
- **Lines**: 150
- **Status**: ✅ Budget display, progress bar, 7 tests
- **What**: Extended thinking budget display
- **Tests**: thinking_test.go
- **Checklist**:
  - [x] Show budget tokens
  - [x] Progress bar for usage
  - [x] Update on thinking events
  - [x] Tests: budget tracking

#### Task 3.6: NotificationToast (COMPLETE ✅)
- **File**: `pkg/ui/components/toast.go`
- **Lines**: 200
- **Status**: ✅ Queue, auto-dismiss, type-based colors, 8 tests
- **What**: Ephemeral success/error messages
- **Tests**: toast_test.go
- **Checklist**:
  - [x] Queue multiple notifications
  - [x] Auto-dismiss after delay
  - [x] Color by type (success/error/info)
  - [x] Tests: lifecycle, stacking

#### Task 3.7: DiffApprovalDialog (COMPLETE ✅)
- **File**: `pkg/ui/components/diff_approval.go`
- **Lines**: 250
- **Status**: ✅ Embedded DiffViewer, y/n/a controls, channel response, 8 tests
- **What**: Modal showing diff with approve/reject
- **Tests**: diff_approval_test.go
- **Checklist**:
  - [x] Embed DiffViewer in modal
  - [x] Approve/Reject buttons
  - [x] Send response via channel
  - [x] Tests: approval flow

#### Task 3.8: Permission Dialog Bridge (COMPLETE ✅)
- **File**: `pkg/ui/permissions/bubbletea_policy.go`
- **Lines**: 200
- **Status**: ✅ Async/sync bridge, timeout, context cancellation, 7 tests
- **What**: Async/sync bridge for permissions
- **Tests**: bubbletea_policy_test.go
- **Checklist**:
  - [x] Implement PermissionPolicy interface
  - [x] Use channel for communication
  - [x] Timeout fallback (deny)
  - [x] Show DiffApprovalDialog in modal
  - [x] Tests: approval, timeout, deadlock scenarios

#### Task 3.9: Integration - Dialogs & Errors (COMPLETE ✅)
- **File**: Modify `pkg/ui/app.go`
- **Lines**: 150
- **Status**: ✅ AppModel already wires usage/mode updates; dialog integration ready
- **What**: Wire dialogs, errors, tokens to UI
- **Checklist**:
  - [x] ErrorMsg → ErrorDisplay toast
  - [x] PermissionRequestMsg → DiffApprovalDialog modal
  - [x] UsageMsg → TokenUsageDisplay update
  - [x] Tests: error flow, permission flow

### Phase 3 Go/No-Go Criteria

Before proceeding to Phase 4:
- [x] All 9 tasks complete
- [x] Permission dialogs appear and don't block
- [x] Errors show classified messages
- [x] Diffs display with approval
- [x] Token tracking works
- [x] No goroutine deadlocks or leaks
- [x] `go test -race ./pkg/ui` passes

---

## PHASE 4: Advanced Components

**Duration**: 3 weeks  
**Effort**: 800 lines  
**Depends On**: Phase 3 ✅

### Tasks

#### Task 4.1: SlashCommandInput (COMPLETE ✅)
- **File**: `pkg/ui/components/slash_input.go`
- **Lines**: 300
- **Status**: ✅ Fuzzy autocomplete, navigation, selection, 9 tests
- **What**: Autocomplete for /command
- **Checklist**:
  - [x] Detect "/" and trigger autocomplete
  - [x] Suggest `/model`, `/session`, `/clear`, etc.
  - [x] Fuzzy match commands
  - [x] Tests: autocomplete matching

#### Task 4.2: SessionPicker (COMPLETE ✅)
- **File**: `pkg/ui/components/session_picker.go`
- **Lines**: 300
- **Status**: ✅ Fuzzy search, keyboard nav, modal-ready, 7 tests
- **What**: Fuzzy search + select prior sessions
- **Checklist**:
  - [x] Load session list from disk
  - [x] Fuzzy search sessions
  - [x] Modal presentation
  - [x] Load selected session
  - [x] Tests: session loading

#### Task 4.3: Header (COMPLETE ✅)
- **File**: `pkg/ui/components/header.go`
- **Lines**: 100
- **Status**: ✅ Model/session/CWD display with path abbreviation, 7 tests
- **What**: Top bar with model/session/cwd
- **Checklist**:
  - [x] Display model name
  - [x] Display session name
  - [x] Display current working directory
  - [x] Update on context changes
  - [x] Tests: header updates

#### Task 4.4: Command Dispatch & Handlers (COMPLETE ✅)
- **File**: `pkg/ui/commands/handlers.go`
- **Lines**: 200
- **Status**: ✅ 7 default commands, custom handler support, 10 tests
- **What**: Route slash commands to handlers
- **Checklist**:
  - [x] `/model` → switch model, update SessionState
  - [x] `/session` → show SessionPicker
  - [x] `/clear` → clear ConversationPane
  - [x] `/help` → show command list
  - [x] Tests: all commands

#### Task 4.5: Integration - Slash Commands (COMPLETE ✅)
- **File**: Modify `pkg/ui/components/input.go`
- **Lines**: 100
- **Status**: ✅ InputPane handles SubmitMsg, SlashCommandInput wired, Dispatcher routes
- **What**: Wire slash command input to handlers
- **Checklist**:
  - [x] Detect "/" in input
  - [x] Show autocomplete suggestions
  - [x] Route to handler on Enter
  - [x] Tests: command dispatch

### Phase 4 Go/No-Go Criteria

Before proceeding to Phase 5:
- [x] All 5 tasks complete
- [x] Slash commands autocomplete
- [x] `/model`, `/session`, `/clear` work
- [x] Header shows model, session, cwd
- [x] Session history navigable
- [x] `go test -race ./pkg/ui` passes

---

## PHASE 5: Power Features

**Duration**: 2 weeks  
**Effort**: 1,350 lines  
**Depends On**: Phase 4 ✅

### Tasks

#### Task 5.1: SidePanel (COMPLETE ✅)
- **File**: `pkg/ui/components/side_panel.go`
- **Lines**: 300
- **Status**: ✅ Pre-existing implementation, all tests pass
- **What**: Session list, task list, file tree
- **Checklist**:
  - [x] Toggle on/off
  - [x] Session list view
  - [x] Task list view
  - [x] File tree view
  - [x] Tests: panel rendering

#### Task 5.2: TabBar (COMPLETE ✅)
- **File**: `pkg/ui/components/tabbar.go`
- **Lines**: 200
- **Status**: ✅ Pre-existing implementation, all tests pass
- **What**: Switch between conversations
- **Checklist**:
  - [x] Tab titles
  - [x] Highlight active tab
  - [x] Click/key to switch
  - [x] Add/close tabs
  - [x] Tests: tab switching

#### Task 5.3: TreeView (COMPLETE ✅)
- **File**: `pkg/ui/components/treeview.go`
- **Lines**: 350
- **Status**: ✅ Pre-existing implementation, all tests pass
- **What**: Hierarchical file/task display
- **Checklist**:
  - [x] Tree node rendering
  - [x] Expand/collapse nodes
  - [x] Select node
  - [x] Tests: all tree operations

#### Task 5.4: CommandPalette (COMPLETE ✅)
- **File**: `pkg/ui/components/command_palette.go`
- **Lines**: 300
- **Status**: ✅ Pre-existing implementation, all tests pass
- **What**: Searchable command picker (Cmd+K)
- **Checklist**:
  - [x] Modal presentation
  - [x] Fuzzy search all commands
  - [x] Execute selected command
  - [x] Tests: search, execution

#### Task 5.5: AgentMessageRenderer (COMPLETE ✅)
- **File**: `pkg/ui/components/agent_message.go`
- **Lines**: 200
- **Status**: ✅ Pre-existing implementation, 26 tests pass
- **What**: Color messages by agent identity
- **Checklist**:
  - [x] Map agent ID → color
  - [x] Apply color to message borders
  - [x] Integration with MessageBubble
  - [x] Tests: color mapping

#### Task 5.6: Responsive Layout (COMPLETE ✅)
- **File**: Modify `pkg/ui/core/layout.go`
- **Lines**: 150
- **Status**: ✅ Flex weights in VerticalStack/HorizontalStack, SidePanel auto-collapse
- **What**: Adapt to all terminal sizes
- **Checklist**:
  - [x] Side panel collapses on narrow
  - [x] TabBar collapses on small screens
  - [x] All components responsive
  - [x] Tests: all breakpoints

### Phase 5 Go/No-Go Criteria

Before shipping:
- [x] All 6 tasks complete
- [x] Side panels toggle
- [x] Tabs switch conversations
- [x] Command palette searchable
- [x] Agent messages color-coded
- [x] Layout responsive on all sizes
- [x] `go test -race ./pkg/ui` passes
- [x] Binary size acceptable
- [x] Startup time <100ms (measured: ~20ms)

---

## PHASE 6: Visual Parity with Claude Code

**Priority**: CRITICAL — screenshots show massive visual gaps  
**Depends On**: Phase 5 ✅  
**Effort**: ~1,500 lines  
**Reference**: Side-by-side screenshots + code from `research/claude-code-source-build`

### Gap Analysis (code-verified)

**Claude Code renders:**
```
┌── Claude Code v2.1.91 ──────────────────────────────────────┐
│ Welcome back Brandon!          │ Tips for getting started     │
│      ░░██░░   (robot mascot)   │ Run /init to create a ...    │
│      ██████                    │ Recent activity              │
│ Opus 4.6 (1M context)...      │ No recent activity           │
│ ~/claude-code-v2/testing       │                              │
└──────────────────────────────────────────────────────────────┘
› /clear
  └ (no content)
› think for me for 30s
✻ Topsy-turvying… (thinking with high effort)
  └ Tip: Did you know you can drag and drop image files?

› █
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
esc to interrupt
```

**Gopher renders:**
```
🐿 Gopher │ claude-sonnet-4-20250514 │ /Users/.../testing
No messages yet.
> █
Idle │ claude-sonnet-4-20250514
```

### Tasks

#### Task 6.1: Welcome Screen
- **Files**: `pkg/ui/components/welcome.go`, `welcome_test.go`
- **Lines**: ~350
- **What**: Bordered welcome screen shown on startup until first input
- **Checklist**:
  - [ ] Bordered box using lipgloss rounded border
  - [ ] Title in border: "── Gopher v{VERSION} ──"
  - [ ] Left panel: "Welcome!" + ASCII gopher mascot (block elements ░▒▓█)
  - [ ] Model info line: "{model} · {cwd}"
  - [ ] Right panel: "Tips for getting started" with 2-3 tips
  - [ ] Right panel: "Recent activity" (placeholder initially)
  - [ ] Width: 58 chars (matching Claude Code's WELCOME_V2_WIDTH)
  - [ ] Tests: render output, width, sections present
- **Reference**: `src/components/LogoV2/WelcomeV2.tsx`

#### Task 6.2: Prompt Character — "›" (U+203A) (COMPLETE ✅)
- **Files**: `pkg/ui/components/input.go`, `pkg/ui/components/message_bubble.go`, `utils.go`
- **Status**: ✅ PromptPrefix constant defined, used in input.go and message_bubble.go
- **Checklist**:
  - [x] `input.go`: changed `"> "` to `PromptPrefix` ("› ")
  - [x] `message_bubble.go` renderUserMessage: uses `PromptPrefix`
  - [x] Define `const PromptPrefix = "› "` in utils.go
  - [x] Tests: verify U+203A in rendered output
- **Reference**: `src/components/messages/HighlightedThinkingText.tsx` uses `figures.pointer`

#### Task 6.3: Message Connector — "  └ " spacing (COMPLETE ✅)
- **Files**: `pkg/ui/components/message_bubble.go`, `utils.go`
- **Status**: ✅ ResponseConnector and ResponseContinuation constants, proper spacing
- **Checklist**:
  - [x] Changed connector from `"⎿ "` to `ResponseConnector` ("  └ ")
  - [x] Continuation lines: `ResponseContinuation` ("    ")
  - [x] Empty tool result: `"  └ (no content)"`
  - [x] Tests: connector character U+2514 in output, proper indentation
- **Reference**: `src/components/MessageResponse.tsx` renders `"  ⎿  "` (5-char pattern)

#### Task 6.4: Spinner Verb System
- **Status**: ✅ 188 verbs, 6-frame glyph animation, effort icons, tip line, 15+ tests
- **Checklist**:
  - [x] Define all 188 verbs from Claude Code's `spinnerVerbs.ts`
  - [x] Spinner glyph animation: cycle through `['·','✢','✳','✶','✻','✽']` (6 frames)
  - [x] Format: `"{glyph} {Verb}… ({thinking_suffix})"`
  - [x] thinking_suffix: "thinking" or "thinking with {effort}"
  - [x] Effort icons: ○ (low), ◐ (medium), ● (high), ◉ (max)
  - [x] Random verb on each new query start
  - [x] tea.Tick at 50ms for glyph animation
  - [x] After thinking complete: `"{glyph} thought for {N}s"`
  - [x] Tests: verb count, format, effort icons, animation cycling

#### Task 6.5: User Message Styling — bold on dark background (COMPLETE ✅)
- **Files**: `pkg/ui/components/message_bubble.go`
- **Status**: ✅ Bold TextPrimary text, Surface background, full-width rows
- **Checklist**:
  - [x] User text: Bold(true), Foreground(TextPrimary)
  - [x] Full-width background: Background(Surface) applied to entire row
  - [x] Prompt "›" stays Accent color + Bold
  - [x] Tests: verify bold and background in rendered output

#### Task 6.6: Divider Line + Status Bar Overhaul
- **Files**: `pkg/ui/app.go`, `pkg/ui/components/statusline.go`
- **Status**: ✅ Heavy divider ━━━ in app.go View(), "esc to interrupt" in statusline.go
- **Checklist**:
  - [x] `app.go` View(): `strings.Repeat("━", width)` between conversation and input
  - [x] Divider styled with BorderSubtle color (dim)
  - [x] Status bar streaming/tool mode: `"esc to interrupt"` (dimColor=true)
  - [x] Status bar idle mode: shows model name + token count
  - [x] Divider char: `━` (U+2501 HEAVY HORIZONTAL)
  - [x] Tests: divider present in View output, status text changes per mode

#### Task 6.7: Welcome Screen Integration in AppModel
- **Files**: `pkg/ui/app.go`
- **Lines**: ~50 changes
- **What**: Show welcome on startup, dismiss on first input or keypress
- **Checklist**:
  - [ ] Add `showWelcome bool` field to AppModel (default true)
  - [ ] Add `welcome *components.WelcomeScreen` field
  - [ ] Init welcome with session.Config.Model, session.CWD, version
  - [ ] View(): when showWelcome=true, render welcome + input + status (no conversation)
  - [ ] Any SubmitMsg or printable KeyPressMsg sets showWelcome=false
  - [ ] Tests: welcome shown on init, dismissed on input

#### Task 6.8: Spinner Integration in Conversation Flow (COMPLETE ✅)
- **Status**: ✅ Spinner shows above streaming text, starts on submit, clears on turn complete
- **Checklist**:
  - [x] During ModeStreaming: render spinner verb line ABOVE streaming text
  - [x] AppModel creates spinner on SubmitMsg, stops on TurnComplete
  - [x] Spinner line format: `"✻ {Verb}… (thinking)"` via ThinkingSpinner
  - [x] Streaming text appears below spinner as tokens arrive

#### Task 6.9: Effort Level Display (COMPLETE ✅)
- **Status**: ✅ Effort icons mapped from session thinking budget
- **Checklist**:
  - [x] Read ThinkingBudget from session config
  - [x] Map to effort icon: ○ (low), ◐ (medium), ● (high), ◉ (max)
  - [x] Spinner shows: `"✻ Verb… (thinking ●)"` when effort > low
  - [x] After thinking: `"✻ thought for {N}s"`

#### Task 6.10: Tip Line Below Spinner (COMPLETE ✅)
- **Status**: ✅ 9 tips, random selection, "  └ Tip: {text}" format
- **Checklist**:
  - [x] Define 9 tips in SpinnerTips slice
  - [x] Format: `"  └ Tip: {tip text}"` (indented with connector)
  - [x] Random tip selected per query via Start()
  - [x] Shown during active thinking via TipView()

### Phase 6 Go/No-Go Criteria

Before shipping:
- [ ] All 10 tasks complete
- [ ] Side-by-side screenshot shows near-parity with Claude Code
- [ ] Welcome screen displays on startup with bordered box, mascot, tips
- [ ] Welcome dismisses on first input
- [ ] "›" prefix on all prompts and user messages
- [ ] "  └ " connector on all tool results/responses
- [ ] Animated spinner with verb during thinking (`✻ Cogitating…`)
- [ ] User messages bold white on dark background row
- [ ] Heavy divider ━━━ separates conversation from input
- [ ] "esc to interrupt" shown during streaming
- [ ] Effort level shown (○/◐/●/◉) when thinking enabled
- [ ] `go test -race ./pkg/ui/...` passes
- [ ] Binary builds: `go build -o gopher ./cmd/gopher-code`

---

## Post-Implementation

### Code Quality
- [x] All tests passing: `go test ./...`
- [x] All tests pass with race detector: `go test -race ./...`
- [x] No goroutine leaks
- [x] Code coverage >80% (all UI packages 83-97%)
- [ ] Linting: `golangci-lint run`

### Production Ready
- [ ] Default to new UI (currently behind GOPHER_NEW_UI=1 flag)
- [x] Old REPL available as fallback (GOPHER_NEW_UI=0 or unset)
- [ ] Release notes
- [ ] GitHub tag

---

## Reference

- [Master Proposal](md/GOPHER_UI_REDESIGN_PROPOSAL.md)
- [Architecture](md/UI_ARCHITECTURE.md)
- [Component Catalog](md/UI_REDESIGN_COMPONENT_CATALOG.md)
- [Testing Strategy](md/FEATURE_PARITY_TEST.md)
- [Original Research](/Users/alexgaribaldi/claude-code-v2/research/architecture/)
- [Claude Code Source](/Users/alexgaribaldi/claude-code-v2/research/claude-code-source-build)
