# Gopher UI Feature Parity Test Plan

## Overview

This document defines the comprehensive testing and validation strategy for ensuring
Gopher's new Bubble Tea UI achieves feature parity with Claude Code's Ink-based UI.

---

## 1. Test Structure and Organization

```
pkg/ui/                          # New UI component package
  testing/                       # UI test utilities
    capture.go                   # Terminal output capture
    golden.go                    # Golden file comparison
    simulate.go                  # Input simulation
    assert.go                    # Component state assertions
    benchmark.go                 # Performance measurement

internal/cli/                    # Existing CLI components (under test)
  tui_test.go                   # Model unit tests
  statusbar_test.go             # StatusBar unit tests
  spinner_test.go               # Spinner unit tests
  permission_dialog_test.go     # PermissionDialog unit tests
  render_test.go                # Event rendering tests
  diff_test.go                  # Diff rendering tests
  markdown_test.go              # Markdown rendering tests
  completions_test.go           # Shell completion tests

internal/cli/testdata/golden/   # Golden files for visual regression
  idle_default.golden           # Default idle state
  idle_with_input.golden        # Idle with partial input
  running_spinner.golden        # Running state with spinner
  output_text.golden            # Text output rendering
  output_tool_start.golden      # Tool start rendering
  output_tool_result.golden     # Tool result rendering
  output_tool_error.golden      # Tool error rendering
  statusbar_empty.golden        # Empty status bar
  statusbar_pills.golden        # Status bar with task pills
  permission_dialog.golden      # Permission dialog layout
  diff_output.golden            # Diff rendering
  markdown_output.golden        # Markdown rendering
  large_output.golden           # 1000+ lines output
```

---

## 2. Golden File Format

Each golden file is a plain text capture of ANSI terminal output:

```
--- golden: idle_default
--- width: 80
--- height: 24
--- description: Default idle state with empty input and ready status

> █

gopher-code ready
```

Golden files include a header with metadata (width, height, description) followed by
the exact expected terminal output including ANSI escape sequences.

### Update process
- `go test -run TestGolden -update` regenerates all golden files
- CI runs `go test -run TestGolden` to compare against committed golden files
- Any diff is a test failure requiring review

---

## 3. Feature Parity Checklist

### 3.1 Core REPL Loop

| # | Feature | Claude Code (Ink) | Gopher (Bubble Tea) | Test Case | Status |
|---|---------|-------------------|----------------------|-----------|--------|
| 1 | REPL prompt display | TextInput component | Model.View() idle state | TestREPLPromptDisplay | Pending |
| 2 | Text input capture | stdin via Ink | tea.KeyMsg handling | TestTextInputCapture | Pending |
| 3 | Enter submits query | onSubmit handler | KeyMsg "enter" in Update | TestEnterSubmitsQuery | Pending |
| 4 | Empty enter ignored | trim check | TrimSpace guard | TestEmptyEnterIgnored | Pending |
| 5 | Ctrl+C exits | process.exit | tea.Quit | TestCtrlCExits | Pending |
| 6 | Ctrl+D exits | process.exit | tea.Quit | TestCtrlDExits | Pending |
| 7 | Input history (up/down) | readline | KeyMsg "up"/"down" | TestInputHistory | Pending |
| 8 | Multi-line input | shift+enter | KeyMsg handling | TestMultiLineInput | Pending |
| 9 | Tab completion | custom handler | KeyMsg "tab" | TestTabCompletion | Pending |
| 10 | Slash command detection | regex match | strings.HasPrefix | TestSlashCommandDetection | Pending |

### 3.2 Streaming Text Display

| # | Feature | Claude Code (Ink) | Gopher (Bubble Tea) | Test Case | Status |
|---|---------|-------------------|----------------------|-----------|--------|
| 11 | Text delta streaming | React state update | QEventTextDelta handling | TestTextDeltaStreaming | Pending |
| 12 | Chunked text assembly | string concatenation | strings.Builder | TestChunkedTextAssembly | Pending |
| 13 | Markdown rendering | marked/terminal | Glamour renderer | TestMarkdownRendering | Pending |
| 14 | Code block syntax highlighting | Ink Code component | Chroma highlighter | TestCodeBlockHighlighting | Pending |
| 15 | Word wrap at terminal width | Ink layout | lipgloss.Width | TestWordWrap | Pending |
| 16 | Auto-scroll to bottom | Ink ScrollArea | viewport.GotoBottom | TestAutoScroll | Pending |
| 17 | Large output handling (>5000 lines) | virtualized list | viewport with buffer | TestLargeOutputHandling | Pending |
| 18 | ANSI color passthrough | raw output | ANSI preservation | TestANSIColorPassthrough | Pending |

### 3.3 Tool Execution Display

| # | Feature | Claude Code (Ink) | Gopher (Bubble Tea) | Test Case | Status |
|---|---------|-------------------|----------------------|-----------|--------|
| 19 | Tool use start indicator | ToolUse component | QEventToolUseStart render | TestToolUseStartIndicator | Pending |
| 20 | Tool name display | component prop | evt.ToolName display | TestToolNameDisplay | Pending |
| 21 | Tool input display | JSON formatted | JSON formatted | TestToolInputDisplay | Pending |
| 22 | Tool result success | green checkmark | green checkmark ANSI | TestToolResultSuccess | Pending |
| 23 | Tool result error | red X | red X ANSI | TestToolResultError | Pending |
| 24 | Tool result truncation | maxLen slice | truncate() at 200 chars | TestToolResultTruncation | Pending |
| 25 | Concurrent tool indicators | parallel spinners | parallel pills | TestConcurrentToolIndicators | Pending |
| 26 | Tool progress spinner | Spinner component | Spinner goroutine | TestToolProgressSpinner | Pending |
| 27 | Bash command display | code block | formatted output | TestBashCommandDisplay | Pending |
| 28 | File diff display | DiffView component | RenderDiff() | TestFileDiffDisplay | Pending |

### 3.4 Permission System UI

| # | Feature | Claude Code (Ink) | Gopher (Bubble Tea) | Test Case | Status |
|---|---------|-------------------|----------------------|-----------|--------|
| 29 | Permission dialog display | PermissionRequest | ShowPermissionDialog | TestPermissionDialogDisplay | Pending |
| 30 | Yes/Allow option | select option | huh.Select "allow" | TestPermissionAllow | Pending |
| 31 | No/Deny option | select option | huh.Select "deny" | TestPermissionDeny | Pending |
| 32 | Always allow option | select option | huh.Select "always" | TestPermissionAlways | Pending |
| 33 | Tool name in dialog | prop display | fmt.Sprintf title | TestToolNameInDialog | Pending |
| 34 | Description truncation | maxLen | truncateForDialog(200) | TestDialogDescriptionTruncation | Pending |
| 35 | Keyboard shortcut (y/n/a) | key handler | interactive policy | TestPermissionKeyboardShortcuts | Pending |
| 36 | Case insensitive input | toLowerCase | strings.ToLower | TestPermissionCaseInsensitive | Pending |
| 37 | Default on empty enter | "allow" | AllowDecision | TestPermissionDefaultAllow | Pending |
| 38 | Per-tool memory | Map<tool, bool> | map[string]always | TestPermissionPerToolMemory | Pending |

### 3.5 Status Bar

| # | Feature | Claude Code (Ink) | Gopher (Bubble Tea) | Test Case | Status |
|---|---------|-------------------|----------------------|-----------|--------|
| 39 | Status text display | footer component | StatusBar.Render() | TestStatusTextDisplay | Pending |
| 40 | Ready state | "ready" text | "gopher-code ready" | TestReadyState | Pending |
| 41 | Thinking state | "thinking..." | "thinking..." | TestThinkingState | Pending |
| 42 | Background task pills | TaskPill components | StatusBar.AddPill() | TestBackgroundTaskPills | Pending |
| 43 | Pill running icon | spinner | "running" icon | TestPillRunningIcon | Pending |
| 44 | Pill completed icon | checkmark | "completed" icon | TestPillCompletedIcon | Pending |
| 45 | Pill failed icon | X mark | "failed" icon | TestPillFailedIcon | Pending |
| 46 | Pill add/remove | dynamic list | AddPill/RemovePill | TestPillAddRemove | Pending |
| 47 | Multiple pills | horizontal layout | strings.Join | TestMultiplePills | Pending |
| 48 | Token usage display | usage component | QEventUsage render | TestTokenUsageDisplay | Pending |
| 49 | Model name display | config prop | session config | TestModelNameDisplay | Pending |
| 50 | Cost display | cost calculator | provider.Cost | TestCostDisplay | Pending |

### 3.6 Session Management

| # | Feature | Claude Code (Ink) | Gopher (Bubble Tea) | Test Case | Status |
|---|---------|-------------------|----------------------|-----------|--------|
| 51 | Session persistence | JSON file | session.Save() | TestSessionPersistence | Pending |
| 52 | Session resume (-c) | --continue flag | --continue flag | TestSessionResume | Pending |
| 53 | Session resume by ID (-r) | --resume flag | --resume flag | TestSessionResumeByID | Pending |
| 54 | Session list display | list component | formatted output | TestSessionListDisplay | Pending |
| 55 | Conversation history | message array | SessionState.Messages | TestConversationHistory | Pending |
| 56 | Turn counter display | turn count | TurnCount field | TestTurnCounterDisplay | Pending |

### 3.7 Output Formats

| # | Feature | Claude Code (Ink) | Gopher (Bubble Tea) | Test Case | Status |
|---|---------|-------------------|----------------------|-----------|--------|
| 57 | Text output (--output-format text) | plain text | PlainTextCallback | TestTextOutputFormat | Pending |
| 58 | JSON output (--output-format json) | JSON envelope | JSONCollector.Emit() | TestJSONOutputFormat | Pending |
| 59 | Stream JSON (--output-format stream-json) | NDJSON | StreamJSONCallback | TestStreamJSONOutputFormat | Pending |
| 60 | Print mode (-p) | print and exit | --print flag | TestPrintMode | Pending |

### 3.8 Keyboard Shortcuts

| # | Feature | Claude Code (Ink) | Gopher (Bubble Tea) | Test Case | Status |
|---|---------|-------------------|----------------------|-----------|--------|
| 61 | Ctrl+C abort | interrupt handler | KeyMsg "ctrl+c" | TestCtrlCAbort | Pending |
| 62 | Ctrl+D exit | EOF handler | KeyMsg "ctrl+d" | TestCtrlDExit | Pending |
| 63 | Up arrow history | readline history | KeyMsg "up" | TestUpArrowHistory | Pending |
| 64 | Down arrow history | readline history | KeyMsg "down" | TestDownArrowHistory | Pending |
| 65 | Escape cancel | cancel handler | KeyMsg "esc" | TestEscapeCancel | Pending |
| 66 | Ctrl+L clear | clear screen | KeyMsg "ctrl+l" | TestCtrlLClear | Pending |
| 67 | Tab complete | completion handler | KeyMsg "tab" | TestTabComplete | Pending |

### 3.9 Error Handling UI

| # | Feature | Claude Code (Ink) | Gopher (Bubble Tea) | Test Case | Status |
|---|---------|-------------------|----------------------|-----------|--------|
| 68 | API error display | ErrorBoundary | Model.Err rendering | TestAPIErrorDisplay | Pending |
| 69 | Network error retry | retry with backoff | retryablehttp | TestNetworkErrorRetry | Pending |
| 70 | Rate limit display | 429 handler | rate limit message | TestRateLimitDisplay | Pending |
| 71 | Auth error display | 401 handler | auth error message | TestAuthErrorDisplay | Pending |
| 72 | Context too long error | 413 handler | compact + retry | TestContextTooLongError | Pending |
| 73 | Tool execution error | error in result | IsError flag render | TestToolExecutionError | Pending |
| 74 | Graceful degradation | fallback rendering | raw text fallback | TestGracefulDegradation | Pending |

### 3.10 Window Management

| # | Feature | Claude Code (Ink) | Gopher (Bubble Tea) | Test Case | Status |
|---|---------|-------------------|----------------------|-----------|--------|
| 75 | Terminal resize handling | Ink resize | tea.WindowSizeMsg | TestTerminalResize | Pending |
| 76 | Width adaptation | responsive layout | Model.Width update | TestWidthAdaptation | Pending |
| 77 | Height adaptation | responsive layout | Model.Height update | TestHeightAdaptation | Pending |
| 78 | Minimum terminal size | min width check | minimum size guard | TestMinimumTerminalSize | Pending |

### 3.11 Theming and Colors

| # | Feature | Claude Code (Ink) | Gopher (Bubble Tea) | Test Case | Status |
|---|---------|-------------------|----------------------|-----------|--------|
| 79 | Dark theme | default dark | lipgloss dark colors | TestDarkTheme | Pending |
| 80 | Light theme | light variant | lipgloss light colors | TestLightTheme | Pending |
| 81 | High contrast | accessibility mode | high contrast theme | TestHighContrastTheme | Pending |
| 82 | ANSI 256 color support | terminal detect | colorprofile detect | TestANSI256Colors | Pending |
| 83 | True color support | terminal detect | colorprofile detect | TestTrueColorSupport | Pending |
| 84 | Tool name cyan coloring | CSS/Ink style | ANSI \033[36m | TestToolNameCyanColor | Pending |
| 85 | Error red coloring | CSS/Ink style | ANSI \033[31m | TestErrorRedColor | Pending |
| 86 | Success green coloring | CSS/Ink style | ANSI \033[32m | TestSuccessGreenColor | Pending |

### 3.12 Advanced Features

| # | Feature | Claude Code (Ink) | Gopher (Bubble Tea) | Test Case | Status |
|---|---------|-------------------|----------------------|-----------|--------|
| 87 | Plan mode display | plan component | plan mode rendering | TestPlanModeDisplay | Pending |
| 88 | Plan approval dialog | approval form | huh confirm dialog | TestPlanApprovalDialog | Pending |
| 89 | Agent/teammate display | agent component | teammate rendering | TestAgentTeammateDisplay | Pending |
| 90 | MCP tool display | tool list | formatted tool list | TestMCPToolDisplay | Pending |
| 91 | Skill invocation | /skill command | slash command handler | TestSkillInvocation | Pending |
| 92 | Image display (kitty/sixel) | image protocol | image protocol | TestImageDisplay | Pending |
| 93 | PDF rendering | pdf viewer | PDF read rendering | TestPDFRendering | Pending |
| 94 | Web fetch result display | html render | formatted web content | TestWebFetchDisplay | Pending |
| 95 | Task list display | todo component | task list rendering | TestTaskListDisplay | Pending |

### 3.13 CLI Flag Parity

| # | Feature | Claude Code | Gopher | Test Case | Status |
|---|---------|-------------|--------|-----------|--------|
| 96 | --model flag | model selection | model selection | TestModelFlag | Pending |
| 97 | --query one-shot | one-shot mode | one-shot mode | TestQueryFlag | Pending |
| 98 | --cwd flag | working directory | working directory | TestCwdFlag | Pending |
| 99 | --system-prompt flag | custom system prompt | custom system prompt | TestSystemPromptFlag | Pending |
| 100 | --max-turns flag | turn limit | turn limit | TestMaxTurnsFlag | Pending |
| 101 | --thinking flag | thinking mode | thinking mode | TestThinkingFlag | Pending |
| 102 | --effort flag | effort level | effort level | TestEffortFlag | Pending |
| 103 | --verbose flag | verbose output | verbose output | TestVerboseFlag | Pending |
| 104 | --permission-mode flag | permission mode | permission mode | TestPermissionModeFlag | Pending |
| 105 | --output-format flag | output format | output format | TestOutputFormatFlag | Pending |

---

## 4. Performance Benchmarks

### 4.1 Render Performance

| Benchmark | Target | Measurement |
|-----------|--------|-------------|
| BenchmarkInitialRender | < 5ms | Time from InitialModel() to first View() |
| BenchmarkTextDeltaRender | < 1ms | Time to process one QEventTextDelta |
| BenchmarkFullViewRender | < 16ms | Time for complete Model.View() (60fps) |
| BenchmarkViewWith100Lines | < 10ms | View() with 100 output lines |
| BenchmarkViewWith1000Lines | < 50ms | View() with 1000 output lines |
| BenchmarkViewWith5000Lines | < 100ms | View() with 5000 output lines |
| BenchmarkStatusBarRender | < 1ms | StatusBar.Render() with 5 pills |
| BenchmarkMarkdownRender | < 50ms | RenderMarkdown() for 500-line doc |
| BenchmarkDiffRender | < 20ms | RenderDiff() for 200-line diff |
| BenchmarkSpinnerFrame | < 0.1ms | Single spinner frame update |

### 4.2 Input Performance

| Benchmark | Target | Measurement |
|-----------|--------|-------------|
| BenchmarkKeyPress | < 1ms | Time from KeyMsg to model update |
| BenchmarkEnterSubmit | < 5ms | Time from enter to query dispatch |
| BenchmarkPermissionDialog | < 10ms | Time to render permission form |

### 4.3 Memory Performance

| Benchmark | Target | Measurement |
|-----------|--------|-------------|
| BenchmarkMemoryIdleSession | < 10MB | Memory after InitialModel() |
| BenchmarkMemory100Turns | < 30MB | Memory after 100 tool turns |
| BenchmarkMemory1000Lines | < 20MB | Memory with 1000 output lines |
| BenchmarkMemoryLongSession | < 50MB | Memory after 30 minutes simulated |

### 4.4 Goroutine Safety

| Test | Description |
|------|-------------|
| TestNoGoroutineLeaks_Spinner | Start/stop spinner, verify no leak (goleak) |
| TestNoGoroutineLeaks_StatusBar | Add/remove pills, verify no leak |
| TestNoGoroutineLeaks_Session | Full session lifecycle, verify no leak |
| TestNoGoroutineLeaks_ToolExecution | Concurrent tool execution, verify no leak |
| TestRaceCondition_StatusBar | StatusBar under concurrent access (-race) |
| TestRaceCondition_Spinner | Spinner start/stop under concurrent access |
| TestRaceCondition_EventLog | EventLog under concurrent writes |

---

## 5. Test Implementations

### 5.1 Unit Test Pattern — Component Isolation

```go
// internal/cli/tui_test.go
func TestModel_IdleState_ShowsPrompt(t *testing.T) {
    m := InitialModel()
    view := m.View()
    if !strings.Contains(view.Body(), ">") {
        t.Error("idle state should show prompt character")
    }
}

func TestModel_Update_EnterWithInput(t *testing.T) {
    m := InitialModel()
    m.Input = "hello"
    updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
    model := updated.(Model)
    if model.State != StateRunning {
        t.Errorf("expected StateRunning, got %v", model.State)
    }
    if model.Input != "" {
        t.Error("input should be cleared after submit")
    }
}

func TestModel_Update_CtrlC(t *testing.T) {
    m := InitialModel()
    _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
    // cmd should be tea.Quit
    if cmd == nil {
        t.Error("ctrl+c should produce quit command")
    }
}

func TestModel_Update_WindowResize(t *testing.T) {
    m := InitialModel()
    updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
    model := updated.(Model)
    if model.Width != 120 || model.Height != 40 {
        t.Errorf("expected 120x40, got %dx%d", model.Width, model.Height)
    }
}
```

### 5.2 Golden File Test Pattern

```go
// pkg/ui/testing/golden.go
func CompareGolden(t *testing.T, name string, actual string) {
    t.Helper()
    goldenPath := filepath.Join("testdata", "golden", name+".golden")

    if os.Getenv("UPDATE_GOLDEN") != "" {
        os.MkdirAll(filepath.Dir(goldenPath), 0755)
        os.WriteFile(goldenPath, []byte(actual), 0644)
        return
    }

    expected, err := os.ReadFile(goldenPath)
    if err != nil {
        t.Fatalf("golden file %s not found (run with UPDATE_GOLDEN=1 to create)", name)
    }
    if string(expected) != actual {
        t.Errorf("golden mismatch for %s:\n%s",
            name, unifiedDiff(string(expected), actual))
    }
}

// Usage in tests:
func TestGolden_IdleDefault(t *testing.T) {
    m := InitialModel()
    m.Width = 80
    m.Height = 24
    CompareGolden(t, "idle_default", m.View().Body())
}

func TestGolden_ToolResult(t *testing.T) {
    var buf strings.Builder
    captureRender(&buf, query.QueryEvent{
        Type:    query.QEventToolResult,
        Content: "file.go written successfully",
    })
    CompareGolden(t, "output_tool_result", buf.String())
}
```

### 5.3 Integration Test Pattern — Component Composition

```go
func TestIntegration_FullQueryCycle(t *testing.T) {
    // Setup scripted provider
    prov := testharness.NewScriptedProvider(
        testharness.MakeTextTurn("Hello! How can I help?", provider.StopReasonEndTurn),
    )

    // Setup session
    sess := testharness.MakeSession()

    // Collect events
    cb, log := testharness.NewEventCollector()

    // Run query loop
    err := query.RunLoop(context.Background(), sess, prov, registry, orch, cb)
    if err != nil {
        t.Fatalf("query loop failed: %v", err)
    }

    // Verify event sequence for UI consumption
    events := log.Events()
    hasTextDelta := false
    hasTurnComplete := false
    for _, e := range events {
        if e.Type == query.QEventTextDelta { hasTextDelta = true }
        if e.Type == query.QEventTurnComplete { hasTurnComplete = true }
    }
    if !hasTextDelta { t.Error("missing text delta event") }
    if !hasTurnComplete { t.Error("missing turn complete event") }
}

func TestIntegration_ToolPermissionFlow(t *testing.T) {
    // Test that tool execution triggers permission check
    // and UI displays the dialog correctly
    spy := testharness.NewSpyTool("Bash", false)
    registry := tools.NewRegistry()
    registry.Register(spy)

    // Simulate tool use turn
    prov := testharness.NewScriptedProvider(
        testharness.MakeToolTurn("t1", "Bash",
            json.RawMessage(`{"command":"ls"}`), provider.StopReasonToolUse),
        testharness.MakeTextTurn("Done!", provider.StopReasonEndTurn),
    )

    cb, log := testharness.NewEventCollector()
    sess := testharness.MakeSession()

    query.RunLoop(context.Background(), sess, prov, registry,
        tools.NewOrchestrator(registry), cb)

    // Verify tool events were emitted for UI
    toolStarts := 0
    toolResults := 0
    for _, e := range log.Events() {
        if e.Type == query.QEventToolUseStart { toolStarts++ }
        if e.Type == query.QEventToolResult { toolResults++ }
    }
    if toolStarts != 1 { t.Errorf("expected 1 tool start, got %d", toolStarts) }
    if toolResults != 1 { t.Errorf("expected 1 tool result, got %d", toolResults) }
}
```

### 5.4 Visual Regression Test Pattern

```go
func TestVisualRegression_AllStates(t *testing.T) {
    states := []struct {
        name  string
        setup func() Model
    }{
        {"idle_empty", func() Model { return InitialModel() }},
        {"idle_with_input", func() Model {
            m := InitialModel()
            m.Input = "hello world"
            return m
        }},
        {"running", func() Model {
            m := InitialModel()
            m.State = StateRunning
            m.StatusText = "thinking..."
            return m
        }},
        {"with_output", func() Model {
            m := InitialModel()
            m.Output = []string{"> hello", "Hi there! How can I help?"}
            return m
        }},
        {"with_tool_output", func() Model {
            m := InitialModel()
            m.Output = []string{
                "> read file.go",
                "\033[36m⚙ Read\033[0m",
                "\033[32m✓ file.go (45 lines)\033[0m",
                "Here is the file content...",
            }
            return m
        }},
        {"with_error", func() Model {
            m := InitialModel()
            m.Output = []string{
                "> bad command",
                "\033[36m⚙ Bash\033[0m",
                "\033[31m✗ Error: command not found\033[0m",
            }
            return m
        }},
    }

    for _, s := range states {
        t.Run(s.name, func(t *testing.T) {
            m := s.setup()
            m.Width = 80
            m.Height = 24
            CompareGolden(t, s.name, m.View().Body())
        })
    }
}
```

### 5.5 Performance Benchmark Pattern

```go
func BenchmarkModelView(b *testing.B) {
    m := InitialModel()
    m.Width = 120
    m.Height = 40
    for i := 0; i < 100; i++ {
        m.Output = append(m.Output, fmt.Sprintf("Line %d: some output text here", i))
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _ = m.View()
    }
}

func BenchmarkModelViewLargeOutput(b *testing.B) {
    m := InitialModel()
    m.Width = 120
    m.Height = 40
    for i := 0; i < 1000; i++ {
        m.Output = append(m.Output, fmt.Sprintf("Line %d: %s", i,
            strings.Repeat("x", 80)))
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _ = m.View()
    }
}

func BenchmarkStatusBarRender(b *testing.B) {
    sb := NewStatusBar()
    sb.AddPill("p1", "Agent 1")
    sb.AddPill("p2", "Agent 2")
    sb.AddPill("p3", "MCP server")
    sb.AddPill("p4", "Background task")
    sb.AddPill("p5", "File watcher")

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _ = sb.Render()
    }
}

func BenchmarkMarkdownRender(b *testing.B) {
    // 500-line markdown document
    var md strings.Builder
    for i := 0; i < 500; i++ {
        md.WriteString(fmt.Sprintf("## Section %d\n\nSome text with **bold** and `code`.\n\n", i))
    }
    input := md.String()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = RenderMarkdown(input)
    }
}
```

### 5.6 Goroutine Leak Test Pattern

```go
// Requires: go.uber.org/goleak
import "go.uber.org/goleak"

func TestNoGoroutineLeaks_Spinner(t *testing.T) {
    defer goleak.VerifyNone(t)

    s := NewSpinner("testing")
    s.Start()
    time.Sleep(200 * time.Millisecond)
    s.Stop()
    time.Sleep(100 * time.Millisecond) // Allow goroutine to exit
}

func TestNoGoroutineLeaks_StatusBar(t *testing.T) {
    defer goleak.VerifyNone(t)

    sb := NewStatusBar()
    for i := 0; i < 100; i++ {
        id := fmt.Sprintf("pill-%d", i)
        sb.AddPill(id, "test")
        sb.Render()
        sb.RemovePill(id)
    }
}
```

---

## 6. UI Test Utilities (pkg/ui/testing/)

### 6.1 capture.go — Terminal Output Capture

```go
// CaptureOutput renders a Bubble Tea model and returns its view string.
func CaptureOutput(m tea.Model) string {
    return m.View().(tea.View).Body()
}

// CaptureEventRender renders a QueryEvent through PrintEvent and captures output.
func CaptureEventRender(evt query.QueryEvent) string {
    // Redirect stdout, call PrintEvent, capture output
}
```

### 6.2 simulate.go — Input Simulation

```go
// SimulateKeyPress sends a key message to a model and returns the updated model.
func SimulateKeyPress(m tea.Model, key string) (tea.Model, tea.Cmd) {
    return m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
}

// SimulateEnter sends an enter key press.
func SimulateEnter(m tea.Model) (tea.Model, tea.Cmd) {
    return m.Update(tea.KeyMsg{Type: tea.KeyEnter})
}

// SimulateResize sends a window resize message.
func SimulateResize(m tea.Model, w, h int) (tea.Model, tea.Cmd) {
    return m.Update(tea.WindowSizeMsg{Width: w, Height: h})
}

// SimulateSequence sends a sequence of inputs and returns the final model.
func SimulateSequence(m tea.Model, inputs ...tea.Msg) tea.Model {
    for _, input := range inputs {
        m, _ = m.Update(input)
    }
    return m
}
```

### 6.3 assert.go — Component State Assertions

```go
// AssertState checks that a Model is in the expected state.
func AssertState(t *testing.T, m Model, expected AppState) {
    t.Helper()
    if m.State != expected {
        t.Errorf("expected state %v, got %v", expected, m.State)
    }
}

// AssertOutputContains checks that the view contains expected text.
func AssertOutputContains(t *testing.T, m tea.Model, expected string) {
    t.Helper()
    view := m.View().(tea.View).Body()
    if !strings.Contains(view, expected) {
        t.Errorf("view does not contain %q", expected)
    }
}

// AssertOutputNotContains checks that the view does NOT contain text.
func AssertOutputNotContains(t *testing.T, m tea.Model, unexpected string) {
    t.Helper()
    view := m.View().(tea.View).Body()
    if strings.Contains(view, unexpected) {
        t.Errorf("view should not contain %q", unexpected)
    }
}

// AssertEventSequence checks events match expected types in order.
func AssertEventSequence(t *testing.T, events []query.QueryEvent, expectedTypes ...query.QEventType) {
    t.Helper()
    idx := 0
    for _, e := range events {
        if idx < len(expectedTypes) && e.Type == expectedTypes[idx] {
            idx++
        }
    }
    if idx != len(expectedTypes) {
        t.Errorf("event sequence mismatch: matched %d of %d expected types", idx, len(expectedTypes))
    }
}
```

---

## 7. CI/CD Integration

### 7.1 Test Pipeline

```yaml
# .github/workflows/ui-tests.yml
jobs:
  unit-tests:
    runs-on: ubuntu-latest
    steps:
      - run: go test ./internal/cli/... -v -race -count=1
      - run: go test ./pkg/ui/... -v -race -count=1

  golden-tests:
    runs-on: ubuntu-latest
    steps:
      - run: go test ./internal/cli/... -run TestGolden -v
      - run: go test ./internal/cli/... -run TestVisualRegression -v

  benchmarks:
    runs-on: ubuntu-latest
    steps:
      - run: go test ./internal/cli/... -bench=. -benchmem -count=3
      - run: go test ./pkg/ui/... -bench=. -benchmem -count=3

  goroutine-leaks:
    runs-on: ubuntu-latest
    steps:
      - run: go test ./internal/cli/... -run TestNoGoroutineLeaks -v
      - run: go test ./internal/cli/... -race -count=5

  parity-tests:
    runs-on: ubuntu-latest
    steps:
      - run: go test ./pkg/query/... -run TestParity -v
      - run: go test ./pkg/tools/... -run TestParity -v
      - run: go test ./pkg/tools/... -run TestBehavior -v
```

### 7.2 Pre-commit Hook

```bash
# Run fast UI tests before commit
go test ./internal/cli/... -short -count=1
```

### 7.3 Benchmark Tracking

- Store benchmark results in CI artifacts
- Compare against previous run to detect regressions
- Alert if any benchmark exceeds 2x its target threshold

---

## 8. Manual Testing Scenarios

### Scenario 1: Fresh Start
1. Launch `gopher-code` with no arguments
2. Verify: REPL prompt appears with cursor
3. Verify: Status bar shows "gopher-code ready"
4. Verify: Terminal dimensions are detected

### Scenario 2: Basic Query
1. Type "What is 2+2?" and press Enter
2. Verify: Input clears and status shows "thinking..."
3. Verify: Streaming text appears character by character
4. Verify: Response completes and prompt returns
5. Verify: Status bar returns to "ready"

### Scenario 3: Tool Execution (Read)
1. Type "Read the file go.mod"
2. Verify: Tool indicator shows "Read" in cyan
3. Verify: Tool result shows green checkmark
4. Verify: File contents displayed
5. Verify: Response continues after tool result

### Scenario 4: Tool Execution (Bash)
1. Type "Run ls -la in the current directory"
2. Verify: Permission dialog appears
3. Select "Yes, allow this time"
4. Verify: Tool executes and shows output
5. Verify: Progress spinner during execution

### Scenario 5: Permission Deny
1. Type "Delete all files in /tmp/test"
2. Verify: Permission dialog appears
3. Select "No, deny"
4. Verify: Tool is not executed
5. Verify: Assistant acknowledges denial

### Scenario 6: Permission Always Allow
1. Type "Read the README"
2. Verify: Permission dialog appears
3. Select "Always allow this tool"
4. Type "Read the go.mod"
5. Verify: No permission dialog (remembered)

### Scenario 7: Multiple Tool Calls
1. Type "Read go.mod and go.sum at the same time"
2. Verify: Both tools shown as concurrent
3. Verify: Both results displayed
4. Verify: Correct ordering maintained

### Scenario 8: Large Output
1. Type "Show me the contents of a large file (>1000 lines)"
2. Verify: Output renders without freezing
3. Verify: Auto-scroll follows output
4. Verify: Terminal remains responsive

### Scenario 9: Error Handling
1. Type "Read /nonexistent/file.txt"
2. Verify: Error displayed in red
3. Verify: Assistant handles error gracefully
4. Verify: REPL remains functional

### Scenario 10: Markdown Rendering
1. Type "Write me a markdown document with headers, code blocks, and lists"
2. Verify: Headers are bold/large
3. Verify: Code blocks have syntax highlighting
4. Verify: Lists are properly indented
5. Verify: Links are rendered

### Scenario 11: Diff Display
1. Type "Edit a file to add a comment"
2. Verify: Diff shows with red (removed) and green (added) lines
3. Verify: File path is displayed in header
4. Verify: Context lines shown

### Scenario 12: Session Continue
1. Complete a conversation
2. Exit gopher-code
3. Run `gopher-code -c`
4. Verify: Previous conversation loaded
5. Verify: Context maintained

### Scenario 13: Session Resume by ID
1. Complete a conversation
2. Note session ID
3. Run `gopher-code -r <session-id>`
4. Verify: Specific session loaded

### Scenario 14: One-Shot Mode
1. Run `gopher-code -p "What is Go?"`
2. Verify: Response printed to stdout
3. Verify: Process exits after response
4. Verify: No REPL prompt

### Scenario 15: JSON Output
1. Run `gopher-code --output-format json -p "hello"`
2. Verify: JSON envelope printed
3. Verify: Contains type, role, content, stop_reason
4. Parse output as valid JSON

### Scenario 16: Stream JSON Output
1. Run `gopher-code --output-format stream-json -p "hello"`
2. Verify: NDJSON lines printed
3. Verify: Each line is valid JSON
4. Verify: Events match query event types

### Scenario 17: Terminal Resize
1. Start gopher-code in a small terminal
2. Resize terminal to larger size
3. Verify: Layout adapts to new dimensions
4. Verify: No rendering artifacts

### Scenario 18: Ctrl+C During Query
1. Start a long query
2. Press Ctrl+C during execution
3. Verify: Query aborted cleanly
4. Verify: Prompt returns
5. Verify: No orphan goroutines

### Scenario 19: Spinner Animation
1. Trigger a slow tool execution
2. Verify: Spinner animates smoothly (80ms frames)
3. Verify: Spinner stops when tool completes
4. Verify: Spinner line cleared after completion

### Scenario 20: Status Bar Pills
1. Trigger multiple background tasks
2. Verify: Pills appear in status bar
3. Verify: Pills show correct icons (running/completed/failed)
4. Verify: Pills removed when tasks complete

### Scenario 21: Slash Commands
1. Type "/help" at the prompt
2. Verify: Help information displayed
3. Type "/compact" at the prompt
4. Verify: Compaction executed

### Scenario 22: Plan Mode
1. Enable plan mode
2. Submit a complex request
3. Verify: Plan displayed for review
4. Approve or reject the plan
5. Verify: Execution proceeds or stops accordingly

### Scenario 23: Cost Display
1. Complete a multi-turn conversation
2. Verify: Token usage displayed
3. Verify: Cost estimate shown
4. Verify: Per-model pricing applied

### Scenario 24: Theme Detection
1. Run in a dark terminal
2. Verify: Dark theme colors applied
3. Switch to light terminal (if possible)
4. Verify: Light theme colors applied

### Scenario 25: Completions
1. Run `gopher-code --completion bash` (or zsh/fish)
2. Verify: Completion script output
3. Verify: All flags present
4. Source the completion and verify tab completion works

---

## 9. Success Criteria

Feature parity is declared achieved when ALL of the following are met:

1. **Unit Test Coverage**: >= 80% line coverage for `internal/cli/` and `pkg/ui/`
2. **Golden File Tests**: All golden file tests pass (0 visual regressions)
3. **Parity Checklist**: >= 95 of 105 items marked as passing
4. **Performance Benchmarks**: All benchmarks within 2x of target thresholds
5. **Goroutine Safety**: All goroutine leak tests pass with -race flag
6. **Integration Tests**: All integration tests pass
7. **Manual Testing**: All 25 scenarios verified by at least one reviewer
8. **No P0 Bugs**: Zero critical or blocking issues in the UI
9. **Behavioral Parity**: All existing parity_test.go / parity_gaps_test.go still pass
10. **Differential Tests**: Zero behavioral regressions detected by differential harness

---

## 10. Measurement Framework Summary

| Utility | Purpose | Location |
|---------|---------|----------|
| `CaptureOutput(model)` | Render model to string | pkg/ui/testing/capture.go |
| `CompareGolden(t, name, actual)` | Golden file comparison | pkg/ui/testing/golden.go |
| `SimulateKeyPress(model, key)` | Input simulation | pkg/ui/testing/simulate.go |
| `SimulateSequence(model, msgs...)` | Multi-step input | pkg/ui/testing/simulate.go |
| `AssertState(t, model, state)` | State assertions | pkg/ui/testing/assert.go |
| `AssertOutputContains(t, model, text)` | View content check | pkg/ui/testing/assert.go |
| `AssertEventSequence(t, events, types...)` | Event order check | pkg/ui/testing/assert.go |
| `NewEventCollector()` | Event capture | internal/testharness/event_collector.go |
| `NewScriptedProvider(turns...)` | Deterministic provider | internal/testharness/scripted_provider.go |
| `NewSpyTool(name, readOnly)` | Tool call recording | internal/testharness/spy_tool.go |
| `RunDifferential(...)` | Behavioral comparison | internal/testharness/differential.go |
| `NewRecordingProvider(inner)` | Session recording | internal/testharness/recorder.go |
| `NewReplayProvider(session)` | Session replay | internal/testharness/recorder.go |
