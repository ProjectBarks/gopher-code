# Gopher UI Framework — Troubleshooting Guide

This guide covers common issues when using the Gopher UI framework and how to resolve them.

## Common Issues

### Rendering & Display

#### Issue: Components not appearing on screen

**Symptoms**: Application runs but components are invisible

**Causes**:
1. Component size not set — SetSize() not called
2. View() returns empty string
3. Terminal too small for layout

**Solution**:
```go
// Ensure SetSize is called on all components
component.SetSize(width, height)

// Check View() returns non-empty content
view := component.View()
if view.Content == "" {
    // Debug: component not rendering
}

// Test with larger terminal (e.g., 120x40)
```

#### Issue: Text gets cut off at edges

**Symptoms**: Long messages truncated in ConversationPane

**Causes**:
1. Width not set correctly in SetSize()
2. Text wrapping disabled in MessageBubble
3. Lipgloss width constraint not applied

**Solution**:
```go
// Ensure width is correctly set
pane.SetSize(width, height)

// Check message rendering
msg := message.UserMessage("Long text here...")
rendered := bubble.Render(&msg, width)
// Verify rendered length <= width
```

#### Issue: Flickering on update

**Symptoms**: UI flickers when new messages arrive

**Causes**:
1. Viewport not configured for smooth scrolling
2. Message pre-rendering causing delays
3. Too many goroutines updating simultaneously

**Solution**:
```go
// ConversationPane handles this internally
// If custom implementation, ensure:
// 1. Pre-render messages before Update()
// 2. Use viewport's built-in scrolling
// 3. Batch updates in single render pass
```

### Keyboard & Input

#### Issue: Keyboard input not working

**Symptoms**: Typing doesn't appear in InputPane

**Causes**:
1. Component not focused — Focus() not called
2. InputPane not in focus chain
3. KeyPress message not reaching component

**Solution**:
```go
// Verify component is focused
if !input.Focused() {
    input.Focus()
}

// Check FocusManager is routing Tab correctly
focusManager.Next()  // Should advance focus
focusManager.Prev()  // Should go back

// Debug: log messages
switch msg := msg.(type) {
case tea.KeyPressMsg:
    log.Printf("Key pressed: %s", msg.String())
}
```

#### Issue: Tab/Shift+Tab not switching focus

**Symptoms**: Can't navigate between components with Tab key

**Causes**:
1. FocusManager not initialized
2. Components not added to focus ring
3. Modal is blocking focus navigation

**Solution**:
```go
// FocusManager should be created in AppModel
app.focusManager = core.NewFocusManager()

// Add components to focus ring (in order)
app.focusManager.Register(app.conversationPane)
app.focusManager.Register(app.inputPane)

// If modal is open, focus should go to modal
if app.modalStack.Len() > 0 {
    // Modal takes precedence
}
```

#### Issue: Ctrl+C not quitting

**Symptoms**: Application won't exit with Ctrl+C

**Causes**:
1. KeyPress message consumed before reaching AppModel
2. Component intercepting Ctrl+C
3. Program not handling tea.QuitMsg

**Solution**:
```go
// AppModel.Update should handle Ctrl+C globally
switch msg := msg.(type) {
case tea.KeyPressMsg:
    if msg.String() == "ctrl+c" {
        return app, tea.Quit
    }
}

// Or let component handle it and return tea.Quit
```

### Focus & Selection

#### Issue: Component stuck in focus

**Symptoms**: Component stays focused, can't move focus away

**Causes**:
1. Focused() always returns true
2. Blur() not clearing focused state
3. FocusManager bug in focus cycling

**Solution**:
```go
// Check Blur() implementation
func (c *MyComponent) Blur() {
    c.focused = false  // Must clear state
}

// Check Focused() implementation
func (c *MyComponent) Focused() bool {
    return c.focused  // Must reflect actual state
}

// Test focus cycling
focusManager := core.NewFocusManager()
focusManager.Next()
focusManager.Prev()
// Should cycle through all components correctly
```

#### Issue: Modals not getting focus

**Symptoms**: Modal appears but keyboard input goes to background

**Causes**:
1. Modal not added to focus manager
2. Modal not implemented Focusable interface
3. Focus override not working

**Solution**:
```go
// Modals should override normal focus
if app.modalStack.Len() > 0 {
    topModal := app.modalStack.Top()
    topModal.Update(msg)  // Route to modal first
} else {
    // Normal focus chain
    focused.Update(msg)
}
```

### Messages & Events

#### Issue: Messages not reaching components

**Symptoms**: Component.Update() not called or receives wrong messages

**Causes**:
1. Message type not matching component's expected types
2. AppModel not routing to correct component
3. Message lost in event bridge

**Solution**:
```go
// Check message routing in AppModel
func (app *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case AddMessageMsg:
        // Should route to ConversationPane
        app.conversationPane.Update(msg)
    case tea.KeyPressMsg:
        // Route to focused component
        if app.modalStack.Len() > 0 {
            app.modalStack.Top().Update(msg)
        } else {
            // Route to focused component
        }
    }
    return app, nil
}
```

#### Issue: Query events not appearing in UI

**Symptoms**: Query results don't show in ConversationPane

**Causes**:
1. EventBridge not connected
2. Program not set in EventBridge
3. Message not being sent to program

**Solution**:
```go
// Create and connect EventBridge
bridge := ui.NewEventBridge()
p := tea.NewProgram(app)
bridge.SetProgram(p)

// Pass bridge callback to query system
callback := bridge.BridgeCallback()
provider.OnQueryEvent(callback)

// EventBridge should route to AppModel via program.Send()
```

#### Issue: Memory leak from goroutines

**Symptoms**: Application memory grows over time

**Causes**:
1. Goroutines not cleaned up on exit
2. Channels not closed
3. Event bridge holding references

**Solution**:
```bash
# Run tests with -race to detect issues
go test -race ./pkg/ui

# Check for goroutine leaks
go test -run TestGoroutineLeaks ./pkg/ui

# Verify cleanup in app.Init() and tea.Cmd termination
```

### Theme & Colors

#### Issue: Colors not applying

**Symptoms**: All text is white, no colors visible

**Causes**:
1. Theme not initialized
2. Terminal doesn't support colors
3. TERM variable not set correctly

**Solution**:
```go
// Initialize theme
th := theme.Current()
if th == nil {
    theme.SetTheme(theme.ThemeDark)
}

// Check environment
os.Getenv("TERM")  // Should be xterm-256color or similar

// Test color output
colors := th.Colors()
log.Printf("Primary: %s", colors.Primary)
```

#### Issue: High contrast theme not applying

**Symptoms**: Theme set to HighContrast but colors look same

**Causes**:
1. HighContrast theme not implemented
2. Components not reading from theme
3. Hardcoded colors overriding theme

**Solution**:
```go
// Always read colors from theme
th := theme.Current()
colors := th.Colors()
// Use colors.Primary instead of hardcoded "#0000FF"

// Verify HighContrast is implemented
theme.SetTheme(theme.ThemeHighContrast)
th = theme.Current()
// Should return HighContrast theme instance
```

### Performance

#### Issue: UI slow to respond

**Symptoms**: Lag when typing or scrolling

**Causes**:
1. Too many messages per frame
2. Message pre-rendering too slow
3. Viewport rendering full history

**Solution**:
```go
// Profile with pprof
import _ "net/http/pprof"

// Batch updates
// Pre-render messages on background goroutine
// Use virtual scrolling (don't render off-screen)

// Check message count
log.Printf("Messages: %d", len(messages))
// If > 1000, consider pagination
```

#### Issue: Startup slow

**Symptoms**: App takes >100ms to appear

**Causes**:
1. Loading large session history
2. Rendering all messages at startup
3. Theme initialization slow

**Solution**:
```go
// Lazy load message history
// Render only visible window initially
// Defer rendering off-screen messages

// Profile startup
time.Sleep(100ms)  // Wait for render
// Should be complete by this point
```

### Size & Layout

#### Issue: Components don't resize with terminal

**Symptoms**: Application crashes or displays incorrectly when terminal resized

**Causes**:
1. SetSize() not called on resize
2. Component doesn't handle new size
3. Layout math overflow

**Solution**:
```go
// AppModel.Update should handle tea.WindowSizeMsg
switch msg := msg.(type) {
case tea.WindowSizeMsg:
    // Update all component sizes
    app.setComponentSizes(msg.Width, msg.Height)
}

// setComponentSizes should call SetSize on all components
func (app *AppModel) setComponentSizes(w, h int) {
    // Header: 1 line
    app.header.SetSize(w, 1)
    
    // Conversation: flexible
    app.conversation.SetSize(w, h-5)
    
    // Input: 3 lines
    app.input.SetSize(w, 3)
    
    // Status: 1 line
    app.status.SetSize(w, 1)
}
```

#### Issue: Layout breaks on very small terminals

**Symptoms**: UI doesn't fit or overlaps on small terminals (<80x24)

**Causes**:
1. No minimum size checks
2. Components assume minimum height
3. Layout doesn't adapt

**Solution**:
```go
// Check minimum terminal size
if width < 80 || height < 20 {
    // Warn user or use compact mode
}

// Adapt layout for small sizes
if width < 80 {
    // Hide side panel
    // Use single column
}

// Components should handle zero height gracefully
if height <= 0 {
    return  // Nothing to render
}
```

## Debugging

### Enable Logging

```go
import "log"

// Create debug logger
log.SetFlags(log.LstdFlags | log.Lshortfile)

// Log in Update()
log.Printf("Component received: %T", msg)
log.Printf("Focus: %v", component.Focused())
```

### Capture Output

```bash
# Redirect stderr to file
go run ./cmd/gopher-code 2>debug.log

# View logs
tail -f debug.log
```

### Test Individually

```bash
# Test single component
go test -run TestConversationPane ./pkg/ui/components

# Test with verbose output
go test -v -run TestConversationPane ./pkg/ui/components

# Test with race detector
go test -race -run TestConversationPane ./pkg/ui/components
```

### Inspect State

Add debug output to View():

```go
func (c *MyComponent) View() tea.View {
    debug := fmt.Sprintf(
        "width=%d height=%d focused=%v messages=%d",
        c.width, c.height, c.focused, len(c.messages),
    )
    return tea.NewView(debug)
}
```

## Performance Profiling

### CPU Profile

```go
import _ "net/http/pprof"

// In main:
go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()

// While running, in another terminal:
// go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
```

### Memory Profile

```bash
go test -memprofile=mem.prof ./pkg/ui
go tool pprof mem.prof
(pprof) top
```

### Benchmark

```bash
go test -bench=. -benchmem ./pkg/ui

# Profile benchmark
go test -bench=. -benchmem -cpuprofile=cpu.prof ./pkg/ui
go tool pprof cpu.prof
```

## Getting Help

1. **Check the examples** — `/examples/` has working code
2. **Read the godocs** — `go doc ./pkg/ui/...`
3. **Run tests** — `go test -v ./pkg/ui`
4. **Check GitHub issues** — Known bugs and solutions
5. **File an issue** — Reproduce in minimal example

## Common Commands

```bash
# Build
go build -o gopher ./cmd/gopher-code

# Run tests
go test ./pkg/ui

# Run with race detector
go test -race ./pkg/ui

# Check coverage
go test -cover ./pkg/ui

# Generate coverage report
go test -coverprofile=cover.out ./pkg/ui
go tool cover -html=cover.out
```

## Further Reading

- [UI_ARCHITECTURE.md](../UI_ARCHITECTURE.md) — Detailed architecture
- [README.md](./README.md) — Getting started guide
- [Bubbletea FAQ](https://github.com/charmbracelet/bubbletea/wiki/FAQ)
- [Lipgloss Styling](https://github.com/charmbracelet/lipgloss)
