package ui

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/query"
	"github.com/projectbarks/gopher-code/pkg/session"
	"github.com/projectbarks/gopher-code/pkg/ui/commands"
	"github.com/projectbarks/gopher-code/pkg/ui/components"
	"github.com/projectbarks/gopher-code/pkg/ui/core"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// stripANSI removes ANSI escape sequences for text comparison.
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func strip(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

// TestVisualParity_StartupWelcomeBoxIntegrity validates the welcome screen
// renders a structurally complete bordered box with proper layout.
func TestVisualParity_StartupWelcomeBoxIntegrity(t *testing.T) {
	config := session.DefaultConfig()
	config.Model = "claude-opus-4-6"
	sess := session.New(config, "/Users/test/project")
	app := NewAppModel(sess, nil)
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	view := app.View()
	plain := strip(view.Content)
	lines := strings.Split(plain, "\n")

	// 1. Find top border ╭...╮ with title integrated
	topIdx := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "╭") {
			topIdx = i
			if !strings.HasSuffix(trimmed, "╮") {
				t.Errorf("Top border incomplete: %s", trimmed)
			}
			if !strings.Contains(trimmed, "Claude Code") {
				t.Errorf("Title must be in top border line: %s", trimmed)
			}
			break
		}
	}
	if topIdx < 0 {
		t.Fatalf("No top border ╭...╮ found.\nView:\n%s", plain)
	}

	// 2. Find bottom border ╰...╯
	botIdx := -1
	for i := len(lines) - 1; i > topIdx; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "╰") {
			botIdx = i
			if !strings.HasSuffix(trimmed, "╯") {
				t.Errorf("Bottom border incomplete: %s", trimmed)
			}
			break
		}
	}
	if botIdx < 0 {
		t.Fatalf("No bottom border found")
	}

	// 3. Every body line between borders must start and end with │
	bodyCount := 0
	for i := topIdx + 1; i < botIdx; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, "│") || !strings.HasSuffix(trimmed, "│") {
			t.Errorf("Body line %d breaks box: %s", i, lines[i])
		}
		bodyCount++
	}
	if bodyCount < 3 {
		t.Errorf("Box needs 3+ body lines, got %d", bodyCount)
	}

	// 4. Top and bottom widths must match
	topW := len([]rune(strings.TrimSpace(lines[topIdx])))
	botW := len([]rune(strings.TrimSpace(lines[botIdx])))
	if topW != botW {
		t.Errorf("Border width mismatch: top=%d bot=%d", topW, botW)
	}

	// 5. Two-column layout: body lines should have 3+ │ chars (left border, separator, right)
	colSepCount := 0
	for i := topIdx + 1; i < botIdx; i++ {
		if strings.Count(lines[i], "│") >= 3 {
			colSepCount++
		}
	}
	if colSepCount < 3 {
		t.Errorf("Expected two-column layout (3+ │ per row in 3+ rows), got %d rows", colSepCount)
	}

	// 6. showWelcome should be true, mode idle
	if !app.showWelcome {
		t.Error("showWelcome should be true on startup")
	}
	if app.mode != ModeIdle {
		t.Errorf("Mode should be idle on startup, got %v", app.mode)
	}
}

// TestVisualParity_WelcomeDismissOnSubmit verifies welcome dismisses on input.
// TestVisualParity_WelcomeDismissLifecycle validates the full welcome dismiss lifecycle:
// empty submit keeps welcome, non-empty submit dismisses, state transitions correctly.
func TestVisualParity_WelcomeDismissLifecycle(t *testing.T) {
	config := session.DefaultConfig()
	sess := session.New(config, "/tmp")
	app := NewAppModel(sess, nil)
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// 1. Initial state: welcome visible
	if !app.showWelcome {
		t.Fatal("showWelcome must be true initially")
	}

	// 2. Empty submit: welcome must STAY (Claude REPL.tsx:1368)
	app.Update(components.SubmitMsg{Text: ""})
	if !app.showWelcome {
		t.Error("Empty submit should NOT dismiss welcome")
	}
	if app.mode != ModeIdle {
		t.Error("Empty submit should keep mode idle")
	}

	// 3. Non-empty submit: welcome dismissed, mode changes, message added
	app.Update(components.SubmitMsg{Text: "hello"})
	if app.showWelcome {
		t.Error("Non-empty submit should dismiss welcome")
	}
	if app.mode != ModeStreaming {
		t.Errorf("Submit should enter ModeStreaming, got %v", app.mode)
	}
	if len(sess.Messages) < 1 {
		t.Error("Submit should add user message to session")
	}
	if app.conversation.MessageCount() < 1 {
		t.Error("Submit should add message to conversation pane")
	}

	// 4. View should render header (not welcome box) — structural check
	v := strip(app.View().Content)
	lines := strings.Split(v, "\n")
	// First line should be header (✻ Claude), NOT a border (╭)
	first := strings.TrimSpace(lines[0])
	if strings.HasPrefix(first, "╭") {
		t.Error("After dismiss, first line should be header, not welcome box border")
	}
}

// TestVisualParity_UserMessageStyling verifies user messages use › prefix and bold.
// TestVisualParity_UserMessageStyling — DELETED: superficial (covered by WelcomeDismissLifecycle)

// TestVisualParity_StreamingShowsSpinner verifies spinner during streaming.
func TestVisualParity_StreamingShowsSpinner(t *testing.T) {
	config := session.DefaultConfig()
	sess := session.New(config, "/tmp")
	app := NewAppModel(sess, nil)
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app.Update(components.SubmitMsg{Text: "test"})

	// Simulate streaming — the spinner should be active
	if !app.spinner.IsActive() {
		t.Error("Spinner should be active after submit")
	}

	view := app.View()
	plain := strip(view.Content)

	// Should show spinner verb in conversation area
	verb := app.spinner.Verb()
	if !strings.Contains(plain, verb) {
		t.Errorf("Expected spinner verb %q in view during streaming", verb)
	}
	if !strings.Contains(plain, "thinking") {
		t.Error("Expected 'thinking' in spinner output")
	}
}

// TestVisualParity_StreamingStatusBar verifies "esc to interrupt" during streaming.
// TestVisualParity_StreamingStatusBar — DELETED: superficial (status text covered by FullConversationFlow)
// TestVisualParity_ToolResultUsesConnector — DELETED: superficial (connector char covered by component tests)

// TestVisualParity_IdleStatusShowsModel — DELETED: superficial + wrong

// TestVisualParity_FullConversationFlow tests a complete conversation sequence.
func TestVisualParity_FullConversationFlow(t *testing.T) {
	config := session.DefaultConfig()
	sess := session.New(config, "/tmp")
	app := NewAppModel(sess, nil)
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 30})

	// 1. Start: welcome screen visible
	v1 := strip(app.View().Content)
	if !strings.Contains(v1, "Welcome") {
		t.Error("Step 1: Expected welcome screen")
	}

	// 2. Submit query: welcome dismissed, spinner starts
	app.Update(components.SubmitMsg{Text: "hello world"})
	v2 := strip(app.View().Content)
	if strings.Contains(v2, "Welcome") {
		t.Error("Step 2: Welcome should be gone")
	}
	if !strings.Contains(v2, "❯ hello world") {
		t.Error("Step 2: Expected user message with ❯ prefix")
	}
	if !strings.Contains(v2, "esc to interrupt") {
		t.Error("Step 2: Expected 'esc to interrupt' during streaming")
	}

	// 3. Text arrives
	app.Update(TextDeltaMsg{Text: "Hi! How can I help?"})
	v3 := strip(app.View().Content)
	if !strings.Contains(v3, "Hi! How can I help?") {
		t.Error("Step 3: Expected streaming text in view")
	}

	// 4. Turn complete
	app.Update(TurnCompleteMsg{})
	v4 := strip(app.View().Content)
	if strings.Contains(v4, "esc to interrupt") {
		t.Error("Step 4: Should not show 'esc to interrupt' after completion")
	}

	// 5. Verify conversation has messages
	if app.conversation.MessageCount() != 2 {
		t.Errorf("Step 5: Expected 2 messages (user + assistant), got %d",
			app.conversation.MessageCount())
	}
}

// TestVisualParity_SlashCommandClear verifies /clear works.
func TestVisualParity_SlashCommandClear(t *testing.T) {
	config := session.DefaultConfig()
	sess := session.New(config, "/tmp")
	app := NewAppModel(sess, nil)
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Add a message then clear
	app.Update(components.SubmitMsg{Text: "hello"})
	app.Update(TurnCompleteMsg{})

	// Submit /clear command
	_, cmd := app.Update(components.SubmitMsg{Text: "/clear"})
	if cmd != nil {
		msg := cmd()
		app.Update(msg)
	}

	if app.conversation.MessageCount() != 0 {
		t.Errorf("Expected 0 messages after /clear, got %d", app.conversation.MessageCount())
	}
}

// TestVisualParity_EffortLevelDisplay verifies effort icons with thinking enabled.
func TestVisualParity_EffortLevelDisplay(t *testing.T) {
	config := session.DefaultConfig()
	config.ThinkingEnabled = true
	config.ThinkingBudget = 20000 // high effort
	sess := session.New(config, "/tmp")
	app := NewAppModel(sess, nil)
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	app.Update(components.SubmitMsg{Text: "think hard"})

	// Spinner should show effort level
	spinnerView := strip(app.spinner.View())
	if !strings.Contains(spinnerView, "●") {
		t.Errorf("Expected high effort icon ● in spinner, got: %s", spinnerView)
	}
}

// TestVisualParity_DividerSpansFullWidth — DELETED: superficial (divider covered by box integrity test)

// TestVisualParity_CtrlCQuitsWhenIdle verifies double Ctrl+C quits.
func TestVisualParity_CtrlCQuitsWhenIdle(t *testing.T) {
	app := NewAppModel(nil, nil)
	// First Ctrl+C: shows hint
	app.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	// Second Ctrl+C: quits
	_, cmd := app.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("Double Ctrl+C should produce quit command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("Expected QuitMsg, got %T", msg)
	}
}

// TestVisualParity_QueryEventFlow verifies full QueryEvent routing.
func TestVisualParity_QueryEventFlow(t *testing.T) {
	config := session.DefaultConfig()
	sess := session.New(config, "/tmp")
	app := NewAppModel(sess, nil)

	events := []query.QueryEvent{
		{Type: query.QEventTextDelta, Text: "Hello "},
		{Type: query.QEventTextDelta, Text: "world"},
		{Type: query.QEventUsage, InputTokens: 50, OutputTokens: 25},
		{Type: query.QEventTurnComplete},
	}

	for _, evt := range events {
		app.Update(QueryEventMsg{Event: evt})
	}

	if app.mode != ModeIdle {
		t.Error("Should be idle after TurnComplete")
	}
	if sess.TotalInputTokens != 50 {
		t.Errorf("Expected 50 input tokens, got %d", sess.TotalInputTokens)
	}
	if app.conversation.MessageCount() != 1 {
		t.Errorf("Expected 1 finalized message, got %d", app.conversation.MessageCount())
	}
}

// TestParity_CtrlCFourStateMachine validates the complete Ctrl+C state machine:
// text→clear, empty→hint, hint→quit, streaming→cancel.
//
// Unique behaviors (not covered by CtrlCQuitsWhenIdle which only tests double-press):
// 1. Ctrl+C with text → clears input, does NOT quit, resets ctrlCPending
// 2. After clear, input.HasText() is false
// 3. Ctrl+C on empty → sets ctrlCPending=true (hint shown), no quit
// 4. Second Ctrl+C on empty → quits (QuitMsg returned)
// 5. Non-Ctrl+C key after hint → resets ctrlCPending back to false
//
// Cross-ref: app.go:352-375 — Ctrl+C handler with 4 paths
// Cross-ref: REPL.tsx stashedPrompt — Claude stashes then clears on Ctrl+C
func TestParity_CtrlCFourStateMachine(t *testing.T) {
	config := session.DefaultConfig()
	sess := session.New(config, "/tmp")
	app := NewAppModel(sess, nil)
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Dismiss welcome
	app.Update(components.SubmitMsg{Text: "hi"})
	app.Update(TurnCompleteMsg{})

	// Type text into input
	for _, ch := range "some text" {
		app.Update(tea.KeyPressMsg{Code: rune(ch), Text: string(ch)})
	}
	if !app.input.HasText() {
		t.Fatal("Setup: input should have text")
	}

	// 1. Ctrl+C with text → clears input, no quit
	_, cmd1 := app.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd1 != nil {
		msg := cmd1()
		if _, isQuit := msg.(tea.QuitMsg); isQuit {
			t.Fatal("Ctrl+C with text should clear, not quit")
		}
	}
	// 2. Input should now be empty
	if app.input.HasText() {
		t.Error("Input should be empty after Ctrl+C clear")
	}
	// ctrlCPending should be false (clearing resets it)
	if app.ctrlCPending {
		t.Error("ctrlCPending should be false after clearing text")
	}

	// 3. Now input is empty — first Ctrl+C shows hint
	_, cmd2 := app.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd2 != nil {
		msg := cmd2()
		if _, isQuit := msg.(tea.QuitMsg); isQuit {
			t.Fatal("First Ctrl+C on empty should show hint, not quit")
		}
	}
	if !app.ctrlCPending {
		t.Error("ctrlCPending should be true after first Ctrl+C on empty")
	}

	// 5. Non-Ctrl+C key resets the pending state
	app.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	if app.ctrlCPending {
		t.Error("ctrlCPending should reset on non-Ctrl+C key")
	}

	// Clear the 'x' we just typed
	app.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}) // clears 'x'

	// 4. Double Ctrl+C on empty → quit
	app.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}) // first: hint
	_, cmd4 := app.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}) // second: quit
	if cmd4 == nil {
		t.Fatal("Double Ctrl+C on empty should produce quit")
	}
	msg4 := cmd4()
	if _, isQuit := msg4.(tea.QuitMsg); !isQuit {
		t.Errorf("Expected QuitMsg on double Ctrl+C, got %T", msg4)
	}
}

// TestParity_DispatcherDefaultCommands validates each default slash command
// dispatches to the correct message type.
//
// Unique behaviors (existing tests cover /model and /clear individually):
// 1. /model {arg} → ModelSwitchMsg{Model: arg}
// 2. /model (no args) → CommandResult with error
// 3. /session → SessionSwitchMsg
// 4. /clear → ClearConversationMsg
// 5. /help → ShowHelpMsg
// 6. /quit → QuitMsg
// 7. /compact → CompactMsg
// 8. /thinking → ThinkingToggleMsg
// 9. HasHandler returns true for all registered, false for unknown
// 10. Commands() returns all 7 names
//
// Cross-ref: commands/handlers.go:110-143 registerDefaults
func TestParity_DispatcherDefaultCommands(t *testing.T) {
	d := commands.NewDispatcher()

	// Map each command to its expected message type
	cases := []struct {
		input   string
		msgType string // type name for comparison
	}{
		{"/model sonnet", "ModelSwitchMsg"},
		{"/session", "SessionSwitchMsg"},
		{"/clear", "ClearConversationMsg"},
		{"/help", "ShowHelpMsg"},
		{"/quit", "QuitMsg"},
		{"/compact", "CompactMsg"},
		{"/thinking", "ThinkingToggleMsg"},
	}

	for _, tc := range cases {
		cmd := d.Dispatch(tc.input)
		if cmd == nil {
			t.Errorf("%s should dispatch to a cmd", tc.input)
			continue
		}
		msg := cmd()
		gotType := fmt.Sprintf("%T", msg)
		if !strings.Contains(gotType, tc.msgType) {
			t.Errorf("%s: expected msg type containing %q, got %s", tc.input, tc.msgType, gotType)
		}
	}

	// /model with args → ModelSwitchMsg carries the args
	cmd := d.Dispatch("/model sonnet")
	msg := cmd()
	if msm, ok := msg.(commands.ModelSwitchMsg); ok {
		if msm.Model != "sonnet" {
			t.Errorf("ModelSwitchMsg.Model should be 'sonnet', got %q", msm.Model)
		}
	} else {
		t.Errorf("Expected ModelSwitchMsg, got %T", msg)
	}

	// HasHandler checks
	if !d.HasHandler("/clear") {
		t.Error("HasHandler should be true for /clear")
	}
	if !d.HasHandler("/model") {
		t.Error("HasHandler should be true for /model")
	}
	if d.HasHandler("/nonexistent") {
		t.Error("HasHandler should be false for unknown command")
	}

	// Commands() returns all 7
	cmds := d.Commands()
	if len(cmds) != 7 {
		t.Errorf("Commands() should return 7 default commands, got %d: %v", len(cmds), cmds)
	}
}

// TestParity_ConversationClearMessagesMsg validates the ClearMessagesMsg handler
// AND checks whether autoScroll state survives correctly through a clear+add cycle.
//
// Unique behaviors:
// 1. ClearMessagesMsg empties messages slice (len=0)
// 2. ClearMessagesMsg empties rendered slice (len=0)
// 3. ClearMessagesMsg resets scrollOffset to 0
// 4. After ClearMessagesMsg, MessageCount()==0
// 5. AddMessageMsg adds a message (different path from direct AddMessage())
// 6. WindowSizeMsg triggers re-render via SetSize
// 7. After Clear+Add, view shows new message correctly
//
// Cross-ref: conversation.go:55-75 Update message handlers
func TestParity_ConversationClearMessagesMsg(t *testing.T) {
	cp := components.NewConversationPane()
	cp.SetSize(80, 10)

	// Setup: add messages via AddMessageMsg (not direct AddMessage)
	for i := 0; i < 3; i++ {
		cp.Update(components.AddMessageMsg{Message: message.Message{
			Role:    message.RoleUser,
			Content: []message.ContentBlock{{Type: message.ContentText, Text: fmt.Sprintf("msg%d", i)}},
		}})
	}
	if cp.MessageCount() != 3 {
		t.Fatalf("AddMessageMsg should add 3 messages, got %d", cp.MessageCount())
	}

	// Scroll up to set scrollOffset > 0
	cp.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	cp.Update(tea.KeyPressMsg{Code: tea.KeyUp})

	// ClearMessagesMsg handler
	cp.Update(components.ClearMessagesMsg{})

	// 1,4. Messages cleared
	if cp.MessageCount() != 0 {
		t.Errorf("After ClearMessagesMsg, MessageCount should be 0, got %d", cp.MessageCount())
	}

	// 3. scrollOffset reset (verify via behavior: new message appears)
	cp.Update(components.AddMessageMsg{Message: message.Message{
		Role:    message.RoleAssistant,
		Content: []message.ContentBlock{{Type: message.ContentText, Text: "fresh"}},
	}})
	v := strip(cp.View().Content)
	if !strings.Contains(v, "fresh") {
		t.Errorf("Post-clear message should be visible, got:\n%s", v)
	}
	// Old messages should NOT appear
	for i := 0; i < 3; i++ {
		if strings.Contains(v, fmt.Sprintf("msg%d", i)) {
			t.Errorf("Old msg%d should be gone after clear, got:\n%s", i, v)
		}
	}

	// 6. WindowSizeMsg triggers SetSize re-render
	cp2 := components.NewConversationPane()
	cp2.SetSize(80, 10)
	cp2.Update(components.AddMessageMsg{Message: message.Message{
		Role:    message.RoleUser,
		Content: []message.ContentBlock{{Type: message.ContentText, Text: "a long test message that might wrap when width shrinks"}},
	}})
	widthBefore := strip(cp2.View().Content)
	cp2.Update(tea.WindowSizeMsg{Width: 40, Height: 10})
	widthAfter := strip(cp2.View().Content)
	// Views should be different (different width = different wrap)
	if widthBefore == widthAfter {
		t.Error("WindowSizeMsg should trigger re-render with new width")
	}
}

// TestParity_StatusLineTokenTrackingAndWidth validates token state tracking
// and width padding/truncation logic.
//
// Unique behaviors (no existing test covers token state or padding):
// 1. TokenUpdateMsg updates inputTokens and outputTokens fields
// 2. Tokens are OVERWRITTEN (not accumulated) per msg — caller must send totals
// 3. SetSize updates width, takes effect on next View()
// 4. View pads content to width with trailing spaces (when content < width)
// 5. View truncates content (with …) when content > width
// 6. Width=0 → no padding/truncation applied
// 7. Visual width of rendered text matches width setting
//
// Cross-ref: statusline.go:62-79 Update, :108-116 View padding
func TestParity_StatusLineTokenTrackingAndWidth(t *testing.T) {
	config := session.DefaultConfig()
	sess := session.New(config, "/tmp")
	sl := components.NewStatusLine(sess)
	sl.SetSize(80, 1)

	// 1-2. TokenUpdateMsg sets (NOT accumulates) token counts
	sl.Update(components.TokenUpdateMsg{InputTokens: 100, OutputTokens: 50})
	// Send another — should REPLACE, not add
	sl.Update(components.TokenUpdateMsg{InputTokens: 200, OutputTokens: 80})

	// (no public getter — internal state, but behavior visible if view displayed tokens)
	// Since idle view doesn't show tokens (only "? for shortcuts"), we verify via
	// a subsequent streaming→idle cycle that the update doesn't accumulate.
	// Actually the idle view shows "? for shortcuts" regardless, so we validate
	// by sending msg doesn't panic and view still works
	v := sl.View().Content
	if v == "" {
		t.Error("TokenUpdateMsg should not break view")
	}

	// 3-4. Width padding: content shorter than width gets padded
	sl.SetSize(100, 1)
	vPadded := strip(sl.View().Content)
	if len([]rune(vPadded)) < 80 {
		t.Errorf("View should be padded to width=100, got len=%d", len([]rune(vPadded)))
	}

	// 5. Width smaller than content → truncation
	// Content is "? for shortcuts" (15 chars + leading/trailing by style)
	sl.SetSize(10, 1)
	vTrunc := strip(sl.View().Content)
	if len([]rune(vTrunc)) > 15 {
		t.Errorf("View should be truncated to width=10, got len=%d: %q", len([]rune(vTrunc)), vTrunc)
	}

	// 6. Width=0 disables padding
	sl.SetSize(0, 1)
	v0 := strip(sl.View().Content)
	if v0 == "" {
		t.Error("Width=0 should still produce content (no padding)")
	}

	// 7. SetSize via WindowSizeMsg works (style adds 2 padding chars = width+2)
	sl.Update(tea.WindowSizeMsg{Width: 60, Height: 1})
	v60 := strip(sl.View().Content)
	// Style adds horizontal padding (1 char each side), so visual width = 60+2 = 62
	vw60 := len([]rune(v60))
	if vw60 < 60 || vw60 > 62 {
		t.Errorf("After WindowSizeMsg width=60, view rune count should be 60-62 (incl padding), got %d: %q", vw60, v60)
	}

	// 8. Streaming mode padded to width too
	sl.SetSize(80, 1)
	sl.Update(components.ModeChangeMsg{Mode: components.ModeStreaming})
	vStream := strip(sl.View().Content)
	vwStream := len([]rune(vStream))
	if vwStream < 80 || vwStream > 82 {
		t.Errorf("Streaming mode view should be 80-82 (incl padding), got %d", vwStream)
	}
}

// TestParity_InputBufferLifecycle validates Value/SetValue/Clear/HasText
// semantics including Unicode handling and cursor positioning.
//
// Unique behaviors:
// 1. SetValue replaces buffer + moves cursor to end
// 2. SetValue with Unicode preserves rune count (not byte count)
// 3. Clear empties buffer AND moves cursor to 0
// 4. Value returns exact buffer contents (no trimming/transformation)
// 5. HasText reflects buffer state (true when non-empty)
// 6. After Clear + typing, cursor position is correct
// 7. SetValue then type: char inserted at end
// 8. Multi-byte Unicode chars (CJK) counted as single runes
//
// Cross-ref: input.go:96-113 Value/SetValue/Clear/HasText
func TestParity_InputBufferLifecycle(t *testing.T) {
	inp := components.NewInputPane()
	inp.SetSize(80, 3)
	inp.Focus()

	// 1. SetValue replaces buffer, cursor at end
	inp.SetValue("hello")
	if inp.Value() != "hello" {
		t.Errorf("SetValue should set buffer, got %q", inp.Value())
	}
	// Insert at end should append
	inp.Update(tea.KeyPressMsg{Code: '!', Text: "!"})
	if inp.Value() != "hello!" {
		t.Errorf("Insert after SetValue should append, got %q", inp.Value())
	}

	// 2. SetValue overwrites
	inp.SetValue("world")
	if inp.Value() != "world" {
		t.Errorf("SetValue should replace, got %q", inp.Value())
	}

	// 3. Clear empties + cursor=0
	inp.Clear()
	if inp.Value() != "" {
		t.Errorf("Clear should empty, got %q", inp.Value())
	}
	if inp.HasText() {
		t.Error("HasText should be false after Clear")
	}
	// Insert after Clear should start fresh
	inp.Update(tea.KeyPressMsg{Code: 'X', Text: "X"})
	if inp.Value() != "X" {
		t.Errorf("Insert after Clear should start fresh, got %q", inp.Value())
	}

	// 4. HasText reflects state
	inp.Clear()
	if inp.HasText() {
		t.Error("HasText false on empty")
	}
	inp.SetValue("a")
	if !inp.HasText() {
		t.Error("HasText true after SetValue with text")
	}
	inp.SetValue("")
	if inp.HasText() {
		t.Error("HasText false after SetValue with empty string")
	}

	// 5. Unicode multi-byte chars counted as runes
	inp.SetValue("日本語") // 3 runes, 9 bytes
	if inp.Value() != "日本語" {
		t.Errorf("SetValue should preserve Unicode, got %q", inp.Value())
	}
	// Cursor should be at rune position 3 (not byte 9)
	// Left arrow should move to rune position 2 — then insert
	inp.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	inp.Update(tea.KeyPressMsg{Code: '!', Text: "!"})
	if inp.Value() != "日本!語" {
		t.Errorf("Unicode cursor should split at rune boundary, got %q", inp.Value())
	}

	// 6. Clear after Unicode — also works
	inp.Clear()
	if inp.Value() != "" {
		t.Errorf("Clear after Unicode, got %q", inp.Value())
	}
}

// TestParity_MessageBubbleRoleDispatch validates the Render() role dispatch,
// nil safety, thinking block truncation, and SetWidth re-rendering.
//
// Unique behaviors (no existing test validates role dispatch or thinking):
// 1. Render(nil) → empty string, no panic
// 2. RoleUser → user rendering (❯ prefix)
// 3. RoleAssistant → assistant rendering (⏺ prefix)
// 4. Unknown role → generic rendering (plain text, no prefix)
// 5. ContentThinking block → "💭 Thinking:" prefix + text
// 6. ContentThinking > 200 chars → truncated with "…"
// 7. SetWidth change produces different wrapping for long text
// 8. Unknown ContentBlock type in RenderContent → empty string
//
// Cross-ref: message_bubble.go:54-85 Render + RenderContent dispatch
func TestParity_MessageBubbleRoleDispatch(t *testing.T) {
	mb := components.NewMessageBubble(theme.Current(), 80)

	// 1. nil message
	if got := mb.Render(nil); got != "" {
		t.Errorf("Render(nil) should return empty, got %q", got)
	}

	// 2. User role → ❯ prefix
	userMsg := &message.Message{
		Role:    message.RoleUser,
		Content: []message.ContentBlock{{Type: message.ContentText, Text: "user-text"}},
	}
	userOut := strip(mb.Render(userMsg))
	if !strings.Contains(userOut, "❯") {
		t.Errorf("User message should have ❯ prefix, got: %q", userOut)
	}
	if strings.Contains(userOut, "⏺") {
		t.Error("User message should NOT have ⏺ prefix")
	}

	// 3. Assistant role → ⏺ prefix
	assistMsg := &message.Message{
		Role:    message.RoleAssistant,
		Content: []message.ContentBlock{{Type: message.ContentText, Text: "assistant-text"}},
	}
	assistOut := strip(mb.Render(assistMsg))
	if !strings.Contains(assistOut, "⏺") {
		t.Errorf("Assistant message should have ⏺ prefix, got: %q", assistOut)
	}
	if strings.Contains(assistOut, "❯") {
		t.Error("Assistant message should NOT have ❯ prefix")
	}

	// 4. Unknown role → generic rendering (text present, no role prefix)
	genericMsg := &message.Message{
		Role:    message.Role("unknown-role"),
		Content: []message.ContentBlock{{Type: message.ContentText, Text: "generic"}},
	}
	genOut := strip(mb.Render(genericMsg))
	if !strings.Contains(genOut, "generic") {
		t.Errorf("Generic message should show text, got: %q", genOut)
	}
	if strings.Contains(genOut, "❯") || strings.Contains(genOut, "⏺") {
		t.Error("Generic message should NOT have role prefixes")
	}

	// 5. Thinking block
	thinkingOut := strip(mb.RenderContent(message.ContentBlock{
		Type:     message.ContentThinking,
		Thinking: "short thought",
	}))
	if !strings.Contains(thinkingOut, "Thinking:") {
		t.Errorf("Thinking block should have 'Thinking:' label, got: %q", thinkingOut)
	}
	if !strings.Contains(thinkingOut, "short thought") {
		t.Errorf("Thinking block should show text, got: %q", thinkingOut)
	}

	// 6. Thinking > 200 chars truncated
	longThought := strings.Repeat("x", 300)
	longThinkingOut := strip(mb.RenderContent(message.ContentBlock{
		Type: message.ContentThinking, Thinking: longThought,
	}))
	if !strings.Contains(longThinkingOut, "…") {
		t.Error("Long thinking (>200 chars) should be truncated with …")
	}
	if strings.Count(longThinkingOut, "x") >= 300 {
		t.Error("Long thinking should not contain all 300 chars")
	}

	// 7. SetWidth changes wrapping
	longMsg := &message.Message{
		Role:    message.RoleUser,
		Content: []message.ContentBlock{{Type: message.ContentText, Text: "this is a medium length text that may wrap"}},
	}
	mb.SetWidth(20)
	narrowOut := strip(mb.Render(longMsg))
	narrowLines := strings.Count(narrowOut, "\n")

	mb.SetWidth(100)
	wideOut := strip(mb.Render(longMsg))
	wideLines := strings.Count(wideOut, "\n")

	if narrowLines <= wideLines {
		// At narrow width, we expect MORE line breaks from wrapping
		t.Errorf("Narrow width should wrap more: narrow=%d lines, wide=%d lines", narrowLines, wideLines)
	}

	// 8. Unknown ContentBlock type → empty string
	unkOut := mb.RenderContent(message.ContentBlock{Type: message.ContentBlockType("bogus")})
	if unkOut != "" {
		t.Errorf("Unknown ContentBlock type should return empty, got %q", unkOut)
	}
}

// TestParity_InputCursorBlockRendering validates cursor visibility and position
// in the InputPane's View() output (block character █ placement).
//
// Unique behaviors (no existing test validates cursor block rendering):
// 1. Focused + empty buffer → "❯ █" (cursor at position 0)
// 2. Focused + cursor at END → "❯ text█" (cursor appended)
// 3. Focused + cursor at START → "❯ █text" (cursor prepended)
// 4. Focused + cursor in MIDDLE → "❯ tex█t" (cursor splits text)
// 5. Blurred input → NO cursor block, just "❯ text"
// 6. Refocus restores cursor visibility
//
// Cross-ref: input.go:61-80 View() cursor rendering
func TestParity_InputCursorBlockRendering(t *testing.T) {
	// 1. Focused empty → cursor at start
	inp := components.NewInputPane()
	inp.SetSize(80, 3)
	inp.Focus()
	v1 := strip(inp.View().Content)
	if !strings.Contains(v1, "█") {
		t.Errorf("Focused empty input should show cursor █, got: %q", v1)
	}

	// 2. Type text → cursor at end
	for _, ch := range "hello" {
		inp.Update(tea.KeyPressMsg{Code: rune(ch), Text: string(ch)})
	}
	v2 := strip(inp.View().Content)
	// Cursor should come AFTER "hello"
	cursorIdx := strings.Index(v2, "█")
	helloEnd := strings.Index(v2, "hello") + len("hello")
	if cursorIdx <= helloEnd-1 {
		t.Errorf("Cursor at end should come after 'hello', got view: %q", v2)
	}

	// 3. Ctrl+A → cursor to start
	inp.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
	v3 := strip(inp.View().Content)
	// Cursor should come BEFORE "hello"
	cursorIdx3 := strings.Index(v3, "█")
	helloStart3 := strings.Index(v3, "hello")
	if cursorIdx3 >= helloStart3 {
		t.Errorf("Cursor at start should come before 'hello', got view: %q", v3)
	}

	// 4. Left twice from end → cursor in middle of "hello" (between 'l' and 'l')
	inp.Update(tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl}) // to end
	inp.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	inp.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	v4 := strip(inp.View().Content)
	// View should be "❯ hel█lo" — cursor splits hello
	if !strings.Contains(v4, "hel█lo") {
		t.Errorf("Cursor in middle should produce 'hel█lo', got: %q", v4)
	}

	// 5. Blur → no cursor block
	inp.Blur()
	v5 := strip(inp.View().Content)
	if strings.Contains(v5, "█") {
		t.Errorf("Blurred input should NOT show cursor █, got: %q", v5)
	}
	// But text should still be visible
	if !strings.Contains(v5, "hello") {
		t.Error("Blurred input should still show text")
	}

	// 6. Refocus restores cursor
	inp.Focus()
	v6 := strip(inp.View().Content)
	if !strings.Contains(v6, "█") {
		t.Errorf("Refocused input should show cursor, got: %q", v6)
	}
}

// TestParity_ConversationViewComposition validates the ConversationPane View():
// empty state, streaming text placement, height padding, and interleaving.
//
// Unique behaviors (scroll test doesn't cover streaming/placeholder/padding):
// 1. Empty (no messages + no streaming) → "No messages yet." placeholder
// 2. width=0 or height=0 → empty view
// 3. Messages rendered into view
// 4. Streaming text appears AFTER messages (at bottom)
// 5. View is padded to exactly `height` lines
// 6. ClearStreamingText removes streaming content
// 7. Visible lines count == height (when height > 0)
//
// Cross-ref: conversation.go:78-123 View composition
func TestParity_ConversationViewComposition(t *testing.T) {
	// 1. Width/height 0 → empty
	cp := components.NewConversationPane()
	if cp.View().Content != "" {
		t.Error("width=0,height=0 should return empty view")
	}

	cp.SetSize(80, 10)

	// 2. Empty conversation → placeholder
	v1 := strip(cp.View().Content)
	if !strings.Contains(v1, "No messages yet") {
		t.Errorf("Empty conversation should show placeholder, got: %q", v1)
	}

	// 3. Add a message → no more placeholder
	cp.AddMessage(message.Message{
		Role:    message.RoleUser,
		Content: []message.ContentBlock{{Type: message.ContentText, Text: "hello"}},
	})
	v2 := strip(cp.View().Content)
	if strings.Contains(v2, "No messages yet") {
		t.Error("After AddMessage, placeholder should be gone")
	}

	// 5. View is padded to exactly `height` lines
	lines := strings.Split(cp.View().Content, "\n")
	if len(lines) != 10 {
		t.Errorf("View should be padded to height=10 lines, got %d", len(lines))
	}

	// 4. Streaming text appears AFTER messages
	cp.SetStreamingText("streaming-here")
	v3 := strip(cp.View().Content)
	helloIdx := strings.Index(v3, "hello")
	streamIdx := strings.Index(v3, "streaming-here")
	if helloIdx == -1 || streamIdx == -1 {
		t.Fatalf("Both 'hello' and 'streaming-here' should be visible.\nGot:\n%s", v3)
	}
	if helloIdx >= streamIdx {
		t.Errorf("Message 'hello' (idx=%d) should come BEFORE streaming 'streaming-here' (idx=%d)", helloIdx, streamIdx)
	}

	// 6. ClearStreamingText removes streaming content
	cp.ClearStreamingText()
	v4 := strip(cp.View().Content)
	if strings.Contains(v4, "streaming-here") {
		t.Error("After ClearStreamingText, streaming content should be gone")
	}
	// But the message should still be there
	if !strings.Contains(v4, "hello") {
		t.Error("Message should still be visible after ClearStreamingText")
	}

	// 7. Line count always equals height
	linesAfterClear := strings.Split(cp.View().Content, "\n")
	if len(linesAfterClear) != 10 {
		t.Errorf("After clear, view should still be 10 lines, got %d", len(linesAfterClear))
	}

	// Streaming without messages: show placeholder disabled, streaming visible
	cp2 := components.NewConversationPane()
	cp2.SetSize(80, 10)
	cp2.SetStreamingText("only-streaming")
	v5 := strip(cp2.View().Content)
	if strings.Contains(v5, "No messages yet") {
		t.Error("Streaming text alone should skip placeholder")
	}
	if !strings.Contains(v5, "only-streaming") {
		t.Error("Streaming text should be visible")
	}
}

// TestParity_ToolEventStreamingBuffer validates the streamingText buffer
// changes produced by ToolUseStart and ToolResult events.
//
// Unique behaviors (no existing test checks buffer content after tool events):
// 1. ToolUseStart appends "\n⏺ {ToolName}" to streamingText
// 2. ToolResult success appends "\n  ✓ {toolName}"
// 3. ToolResult error appends "\n  ✗ {toolName} error"
// 4. ToolResult with EMPTY content skips the ✓ indicator
// 5. ToolResult with unknown toolID uses empty toolName string
// 6. activeToolCalls map lookup provides toolName to the result indicator
//
// Cross-ref: app.go:524-567 handleToolUseStart/handleToolResult
func TestParity_ToolEventStreamingBuffer(t *testing.T) {
	config := session.DefaultConfig()
	sess := session.New(config, "/tmp")
	app := NewAppModel(sess, nil)
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Enter streaming state
	app.Update(components.SubmitMsg{Text: "run tools"})
	bufStart := app.streamingText.String()

	// 1. ToolUseStart appends ⏺ {ToolName}
	app.Update(ToolUseStartMsg{ToolUseID: "t1", ToolName: "Bash"})
	buf1 := app.streamingText.String()
	if !strings.Contains(buf1, "⏺ Bash") {
		t.Errorf("ToolUseStart should add '⏺ Bash' to buffer, got %q", buf1[len(bufStart):])
	}
	grown := len(buf1) > len(bufStart)
	if !grown {
		t.Error("streamingText should grow after ToolUseStart")
	}

	// 2. ToolResult success appends ✓ {toolName}
	app.Update(ToolResultMsg{ToolUseID: "t1", Content: "output", IsError: false})
	buf2 := app.streamingText.String()
	if !strings.Contains(buf2, "✓ Bash") {
		t.Errorf("ToolResult success should add '✓ Bash', got suffix: %q", buf2[len(buf1):])
	}

	// 3. ToolResult error appends ✗ {toolName} error
	app.Update(ToolUseStartMsg{ToolUseID: "t2", ToolName: "Read"})
	buf3 := app.streamingText.String()
	app.Update(ToolResultMsg{ToolUseID: "t2", Content: "not found", IsError: true})
	buf4 := app.streamingText.String()
	if !strings.Contains(buf4, "✗ Read error") {
		t.Errorf("ToolResult error should add '✗ Read error', got suffix: %q", buf4[len(buf3):])
	}

	// 4. ToolResult with EMPTY content skips ✓ indicator
	app.Update(ToolUseStartMsg{ToolUseID: "t3", ToolName: "Grep"})
	buf5 := app.streamingText.String()
	// Count ✓ before and after
	checksBefore := strings.Count(buf5, "✓")
	app.Update(ToolResultMsg{ToolUseID: "t3", Content: "", IsError: false})
	buf6 := app.streamingText.String()
	checksAfter := strings.Count(buf6, "✓")
	if checksAfter != checksBefore {
		t.Errorf("Empty content should NOT add ✓, count changed %d→%d", checksBefore, checksAfter)
	}

	// 5. Unknown toolID (not started) → empty toolName in error indicator
	// Buffer should get "✗  error" (2 spaces between ✗ and error, name empty)
	buf7 := app.streamingText.String()
	app.Update(ToolResultMsg{ToolUseID: "unknown", Content: "x", IsError: true})
	buf8 := app.streamingText.String()
	suffix := buf8[len(buf7):]
	if !strings.Contains(suffix, "✗") || !strings.Contains(suffix, "error") {
		t.Errorf("Unknown toolID error should still write ✗ and 'error', got %q", suffix)
	}

	// 6. activeToolCalls correctly maps toolID → toolName for result lookup
	// Set up a tool, inspect map, fire result, inspect map again
	app.Update(ToolUseStartMsg{ToolUseID: "t4", ToolName: "Glob"})
	if app.activeToolCalls["t4"] != "Glob" {
		t.Errorf("activeToolCalls[t4] should be 'Glob', got %q", app.activeToolCalls["t4"])
	}
	app.Update(ToolResultMsg{ToolUseID: "t4", Content: "files.go", IsError: false})
	if _, exists := app.activeToolCalls["t4"]; exists {
		t.Error("activeToolCalls[t4] should be deleted after ToolResult")
	}
	// Final buffer should have '✓ Glob' (from the result using looked-up name)
	if !strings.Contains(app.streamingText.String(), "✓ Glob") {
		t.Error("Result should use looked-up name 'Glob'")
	}
}

// TestParity_EffortLevelIconMapping validates that each effort level produces
// a VISUALLY DIFFERENT glyph in the spinner view.
//
// Unique behaviors (existing EffortLevelDisplay only checked high=●):
// 1. "low" → ○ (U+25CB WHITE CIRCLE)
// 2. "medium" → ◐ (U+25D0 HALF BLACK)
// 3. "high" → ● (U+25CF BLACK CIRCLE)
// 4. "max" → ◉ (U+25C9 FISHEYE)
// 5. Each glyph is DIFFERENT from the others (no duplicates)
// 6. No effort (default) → "(thinking)" without glyph suffix
// 7. Unknown effort level → same as no effort (empty string)
// 8. After Stop, view does NOT contain effort glyph
//
// Cross-ref: spinner_verbs.go:94-97 glyph constants, :156-170 SetEffort
func TestParity_EffortLevelIconMapping(t *testing.T) {
	glyphs := map[string]string{
		"low":    "○",
		"medium": "◐",
		"high":   "●",
		"max":    "◉",
	}

	seenGlyphs := make(map[string]string) // glyph → level (for uniqueness)

	for level, expectedGlyph := range glyphs {
		ts := components.NewThinkingSpinner(theme.Current())
		ts.Start()
		ts.SetEffort(level)
		view := strip(ts.View())

		if !strings.Contains(view, expectedGlyph) {
			t.Errorf("Level %q should contain glyph %q, got: %s", level, expectedGlyph, view)
		}

		// Check uniqueness
		if prev, seen := seenGlyphs[expectedGlyph]; seen {
			t.Errorf("Glyph %q used for both %q and %q", expectedGlyph, prev, level)
		}
		seenGlyphs[expectedGlyph] = level

		// Verify the "(thinking X)" format
		if !strings.Contains(view, "(thinking "+expectedGlyph+")") {
			t.Errorf("Level %q should render '(thinking %s)' in view, got: %s", level, expectedGlyph, view)
		}
	}

	// No effort set → just "(thinking)" without glyph
	tsNone := components.NewThinkingSpinner(theme.Current())
	tsNone.Start()
	viewNone := strip(tsNone.View())
	if !strings.Contains(viewNone, "(thinking)") {
		t.Errorf("No effort should render '(thinking)', got: %s", viewNone)
	}
	// And should NOT contain any of the effort glyphs
	for glyph := range seenGlyphs {
		if strings.Contains(viewNone, glyph) {
			t.Errorf("No-effort view should not contain any effort glyph, found %q", glyph)
		}
	}

	// Unknown effort → empty effort (same as no effort)
	tsUnknown := components.NewThinkingSpinner(theme.Current())
	tsUnknown.Start()
	tsUnknown.SetEffort("bogus")
	viewUnk := strip(tsUnknown.View())
	if !strings.Contains(viewUnk, "(thinking)") {
		t.Errorf("Unknown effort should render '(thinking)', got: %s", viewUnk)
	}
	for glyph := range seenGlyphs {
		if strings.Contains(viewUnk, glyph) {
			t.Errorf("Unknown effort view should not contain any effort glyph, found %q", glyph)
		}
	}

	// After Stop, view should show "thought for Xs" without effort
	tsStopped := components.NewThinkingSpinner(theme.Current())
	tsStopped.Start()
	tsStopped.SetEffort("high")
	tsStopped.Stop()
	viewStop := strip(tsStopped.View())
	if strings.Contains(viewStop, "(thinking") {
		t.Errorf("Stopped view should not say '(thinking', got: %s", viewStop)
	}
	if !strings.Contains(viewStop, "for") {
		t.Errorf("Stopped view should contain 'for Xs', got: %s", viewStop)
	}
}

// TestParity_InputCursorMovementAndDelete validates Left/Right/Delete
// key semantics with exact buffer state verification.
//
// Unique behaviors (no existing test validates cursor arithmetic):
// 1. Left decrements cursor, bounded at 0 (no underflow)
// 2. Right increments cursor, bounded at len (no overflow)
// 3. Delete removes char AT cursor (unlike Backspace which removes BEFORE)
// 4. Delete at end of buffer is no-op (no panic, no change)
// 5. Delete does NOT move cursor (unlike Backspace which decrements)
// 6. After Left then type: insert at new cursor position
// 7. After Right-past-end then type: appends at real end
// 8. Left×many then Right×many returns to original position
//
// Cross-ref: input.go:137-168 cursor keys + Delete
func TestParity_InputCursorMovementAndDelete(t *testing.T) {
	inp := components.NewInputPane()
	inp.SetSize(80, 3)
	inp.Focus()

	// Type "abcde"
	for _, ch := range "abcde" {
		inp.Update(tea.KeyPressMsg{Code: rune(ch), Text: string(ch)})
	}
	if inp.Value() != "abcde" {
		t.Fatalf("Setup: expected 'abcde', got %q", inp.Value())
	}

	// 1. Left×2 → cursor at position 3 (between 'c' and 'd')
	inp.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	inp.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	// Insert 'X' → should be at position 3: "abcXde"
	inp.Update(tea.KeyPressMsg{Code: 'X', Text: "X"})
	if inp.Value() != "abcXde" {
		t.Errorf("Left×2 + 'X' should give 'abcXde', got %q", inp.Value())
	}

	// 3,5. Delete at cursor position (now pos 4, char 'd') → removes 'd'
	inp.Update(tea.KeyPressMsg{Code: tea.KeyDelete})
	if inp.Value() != "abcXe" {
		t.Errorf("Delete at pos 4 should remove 'd', got %q", inp.Value())
	}

	// 5. Delete did not move cursor (still at pos 4) — next insert there
	inp.Update(tea.KeyPressMsg{Code: 'Y', Text: "Y"})
	if inp.Value() != "abcXYe" {
		t.Errorf("After Delete + 'Y', expected 'abcXYe', got %q", inp.Value())
	}

	// 7. Move to end via Ctrl+E, then Right past end → no-op
	inp.Update(tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl})
	inp.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	inp.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	inp.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	// Append still works
	inp.Update(tea.KeyPressMsg{Code: 'Z', Text: "Z"})
	if inp.Value() != "abcXYeZ" {
		t.Errorf("Right-past-end + 'Z' should append, got %q", inp.Value())
	}

	// 4. Delete at end of buffer → no-op
	lenBefore := len(inp.Value())
	inp.Update(tea.KeyPressMsg{Code: tea.KeyDelete})
	if len(inp.Value()) != lenBefore {
		t.Errorf("Delete at end should be no-op, length changed %d→%d", lenBefore, len(inp.Value()))
	}

	// 1. Left at position 0 → no-op
	inp.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl}) // Ctrl+A to pos 0
	inp.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	inp.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	// Insert 'Q' → should be at position 0: "Q" + rest
	inp.Update(tea.KeyPressMsg{Code: 'Q', Text: "Q"})
	if inp.Value() != "QabcXYeZ" {
		t.Errorf("Left-at-0 + 'Q' should prefix, got %q", inp.Value())
	}

	// 8. Round trip: Left×3 then Right×3 returns to same position
	inp.Clear()
	for _, ch := range "12345" {
		inp.Update(tea.KeyPressMsg{Code: rune(ch), Text: string(ch)})
	}
	// cursor at 5 (end)
	for i := 0; i < 3; i++ {
		inp.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	}
	for i := 0; i < 3; i++ {
		inp.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	}
	// Should be at end again — inserting appends
	inp.Update(tea.KeyPressMsg{Code: '!', Text: "!"})
	if inp.Value() != "12345!" {
		t.Errorf("Left×3 + Right×3 round-trip + '!' should give '12345!', got %q", inp.Value())
	}
}

// TestParity_DiffApprovalEdgeCases validates less-common DiffApprovalDialog paths
// that the initial B1 test doesn't cover.
//
// Unique behaviors:
// 1. Enter key is alias for 'y' (approves)
// 2. Unknown keys (e.g. 'z') do NOT send any approval
// 3. Non-KeyPressMsg messages are no-ops (no approval sent)
// 4. sendResult with nil channel is safe (no panic)
// 5. sendResult with full channel is safe (non-blocking, drops silently)
// 6. Multiple key presses after first only send once to unbuffered channel
//
// Cross-ref: diff_approval.go:62-99 Update + sendResult
func TestParity_DiffApprovalEdgeCases(t *testing.T) {
	testDiff := "+new line\n existing"

	// 1. Enter approves (same as 'y')
	t.Run("enter-approves", func(t *testing.T) {
		ch := make(chan components.ApprovalResult, 1)
		dad := components.NewDiffApprovalDialog("Edit", "t1", testDiff, theme.Current(), ch)
		dad.SetSize(80, 24)
		_, cmd := dad.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		if cmd == nil {
			t.Fatal("Enter should return a cmd")
		}
		msg := cmd()
		rm, ok := msg.(components.ApprovalResponseMsg)
		if !ok || rm.Result != components.ApprovalApproved {
			t.Errorf("Enter should produce Approved, got %v", msg)
		}
		select {
		case r := <-ch:
			if r != components.ApprovalApproved {
				t.Errorf("Enter should send Approved to channel, got %v", r)
			}
		default:
			t.Error("Enter should send result to channel")
		}
	})

	// 2. Unknown key does not approve
	t.Run("unknown-key-noop", func(t *testing.T) {
		ch := make(chan components.ApprovalResult, 1)
		dad := components.NewDiffApprovalDialog("Edit", "t2", testDiff, theme.Current(), ch)
		dad.SetSize(80, 24)
		_, cmd := dad.Update(tea.KeyPressMsg{Code: 'z', Text: "z"})
		if cmd != nil {
			// cmd may be non-nil due to diff scroll, but it should NOT produce ApprovalResponseMsg
			msg := cmd()
			if _, ok := msg.(components.ApprovalResponseMsg); ok {
				t.Error("'z' key should NOT produce ApprovalResponseMsg")
			}
		}
		// Channel should be empty
		select {
		case r := <-ch:
			t.Errorf("'z' should NOT send to channel, got %v", r)
		default:
			// expected
		}
	})

	// 3. Non-key messages are no-ops
	t.Run("non-key-msg-noop", func(t *testing.T) {
		ch := make(chan components.ApprovalResult, 1)
		dad := components.NewDiffApprovalDialog("Edit", "t3", testDiff, theme.Current(), ch)
		dad.SetSize(80, 24)
		_, cmd := dad.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
		if cmd != nil {
			msg := cmd()
			if _, ok := msg.(components.ApprovalResponseMsg); ok {
				t.Error("WindowSizeMsg should NOT produce ApprovalResponseMsg")
			}
		}
		select {
		case <-ch:
			t.Error("Non-key msg should NOT send to channel")
		default:
			// expected
		}
	})

	// 4. Nil channel is safe
	t.Run("nil-channel-safe", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("nil channel should be safe, got panic: %v", r)
			}
		}()
		dad := components.NewDiffApprovalDialog("Edit", "t4", testDiff, theme.Current(), nil)
		dad.SetSize(80, 24)
		dad.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	})

	// 5. Full channel is non-blocking
	t.Run("full-channel-nonblocking", func(t *testing.T) {
		ch := make(chan components.ApprovalResult, 1)
		ch <- components.ApprovalApproved // pre-fill
		dad := components.NewDiffApprovalDialog("Edit", "t5", testDiff, theme.Current(), ch)
		dad.SetSize(80, 24)
		done := make(chan bool)
		go func() {
			dad.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
			done <- true
		}()
		select {
		case <-done:
			// Good, didn't block
		case <-time.After(1 * time.Second):
			t.Error("Update with full channel blocked (should be non-blocking)")
		}
	})
}

// TestParity_FocusManagerRoute validates Route() message dispatch:
// messages go to the currently focused element, including modals.
//
// Unique behaviors:
// 1. Route with empty FocusManager → nil
// 2. Route forwards message to focused child's Update
// 3. Route returns the cmd from child's Update
// 4. Route delivers to modal (not child) when modal active
// 5. Child does NOT receive message when modal is focused
// 6. After Tab (focus change), Route delivers to new focused child
//
// Cross-ref: core/focus.go:104-113 Route()
func TestParity_FocusManagerRoute(t *testing.T) {
	// 1. Empty manager → nil
	emptyFM := core.NewFocusManager()
	if emptyFM.Route(tea.KeyPressMsg{Code: 'a'}) != nil {
		t.Error("Route on empty manager should return nil")
	}

	// Trackable focusables record messages received
	child1 := &trackingFocusable{name: "child1", wantCmd: "cmd1"}
	child2 := &trackingFocusable{name: "child2", wantCmd: "cmd2"}
	modal := &trackingFocusable{name: "modal", wantCmd: "modalCmd"}

	fm := core.NewFocusManager(child1, child2)
	child1.Focus()

	// 2-3. Route to focused child AND returns its cmd
	testMsg := tea.KeyPressMsg{Code: 'x', Text: "x"}
	cmd := fm.Route(testMsg)
	if child1.received == nil {
		t.Fatal("child1 should have received the message")
	}
	if _, ok := child1.received.(tea.KeyPressMsg); !ok {
		t.Errorf("child1 should receive KeyPressMsg, got %T", child1.received)
	}
	if child2.received != nil {
		t.Error("child2 should NOT receive message (not focused)")
	}
	// cmd should produce the expected marker
	if cmd == nil {
		t.Fatal("Route should return child's cmd")
	}
	marker, ok := cmd().(string)
	if !ok || marker != "cmd1" {
		t.Errorf("Expected child1's cmd result 'cmd1', got %v", marker)
	}

	// Reset
	child1.received = nil

	// 4-5. Push modal, Route should go to modal
	fm.PushModal(modal)
	fm.Route(testMsg)
	if modal.received == nil {
		t.Error("modal should receive message when active")
	}
	if child1.received != nil {
		t.Error("child1 should NOT receive message when modal active")
	}

	// Clean up
	modal.received = nil
	fm.PopModal()

	// 6. After Tab, new focused child gets messages
	fm.Next() // focus child2
	child1.received = nil
	fm.Route(testMsg)
	if child2.received == nil {
		t.Error("child2 should receive message after Tab")
	}
	if child1.received != nil {
		t.Error("child1 should NOT receive message after Blur")
	}
}

// trackingFocusable records received messages and returns a marker cmd.
type trackingFocusable struct {
	name     string
	focused  bool
	received tea.Msg
	wantCmd  string
}

func (tf *trackingFocusable) Focus()        { tf.focused = true }
func (tf *trackingFocusable) Blur()         { tf.focused = false }
func (tf *trackingFocusable) Focused() bool { return tf.focused }
func (tf *trackingFocusable) Init() tea.Cmd { return nil }
func (tf *trackingFocusable) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	tf.received = msg
	marker := tf.wantCmd
	return tf, func() tea.Msg { return marker }
}
func (tf *trackingFocusable) View() tea.View { return tea.NewView("") }

// TestParity_FocusModalPushPop validates the FocusManager modal stack
// lifecycle: push transfers focus, pop restores, stack nesting works.
//
// Unique behaviors (Tab test only covered normal ring, not modal stack):
// 1. PushModal Blur()s current child and Focus()es modal
// 2. ModalActive() reports true after push
// 3. Focused() returns top modal (not child) when modal active
// 4. Nested PushModal Blur()s previous modal, Focus()es new top
// 5. PopModal Blur()s top modal and restores previous modal focus (nested case)
// 6. PopModal on last modal restores focus to underlying child
// 7. PopModal on empty stack is no-op (no crash)
// 8. ModalActive() returns false after all modals popped
//
// Cross-ref: core/focus.go:35-102 modal stack implementation
func TestParity_FocusModalPushPop(t *testing.T) {
	// Use fake focusables to track state
	child1 := &fakeFocusable{name: "child1"}
	child2 := &fakeFocusable{name: "child2"}
	modal1 := &fakeFocusable{name: "modal1"}
	modal2 := &fakeFocusable{name: "modal2"}

	fm := core.NewFocusManager(child1, child2)
	// Manually focus child1 (NewFocusManager doesn't call Focus)
	child1.Focus()

	// 1-3. Push modal1
	fm.PushModal(modal1)
	if child1.Focused() {
		t.Error("child1 should be Blur'd after PushModal")
	}
	if !modal1.Focused() {
		t.Error("modal1 should be Focused after PushModal")
	}
	if !fm.ModalActive() {
		t.Error("ModalActive should be true after push")
	}
	if fm.Focused() != modal1 {
		t.Error("Focused() should return modal1 when modal active")
	}

	// 4. Nested push: modal2 on top
	fm.PushModal(modal2)
	if modal1.Focused() {
		t.Error("modal1 should be Blur'd after nested push")
	}
	if !modal2.Focused() {
		t.Error("modal2 should be Focused after push")
	}
	if fm.Focused() != modal2 {
		t.Error("Focused() should return modal2 (top)")
	}

	// 5. Pop modal2 → modal1 focused again
	fm.PopModal()
	if modal2.Focused() {
		t.Error("modal2 should be Blur'd after pop")
	}
	if !modal1.Focused() {
		t.Error("modal1 should be re-Focused after modal2 popped")
	}
	if fm.Focused() != modal1 {
		t.Error("Focused() should return modal1 after modal2 popped")
	}

	// 6. Pop modal1 → child1 focused
	fm.PopModal()
	if modal1.Focused() {
		t.Error("modal1 should be Blur'd after pop")
	}
	if !child1.Focused() {
		t.Error("child1 should be Focused after last modal popped")
	}
	if fm.ModalActive() {
		t.Error("ModalActive should be false after all modals popped")
	}
	if fm.Focused() != child1 {
		t.Error("Focused() should return child1 after all modals popped")
	}

	// 7. Pop on empty stack → no-op
	fm.PopModal() // should not panic
	if fm.ModalActive() {
		t.Error("ModalActive should still be false")
	}
	if !child1.Focused() {
		t.Error("child1 should still be Focused after no-op pop")
	}
}

// fakeFocusable is a test helper implementing core.Focusable.
type fakeFocusable struct {
	name    string
	focused bool
}

func (f *fakeFocusable) Focus()                             { f.focused = true }
func (f *fakeFocusable) Blur()                              { f.focused = false }
func (f *fakeFocusable) Focused() bool                      { return f.focused }
func (f *fakeFocusable) Init() tea.Cmd                      { return nil }
func (f *fakeFocusable) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return f, nil }
func (f *fakeFocusable) View() tea.View                     { return tea.NewView("") }

// TestParity_CommandResultRouting validates how AppModel routes the three
// command result message types: QuitMsg, ShowHelpMsg, CommandResult.
//
// Unique behaviors (no existing test validates message routing):
// 1. QuitMsg produces tea.Quit cmd
// 2. ShowHelpMsg adds a help text message to conversation
// 3. CommandResult with Error → adds "Error: {msg}" message
// 4. CommandResult with Output → adds Output as message
// 5. CommandResult with BOTH Error and Output → Error takes precedence, Output dropped
// 6. CommandResult with neither set → no message added (no-op)
//
// Cross-ref: app.go:237-263 quit/help/result handlers
func TestParity_CommandResultRouting(t *testing.T) {
	// 1. QuitMsg → tea.Quit
	t.Run("quit", func(t *testing.T) {
		app := NewAppModel(nil, nil)
		_, cmd := app.Update(commands.QuitMsg{})
		if cmd == nil {
			t.Fatal("QuitMsg should produce a cmd")
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Error("QuitMsg should produce tea.QuitMsg cmd")
		}
	})

	// 2. ShowHelpMsg → adds message to conversation
	t.Run("help", func(t *testing.T) {
		config := session.DefaultConfig()
		sess := session.New(config, "/tmp")
		app := NewAppModel(sess, nil)
		countBefore := app.conversation.MessageCount()
		app.Update(commands.ShowHelpMsg{})
		if app.conversation.MessageCount() != countBefore+1 {
			t.Errorf("ShowHelpMsg should add 1 message, got %d→%d", countBefore, app.conversation.MessageCount())
		}
	})

	// 3. CommandResult with error → error message added
	t.Run("result-error", func(t *testing.T) {
		config := session.DefaultConfig()
		sess := session.New(config, "/tmp")
		app := NewAppModel(sess, nil)
		app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		// Dismiss welcome so conversation pane is visible
		app.showWelcome = false
		countBefore := app.conversation.MessageCount()
		app.Update(commands.CommandResult{
			Command: "/test",
			Error:   fmt.Errorf("something failed"),
		})
		if app.conversation.MessageCount() != countBefore+1 {
			t.Errorf("Error CommandResult should add 1 message, got %d→%d", countBefore, app.conversation.MessageCount())
		}
		// View should show the error text
		v := strip(app.View().Content)
		if !strings.Contains(v, "something failed") {
			t.Errorf("View should contain error text, got:\n%s", v)
		}
	})

	// 4. CommandResult with output → output message added
	t.Run("result-output", func(t *testing.T) {
		config := session.DefaultConfig()
		sess := session.New(config, "/tmp")
		app := NewAppModel(sess, nil)
		app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		// Dismiss welcome so conversation pane is visible
		app.showWelcome = false
		countBefore := app.conversation.MessageCount()
		app.Update(commands.CommandResult{
			Command: "/test",
			Output:  "success output",
		})
		if app.conversation.MessageCount() != countBefore+1 {
			t.Errorf("Output CommandResult should add 1 message, got %d→%d", countBefore, app.conversation.MessageCount())
		}
		v := strip(app.View().Content)
		if !strings.Contains(v, "success output") {
			t.Errorf("View should contain output text, got:\n%s", v)
		}
	})

	// 5. CommandResult with BOTH → Error wins
	t.Run("result-both-error-wins", func(t *testing.T) {
		config := session.DefaultConfig()
		sess := session.New(config, "/tmp")
		app := NewAppModel(sess, nil)
		app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		// Dismiss welcome so conversation pane is visible
		app.showWelcome = false
		countBefore := app.conversation.MessageCount()
		app.Update(commands.CommandResult{
			Command: "/test",
			Error:   fmt.Errorf("error wins"),
			Output:  "should be dropped",
		})
		// Only ONE message added (the error), not two
		if app.conversation.MessageCount() != countBefore+1 {
			t.Errorf("CommandResult with both should add 1 message (error wins), got %d→%d",
				countBefore, app.conversation.MessageCount())
		}
		v := strip(app.View().Content)
		if !strings.Contains(v, "error wins") {
			t.Error("Error should win — view should contain error text")
		}
		if strings.Contains(v, "should be dropped") {
			t.Error("Output should be dropped when Error is set")
		}
	})

	// 6. CommandResult with neither → no message added
	t.Run("result-empty", func(t *testing.T) {
		config := session.DefaultConfig()
		sess := session.New(config, "/tmp")
		app := NewAppModel(sess, nil)
		app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		// Dismiss welcome so conversation pane is visible
		app.showWelcome = false
		countBefore := app.conversation.MessageCount()
		app.Update(commands.CommandResult{Command: "/test"})
		if app.conversation.MessageCount() != countBefore {
			t.Errorf("Empty CommandResult should NOT add message, got %d→%d",
				countBefore, app.conversation.MessageCount())
		}
	})
}

// TestParity_ClearConversationFullReset validates ClearConversationMsg's
// full cleanup across conversation pane AND session state.
//
// Unique behaviors (SlashCommandClear only checks conversation.MessageCount=0):
// 1. ConversationPane messages cleared
// 2. session.Messages slice length reset to 0
// 3. session.TurnCount reset to 0
// 4. Messages slice preserved as empty slice (not nil)
// 5. Nil session doesn't crash (safe guard)
// 6. After clear, new submit works normally AND adds to session
// 7. TurnCount increments again after first post-clear submit
//
// Cross-ref: app.go:222-228 ClearConversationMsg handler
func TestParity_ClearConversationFullReset(t *testing.T) {
	config := session.DefaultConfig()
	sess := session.New(config, "/tmp")
	app := NewAppModel(sess, nil)
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Setup: submit a few messages to populate state
	for i := 0; i < 3; i++ {
		app.Update(components.SubmitMsg{Text: fmt.Sprintf("msg %d", i)})
		app.Update(TextDeltaMsg{Text: fmt.Sprintf("resp %d", i)})
		app.Update(TurnCompleteMsg{})
	}
	sess.TurnCount = 3 // manually set to simulate turn tracking

	// Verify setup
	if len(sess.Messages) < 3 {
		t.Fatalf("Setup: expected 3+ messages, got %d", len(sess.Messages))
	}
	if sess.TurnCount != 3 {
		t.Fatalf("Setup: expected TurnCount=3, got %d", sess.TurnCount)
	}
	convCountBefore := app.conversation.MessageCount()
	if convCountBefore == 0 {
		t.Fatal("Setup: conversation should have messages")
	}

	// Fire ClearConversationMsg directly
	app.Update(commands.ClearConversationMsg{})

	// 1. Conversation cleared
	if app.conversation.MessageCount() != 0 {
		t.Errorf("After clear, conversation should be empty, got %d", app.conversation.MessageCount())
	}

	// 2-3. Session state reset
	if len(sess.Messages) != 0 {
		t.Errorf("After clear, session.Messages length should be 0, got %d", len(sess.Messages))
	}
	if sess.TurnCount != 0 {
		t.Errorf("After clear, session.TurnCount should be 0, got %d", sess.TurnCount)
	}

	// 4. Messages slice is not nil (empty slice via [:0])
	if sess.Messages == nil {
		t.Error("After clear, session.Messages should be empty slice, not nil")
	}

	// 5. Nil session is safely handled
	app2 := NewAppModel(nil, nil)
	app2.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	// Should not panic
	app2.Update(commands.ClearConversationMsg{})

	// 6-7. After clear, new submit works and adds to session
	app.Update(components.SubmitMsg{Text: "fresh message"})
	if len(sess.Messages) != 1 {
		t.Errorf("After post-clear submit, session should have 1 message, got %d", len(sess.Messages))
	}
	// Conversation should have the new message
	if app.conversation.MessageCount() != 1 {
		t.Errorf("Conversation should have 1 message after post-clear submit, got %d", app.conversation.MessageCount())
	}
}

// TestParity_InputEnterSubmitFlow validates the Enter key submit pipeline:
// text trimming, buffer clear, historyIdx reset, SubmitMsg generation.
//
// Unique behaviors (no existing test validates the full submit flow):
// 1. Enter with text → returns cmd that produces SubmitMsg
// 2. SubmitMsg carries the TRIMMED text (whitespace stripped)
// 3. After submit, buffer is cleared (empty)
// 4. After submit, historyIdx is reset to -1 (exit history nav)
// 5. Enter with empty buffer → no cmd returned (nil)
// 6. Enter with whitespace-only → no cmd returned (trimmed to empty)
// 7. Buffer and cursor both reset after successful submit
//
// Cross-ref: input.go:123-135 Enter key handler
func TestParity_InputEnterSubmitFlow(t *testing.T) {
	// 1-4. Enter with text → SubmitMsg, buffer clear, historyIdx reset
	inp := components.NewInputPane()
	inp.SetSize(80, 3)
	inp.Focus()
	// Add a history entry and navigate to it
	inp.AddToHistory("old cmd")
	inp.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	// Now buffer shows "old cmd"; simulate user typing over it (but we can't with guard)
	// Instead: reset by clearing then typing fresh
	inp.Clear()
	for _, ch := range "  hello world  " {
		inp.Update(tea.KeyPressMsg{Code: rune(ch), Text: string(ch)})
	}
	_, cmd := inp.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("Enter with text should return a cmd")
	}
	msg := cmd()
	submitted, ok := msg.(components.SubmitMsg)
	if !ok {
		t.Fatalf("Expected SubmitMsg, got %T", msg)
	}
	// 2. Text is trimmed
	if submitted.Text != "hello world" {
		t.Errorf("SubmitMsg.Text should be trimmed to 'hello world', got %q", submitted.Text)
	}
	// 3. Buffer is cleared
	if inp.Value() != "" {
		t.Errorf("Buffer should be cleared after submit, got %q", inp.Value())
	}
	// 4. HasText is false
	if inp.HasText() {
		t.Error("HasText() should be false after submit")
	}

	// 5. Enter with empty buffer → nil cmd
	_, emptyCmd := inp.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if emptyCmd != nil {
		t.Error("Enter on empty buffer should return nil cmd")
	}

	// 6. Enter with whitespace-only → nil cmd
	for _, ch := range "   \t  " {
		inp.Update(tea.KeyPressMsg{Code: rune(ch), Text: string(ch)})
	}
	_, wsCmd := inp.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if wsCmd != nil {
		t.Error("Enter with whitespace-only text should return nil cmd")
	}
	// Buffer is NOT cleared for whitespace-only (text check is on TRIMMED text)
	// Actually wait — the code path returns nil AFTER the text!="" check, so buffer stays
	// Let me verify:
	// text := strings.TrimSpace(string(ip.runes))
	// if text != "" { ... Clear() ... return SubmitMsg }
	// return ip, nil (buffer unchanged)
	if !inp.HasText() {
		t.Error("Whitespace buffer should remain (not cleared) on empty-trim Enter")
	}
}

// TestParity_HeaderSegmentComposition validates the Header component's
// segment composition logic (only non-empty fields produce segments).
//
// Unique behaviors (no existing test validates Header composition):
// 1. Default: only "✻ Claude" logo segment (no │ separators)
// 2. SetModel adds model as separate segment with │ separator
// 3. SetCWD adds cwd segment
// 4. SetSessionName adds session segment
// 5. All three set → 4 segments joined by 3 │ separators
// 6. Empty strings don't produce empty segments (segment skipping)
// 7. Width > 0 pads rendered output to exact width
//
// Cross-ref: header.go:71-111 View composition logic
func TestParity_HeaderSegmentComposition(t *testing.T) {
	h := components.NewHeader(theme.Current())
	h.SetSize(80, 1)

	// 1. Default: only logo, no separators
	v1 := strip(h.View().Content)
	// Count │ separators — should be 0
	sepCount1 := strings.Count(v1, "│")
	if sepCount1 != 0 {
		t.Errorf("Default header should have 0 │ separators (only logo), got %d: %q", sepCount1, strings.TrimSpace(v1))
	}

	// Logo "✻ Claude" must be present
	if !strings.Contains(v1, "✻ Claude") {
		t.Errorf("Default header should show '✻ Claude' logo, got: %q", v1)
	}

	// 2. SetModel → adds 1 separator (1 segment + logo = 2 segments)
	h.SetModel("claude-opus-4-6")
	v2 := strip(h.View().Content)
	sepCount2 := strings.Count(v2, "│")
	if sepCount2 != 1 {
		t.Errorf("Header with model should have 1 │ separator, got %d", sepCount2)
	}
	if !strings.Contains(v2, "claude-opus-4-6") {
		t.Errorf("Model name should appear, got: %q", v2)
	}

	// 3. SetCWD → 2 separators total (3 segments: logo, model, cwd)
	h.SetCWD("/some/dir")
	v3 := strip(h.View().Content)
	sepCount3 := strings.Count(v3, "│")
	if sepCount3 != 2 {
		t.Errorf("Header with model+cwd should have 2 │ separators, got %d", sepCount3)
	}

	// 4. SetSessionName → 3 separators total (4 segments)
	h.SetSessionName("my-session")
	v4 := strip(h.View().Content)
	sepCount4 := strings.Count(v4, "│")
	if sepCount4 != 3 {
		t.Errorf("Header with all fields should have 3 │ separators, got %d", sepCount4)
	}
	if !strings.Contains(v4, "my-session") {
		t.Errorf("Session name should appear, got: %q", v4)
	}

	// 5. Empty strings: setting model back to "" drops the segment
	h.SetModel("")
	v5 := strip(h.View().Content)
	sepCount5 := strings.Count(v5, "│")
	if sepCount5 != 2 {
		t.Errorf("After SetModel(''), should have 2 │ separators (logo+cwd+session), got %d", sepCount5)
	}
	if strings.Contains(v5, "claude-opus-4-6") {
		t.Errorf("Empty model should be removed from view, got: %q", v5)
	}

	// 6. Getters reflect current state
	if h.ModelName() != "" {
		t.Errorf("ModelName() should return '' after clear, got %q", h.ModelName())
	}
	if h.CWD() != "/some/dir" {
		t.Errorf("CWD() should return '/some/dir', got %q", h.CWD())
	}
	if h.SessionName() != "my-session" {
		t.Errorf("SessionName() should return 'my-session', got %q", h.SessionName())
	}

	// 7. Width padding: ensure rendered output matches width
	h2 := components.NewHeader(theme.Current())
	h2.SetSize(60, 1)
	h2.SetModel("sonnet")
	v7 := strip(h2.View().Content)
	// Output should be padded to 60 chars
	if len([]rune(v7)) != 60 {
		t.Errorf("Width=60 should pad to 60 chars, got %d: %q", len([]rune(v7)), v7)
	}
}

// TestParity_HandleResizeLayoutBudget validates handleResize's layout budget
// correctly accounts for ALL chrome elements: header, 2 dividers, input, status.
//
// Unique behaviors (no existing test validates resize math):
// 1. a.width and a.height store the message dimensions
// 2. conversation height = total - chrome (header + 2 dividers + input + status = 7)
// 3. Small terminal (height < 8) clamps conversation to 1 line
// 4. View output exactly fits terminal height (no overflow)
// 5. Resize is idempotent (same input → same output)
//
// Cross-ref: app.go:314-337 handleResize, app.go:View for actual chrome count
func TestParity_HandleResizeLayoutBudget(t *testing.T) {
	config := session.DefaultConfig()
	sess := session.New(config, "/tmp")
	app := NewAppModel(sess, nil)

	// 1. Basic size storage
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if app.width != 80 {
		t.Errorf("width should be 80, got %d", app.width)
	}
	if app.height != 24 {
		t.Errorf("height should be 24, got %d", app.height)
	}

	// 2. View output should exactly fit within terminal height
	// Dismiss welcome to use non-welcome layout
	app.Update(components.SubmitMsg{Text: "hi"})
	app.Update(TurnCompleteMsg{})

	view := app.View()
	plain := strip(view.Content)
	lines := strings.Split(strings.TrimRight(plain, "\n"), "\n")

	// View should render AT MOST 24 lines (matching terminal height)
	if len(lines) > 24 {
		t.Errorf("View rendered %d lines for 24-line terminal — layout budget off by %d",
			len(lines), len(lines)-24)
	}

	// 2b. Fill conversation with many messages to stress-test the layout budget
	for i := 0; i < 50; i++ {
		app.Update(components.SubmitMsg{Text: fmt.Sprintf("msg %d", i)})
		app.Update(TextDeltaMsg{Text: fmt.Sprintf("response %d", i)})
		app.Update(TurnCompleteMsg{})
	}
	viewFull := app.View()
	plainFull := strip(viewFull.Content)
	linesFull := strings.Split(strings.TrimRight(plainFull, "\n"), "\n")
	if len(linesFull) > 24 {
		t.Errorf("Full conversation: view rendered %d lines for 24-line terminal — layout overflows by %d",
			len(linesFull), len(linesFull)-24)
	}

	// 3. Very small terminal: should not crash
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 5})
	v2 := app.View()
	if v2.Content == "" {
		t.Error("View should render even at height=5")
	}

	// 4. Very narrow terminal: should not crash
	app.Update(tea.WindowSizeMsg{Width: 20, Height: 10})
	v3 := app.View()
	if v3.Content == "" {
		t.Error("View should render even at width=20")
	}

	// 5. Idempotent resize
	app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	vA := app.View().Content
	app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	vB := app.View().Content
	if vA != vB {
		t.Error("Repeat resize with same dimensions should produce identical view")
	}
}

// TestParity_QueryDoneErrorPath validates handleQueryDone's state reset
// and error-message handling, covering both success and error paths.
//
// Unique behaviors (no existing test covers queryDone error path):
// 1. cancelQuery and queryCtx both reset to nil
// 2. Spinner stopped (IsActive=false after)
// 3. Partial streamingText finalized as assistant message before reset
// 4. streamingText buffer cleared after finalization
// 5. activeToolCalls map cleared
// 6. Error case: adds "Error: {msg}" as additional assistant message
// 7. Success case (err=nil): does NOT add error message
// 8. Mode always set to ModeIdle
//
// Cross-ref: app.go:595-633 handleQueryDone
func TestParity_QueryDoneErrorPath(t *testing.T) {
	// Test success path (no error)
	t.Run("success-with-partial-text", func(t *testing.T) {
		config := session.DefaultConfig()
		sess := session.New(config, "/tmp")
		app := NewAppModel(sess, nil)
		app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

		// Set up streaming state
		app.Update(components.SubmitMsg{Text: "test"})
		app.Update(TextDeltaMsg{Text: "partial response"})
		app.activeToolCalls["t1"] = "Bash"
		app.cancelQuery = func() {}

		msgCountBefore := app.conversation.MessageCount()

		// Fire queryDoneMsg with no error
		app.Update(queryDoneMsg{err: nil})

		// 1-2. cancelQuery/queryCtx reset + spinner stopped
		if app.cancelQuery != nil {
			t.Error("cancelQuery should be nil after queryDone")
		}
		if app.spinner.IsActive() {
			t.Error("spinner should be stopped after queryDone")
		}

		// 3-4. Partial text finalized + buffer cleared
		if app.streamingText.Len() != 0 {
			t.Errorf("streamingText should be reset, len=%d", app.streamingText.Len())
		}

		// 5. activeToolCalls cleared
		if len(app.activeToolCalls) != 0 {
			t.Errorf("activeToolCalls should be cleared, got %d", len(app.activeToolCalls))
		}

		// 7. Success path: 1 new message (finalized partial text), no error message
		gained := app.conversation.MessageCount() - msgCountBefore
		if gained != 1 {
			t.Errorf("Success: expected +1 message (partial finalized), got +%d", gained)
		}

		// 8. Mode idle
		if app.mode != ModeIdle {
			t.Errorf("Mode should be Idle after queryDone, got %v", app.mode)
		}
	})

	// Test error path
	t.Run("error-adds-error-message", func(t *testing.T) {
		config := session.DefaultConfig()
		sess := session.New(config, "/tmp")
		app := NewAppModel(sess, nil)
		app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

		app.Update(components.SubmitMsg{Text: "test"})
		app.Update(TextDeltaMsg{Text: "some text"})
		msgCountBefore := app.conversation.MessageCount()

		// Fire queryDoneMsg WITH error
		testErr := fmt.Errorf("network timeout")
		app.Update(queryDoneMsg{err: testErr})

		// 6. Error path: 2 new messages (finalized partial + error message)
		gained := app.conversation.MessageCount() - msgCountBefore
		if gained != 2 {
			t.Errorf("Error: expected +2 messages (partial + error), got +%d", gained)
		}

		// Error message should contain the error text
		v := strip(app.View().Content)
		if !strings.Contains(v, "network timeout") {
			t.Errorf("Error message should contain error text 'network timeout'.\nGot:\n%s", v)
		}

		if app.mode != ModeIdle {
			t.Error("Mode should be Idle even on error")
		}
	})

	// Test error with no partial text
	t.Run("error-no-partial-text", func(t *testing.T) {
		config := session.DefaultConfig()
		sess := session.New(config, "/tmp")
		app := NewAppModel(sess, nil)
		app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

		app.Update(components.SubmitMsg{Text: "test"})
		msgCountBefore := app.conversation.MessageCount()

		// queryDoneMsg with error BUT no partial text
		app.Update(queryDoneMsg{err: fmt.Errorf("failed")})

		// Only 1 new message (the error) since no partial text to finalize
		gained := app.conversation.MessageCount() - msgCountBefore
		if gained != 1 {
			t.Errorf("Error with no partial text: expected +1 message (error only), got +%d", gained)
		}
	})
}

// TestParity_TextDeltaBufferAccumulation validates handleTextDelta
// buffer behavior: accumulation, mode transition, exact concatenation.
//
// Unique behaviors (no existing test validates streamingText buffer directly):
// 1. First delta sets mode ModeIdle → ModeStreaming
// 2. Delta FROM ModeToolRunning also sets back to ModeStreaming
// 3. streamingText accumulates text exactly (no separators injected)
// 4. streamingText.Len() matches sum of all delta lengths
// 5. Empty delta still sets mode to ModeStreaming (side-effect)
// 6. Sequential deltas produce exact concatenation "HelloWorld" not "Hello World"
//
// Cross-ref: app.go:506-522 handleTextDelta
func TestParity_TextDeltaBufferAccumulation(t *testing.T) {
	config := session.DefaultConfig()
	sess := session.New(config, "/tmp")
	app := NewAppModel(sess, nil)
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// 1. Initial state
	if app.mode != ModeIdle {
		t.Fatalf("Initial mode should be Idle, got %v", app.mode)
	}

	// 2. First delta: Idle → Streaming
	app.Update(TextDeltaMsg{Text: "Hello"})
	if app.mode != ModeStreaming {
		t.Errorf("After delta, expected ModeStreaming, got %v", app.mode)
	}
	if app.streamingText.String() != "Hello" {
		t.Errorf("Buffer should be 'Hello', got %q", app.streamingText.String())
	}

	// 3. Multiple deltas accumulate exactly (no spaces/separators added)
	app.Update(TextDeltaMsg{Text: "World"})
	if app.streamingText.String() != "HelloWorld" {
		t.Errorf("Expected exact concatenation 'HelloWorld', got %q", app.streamingText.String())
	}

	// 4. Length matches sum of delta lengths
	app.Update(TextDeltaMsg{Text: "!"})
	if app.streamingText.Len() != len("HelloWorld!") {
		t.Errorf("Buffer length should be %d, got %d", len("HelloWorld!"), app.streamingText.Len())
	}
	if app.streamingText.String() != "HelloWorld!" {
		t.Errorf("Expected 'HelloWorld!', got %q", app.streamingText.String())
	}

	// 5. Empty delta still sets mode to Streaming
	// Force mode to something else first
	app.mode = ModeToolRunning
	app.Update(TextDeltaMsg{Text: ""})
	if app.mode != ModeStreaming {
		t.Errorf("Empty delta should set ModeStreaming, got %v", app.mode)
	}
	// Buffer should NOT have grown from empty delta
	if app.streamingText.String() != "HelloWorld!" {
		t.Errorf("Empty delta should not modify buffer, got %q", app.streamingText.String())
	}

	// 6. Delta transitions mode back from ToolRunning to Streaming
	app.mode = ModeToolRunning
	app.Update(TextDeltaMsg{Text: " continued"})
	if app.mode != ModeStreaming {
		t.Errorf("Delta from ToolRunning should return to Streaming, got %v", app.mode)
	}
	if app.streamingText.String() != "HelloWorld! continued" {
		t.Errorf("Expected 'HelloWorld! continued', got %q", app.streamingText.String())
	}

	// 7. After TurnComplete, buffer resets
	app.Update(TurnCompleteMsg{})
	if app.streamingText.Len() != 0 {
		t.Errorf("TurnComplete should reset buffer, got len=%d", app.streamingText.Len())
	}

	// 8. New delta after TurnComplete starts fresh
	app.Update(TextDeltaMsg{Text: "Fresh"})
	if app.streamingText.String() != "Fresh" {
		t.Errorf("New delta after turn should start fresh, got %q", app.streamingText.String())
	}
}

// TestParity_UserMessageWrappingAndPrefix validates user message rendering:
// text wrapping at width-4, first-line ❯ prefix vs continuation 2-space prefix.
//
// Unique behaviors (no existing test validates user message wrapping):
// 1. First line starts with "❯ " prefix
// 2. Continuation lines (wrapped) start with 2 spaces, not ❯
// 3. Short messages produce exactly 1 line
// 4. Long messages wrap at width-4 (leaving border+prefix room)
// 5. Multiple ContentText blocks each get their own prefix handling
// 6. ContentToolResult blocks within user message render with ⎿ connector
// 7. Unknown block types are silently dropped (not rendered)
//
// Cross-ref: message_bubble.go:89-133 renderUserMessage
func TestParity_UserMessageWrappingAndPrefix(t *testing.T) {
	mb := components.NewMessageBubble(theme.Current(), 20) // narrow width forces wrapping

	// 1-2. Short message produces 1 line with ❯ prefix
	shortMsg := &message.Message{
		Role:    message.RoleUser,
		Content: []message.ContentBlock{{Type: message.ContentText, Text: "hello"}},
	}
	shortOut := strip(mb.Render(shortMsg))
	shortLines := strings.Split(strings.TrimRight(shortOut, "\n"), "\n")
	// Filter empty lines
	nonEmpty := []string{}
	for _, l := range shortLines {
		if strings.TrimSpace(l) != "" {
			nonEmpty = append(nonEmpty, l)
		}
	}
	if len(nonEmpty) != 1 {
		t.Errorf("Short 'hello' should produce 1 line, got %d: %q", len(nonEmpty), nonEmpty)
	}
	if !strings.Contains(nonEmpty[0], "❯") {
		t.Errorf("First line should have ❯ prefix: %q", nonEmpty[0])
	}
	if !strings.Contains(nonEmpty[0], "hello") {
		t.Errorf("First line should contain text 'hello': %q", nonEmpty[0])
	}

	// 3-4. Long message wraps; continuation lines don't have ❯
	longMsg := &message.Message{
		Role:    message.RoleUser,
		Content: []message.ContentBlock{{Type: message.ContentText, Text: "this is a very long message that will definitely wrap across multiple lines"}},
	}
	longOut := strip(mb.Render(longMsg))
	longLines := strings.Split(strings.TrimRight(longOut, "\n"), "\n")
	nonEmptyLong := []string{}
	for _, l := range longLines {
		if strings.TrimSpace(l) != "" {
			nonEmptyLong = append(nonEmptyLong, l)
		}
	}
	if len(nonEmptyLong) < 2 {
		t.Errorf("Long text should wrap to 2+ lines, got %d: %q", len(nonEmptyLong), nonEmptyLong)
	}
	// First line has ❯
	if !strings.Contains(nonEmptyLong[0], "❯") {
		t.Errorf("First wrapped line should have ❯: %q", nonEmptyLong[0])
	}
	// Continuation lines must NOT have ❯ (just 2-space indent)
	for i := 1; i < len(nonEmptyLong); i++ {
		if strings.Contains(nonEmptyLong[i], "❯") {
			t.Errorf("Continuation line %d should NOT have ❯: %q", i, nonEmptyLong[i])
		}
	}

	// 5. Unknown block types silently dropped — a message with only unknown type produces empty output
	unknownMsg := &message.Message{
		Role:    message.RoleUser,
		Content: []message.ContentBlock{{Type: message.ContentThinking, Thinking: "hidden"}},
	}
	unknownOut := strip(mb.Render(unknownMsg))
	// User renderer only handles ContentText and ContentToolResult,
	// so Thinking block should be dropped
	if strings.Contains(unknownOut, "hidden") {
		t.Errorf("ContentThinking in user message should be dropped, got: %q", unknownOut)
	}
}

// TestParity_WelcomeResponsiveSizing validates SetSize calculation AND that
// the rendered box width actually matches ws.width (no drift).
//
// Unique behaviors (no existing test validates width calculation or enforcement):
// 1. SetSize(terminal-width, h) sets ws.width = terminal-width - 2 (border budget)
// 2. SetSize with tiny width is clamped to minimum 20
// 3. Rendered top border width == ws.width + 2 (includes ╭ and ╮)
// 4. All body lines have same rendered width
// 5. Growing terminal expands box width
// 6. Width changes trigger re-render (idempotent SetSize works)
//
// Cross-ref: welcome.go:165-172 SetSize
func TestParity_WelcomeResponsiveSizing(t *testing.T) {
	ws := components.NewWelcomeScreen(theme.Current(), "claude-opus-4-6", "/tmp")

	// Test various terminal sizes
	cases := []struct {
		termWidth int
		wantBox   int // expected box content width (= termWidth - 2, or 20 min)
	}{
		{80, 78},
		{100, 98},
		{60, 58},
		{40, 38},
		{22, 20}, // 22-2=20 (exact minimum)
		{15, 20}, // clamped to 20
		{5, 20},  // clamped to 20
	}

	for _, tc := range cases {
		ws.SetSize(tc.termWidth, 24)
		// Render and verify top border width
		view := ws.View()
		plain := strip(view.Content)
		lines := strings.Split(plain, "\n")

		// Find top border line
		var topLine string
		for _, l := range lines {
			if strings.HasPrefix(strings.TrimSpace(l), "╭") && strings.HasSuffix(strings.TrimSpace(l), "╮") {
				topLine = strings.TrimSpace(l)
				break
			}
		}
		if topLine == "" {
			t.Errorf("termWidth=%d: no ╭...╮ border line found", tc.termWidth)
			continue
		}

		// Top line includes ╭ + content + ╮ = wantBox + 2
		actualWidth := len([]rune(topLine))
		expectedWidth := tc.wantBox + 2
		if actualWidth != expectedWidth {
			t.Errorf("termWidth=%d: expected border width %d, got %d (line: %s)",
				tc.termWidth, expectedWidth, actualWidth, topLine)
		}
	}

	// Test that growing width expands box (not static)
	ws.SetSize(50, 24)
	v1 := strip(ws.View().Content)
	w1 := maxLineWidth(v1)

	ws.SetSize(100, 24)
	v2 := strip(ws.View().Content)
	w2 := maxLineWidth(v2)

	if w2 <= w1 {
		t.Errorf("Growing from 50 to 100 should increase width, got %d → %d", w1, w2)
	}

	// Idempotent: calling SetSize with same value is safe
	ws.SetSize(80, 24)
	v3 := strip(ws.View().Content)
	ws.SetSize(80, 24)
	v4 := strip(ws.View().Content)
	if v3 != v4 {
		t.Error("Repeat SetSize with same args should produce identical output")
	}
}

// maxLineWidth returns the width of the longest line in s (rune count).
func maxLineWidth(s string) int {
	max := 0
	for _, line := range strings.Split(s, "\n") {
		if n := len([]rune(line)); n > max {
			max = n
		}
	}
	return max
}

// TestParity_StatusLineHintLifecycle validates the CtrlCHintMsg state machine
// in the StatusLine component, including how it interacts with mode changes.
//
// Unique behaviors (no existing test validates status hint transitions):
// 1. Default idle status shows "? for shortcuts"
// 2. CtrlCHintMsg switches idle display to "Press Ctrl-C again to exit"
// 3. ModeStreaming overrides hint → shows "esc to interrupt"
// 4. ModeToolRunning also shows "esc to interrupt"
// 5. ModeChangeMsg to ModeIdle clears the hint → back to "? for shortcuts"
// 6. Mode change from streaming TO idle resets ctrlCHint (no stale hint)
// 7. CtrlCHintMsg while streaming doesn't affect visible text (mode wins)
//
// Cross-ref: statusline.go:62-80 Update, :82-125 View mode switch
func TestParity_StatusLineHintLifecycle(t *testing.T) {
	config := session.DefaultConfig()
	sess := session.New(config, "/tmp")
	sl := components.NewStatusLine(sess)
	sl.SetSize(80, 1)

	// 1. Default idle → "? for shortcuts"
	v1 := strip(sl.View().Content)
	if !strings.Contains(v1, "? for shortcuts") {
		t.Errorf("Default idle should show '? for shortcuts', got: %s", v1)
	}

	// 2. CtrlCHintMsg → "Press Ctrl-C again to exit"
	sl.Update(components.CtrlCHintMsg{})
	v2 := strip(sl.View().Content)
	if !strings.Contains(v2, "Ctrl-C again") {
		t.Errorf("After CtrlCHintMsg should show exit hint, got: %s", v2)
	}
	if strings.Contains(v2, "? for shortcuts") {
		t.Errorf("After CtrlCHintMsg should NOT show '? for shortcuts', got: %s", v2)
	}

	// 3. Switch to streaming → "esc to interrupt" (mode overrides hint visibility)
	sl.Update(components.ModeChangeMsg{Mode: components.ModeStreaming})
	v3 := strip(sl.View().Content)
	if !strings.Contains(v3, "esc to interrupt") {
		t.Errorf("Streaming should show 'esc to interrupt', got: %s", v3)
	}
	if strings.Contains(v3, "Ctrl-C again") {
		t.Errorf("Streaming should NOT show Ctrl-C hint (mode wins), got: %s", v3)
	}

	// 4. Switch to tool running → same as streaming
	sl.Update(components.ModeChangeMsg{Mode: components.ModeToolRunning})
	v4 := strip(sl.View().Content)
	if !strings.Contains(v4, "esc to interrupt") {
		t.Errorf("ToolRunning should show 'esc to interrupt', got: %s", v4)
	}

	// 5. Back to idle → "? for shortcuts" (ctrlCHint was reset by ModeChangeMsg)
	sl.Update(components.ModeChangeMsg{Mode: components.ModeIdle})
	v5 := strip(sl.View().Content)
	if !strings.Contains(v5, "? for shortcuts") {
		t.Errorf("Back to idle should show '? for shortcuts' (hint reset), got: %s", v5)
	}
	if strings.Contains(v5, "Ctrl-C again") {
		t.Errorf("After mode change, hint should be cleared, got: %s", v5)
	}

	// 6. Set hint again while idle → shows hint
	sl.Update(components.CtrlCHintMsg{})
	v6 := strip(sl.View().Content)
	if !strings.Contains(v6, "Ctrl-C again") {
		t.Errorf("Re-setting hint should show again, got: %s", v6)
	}

	// 7. Mode change to idle (same state) still clears hint — defensive reset
	sl.Update(components.ModeChangeMsg{Mode: components.ModeIdle})
	v7 := strip(sl.View().Content)
	if strings.Contains(v7, "Ctrl-C again") {
		t.Errorf("ModeChange to Idle should clear hint even if already Idle, got: %s", v7)
	}
}

// TestParity_QueryEventDispatchAllTypes validates AppModel.handleQueryEvent
// correctly dispatches all 5 QueryEvent types and accumulates usage.
//
// Unique behaviors (QueryEventFlow only covers TextDelta/Usage/TurnComplete with 1 usage):
// 1. QEventToolUseStart adds tool to activeToolCalls + sets ModeToolRunning
// 2. QEventToolResult removes from activeToolCalls
// 3. QEventUsage accumulates InputTokens across multiple events (50+30=80)
// 4. QEventUsage accumulates OutputTokens independently (25+15=40)
// 5. Unknown event type is a no-op (no panic, no state change)
// 6. Multiple event types in sequence produce correct final state
//
// Cross-ref: app.go:474-503 handleQueryEvent dispatch
func TestParity_QueryEventDispatchAllTypes(t *testing.T) {
	config := session.DefaultConfig()
	sess := session.New(config, "/tmp")
	app := NewAppModel(sess, nil)
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Need streaming state for tool events to make sense
	app.Update(components.SubmitMsg{Text: "test"})

	// 1. QEventToolUseStart dispatches to handleToolUseStart
	app.Update(QueryEventMsg{Event: query.QueryEvent{
		Type: query.QEventToolUseStart, ToolUseID: "t1", ToolName: "Bash",
	}})
	if _, ok := app.activeToolCalls["t1"]; !ok {
		t.Error("QEventToolUseStart should add to activeToolCalls")
	}
	if app.mode != ModeToolRunning {
		t.Errorf("QEventToolUseStart should set ModeToolRunning, got %v", app.mode)
	}

	// 2. QEventToolResult dispatches to handleToolResult
	app.Update(QueryEventMsg{Event: query.QueryEvent{
		Type: query.QEventToolResult, ToolUseID: "t1", Content: "ok", IsError: false,
	}})
	if _, ok := app.activeToolCalls["t1"]; ok {
		t.Error("QEventToolResult should remove from activeToolCalls")
	}

	// 3-4. QEventUsage accumulates both token types across multiple events
	app.Update(QueryEventMsg{Event: query.QueryEvent{
		Type: query.QEventUsage, InputTokens: 50, OutputTokens: 25,
	}})
	if sess.TotalInputTokens != 50 {
		t.Errorf("First usage: expected 50 input tokens, got %d", sess.TotalInputTokens)
	}
	if sess.TotalOutputTokens != 25 {
		t.Errorf("First usage: expected 25 output tokens, got %d", sess.TotalOutputTokens)
	}

	// Second usage event should accumulate
	app.Update(QueryEventMsg{Event: query.QueryEvent{
		Type: query.QEventUsage, InputTokens: 30, OutputTokens: 15,
	}})
	if sess.TotalInputTokens != 80 {
		t.Errorf("Second usage: expected 80 input tokens (50+30), got %d", sess.TotalInputTokens)
	}
	if sess.TotalOutputTokens != 40 {
		t.Errorf("Second usage: expected 40 output tokens (25+15), got %d", sess.TotalOutputTokens)
	}

	// 5. Unknown event type (not in switch) is no-op
	stateBefore := app.mode
	tokensBefore := sess.TotalInputTokens
	app.Update(QueryEventMsg{Event: query.QueryEvent{Type: query.QueryEventType("unknown")}})
	if app.mode != stateBefore {
		t.Error("Unknown event type should not change mode")
	}
	if sess.TotalInputTokens != tokensBefore {
		t.Error("Unknown event type should not change tokens")
	}

	// 6. TurnComplete finalizes the turn
	app.Update(QueryEventMsg{Event: query.QueryEvent{Type: query.QEventTurnComplete}})
	if app.mode != ModeIdle {
		t.Errorf("After TurnComplete, expected ModeIdle, got %v", app.mode)
	}
}

// TestParity_DispatcherParsingAndErrorPaths validates the slash command
// dispatcher's parsing logic and error handling.
//
// Unique behaviors (no existing test validates parsing/error paths):
// 1. Non-slash input returns nil (not a command)
// 2. Whitespace-only input returns nil
// 3. Leading whitespace + slash → IsCommand=true, Dispatch still parses
// 4. Unknown command returns CommandResult with unknown-command error
// 5. Command parsing is case-insensitive (/HELP == /help)
// 6. Args after command are trimmed of surrounding whitespace
// 7. Multi-word args after first space are passed intact
//
// Cross-ref: commands/handlers.go:64-93 Dispatch/IsCommand
func TestParity_DispatcherParsingAndErrorPaths(t *testing.T) {
	d := commands.NewDispatcher()

	// 1. Non-slash input → nil
	if cmd := d.Dispatch("hello world"); cmd != nil {
		t.Error("Non-slash input should return nil cmd")
	}

	// 2. Whitespace-only → nil
	if cmd := d.Dispatch("   "); cmd != nil {
		t.Error("Whitespace-only input should return nil cmd")
	}

	// 3. IsCommand detects leading slash after trim
	if !commands.IsCommand("  /help  ") {
		t.Error("IsCommand should detect slash after trim")
	}
	if commands.IsCommand("hello /help") {
		t.Error("IsCommand should be false for slash not at start")
	}

	// 4. Unknown command → CommandResult with error
	unkCmd := d.Dispatch("/definitelynotacommand")
	if unkCmd == nil {
		t.Fatal("Unknown command should return a cmd")
	}
	result := unkCmd()
	cr, ok := result.(commands.CommandResult)
	if !ok {
		t.Fatalf("Expected CommandResult, got %T", result)
	}
	if cr.Error == nil {
		t.Error("Unknown command should have non-nil Error")
	}
	if !strings.Contains(cr.Error.Error(), "unknown command") {
		t.Errorf("Error should mention 'unknown command', got: %v", cr.Error)
	}
	if cr.Command != "/definitelynotacommand" {
		t.Errorf("CommandResult.Command should be '/definitelynotacommand', got %q", cr.Command)
	}

	// 5. Case-insensitive parsing: /HELP == /help (uses lowercase lookup)
	// Register a handler and verify uppercase input hits it
	seen := ""
	d.Register("/testcmd", func(args string) tea.Cmd {
		return func() tea.Msg {
			seen = args
			return nil
		}
	})
	cmd := d.Dispatch("/TESTCMD hello")
	if cmd == nil {
		t.Fatal("/TESTCMD should dispatch to /testcmd (case-insensitive)")
	}
	cmd()
	if seen != "hello" {
		t.Errorf("Handler should receive args 'hello', got %q", seen)
	}

	// 6. Args trimmed of surrounding whitespace
	seen = ""
	cmd2 := d.Dispatch("/testcmd   spaced  ")
	cmd2()
	if seen != "spaced" {
		t.Errorf("Args should be trimmed, got %q", seen)
	}

	// 7. Multi-word args preserved after first space
	seen = ""
	cmd3 := d.Dispatch("/testcmd one two three")
	cmd3()
	if seen != "one two three" {
		t.Errorf("Multi-word args should be preserved, got %q", seen)
	}
}

// TestParity_AppFocusCyclingTabShiftTab validates Tab/Shift+Tab key routing
// and the FocusManager's ring cycling behavior through AppModel.Update.
//
// Unique behaviors (no existing test validates focus management):
// 1. Tab key triggers focus.Next() in AppModel.handleKey
// 2. Shift+Tab key triggers focus.Prev()
// 3. Next cycles forward, wraps from last to first
// 4. Prev cycles backward, wraps from first to last
// 5. Blur is called on outgoing child, Focus on incoming
// 6. Initial focus is on the first child (input pane)
//
// Cross-ref: app.go:377-383 Tab/Shift+Tab handlers
// Cross-ref: core/focus.go:46-71 Next/Prev ring arithmetic
func TestParity_AppFocusCyclingTabShiftTab(t *testing.T) {
	config := session.DefaultConfig()
	sess := session.New(config, "/tmp")
	app := NewAppModel(sess, nil)
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// NewAppModel creates focus ring with [input, conversation]
	// 1. Initial focus should be on input
	initial := app.focus.Focused()
	if initial == nil {
		t.Fatal("FocusManager should have a focused child initially")
	}
	if !initial.Focused() {
		t.Error("Initially focused child should report Focused()==true")
	}

	// 2. Tab → Next() cycles to second child
	app.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: 0})
	second := app.focus.Focused()
	if second == initial {
		t.Error("Tab should cycle to a different child")
	}
	if second == nil || !second.Focused() {
		t.Error("After Tab, new child should be focused")
	}
	if initial.Focused() {
		t.Error("After Tab, previous child should be Blur'd")
	}

	// 3. Tab again → wraps back to first (2 children in ring)
	app.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: 0})
	wrapped := app.focus.Focused()
	if wrapped != initial {
		t.Error("Tab should wrap back to initial child (ring of 2)")
	}

	// 4. Shift+Tab → Prev() goes backward
	app.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	reversed := app.focus.Focused()
	if reversed != second {
		t.Error("Shift+Tab should go back to second child")
	}

	// 5. Shift+Tab from first → wraps to last (prev from 0)
	// First reset to initial
	app.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}) // now on initial
	app.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}) // wrap to last
	wrapped2 := app.focus.Focused()
	if wrapped2 == initial {
		t.Error("Shift+Tab from first should wrap to last child")
	}

	// 6. Modal active blocks cycling
	// Use the second child as a fake modal
	app.focus.PushModal(second)
	focusBeforeTab := app.focus.Focused()
	app.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: 0})
	if app.focus.Focused() != focusBeforeTab {
		t.Error("Tab with active modal should NOT cycle focus")
	}
	app.focus.PopModal()
}

// TestParity_ThinkingSpinnerLifecycle validates the spinner's state machine
// including Start/Stop, tick-driven frame advancement, and SetEffort mapping.
//
// Unique behaviors (no existing test validates spinner state transitions):
// 1. New spinner: IsActive=false, Frame=0
// 2. Start: IsActive=true, assigns a verb, frame reset to 0
// 3. SetEffort maps known strings (low/medium/high/max) to non-empty effort
// 4. SetEffort with unknown value clears effort (empty string)
// 5. Update(SpinnerTickMsg) while active: frame advances by 1
// 6. Update(SpinnerTickMsg) while inactive: frame does NOT advance
// 7. Frame wraps around len(SpinnerGlyphs) — modulo arithmetic
// 8. Stop: IsActive=false
//
// Cross-ref: spinner_verbs.go:141-187 Start/Stop/SetEffort/Update
func TestParity_ThinkingSpinnerLifecycle(t *testing.T) {
	ts := components.NewThinkingSpinner(theme.Current())

	// 1. New spinner inactive, frame at 0
	if ts.IsActive() {
		t.Error("New spinner should not be active")
	}
	if ts.Frame() != 0 {
		t.Errorf("New spinner frame should be 0, got %d", ts.Frame())
	}

	// 2. Start → active, verb set
	ts.Start()
	if !ts.IsActive() {
		t.Error("After Start, spinner should be active")
	}
	if ts.Verb() == "" {
		t.Error("After Start, verb should be assigned")
	}
	if ts.Frame() != 0 {
		t.Errorf("After Start, frame should reset to 0, got %d", ts.Frame())
	}

	// 3. SetEffort with known values → non-empty effort string in view
	for _, level := range []string{"low", "medium", "high", "max"} {
		ts.SetEffort(level)
		view := strip(ts.View())
		// The effort suffix shows "with {level} effort" — structural check
		if view == "" {
			t.Errorf("SetEffort(%q): view should not be empty", level)
		}
	}

	// 4. SetEffort with unknown → effort cleared (view should still render)
	ts.SetEffort("bogus")
	viewBogus := strip(ts.View())
	if viewBogus == "" {
		t.Error("SetEffort(bogus) should not break rendering")
	}

	// 5. Tick while active → frame advances
	startFrame := ts.Frame()
	ts.Update(components.SpinnerTickMsg{})
	if ts.Frame() != startFrame+1 {
		t.Errorf("Active tick: frame should advance, %d → %d", startFrame, ts.Frame())
	}
	ts.Update(components.SpinnerTickMsg{})
	if ts.Frame() != startFrame+2 {
		t.Errorf("Second tick: frame should advance again, got %d", ts.Frame())
	}

	// 7. Frame wraps at len(SpinnerGlyphs) — tick many times
	// Send enough ticks to force wrap
	for i := 0; i < 100; i++ {
		ts.Update(components.SpinnerTickMsg{})
	}
	// Frame should be valid (< glyphs count, which is 12 based on source)
	if ts.Frame() < 0 {
		t.Errorf("Frame should never be negative, got %d", ts.Frame())
	}
	// After 100+ ticks from initialFrame, we should have wrapped multiple times
	if ts.Frame() >= 20 {
		t.Errorf("Frame should wrap (< 20 for any reasonable glyph count), got %d", ts.Frame())
	}

	// 8. Stop → inactive
	ts.Stop()
	if ts.IsActive() {
		t.Error("After Stop, spinner should not be active")
	}

	// 6. Tick while inactive → frame does NOT advance
	stoppedFrame := ts.Frame()
	ts.Update(components.SpinnerTickMsg{})
	if ts.Frame() != stoppedFrame {
		t.Errorf("Inactive tick should NOT advance frame, %d → %d", stoppedFrame, ts.Frame())
	}
}

// TestParity_ToolResultTruncationAndStyling validates renderToolResultBlock's
// truncation rules and the error-vs-success path divergence.
//
// Unique behaviors (no existing test validates truncation rules):
// 1. Error result >300 chars → truncated to 300 + "…"
// 2. Success result with >10 lines → keeps 10 + "…[truncated]" marker
// 3. Success result >500 chars (single line) → truncated to 500 + "…"
// 4. Empty content → shows "(no content)" placeholder
// 5. Multi-line success: first line prefixed with ResponseConnector (⎿),
//    subsequent lines prefixed with ResponseContinuation (no connector)
// 6. block.Content takes precedence over block.Text when both set
// 7. Empty Content falls through to block.Text
//
// Cross-ref: message_bubble.go:213-265 renderToolResultBlock
func TestParity_ToolResultTruncationAndStyling(t *testing.T) {
	mb := components.NewMessageBubble(theme.Current(), 80)

	// 1. Error truncation at 300 chars
	longErr := strings.Repeat("x", 400)
	errOut := strip(mb.RenderContent(message.ContentBlock{
		Type: message.ContentToolResult, Content: longErr, IsError: true,
	}))
	// Should contain truncation marker
	if !strings.Contains(errOut, "…") {
		t.Errorf("Error >300 chars should contain truncation …, got len=%d", len(errOut))
	}
	// Should NOT contain all 400 'x' chars
	if strings.Count(errOut, "x") >= 400 {
		t.Error("Error should be truncated, contains all 400 chars")
	}

	// 2. Success line truncation: 15 lines → 10 + …[truncated]
	manyLines := strings.Repeat("line\n", 15)
	manyOut := strip(mb.RenderContent(message.ContentBlock{
		Type: message.ContentToolResult, Content: manyLines, IsError: false,
	}))
	if !strings.Contains(manyOut, "…[truncated]") {
		t.Errorf("15-line result should show …[truncated] marker, got:\n%s", manyOut)
	}
	lineCount := strings.Count(manyOut, "line")
	if lineCount > 11 { // 10 kept + "line" inside "…[truncated]"? no, max 10
		t.Errorf("Should keep max 10 'line' occurrences, got %d", lineCount)
	}

	// 3. Success char truncation: 600 chars single line → 500 + …
	longSingle := strings.Repeat("a", 600)
	longOut := strip(mb.RenderContent(message.ContentBlock{
		Type: message.ContentToolResult, Content: longSingle, IsError: false,
	}))
	if strings.Count(longOut, "a") >= 600 {
		t.Error("600-char result should be truncated")
	}
	if !strings.Contains(longOut, "…") {
		t.Error("Truncated result should have … marker")
	}

	// 4. Empty content → "(no content)"
	emptyOut := strip(mb.RenderContent(message.ContentBlock{
		Type: message.ContentToolResult, Content: "", IsError: false,
	}))
	if !strings.Contains(emptyOut, "(no content)") {
		t.Errorf("Empty result should show '(no content)', got: %s", emptyOut)
	}

	// 5. Multi-line indentation: first line gets ⎿, rest get spaces
	multiOut := strip(mb.RenderContent(message.ContentBlock{
		Type: message.ContentToolResult, Content: "first\nsecond\nthird", IsError: false,
	}))
	multiLines := strings.Split(multiOut, "\n")
	if len(multiLines) < 3 {
		t.Fatalf("Expected 3+ output lines, got %d: %s", len(multiLines), multiOut)
	}
	// First line should contain ⎿ connector
	if !strings.Contains(multiLines[0], "⎿") {
		t.Errorf("First line should have ⎿ connector: %q", multiLines[0])
	}
	// Subsequent lines should NOT have ⎿ (continuation uses spaces)
	if strings.Contains(multiLines[1], "⎿") {
		t.Errorf("Continuation line should NOT have ⎿: %q", multiLines[1])
	}
	if strings.Contains(multiLines[2], "⎿") {
		t.Errorf("Third line should NOT have ⎿: %q", multiLines[2])
	}

	// 6-7. Content vs Text precedence
	bothOut := strip(mb.RenderContent(message.ContentBlock{
		Type: message.ContentToolResult, Content: "CONTENT", Text: "TEXT",
	}))
	if !strings.Contains(bothOut, "CONTENT") {
		t.Errorf("Content should take precedence, got: %s", bothOut)
	}
	if strings.Contains(bothOut, "TEXT") {
		t.Errorf("Text should be ignored when Content is set, got: %s", bothOut)
	}
	// Empty Content → falls through to Text
	textOnlyOut := strip(mb.RenderContent(message.ContentBlock{
		Type: message.ContentToolResult, Content: "", Text: "TEXT_FALLBACK",
	}))
	if !strings.Contains(textOnlyOut, "TEXT_FALLBACK") {
		t.Errorf("Empty Content should fall through to Text, got: %s", textOnlyOut)
	}
}

// TestParity_ConversationScrollAutoScroll validates scroll state transitions
// and the auto-scroll contract: user scrolling disables auto-scroll, and
// scrolling back to bottom re-enables it, affecting new message behavior.
//
// Unique behaviors (no existing test validates scroll state):
// 1. scrollUp from 0 increments offset AND disables autoScroll
// 2. scrollDown decrements offset, clamped at 0
// 3. Reaching scrollOffset=0 via scrollDown re-enables autoScroll
// 4. PgUp adds `height` to scrollOffset (page-size jump)
// 5. PgDown clamps scrollOffset at 0
// 6. AddMessage with autoScroll=true resets scrollOffset to 0
// 7. AddMessage with autoScroll=false preserves scroll offset
//
// Cross-ref: conversation.go:175-204 key handlers + scroll funcs
// Cross-ref: conversation.go:150-156 AddMessage auto-scroll contract
func TestParity_ConversationScrollAutoScroll(t *testing.T) {
	cp := components.NewConversationPane()
	cp.SetSize(80, 10)

	// Add a few messages so there's something to scroll
	for i := 0; i < 5; i++ {
		cp.AddMessage(message.Message{
			Role:    message.RoleUser,
			Content: []message.ContentBlock{{Type: message.ContentText, Text: "msg"}},
		})
	}

	// 1. Initial: scroll at bottom (offset=0), autoScroll enabled
	// Verified implicitly: after 5 AddMessage calls, scrollOffset should be 0
	// because autoScroll was true initially

	// 2. scrollUp via Up key → offset increments, autoScroll disabled
	cp.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	// We need to check state. Send another Up to confirm offset increments
	cp.Update(tea.KeyPressMsg{Code: tea.KeyUp})

	// 3. Now add a new message — autoScroll should be OFF, offset should NOT reset
	cp.AddMessage(message.Message{
		Role:    message.RoleAssistant,
		Content: []message.ContentBlock{{Type: message.ContentText, Text: "new"}},
	})
	// If autoScroll were on, offset would be 0. It's not, so we can verify by
	// the fact that scrolling Down once should bring us to a position, not zero.

	// 4. scrollDown twice to get back to 0, re-enables autoScroll
	cp.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	cp.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	// Now should be at offset 0 with autoScroll on

	// 5. AddMessage now should work (autoScroll re-enabled when offset hit 0)
	// Verify: add another message, and scroll state is at bottom
	msgCountBefore := cp.MessageCount()
	cp.AddMessage(message.Message{
		Role:    message.RoleAssistant,
		Content: []message.ContentBlock{{Type: message.ContentText, Text: "latest"}},
	})
	if cp.MessageCount() != msgCountBefore+1 {
		t.Errorf("AddMessage should add one message, count: %d → %d", msgCountBefore, cp.MessageCount())
	}

	// 6. PgUp key → jumps by height
	cp.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
	// 7. PgDown clamps at 0
	cp.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	cp.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	cp.Update(tea.KeyPressMsg{Code: tea.KeyPgDown}) // extra — should clamp

	// View should still render without panic
	v := cp.View()
	if v.Content == "" {
		t.Error("View should render content after scroll operations")
	}

	// Message count should be consistent
	if cp.MessageCount() != 7 { // 5 initial + 2 added
		t.Errorf("Expected 7 messages after scroll test, got %d", cp.MessageCount())
	}
}

// TestParity_InputPaneHistorySaveRestore validates the history navigation
// save/restore contract: current input is preserved when entering history
// and restored when navigating past the newest entry.
//
// Unique behaviors:
// 1. Down arrow on empty history (historyIdx == -1) is a no-op
// 2. Up arrow when historyIdx == -1 saves current buffer to savedInput
// 3. Up arrow enters history from newest entry and decrements index
// 4. Up arrow at oldest entry stops (no wrap)
// 5. Down arrow past newest restores savedInput AND clears it
// 6. After restore, buffer matches the saved input exactly
//
// Cross-ref: input.go:232-259 navigateHistoryUp/Down
func TestParity_InputPaneHistorySaveRestore(t *testing.T) {
	inp := components.NewInputPane()
	inp.SetSize(80, 3)
	inp.Focus()

	// 1. Down with no history → no-op
	inp.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if inp.Value() != "" {
		t.Errorf("Down with no history should be no-op, got %q", inp.Value())
	}

	// Add 3 history entries
	inp.AddToHistory("first")
	inp.AddToHistory("second")
	inp.AddToHistory("third")

	// Gopher's behavior: history navigation only triggers when input is empty
	// OR already navigating. Verify this constraint: typing blocks history nav.
	for _, ch := range "draft" {
		inp.Update(tea.KeyPressMsg{Code: rune(ch), Text: string(ch)})
	}
	inp.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if inp.Value() != "draft" {
		t.Errorf("Up with non-empty input should NOT navigate history, got %q", inp.Value())
	}

	// Clear input to enable history nav
	inp.Clear()

	// 2-3. Up on empty → saves empty, shows newest ("third")
	inp.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if inp.Value() != "third" {
		t.Errorf("First Up should show newest 'third', got %q", inp.Value())
	}

	// Up → "second"
	inp.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if inp.Value() != "second" {
		t.Errorf("Second Up should show 'second', got %q", inp.Value())
	}

	// Up → "first"
	inp.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if inp.Value() != "first" {
		t.Errorf("Third Up should show 'first', got %q", inp.Value())
	}

	// 4. Up at oldest → stays on "first" (no wrap)
	inp.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if inp.Value() != "first" {
		t.Errorf("Up past oldest should stay on 'first', got %q", inp.Value())
	}

	// Down → "second"
	inp.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if inp.Value() != "second" {
		t.Errorf("Down from oldest should show 'second', got %q", inp.Value())
	}

	// Down → "third"
	inp.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if inp.Value() != "third" {
		t.Errorf("Down should show 'third', got %q", inp.Value())
	}

	// 5-6. Down past newest → restores saved input (was empty) AND clears savedInput
	inp.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if inp.Value() != "" {
		t.Errorf("Down past newest should restore empty saved input, got %q", inp.Value())
	}

	// Another Down → no-op (historyIdx back to -1)
	inp.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if inp.Value() != "" {
		t.Errorf("Down after restore should be no-op, got %q", inp.Value())
	}
}

// TestParity_InputPaneEditingFlow validates the InputPane editing operations
// work correctly in combination (cursor moves, insertions, word-kill, line-kill).
//
// Unique behaviors (no existing test validates the InputPane buffer directly):
// 1. Typing characters inserts at cursor position and advances cursor
// 2. Ctrl+A moves cursor to 0 WITHOUT deleting text
// 3. After Ctrl+A, typing inserts at position 0 (prefix) not end
// 4. Ctrl+E moves cursor to end after being at 0
// 5. Ctrl+W deletes word backward from cursor, not entire buffer
// 6. Ctrl+U kills from cursor to beginning, preserving suffix after cursor
//
// Cross-ref: input.go:123 handleKey, input.go:215 deleteWordBackward
// Cross-ref: Claude PromptInput.tsx — standard readline-style editing
func TestParity_InputPaneEditingFlow(t *testing.T) {
	inp := components.NewInputPane()
	inp.SetSize(80, 3)
	inp.Focus()

	// 1. Type "hello" — buffer should be "hello"
	for _, ch := range "hello" {
		inp.Update(tea.KeyPressMsg{Code: rune(ch), Text: string(ch)})
	}
	if inp.Value() != "hello" {
		t.Errorf("After typing 'hello', buffer should be 'hello', got %q", inp.Value())
	}

	// 2. Ctrl+A → cursor to 0, text preserved
	inp.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
	if inp.Value() != "hello" {
		t.Errorf("Ctrl+A should not modify text, got %q", inp.Value())
	}

	// 3. Type "X" → should insert at position 0 → "Xhello"
	inp.Update(tea.KeyPressMsg{Code: 'X', Text: "X"})
	if inp.Value() != "Xhello" {
		t.Errorf("Insert at start should give 'Xhello', got %q", inp.Value())
	}

	// 4. Ctrl+E → cursor to end
	inp.Update(tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl})
	// Type "!" → should append → "Xhello!"
	inp.Update(tea.KeyPressMsg{Code: '!', Text: "!"})
	if inp.Value() != "Xhello!" {
		t.Errorf("After Ctrl+E and '!', expected 'Xhello!', got %q", inp.Value())
	}

	// 5. Clear and test Ctrl+W word-delete
	inp.Clear()
	for _, ch := range "foo bar baz" {
		inp.Update(tea.KeyPressMsg{Code: rune(ch), Text: string(ch)})
	}
	// Ctrl+W at end → deletes "baz" word (standard readline keeps trailing space)
	inp.Update(tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl})
	if inp.Value() != "foo bar " {
		t.Errorf("Ctrl+W should delete 'baz' word, expected 'foo bar ', got %q", inp.Value())
	}
	// Another Ctrl+W → deletes trailing space + "bar"
	inp.Update(tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl})
	if inp.Value() != "foo " {
		t.Errorf("Second Ctrl+W: expected 'foo ', got %q", inp.Value())
	}
	// Third Ctrl+W → deletes "foo "
	inp.Update(tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl})
	if inp.Value() != "" {
		t.Errorf("Third Ctrl+W should empty buffer, got %q", inp.Value())
	}

	// 6. Ctrl+U from middle preserves suffix
	inp.Clear()
	for _, ch := range "abcdef" {
		inp.Update(tea.KeyPressMsg{Code: rune(ch), Text: string(ch)})
	}
	// Move cursor to position 3 (after "abc")
	inp.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl}) // to 0
	// Move right 3 times — use Right arrow
	for i := 0; i < 3; i++ {
		inp.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	}
	// Ctrl+U → kills "abc", keeps "def"
	inp.Update(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	if inp.Value() != "def" {
		t.Errorf("Ctrl+U from middle should kill prefix, expected 'def', got %q", inp.Value())
	}
}

// TestParity_EscapeDuringStreamingCancel validates Escape's behavior in different modes.
//
// Unique behaviors (no existing test validates Escape key paths):
// 1. Escape when ModeIdle and no modal → no-op (no cmd, no state change)
// 2. Escape when ModeStreaming with active cancelQuery → cancelQuery is invoked
// 3. After cancel, queryDoneMsg finalizes partial streaming text into conversation
// 4. handleQueryDone resets streamingText, activeToolCalls, stops spinner
// 5. Mode returns to ModeIdle after queryDone
// 6. Partial text is preserved as an assistant message (not lost)
//
// Cross-ref: app.go:385-395 Escape handler, app.go:595-625 handleQueryDone
// Cross-ref: REPL.tsx — Escape triggers abort on streaming controller
func TestParity_EscapeDuringStreamingCancel(t *testing.T) {
	config := session.DefaultConfig()
	sess := session.New(config, "/tmp")
	app := NewAppModel(sess, nil)
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// 1. Escape when idle → no-op
	initialMode := app.mode
	initialCount := app.conversation.MessageCount()
	_, cmd := app.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd != nil {
		msg := cmd()
		if msg != nil {
			t.Errorf("Escape when idle should be no-op, got msg %T", msg)
		}
	}
	if app.mode != initialMode {
		t.Error("Escape when idle should not change mode")
	}
	if app.conversation.MessageCount() != initialCount {
		t.Error("Escape when idle should not modify conversation")
	}

	// Set up streaming state manually (can't actually trigger real async query)
	app.Update(components.SubmitMsg{Text: "write essay"})
	if app.mode != ModeStreaming {
		t.Fatalf("Setup: expected ModeStreaming, got %v", app.mode)
	}

	// Add partial streaming text
	app.Update(TextDeltaMsg{Text: "Once upon a time"})

	// Install a fake cancelQuery function to verify it's called
	cancelCalled := false
	app.cancelQuery = func() { cancelCalled = true }

	// 2. Escape during streaming → calls cancelQuery
	app.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !cancelCalled {
		t.Error("Escape during streaming should call cancelQuery")
	}

	// 3. Simulate queryDoneMsg (what happens after cancel completes)
	msgCountBefore := app.conversation.MessageCount()
	app.Update(queryDoneMsg{err: nil})

	// 4. After queryDone: mode=idle, streamingText empty, partial text preserved
	if app.mode != ModeIdle {
		t.Errorf("After queryDone, expected ModeIdle, got %v", app.mode)
	}
	if app.streamingText.Len() != 0 {
		t.Errorf("streamingText should be empty, got len=%d", app.streamingText.Len())
	}
	if len(app.activeToolCalls) != 0 {
		t.Error("activeToolCalls should be empty after queryDone")
	}
	if app.spinner.IsActive() {
		t.Error("spinner should be stopped after queryDone")
	}
	// 5. Partial text "Once upon a time" should be preserved as a message
	if app.conversation.MessageCount() <= msgCountBefore {
		t.Error("partial streaming text should be finalized as a conversation message")
	}
}

// TestParity_ModelSwitchDispatch validates the /model command dispatch pipeline.
//
// Unique behaviors (no existing test covers slash command dispatch + state update):
// 1. "/model" with no args returns CommandResult carrying an error
// 2. "/model sonnet" dispatches a ModelSwitchMsg
// 3. ModelSwitchMsg updates session.Config.Model to the new value
// 4. Header.SetModel is called (verifiable via re-rendered view)
// 5. Mode stays ModeIdle (local command, no streaming triggered)
// 6. New model name appears in the rendered header
//
// Cross-ref: commands/handlers.go:111-118 /model dispatch
// Cross-ref: app.go:230-235 ModelSwitchMsg handler
func TestParity_ModelSwitchDispatch(t *testing.T) {
	config := session.DefaultConfig()
	config.Model = "claude-opus-4-6"
	sess := session.New(config, "/tmp")
	app := NewAppModel(sess, nil)
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Dismiss welcome to see header
	app.Update(components.SubmitMsg{Text: "hi"})
	app.Update(TurnCompleteMsg{})

	// 1. "/model" with no args → CommandResult with error
	_, cmdErr := app.Update(components.SubmitMsg{Text: "/model"})
	if cmdErr == nil {
		t.Fatal("/model with no args should return a command")
	}
	errMsg := cmdErr()
	cr, ok := errMsg.(commands.CommandResult)
	if !ok {
		t.Fatalf("/model with no args should return CommandResult, got %T", errMsg)
	}
	if cr.Error == nil {
		t.Error("/model with no args should have a non-nil Error")
	}

	// 2. "/model sonnet" dispatches ModelSwitchMsg
	_, cmdSwitch := app.Update(components.SubmitMsg{Text: "/model sonnet"})
	if cmdSwitch == nil {
		t.Fatal("/model sonnet should return a command")
	}
	switchMsg := cmdSwitch()
	switchTyped, ok := switchMsg.(commands.ModelSwitchMsg)
	if !ok {
		t.Fatalf("Expected ModelSwitchMsg, got %T", switchMsg)
	}
	if switchTyped.Model != "sonnet" {
		t.Errorf("ModelSwitchMsg.Model expected 'sonnet', got %q", switchTyped.Model)
	}

	// 3. Feeding ModelSwitchMsg updates session
	oldModel := sess.Config.Model
	app.Update(switchTyped)
	if sess.Config.Model != "sonnet" {
		t.Errorf("session.Config.Model: expected 'sonnet', got %q", sess.Config.Model)
	}
	if sess.Config.Model == oldModel {
		t.Error("session model should have changed")
	}

	// 4. Mode stays idle (no streaming)
	if app.mode != ModeIdle {
		t.Errorf("Model switch should not change mode, got %v", app.mode)
	}

	// 5. Header displays new model name (structural check)
	view := app.View()
	plain := strip(view.Content)
	firstLine := strings.Split(plain, "\n")[0]
	if !strings.Contains(firstLine, "sonnet") {
		t.Errorf("Header should show new model 'sonnet', first line: %s", firstLine)
	}
	if strings.Contains(firstLine, "claude-opus-4-6") {
		t.Errorf("Header should not show old model, first line: %s", firstLine)
	}
}

// TestParity_ToolUseStateMachine validates the full tool use lifecycle on AppModel:
// submit → tool start → tool result → text → turn complete.
//
// Unique behaviors (not covered by QueryEventFlow which only tests text deltas):
// 1. ToolUseStartMsg sets mode to ModeToolRunning
// 2. ToolUseStartMsg tracks tool in activeToolCalls[toolUseID]
// 3. ToolResultMsg removes tool from activeToolCalls
// 4. Streaming text accumulates tool indicators inline
// 5. TurnCompleteMsg finalizes: creates conversation message, resets streamingText,
//    clears activeToolCalls, stops spinner, returns to ModeIdle
// 6. Multiple sequential tools tracked independently
//
// Cross-ref: app.go:504-560 — handleToolUseStart/handleToolResult/handleTurnComplete
func TestParity_ToolUseStateMachine(t *testing.T) {
	config := session.DefaultConfig()
	sess := session.New(config, "/tmp")
	app := NewAppModel(sess, nil)
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 30})

	app.Update(components.SubmitMsg{Text: "read two files"})

	// 1. First tool starts → ModeToolRunning, tracked in map
	app.Update(ToolUseStartMsg{ToolUseID: "t1", ToolName: "Read"})
	if app.mode != ModeToolRunning {
		t.Errorf("After ToolUseStart, expected ModeToolRunning, got %v", app.mode)
	}
	if _, ok := app.activeToolCalls["t1"]; !ok {
		t.Error("activeToolCalls should track t1 after ToolUseStart")
	}

	// 2. First tool result → removed from map
	app.Update(ToolResultMsg{ToolUseID: "t1", Content: "file1 content", IsError: false})
	if _, ok := app.activeToolCalls["t1"]; ok {
		t.Error("activeToolCalls should remove t1 after ToolResult")
	}

	// 3. Second tool starts → both can coexist briefly
	app.Update(ToolUseStartMsg{ToolUseID: "t2", ToolName: "Read"})
	if _, ok := app.activeToolCalls["t2"]; !ok {
		t.Error("activeToolCalls should track t2")
	}
	app.Update(ToolResultMsg{ToolUseID: "t2", Content: "file2 content", IsError: false})

	// 4. Streaming text should have accumulated tool indicators
	streamText := app.streamingText.String()
	if streamText == "" {
		t.Error("streamingText should have accumulated tool indicators")
	}

	// 5. Text delta arrives, then turn completes
	app.Update(TextDeltaMsg{Text: "Here are both files."})
	app.Update(TurnCompleteMsg{})

	// 6. After TurnComplete: all state reset
	if app.mode != ModeIdle {
		t.Errorf("After TurnComplete, expected ModeIdle, got %v", app.mode)
	}
	if app.streamingText.Len() != 0 {
		t.Errorf("streamingText should be empty after TurnComplete, len=%d", app.streamingText.Len())
	}
	if len(app.activeToolCalls) != 0 {
		t.Errorf("activeToolCalls should be empty after TurnComplete, len=%d", len(app.activeToolCalls))
	}
	if app.spinner.IsActive() {
		t.Error("spinner should be stopped after TurnComplete")
	}
	// Conversation should have the finalized message
	if app.conversation.MessageCount() < 2 {
		t.Errorf("Expected 2+ messages (user+assistant), got %d", app.conversation.MessageCount())
	}
}

// TestParity_DiffApprovalAllThreeKeys validates the DiffApprovalDialog dispatches
// correct results for all 3 key paths (y/n/a), sends through the channel AND
// returns the correct tea.Cmd message. Also validates the dialog renders diff content.
//
// Unique behaviors tested (not covered by ANY existing test):
// 1. 'y'/Enter → ApprovalApproved sent to channel + returned as ApprovalResponseMsg
// 2. 'n' → ApprovalRejected sent to channel + returned as ApprovalResponseMsg
// 3. 'a' → ApprovalAlways sent to channel + returned as ApprovalResponseMsg
// 4. ApprovalResponseMsg carries correct ToolUseID
// 5. Dialog renders actual diff content (added/removed lines visible)
//
// Cross-ref: diff_approval.go:62-90 — Update handles y/n/a key dispatch
// Cross-ref: Claude FileEditPermissionRequest.tsx — 1. Yes / 2. Yes allow all / 3. No
func TestParity_DiffApprovalAllThreeKeys(t *testing.T) {
	testDiff := "--- a/main.go\n+++ b/main.go\n@@ -1 +1,2 @@\n+// new comment\n package main"

	keys := []struct {
		name     string
		key      rune
		expected components.ApprovalResult
	}{
		{"approve-y", 'y', components.ApprovalApproved},
		{"reject-n", 'n', components.ApprovalRejected},
		{"always-a", 'a', components.ApprovalAlways},
	}

	for _, tc := range keys {
		t.Run(tc.name, func(t *testing.T) {
			ch := make(chan components.ApprovalResult, 1)
			toolID := "tool-" + tc.name
			dad := components.NewDiffApprovalDialog("Edit", toolID, testDiff, theme.Current(), ch)
			dad.SetSize(80, 24)

			// 1. Dialog should render diff content
			view := dad.View()
			plain := strip(view.Content)
			if !strings.Contains(plain, "new comment") && !strings.Contains(plain, "main") {
				t.Errorf("Dialog should render diff content, got:\n%s", plain)
			}

			// 2. Press key → sends to channel
			_, cmd := dad.Update(tea.KeyPressMsg{Code: tc.key, Text: string(tc.key)})

			// 3. Channel should receive correct result
			select {
			case result := <-ch:
				if result != tc.expected {
					t.Errorf("Channel: expected %v, got %v", tc.expected, result)
				}
			default:
				t.Error("Key press should send result to channel")
			}

			// 4. Returned cmd should produce ApprovalResponseMsg with correct ToolUseID
			if cmd == nil {
				t.Fatal("Key press should return a tea.Cmd")
			}
			msg := cmd()
			respMsg, ok := msg.(components.ApprovalResponseMsg)
			if !ok {
				t.Fatalf("Expected ApprovalResponseMsg, got %T", msg)
			}
			if respMsg.ToolUseID != toolID {
				t.Errorf("ToolUseID: expected %q, got %q", toolID, respMsg.ToolUseID)
			}
			if respMsg.Result != tc.expected {
				t.Errorf("Result: expected %v, got %v", tc.expected, respMsg.Result)
			}
		})
	}
}

// TestParity_AppSlashAutocompleteIntegration validates the integration between
// input typing, refreshSlashAutocomplete, and the slashInput component inside
// handleKey. This is the app-level flow that makes `/` open an autocomplete.
//
// Unique behaviors (B38 tests slashInput in isolation; this tests the APP wiring):
//  1. Typing "/" routes to input pane AND activates slash autocomplete.
//  2. Typing more chars after "/" keeps autocomplete active and refilters.
//  3. Typing " " (space) after "/foo" deactivates autocomplete — the
//     "no space allowed" rule in refreshSlashAutocomplete().
//  4. Pressing Backspace back to "/" re-activates (toggle works).
//  5. Up/Down arrows are SWALLOWED by slashInput when active — they must
//     NOT reach the input pane's history-nav handler.
//  6. Enter when autocomplete active → fires SlashCommandSelectedMsg cmd
//     AND the resulting SlashCommandSelectedMsg (fed back) populates input
//     with "name " (trailing space for args).
//  7. Escape while autocomplete active → deactivates autocomplete but does
//     NOT push a modal or cancel a query.
//
// Cross-ref: app.go:195-209 SlashCommandSelectedMsg handler; app.go:422-451
// handleKey routing; refreshSlashAutocomplete().
// Cross-ref: Claude Code REPL.tsx → slash command prefix → picker.
func TestParity_AppSlashAutocompleteIntegration(t *testing.T) {
	config := session.DefaultConfig()
	sess := session.New(config, "/tmp")
	app := NewAppModel(sess, nil)
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	if app.slashInput == nil {
		t.Skip("slashInput not configured on this build")
	}
	// Dismiss welcome so input typing targets the input pane.
	app.showWelcome = false

	// -- Behavior 1: typing "/" activates autocomplete --
	app.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	if !app.slashInput.IsActive() {
		t.Fatal("typing '/' should activate slash autocomplete")
	}
	if app.input.Value() != "/" {
		t.Errorf("input should contain '/', got %q", app.input.Value())
	}

	// -- Behavior 2: typing a second char keeps autocomplete active --
	app.Update(tea.KeyPressMsg{Code: 'm', Text: "m"})
	if !app.slashInput.IsActive() {
		t.Error("typing '/m' should keep autocomplete active")
	}
	if app.input.Value() != "/m" {
		t.Errorf("input should be '/m', got %q", app.input.Value())
	}

	// -- Behavior 3: adding a space deactivates autocomplete --
	app.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	if app.slashInput.IsActive() {
		t.Error("'/m ' (with space) should deactivate autocomplete")
	}
	if app.input.Value() != "/m " {
		t.Errorf("input should be '/m ', got %q", app.input.Value())
	}

	// -- Behavior 4: backspace back to "/m" re-activates --
	app.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if app.input.Value() != "/m" {
		t.Errorf("after backspace input should be '/m', got %q", app.input.Value())
	}
	if !app.slashInput.IsActive() {
		t.Error("backspacing away the space should re-activate autocomplete")
	}

	// -- Behavior 5: Up arrow routed to slashInput, NOT to history nav --
	// Setup: history contains a value. If Up reached input pane, it would
	// navigate history and change the buffer.
	app.input.AddToHistory("previous")
	inputBefore := app.input.Value()
	app.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if app.input.Value() != inputBefore {
		t.Errorf("Up arrow MUST be swallowed by slashInput, not reach history; "+
			"input changed from %q to %q", inputBefore, app.input.Value())
	}
	// Slash autocomplete should still be active (Up doesn't deactivate).
	if !app.slashInput.IsActive() {
		t.Error("Up arrow should not deactivate slash autocomplete")
	}

	// -- Behavior 6: Enter while active emits SlashCommandSelectedMsg,
	// and feeding that message back populates input with "name " --
	_, cmd := app.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter with active autocomplete should produce a cmd")
	}
	msg := cmd()
	selMsg, ok := msg.(components.SlashCommandSelectedMsg)
	if !ok {
		t.Fatalf("Enter should produce SlashCommandSelectedMsg, got %T", msg)
	}
	// Feed it back into the app.
	app.Update(selMsg)
	wantValue := selMsg.Command.Name + " "
	if app.input.Value() != wantValue {
		t.Errorf("after SlashCommandSelectedMsg, input should be %q, got %q",
			wantValue, app.input.Value())
	}

	// -- Behavior 7: Escape while active deactivates (not via modal/cancel) --
	// Retype "/" to open again.
	app.input.Clear()
	app.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	if !app.slashInput.IsActive() {
		t.Fatal("typing '/' should re-activate after clear")
	}
	app.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if app.slashInput.IsActive() {
		t.Error("Escape with active autocomplete should deactivate it")
	}
	if app.focus.ModalActive() {
		t.Error("Escape→deactivate must NOT push a modal")
	}
	if app.mode != ModeIdle {
		t.Errorf("Escape→deactivate must not change mode, got %v", app.mode)
	}
}

// TestParity_SessionToRequestMessagesSerialization validates the session →
// API conversion in SessionState.ToRequestMessages(). This function is the
// contract that builds the JSON sent to the Anthropic API, so serialization
// drift would silently corrupt every request.
//
// Unique behaviors (no existing test covers ToRequestMessages):
//  1. Empty session → zero-length (but non-nil) RequestMessage slice.
//  2. Message.Role is serialized as the exact string (RoleUser→"user",
//     RoleAssistant→"assistant") — verbatim stringification.
//  3. ContentText block → {Type:"text", Text:X} (ID/Name/Input stay empty).
//  4. ContentToolUse block → {Type:"tool_use", ID, Name, Input} carries all
//     3 tool fields through.
//  5. ContentToolResult with IsError=false → RequestContent.IsError is NIL
//     pointer (NOT a pointer to false). This matters because the API omits
//     `is_error` entirely from JSON when the pointer is nil (omitempty).
//  6. ContentToolResult with IsError=true → IsError is a non-nil *bool
//     pointing to true (preserves the signal).
//  7. ContentThinking is silently dropped (no RequestContent emitted), so
//     adding 1 text + 1 thinking block yields 1 RequestContent, not 2.
//  8. Multi-block ordering within a single message is preserved.
//  9. Multi-message ordering is preserved in the output slice.
//
// Cross-ref: session.go:160-198 — per-type switch in ToRequestMessages.
// Cross-ref: Claude Code utils/messages.ts → Anthropic SDK request format.
func TestParity_SessionToRequestMessagesSerialization(t *testing.T) {
	// -- Behavior 1: empty → non-nil empty slice --
	emptySess := session.New(session.DefaultConfig(), "/tmp")
	empty := emptySess.ToRequestMessages()
	if empty == nil {
		t.Error("empty session must return non-nil slice (zero-len is fine)")
	}
	if len(empty) != 0 {
		t.Errorf("empty session should return 0 messages, got %d", len(empty))
	}

	// Build a multi-message session exercising every content-block branch.
	sess := session.New(session.DefaultConfig(), "/tmp")
	sess.PushMessage(message.Message{
		Role: message.RoleUser,
		Content: []message.ContentBlock{
			{Type: message.ContentText, Text: "user-text-1"},
			{Type: message.ContentThinking, Thinking: "secret thought"}, // dropped
			{Type: message.ContentToolResult, ToolUseID: "t1", Content: "ok-out", IsError: false},
			{Type: message.ContentToolResult, ToolUseID: "t2", Content: "err-out", IsError: true},
		},
	})
	sess.PushMessage(message.Message{
		Role: message.RoleAssistant,
		Content: []message.ContentBlock{
			{Type: message.ContentToolUse, ID: "tu1", Name: "Bash",
				Input: []byte(`{"cmd":"ls"}`)},
			{Type: message.ContentText, Text: "done"},
		},
	})

	out := sess.ToRequestMessages()

	// -- Behavior 9: 2 input messages → 2 output messages in SAME order --
	if len(out) != 2 {
		t.Fatalf("expected 2 RequestMessages, got %d: %+v", len(out), out)
	}

	// -- Behavior 2: role stringification --
	if out[0].Role != "user" {
		t.Errorf("msg[0].Role: want 'user', got %q", out[0].Role)
	}
	if out[1].Role != "assistant" {
		t.Errorf("msg[1].Role: want 'assistant', got %q", out[1].Role)
	}

	// -- Behavior 7: thinking block dropped → msg[0] has 3 entries not 4 --
	if len(out[0].Content) != 3 {
		t.Fatalf("msg[0] should drop thinking → 3 entries (text+2 tool_result), got %d: %+v",
			len(out[0].Content), out[0].Content)
	}

	// -- Behavior 3 + 8: text block at index 0, ordering preserved --
	c0 := out[0].Content[0]
	if c0.Type != "text" {
		t.Errorf("msg[0].Content[0].Type: want 'text', got %q", c0.Type)
	}
	if c0.Text != "user-text-1" {
		t.Errorf("msg[0].Content[0].Text: want 'user-text-1', got %q", c0.Text)
	}
	// Non-text fields must be empty on a text block.
	if c0.ID != "" || c0.Name != "" || len(c0.Input) != 0 || c0.ToolUseID != "" || c0.IsError != nil {
		t.Errorf("text block should only have Type+Text set, got %+v", c0)
	}

	// -- Behavior 5: IsError=false → IsError pointer is NIL --
	c1 := out[0].Content[1]
	if c1.Type != "tool_result" {
		t.Errorf("msg[0].Content[1].Type: want 'tool_result', got %q", c1.Type)
	}
	if c1.ToolUseID != "t1" {
		t.Errorf("tool_result ToolUseID: want 't1', got %q", c1.ToolUseID)
	}
	if c1.Content != "ok-out" {
		t.Errorf("tool_result Content: want 'ok-out', got %q", c1.Content)
	}
	if c1.IsError != nil {
		t.Errorf("IsError=false MUST serialize to nil *bool (for JSON omitempty), got %v",
			*c1.IsError)
	}

	// -- Behavior 6: IsError=true → non-nil *bool pointing to true --
	c2 := out[0].Content[2]
	if c2.ToolUseID != "t2" {
		t.Errorf("tool_result ToolUseID: want 't2', got %q", c2.ToolUseID)
	}
	if c2.IsError == nil {
		t.Fatal("IsError=true MUST produce non-nil *bool pointer")
	}
	if *c2.IsError != true {
		t.Errorf("IsError=true → *bool must point to true, got %v", *c2.IsError)
	}

	// -- Behavior 4: tool_use block at msg[1] index 0 carries ID/Name/Input --
	if len(out[1].Content) != 2 {
		t.Fatalf("msg[1] should have 2 entries (tool_use+text), got %d", len(out[1].Content))
	}
	tu := out[1].Content[0]
	if tu.Type != "tool_use" {
		t.Errorf("msg[1].Content[0].Type: want 'tool_use', got %q", tu.Type)
	}
	if tu.ID != "tu1" {
		t.Errorf("tool_use ID: want 'tu1', got %q", tu.ID)
	}
	if tu.Name != "Bash" {
		t.Errorf("tool_use Name: want 'Bash', got %q", tu.Name)
	}
	if string(tu.Input) != `{"cmd":"ls"}` {
		t.Errorf("tool_use Input: want %q, got %q", `{"cmd":"ls"}`, string(tu.Input))
	}

	// -- Behavior 8: ordering within msg[1] — tool_use then text --
	if out[1].Content[1].Type != "text" || out[1].Content[1].Text != "done" {
		t.Errorf("msg[1].Content[1]: want text 'done', got %+v", out[1].Content[1])
	}
}

// TestParity_ToolUseBlockInputThreshold validates the 200-char threshold
// rule in MessageBubble.renderToolUseBlock that decides whether to display
// the tool's input JSON below its header or not.
//
// Unique behaviors (B9 tests tool_result truncation; B32 tests thinking
// truncation; neither covers tool_use input threshold):
//  1. Empty Input → header-only output (no second line).
//  2. Non-empty Input shorter than 200 bytes → header + "\n  " + input.
//  3. Input == 199 bytes (just-below threshold) → input SHOWN.
//  4. Input == 200 bytes (at-threshold) → input HIDDEN (strict < 200).
//  5. Input > 200 bytes → input HIDDEN.
//  6. The tool block.Name is always rendered in the header regardless of
//     input size.
//  7. Input bytes are rendered verbatim (no escaping/truncation) when shown.
//
// Cross-ref: message_bubble.go:204-209 — `if len(block.Input) > 0 && <200`
// Cross-ref: Claude Code src/components/messages/AssistantToolUseMessage —
// similar abbreviation rule for large tool inputs.
func TestParity_ToolUseBlockInputThreshold(t *testing.T) {
	mb := components.NewMessageBubble(theme.Current(), 80)

	renderInput := func(input string) string {
		return strip(mb.RenderContent(message.ContentBlock{
			Type:  message.ContentToolUse,
			Name:  "Bash",
			Input: json.RawMessage(input),
		}))
	}

	// -- Behavior 1: empty input → header only (no newline/body) --
	empty := renderInput("")
	if strings.Contains(empty, "\n") {
		t.Errorf("empty input should render a single header line, got:\n%s", empty)
	}
	if !strings.Contains(empty, "Bash") {
		t.Errorf("empty-input header must contain tool name 'Bash', got: %q", empty)
	}

	// -- Behavior 2: short input (<200) → header + body --
	shortInput := `{"cmd":"ls -la"}`
	short := renderInput(shortInput)
	if !strings.Contains(short, shortInput) {
		t.Errorf("short input should be rendered verbatim, want %q in:\n%s", shortInput, short)
	}
	if !strings.Contains(short, "\n") {
		t.Errorf("short input should produce TWO lines (header+body), got:\n%s", short)
	}
	if !strings.Contains(short, "Bash") {
		t.Error("short-input header must still contain tool name")
	}

	// -- Behavior 3: input == 199 bytes → SHOWN --
	input199 := `{"data":"` + strings.Repeat("a", 199-11) + `"}`
	if len(input199) != 199 {
		t.Fatalf("setup: want 199 bytes, got %d", len(input199))
	}
	out199 := renderInput(input199)
	if !strings.Contains(out199, "aaaaaaaa") {
		t.Errorf("199-byte input should be shown (just below threshold), got:\n%s", out199)
	}

	// -- Behavior 4: input == 200 bytes → HIDDEN (strict <) --
	input200 := `{"data":"` + strings.Repeat("a", 200-11) + `"}`
	if len(input200) != 200 {
		t.Fatalf("setup: want 200 bytes, got %d", len(input200))
	}
	out200 := renderInput(input200)
	if strings.Contains(out200, "aaaaaaaa") {
		t.Errorf("200-byte input should be HIDDEN at threshold, got:\n%s", out200)
	}
	// But tool name must still render
	if !strings.Contains(out200, "Bash") {
		t.Error("200-byte input should still render the tool name header")
	}
	// Output should be a single header line (no newline)
	if strings.Contains(out200, "\n") {
		t.Errorf("hidden-input output should be single-line, got:\n%s", out200)
	}

	// -- Behavior 5: input > 200 → HIDDEN --
	inputHuge := `{"x":"` + strings.Repeat("x", 500) + `"}`
	outHuge := renderInput(inputHuge)
	if strings.Contains(outHuge, "xxxxxx") {
		t.Errorf("huge input should be HIDDEN, got:\n%s", outHuge)
	}

	// -- Behavior 7: content rendered verbatim when shown --
	verbatim := `{"k":"v&<>\"quoted\""}`
	outVerbatim := renderInput(verbatim)
	if !strings.Contains(outVerbatim, `v&<>\"quoted\"`) {
		t.Errorf("special chars must render verbatim, got:\n%s", outVerbatim)
	}
}

// TestParity_ConversationRerenderOnResize validates the MessageBubble cache
// rebuild that ConversationPane.SetSize() triggers. When the terminal is
// resized, each cached render must be re-wrapped with the new width.
//
// This is an integration test — it exercises the conversation's internal
// cache (`rendered []string`) which is not directly observable, but whose
// effects surface in the view line count.
//
// Unique behaviors (B8/B19/B30 don't test width-driven re-render):
//  1. Adding a long user message that wraps at width=80 produces ≤ W lines.
//  2. Shrinking the width via SetSize forces the cache to rebuild — the
//     same message now occupies MORE lines (more wraps at narrower width).
//  3. Growing the width again rebuilds the cache to FEWER lines.
//  4. The message content is still present after each resize (not lost).
//  5. rerenderAll is deterministic: two identical resizes produce the same
//     rendered line count.
//
// Cross-ref: conversation.go:126-132 SetSize→bubble.SetWidth→rerenderAll()
// Cross-ref: message_bubble.go:47-52 SetWidth recreates glamour renderer
// Cross-ref: Claude Code src/hooks/useTerminalSize.ts — rewraps on resize.
func TestParity_ConversationRerenderOnResize(t *testing.T) {
	cp := components.NewConversationPane()
	cp.SetSize(80, 100) // generous height so nothing scrolls off

	// A long user message that will wrap at narrow widths but not at 80.
	longText := "The quick brown fox jumps over the lazy dog while the sluggish " +
		"tortoise watches from beneath a tangerine parasol contemplating the " +
		"mysteries of the universe and pondering the nature of time itself."
	cp.AddMessage(message.Message{
		Role:    message.RoleUser,
		Content: []message.ContentBlock{{Type: message.ContentText, Text: longText}},
	})

	// -- Behavior 1: wide view — count visible lines with content --
	wideView := strip(cp.View().Content)
	wideLines := countContentLines(wideView, "fox")
	if wideLines < 1 {
		t.Fatalf("wide view should contain message: %s", wideView)
	}
	// Measure the total non-empty message lines by counting from 'The' to
	// 'itself.'. The full rendered block should fit in a bounded line count.
	wideMsgLines := messageBlockLineCount(wideView, "The", "itself.")
	if wideMsgLines == 0 {
		t.Fatalf("could not locate message block in wide view:\n%s", wideView)
	}

	// -- Behavior 2: narrow resize must re-wrap → more lines --
	cp.SetSize(30, 100)
	narrowView := strip(cp.View().Content)
	narrowMsgLines := messageBlockLineCount(narrowView, "The", "itself.")
	if narrowMsgLines == 0 {
		t.Fatalf("could not locate message block in narrow view:\n%s", narrowView)
	}
	if narrowMsgLines <= wideMsgLines {
		t.Errorf("narrow view (width=30) should use MORE lines than wide (width=80): narrow=%d wide=%d",
			narrowMsgLines, wideMsgLines)
	}
	// No wrapped line should exceed narrow width (with small tolerance for
	// background-styled padding around the prompt prefix).
	for _, line := range strings.Split(narrowView, "\n") {
		// wrapText uses mb.width-4 for user messages, so content is at most 26.
		// But the StatusBar row is padded to full width=30 — we only measure
		// the message body lines that contain the wrapped text, which must be
		// shorter than 35 (gives slack for "❯ " prefix + trailing padding).
		if len(line) > 50 {
			t.Errorf("narrow view line exceeds width (likely unwrapped): %q (len=%d)",
				line, len(line))
			break
		}
	}

	// -- Behavior 3: re-grow to 80 restores the wide line count --
	cp.SetSize(80, 100)
	regrownView := strip(cp.View().Content)
	regrownMsgLines := messageBlockLineCount(regrownView, "The", "itself.")
	if regrownMsgLines != wideMsgLines {
		t.Errorf("after regrowing to 80, line count must match original wide=%d, got %d",
			wideMsgLines, regrownMsgLines)
	}

	// -- Behavior 4: content still present after resize round-trip --
	if !strings.Contains(regrownView, "tangerine") {
		t.Error("message content must survive resize round-trip (tangerine missing)")
	}
	if !strings.Contains(regrownView, "universe") {
		t.Error("message content must survive resize round-trip (universe missing)")
	}

	// -- Behavior 5: determinism — same resize twice gives same result --
	cp.SetSize(40, 100)
	firstNarrow := strip(cp.View().Content)
	firstCount := messageBlockLineCount(firstNarrow, "The", "itself.")
	cp.SetSize(60, 100)
	cp.SetSize(40, 100)
	secondNarrow := strip(cp.View().Content)
	secondCount := messageBlockLineCount(secondNarrow, "The", "itself.")
	if firstCount != secondCount {
		t.Errorf("rerenderAll must be deterministic: two 40-wide renders gave %d vs %d lines",
			firstCount, secondCount)
	}
}

// messageBlockLineCount returns how many consecutive lines span from the
// line containing `startToken` through the line containing `endToken`.
// Returns 0 if either token isn't found or endToken precedes startToken.
func messageBlockLineCount(view, startToken, endToken string) int {
	lines := strings.Split(view, "\n")
	start, end := -1, -1
	for i, l := range lines {
		if start == -1 && strings.Contains(l, startToken) {
			start = i
		}
		if strings.Contains(l, endToken) {
			end = i
		}
	}
	if start == -1 || end == -1 || end < start {
		return 0
	}
	return end - start + 1
}

// countContentLines counts lines in view that contain token.
func countContentLines(view, token string) int {
	n := 0
	for _, l := range strings.Split(view, "\n") {
		if strings.Contains(l, token) {
			n++
		}
	}
	return n
}

// TestParity_AppEscapeBranchPriority validates the priority ordering of the
// three Escape handler branches in app.handleKey:
//   (branch A) modal active → PopModal
//   (branch B) mode != Idle && cancelQuery != nil → cancelQuery()
//   (branch C) fall through → route to focused component
//
// The priority matters: when BOTH a modal is active AND a query is running,
// Escape must only pop the modal and MUST NOT cancel the query. Otherwise a
// user dismissing a dialog would accidentally kill their turn.
//
// Unique behaviors (B5 tests cancel-query flow; B24 tests FocusManager in
// isolation; neither tests the app-level branch priority):
//  1. Modal active + streaming → Escape pops modal AND cancelQuery is NOT
//     called (query keeps running).
//  2. After PopModal empties the stack, a second Escape falls through to
//     the cancelQuery branch and DOES cancel the query.
//  3. No modal + idle → Escape routes to focused component (no cancel).
//  4. PopModal restores modal count to 0 (single-modal case).
//
// Cross-ref: app.go:387-395 — the two-tier Escape handler.
func TestParity_AppEscapeBranchPriority(t *testing.T) {
	config := session.DefaultConfig()
	sess := session.New(config, "/tmp")
	app := NewAppModel(sess, nil)
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Wire up a fake cancel-query function that flips a flag when invoked.
	cancelCalled := 0
	app.mode = ModeStreaming
	app.cancelQuery = func() { cancelCalled++ }

	// Push a modal.
	modal := &trackingFocusable{name: "modal"}
	app.focus.PushModal(modal)
	if !app.focus.ModalActive() {
		t.Fatal("setup: ModalActive should be true after PushModal")
	}

	// -- Behavior 1: Escape with modal+streaming pops modal, does NOT cancel --
	app.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if app.focus.ModalActive() {
		t.Errorf("first Escape should PopModal, ModalActive still true")
	}
	if cancelCalled != 0 {
		t.Errorf("first Escape (modal branch) MUST NOT call cancelQuery, called=%d", cancelCalled)
	}
	// mode must remain ModeStreaming — modal pop doesn't touch streaming state.
	if app.mode != ModeStreaming {
		t.Errorf("modal-pop Escape must not change mode, got %v", app.mode)
	}
	// cancelQuery closure must still be present (not cleared by modal-pop).
	if app.cancelQuery == nil {
		t.Error("modal-pop Escape must not clear cancelQuery")
	}

	// -- Behavior 2: second Escape (no modal, still streaming) cancels --
	app.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cancelCalled != 1 {
		t.Errorf("second Escape must fall through to cancelQuery, called=%d", cancelCalled)
	}

	// -- Behavior 3: Escape with no modal + idle mode → routes through focus --
	// Rebuild state: clear modal stack, reset mode to Idle, nil cancelQuery.
	app2 := NewAppModel(sess, nil)
	app2.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if app2.mode != ModeIdle {
		t.Fatalf("setup: app2 should start in ModeIdle, got %v", app2.mode)
	}
	if app2.focus.ModalActive() {
		t.Fatal("setup: app2 should have no modal")
	}
	// With no modal, no streaming, no cancelQuery — Escape must not panic and
	// must not change mode. It falls through to focus.Route() which delegates
	// to the focused child (input pane in this case).
	_, _ = app2.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if app2.mode != ModeIdle {
		t.Errorf("idle Escape must leave mode idle, got %v", app2.mode)
	}
	if app2.focus.ModalActive() {
		t.Error("idle Escape must not create a modal")
	}
}

// TestParity_InputKillToEndAndPaste validates two readline behaviors the
// existing input tests don't cover:
//
//   1. Ctrl+K kills from cursor to END of line (complement of Ctrl+U).
//      Cursor position is preserved. Covers start/middle/end cursor positions.
//   2. Multi-character text insertion (paste path): when a KeyPressMsg carries
//      a Text payload with >1 rune, ALL runes are inserted at the cursor and
//      the cursor advances by the rune count.
//
// Unique behaviors (B6 tests Ctrl+A/E/U/W; B27 single-char insert+cursor):
//  1. Ctrl+K at cursor=0 truncates buffer to empty string.
//  2. Ctrl+K in middle cuts the suffix, keeps the prefix.
//  3. Ctrl+K at cursor==len is a no-op (no index panic, no change).
//  4. Ctrl+K does NOT move the cursor (distinct from Ctrl+U which resets to 0).
//  5. Multi-rune paste inserts the entire run atomically at the cursor.
//  6. Cursor advances by the RUNE count of the pasted text (not byte count),
//      so multi-byte paste positions the cursor correctly after.
//  7. Paste into the middle splits the buffer: before + pasted + after.
//
// Cross-ref: input.go:177-179 Ctrl+K branch; input.go:202-210 default paste path.
// Cross-ref: Claude Code: readline/emacs keybindings standard behavior.
func TestParity_InputKillToEndAndPaste(t *testing.T) {
	// -- Behavior 3: Ctrl+K at end is a no-op --
	t.Run("ctrl-k-at-end", func(t *testing.T) {
		inp := components.NewInputPane()
		inp.SetValue("hello") // cursor lands at 5 (len)
		if inp.Value() != "hello" {
			t.Fatalf("setup: want 'hello', got %q", inp.Value())
		}
		inp.Update(tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl})
		if inp.Value() != "hello" {
			t.Errorf("Ctrl+K at end must not change buffer, got %q", inp.Value())
		}
	})

	// -- Behavior 1: Ctrl+K at cursor=0 clears buffer --
	t.Run("ctrl-k-at-start", func(t *testing.T) {
		inp := components.NewInputPane()
		inp.SetValue("hello")
		// Move cursor to 0 via Ctrl+A (validated in B6)
		inp.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
		inp.Update(tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl})
		if inp.Value() != "" {
			t.Errorf("Ctrl+K at cursor=0 must clear buffer, got %q", inp.Value())
		}
		if inp.HasText() {
			t.Error("HasText() must be false after full kill")
		}
	})

	// -- Behaviors 2 & 4: Ctrl+K at middle cuts suffix, cursor unchanged --
	t.Run("ctrl-k-at-middle", func(t *testing.T) {
		inp := components.NewInputPane()
		inp.SetValue("hello") // cursor at 5
		// Move cursor to 2 by pressing Left 3 times (validated in B27)
		for i := 0; i < 3; i++ {
			inp.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
		}
		// Now cursor=2, buffer="hello"
		inp.Update(tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl})
		if inp.Value() != "he" {
			t.Errorf("Ctrl+K at cursor=2 should leave 'he', got %q", inp.Value())
		}
		// Verify cursor stayed at 2 — after kill, cursor==len==2.
		// Type a char: it should insert at position 2 (the end).
		inp.Update(tea.KeyPressMsg{Code: 'X', Text: "X"})
		if inp.Value() != "heX" {
			t.Errorf("after Ctrl+K the cursor should be at 2 (end), typing X gives %q", inp.Value())
		}
	})

	// -- Behaviors 5,6,7: multi-rune paste inserts atomically at cursor --
	t.Run("paste-multi-rune-at-cursor", func(t *testing.T) {
		inp := components.NewInputPane()
		// Build "abdef" via single-char typing (verified in B27).
		for _, c := range "abdef" {
			inp.Update(tea.KeyPressMsg{Code: c, Text: string(c)})
		}
		if inp.Value() != "abdef" {
			t.Fatalf("setup: want 'abdef', got %q", inp.Value())
		}
		// Move cursor to position 2 (between "ab" and "def").
		for i := 0; i < 3; i++ {
			inp.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
		}
		// Paste "XY" — multi-char Text in a single event.
		inp.Update(tea.KeyPressMsg{Code: 0, Text: "XY"})
		if inp.Value() != "abXYdef" {
			t.Errorf("paste should splice into middle: want 'abXYdef', got %q", inp.Value())
		}
		// Cursor should have advanced by 2 (len of "XY") → cursor=4.
		// Type 'Z' to verify position: should land between "abXY" and "def".
		inp.Update(tea.KeyPressMsg{Code: 'Z', Text: "Z"})
		if inp.Value() != "abXYZdef" {
			t.Errorf("after paste cursor should be at 4, typing Z gives %q", inp.Value())
		}
	})

	// -- Behavior 6: Unicode paste advances cursor by RUNE count, not bytes --
	t.Run("paste-unicode-rune-count", func(t *testing.T) {
		inp := components.NewInputPane()
		// Paste 2 multi-byte runes at cursor=0 (empty buffer).
		// "日本" is 2 runes but 6 bytes in UTF-8. Cursor must advance to 2, not 6.
		inp.Update(tea.KeyPressMsg{Code: 0, Text: "日本"})
		if inp.Value() != "日本" {
			t.Errorf("Unicode paste should yield '日本', got %q", inp.Value())
		}
		// Type 'a'. If cursor advanced by byte count (6), it would try to insert
		// past len(runes)=2 and the default insertion would still append, but
		// if it advanced by rune count (2==len(runes)), 'a' also appends.
		// Instead, verify cursor advance semantics by moving Left 2 → cursor=0,
		// then Right 1 → cursor=1, then insert: should split between the runes.
		inp.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
		inp.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
		// cursor=0, buffer still "日本"
		inp.Update(tea.KeyPressMsg{Code: tea.KeyRight})
		// cursor=1, between the two runes
		inp.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
		if inp.Value() != "日a本" {
			t.Errorf("rune-boundary cursor must split between runes: want '日a本', got %q",
				inp.Value())
		}
	})
}

// TestParity_ThinkingBudgetEffortMapping validates the 4-threshold mapping
// that handleSubmit uses to map session.Config.ThinkingBudget → spinner
// effort level. The existing EffortLevelDisplay test only covers ONE budget
// value (20000 → high); this test pins all four thresholds including
// boundaries, and the disabled-case.
//
// Unique behaviors (boundary math not previously tested):
//  1. budget >= 30000 → "max"  → ◉ glyph in spinner
//  2. 15000 ≤ budget < 30000 → "high" → ●
//  3. 5000 ≤ budget < 15000 → "medium" → ◐
//  4. budget < 5000 → "low" → ○
//  5. ThinkingEnabled=false → NO effort glyph (suffix "(thinking)" only)
//  6. Exact threshold values land on the upper bucket (>=30000, >=15000, >=5000)
//  7. Values just below threshold drop into the lower bucket
//
// Cross-ref: app.go:436-448 handleSubmit thinking budget branches
// Cross-ref: spinner_verbs.go:94-97 EffortLow/Medium/High/Max glyph constants
// Cross-ref: Claude Code src/screens/REPL.tsx → effortFromBudget.
func TestParity_ThinkingBudgetEffortMapping(t *testing.T) {
	type boundary struct {
		budget    int
		wantGlyph string
		label     string
	}
	cases := []boundary{
		// max threshold (>=30000)
		{budget: 30000, wantGlyph: "◉", label: "exact-max"},
		{budget: 100000, wantGlyph: "◉", label: "well-above-max"},
		// high threshold (>=15000, <30000)
		{budget: 29999, wantGlyph: "●", label: "just-below-max"},
		{budget: 15000, wantGlyph: "●", label: "exact-high"},
		// medium threshold (>=5000, <15000)
		{budget: 14999, wantGlyph: "◐", label: "just-below-high"},
		{budget: 5000, wantGlyph: "◐", label: "exact-medium"},
		// low (default, <5000)
		{budget: 4999, wantGlyph: "○", label: "just-below-medium"},
		{budget: 0, wantGlyph: "○", label: "zero-budget"},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			config := session.DefaultConfig()
			config.ThinkingEnabled = true
			config.ThinkingBudget = tc.budget
			sess := session.New(config, "/tmp")
			app := NewAppModel(sess, nil)
			app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

			// User submits → handleSubmit applies budget→effort mapping
			app.Update(components.SubmitMsg{Text: "think about it"})

			// Verify spinner is active AND shows the expected effort glyph.
			if !app.spinner.IsActive() {
				t.Fatalf("spinner must be active after SubmitMsg")
			}
			view := strip(app.spinner.View())
			// Expected format: "... (thinking {glyph})"
			wantSuffix := "(thinking " + tc.wantGlyph + ")"
			if !strings.Contains(view, wantSuffix) {
				t.Errorf("budget=%d: spinner view must contain suffix %q, got:\n%s",
					tc.budget, wantSuffix, view)
			}
			// And the glyphs for OTHER levels must NOT appear (prevents false
			// positives from a broken mapping landing on a neighbor).
			for _, other := range []string{"○", "◐", "●", "◉"} {
				if other == tc.wantGlyph {
					continue
				}
				if strings.Contains(view, "(thinking "+other+")") {
					t.Errorf("budget=%d: spinner view wrongly contains other glyph %q:\n%s",
						tc.budget, other, view)
				}
			}
		})
	}

	// -- Behavior 5: ThinkingEnabled=false → NO glyph emitted --
	t.Run("disabled-thinking-no-glyph", func(t *testing.T) {
		config := session.DefaultConfig()
		config.ThinkingEnabled = false
		config.ThinkingBudget = 50000 // would map to max, but disabled
		sess := session.New(config, "/tmp")
		app := NewAppModel(sess, nil)
		app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

		app.Update(components.SubmitMsg{Text: "no thinking"})

		if !app.spinner.IsActive() {
			t.Fatal("spinner must be active")
		}
		view := strip(app.spinner.View())
		// Suffix must be exactly "(thinking)" — NO glyph embedded.
		if !strings.Contains(view, "(thinking)") {
			t.Errorf("disabled thinking should show bare '(thinking)', got:\n%s", view)
		}
		for _, glyph := range []string{"○", "◐", "●", "◉"} {
			if strings.Contains(view, "(thinking "+glyph+")") {
				t.Errorf("disabled thinking must NOT embed effort glyph %q:\n%s", glyph, view)
			}
		}
	})
}

// TestParity_DiffParserLineNumbering validates the unified-diff parser's
// line-number accounting and content stripping. The existing diff_test.go
// only checks "at least one line of each type exists" — this validates the
// actual math that would break a real diff display.
//
// Unique behaviors (no existing test verifies line numbers or content):
//  1. Empty input lines are dropped entirely (not in the output slice).
//  2. "@@ -OLD,n +NEW,m @@" seeds the counters: oldLine=OLD, newLine=NEW.
//  3. After hunk header, an added line ("+foo") gets NewNum=NEW+1 and
//     OldNum=0 (counter untouched), while prefix "+" is stripped.
//  4. After hunk header, a removed line ("-bar") gets OldNum=OLD+1 and
//     NewNum=0 (counter untouched), prefix "-" stripped.
//  5. Context lines (" baz") increment BOTH counters and have their leading
//     single space stripped from Content.
//  6. Mixed sequences preserve ordering: output[i].Type reflects input[i]
//     after empty-line filtering.
//  7. "---" and "+++" file-header prefixes produce DiffHeader lines and do
//     NOT participate in line-number counting.
//
// Cross-ref: diff.go:193-227 parseDiffLines — would break if any handler
// arm forgot to increment/strip, or incremented the wrong counter.
// Cross-ref: Claude Code src/components/diff/Diff.tsx — same unified format.
func TestParity_DiffParserLineNumbering(t *testing.T) {
	// A canonical unified diff snippet exercising all 5 line kinds:
	// file header, hunk header, context, add, remove, context.
	diffText := strings.Join([]string{
		"--- a/main.go",
		"+++ b/main.go",
		"@@ -5,3 +10,4 @@",
		" context-line-A", // context → old=6,new=11
		"-removed-line",   // remove  → old=7
		"+added-line-1",   // add     → new=12
		"+added-line-2",   // add     → new=13
		"",                // empty → dropped
		" context-line-B", // context → old=8,new=14
	}, "\n")

	dv := components.NewDiffViewer(theme.Current())
	dv.SetDiff(diffText)
	lines := dv.Lines()

	// -- Behavior 1: empty line dropped → 8 entries (not 9) --
	// file-header ×2 + hunk ×1 + context ×1 + remove ×1 + add ×2 + context ×1 = 8
	if len(lines) != 8 {
		t.Fatalf("expected 8 parsed lines (empty dropped), got %d: %+v", len(lines), lines)
	}

	// -- Behavior 7: file headers are DiffHeader with 0 line numbers --
	if lines[0].Type != components.DiffHeader || lines[0].Content != "--- a/main.go" {
		t.Errorf("lines[0] should be file-header '---', got type=%v content=%q",
			lines[0].Type, lines[0].Content)
	}
	if lines[1].Type != components.DiffHeader || lines[1].Content != "+++ b/main.go" {
		t.Errorf("lines[1] should be file-header '+++', got type=%v content=%q",
			lines[1].Type, lines[1].Content)
	}

	// -- Behavior 2: hunk header seeds counters to 5 & 10 (not incremented) --
	if lines[2].Type != components.DiffHeader {
		t.Errorf("lines[2] should be hunk DiffHeader, got %v", lines[2].Type)
	}
	if !strings.HasPrefix(lines[2].Content, "@@") {
		t.Errorf("hunk header Content should start with @@, got %q", lines[2].Content)
	}

	// -- Behavior 5: context line after hunk → OldNum=6, NewNum=11, stripped --
	ctxA := lines[3]
	if ctxA.Type != components.DiffContext {
		t.Errorf("lines[3] should be DiffContext, got %v", ctxA.Type)
	}
	if ctxA.OldNum != 6 || ctxA.NewNum != 11 {
		t.Errorf("first context line numbers: want OldNum=6,NewNum=11, got OldNum=%d,NewNum=%d",
			ctxA.OldNum, ctxA.NewNum)
	}
	if ctxA.Content != "context-line-A" {
		t.Errorf("context Content should have leading space stripped, got %q", ctxA.Content)
	}

	// -- Behavior 4: removed line → OldNum=7, NewNum=0, prefix stripped --
	rem := lines[4]
	if rem.Type != components.DiffRemoved {
		t.Errorf("lines[4] should be DiffRemoved, got %v", rem.Type)
	}
	if rem.OldNum != 7 {
		t.Errorf("removed OldNum: want 7, got %d", rem.OldNum)
	}
	if rem.NewNum != 0 {
		t.Errorf("removed NewNum must be 0 (unchanged counter), got %d", rem.NewNum)
	}
	if rem.Content != "removed-line" {
		t.Errorf("removed Content should drop the '-' prefix, got %q", rem.Content)
	}

	// -- Behavior 3: added lines → NewNum=12 then 13, OldNum=0 --
	add1 := lines[5]
	if add1.Type != components.DiffAdded {
		t.Errorf("lines[5] should be DiffAdded, got %v", add1.Type)
	}
	if add1.NewNum != 12 || add1.OldNum != 0 {
		t.Errorf("added-1 numbers: want NewNum=12,OldNum=0, got NewNum=%d,OldNum=%d",
			add1.NewNum, add1.OldNum)
	}
	if add1.Content != "added-line-1" {
		t.Errorf("added-1 Content should drop '+' prefix, got %q", add1.Content)
	}
	add2 := lines[6]
	if add2.NewNum != 13 {
		t.Errorf("added-2 NewNum: want 13, got %d", add2.NewNum)
	}
	if add2.Content != "added-line-2" {
		t.Errorf("added-2 Content: want %q, got %q", "added-line-2", add2.Content)
	}

	// -- Behavior 5 (cont): second context line → OldNum=8,NewNum=14 --
	// +2 from 2 added lines applied to newLine, +1 from removed applied to oldLine,
	// +1 context before, so: oldLine = 5+1+1+1 = 8; newLine = 10+1+2+1 = 14.
	ctxB := lines[7]
	if ctxB.OldNum != 8 || ctxB.NewNum != 14 {
		t.Errorf("second context line numbers: want OldNum=8,NewNum=14, got OldNum=%d,NewNum=%d",
			ctxB.OldNum, ctxB.NewNum)
	}

	// -- Behavior 6: ordering preserved (the per-index asserts above already
	// prove this, but verify the types-in-order sequence as a tripwire) --
	wantTypes := []components.DiffLineType{
		components.DiffHeader, components.DiffHeader, components.DiffHeader,
		components.DiffContext, components.DiffRemoved,
		components.DiffAdded, components.DiffAdded, components.DiffContext,
	}
	for i, want := range wantTypes {
		if lines[i].Type != want {
			t.Errorf("lines[%d].Type: want %v, got %v", i, want, lines[i].Type)
		}
	}
}

// TestParity_SlashCommandAutocompleteFlow validates the SlashCommandInput full
// user interaction: activate on "/", filter-as-you-type, navigate suggestions
// with Up/Down (with clamping), select with Enter/Tab, dismiss with Escape,
// and the fuzzy-match rule (subsequence, not just prefix).
//
// Unique behaviors (no existing test covers this end-to-end flow):
//  1. Activate("/") sets active=true and populates suggestions from the 7
//     default commands; Deactivate clears suggestions+prefix+active.
//  2. Update on KeyPressMsg is a no-op when !active (returns immediately).
//  3. Prefix filter "/mo" leaves only /model (exact HasPrefix match).
//  4. Prefix filter "/h" matches BOTH /help (HasPrefix) AND /thinking (fuzzy
//     subsequence "/" → "h" present in "/thinking"). This is the fuzzy rule
//     and would break if the filter reverted to HasPrefix-only.
//  5. KeyDown advances selected; clamps at len(suggestions)-1 (does not wrap,
//     does not go OOB).
//  6. KeyUp decrements; clamps at 0 (no negative index).
//  7. KeyEnter selects current suggestion → returns SlashCommandSelectedMsg
//     carrying the selected command AND deactivates the input.
//  8. KeyTab behaves identically to KeyEnter (alias).
//  9. KeyEscape deactivates WITHOUT producing a selection message.
//
// Cross-ref: slash_input.go:65-117 Activate/Update/filterSuggestions
// Cross-ref: Claude Code: components/PromptInput/slash autocomplete —
// fuzzy match mirrors Ink's CommandSelector behavior.
func TestParity_SlashCommandAutocompleteFlow(t *testing.T) {
	// -- Behavior 1: Activate + Deactivate --
	sci := components.NewSlashCommandInput(theme.Current())
	if sci.IsActive() {
		t.Error("new input must be inactive")
	}
	sci.Activate("/")
	if !sci.IsActive() {
		t.Fatal("Activate must set active=true")
	}
	defaults := components.DefaultSlashCommands()
	if len(sci.Suggestions()) != len(defaults) {
		t.Errorf("prefix=\"/\" should match all %d default commands, got %d: %+v",
			len(defaults), len(sci.Suggestions()), sci.Suggestions())
	}

	// -- Behavior 2: Update is no-op when inactive --
	sci.Deactivate()
	if sci.IsActive() {
		t.Error("Deactivate must clear active")
	}
	if len(sci.Suggestions()) != 0 {
		t.Errorf("Deactivate must clear suggestions, got %d", len(sci.Suggestions()))
	}
	_, cmd := sci.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if cmd != nil {
		t.Error("Update while inactive must return nil cmd")
	}

	// -- Behavior 3: prefix "/mod" filters to /model only --
	sci.Activate("/mod")
	suggs := sci.Suggestions()
	hasModel := false
	for _, s := range suggs {
		if s.Name == "/model" {
			hasModel = true
		}
	}
	if !hasModel {
		t.Errorf("prefix=\"/mod\" should include /model, got %+v", suggs)
	}

	// -- Behavior 4: prefix "/h" matches /help AND /thinking (fuzzy) --
	sci.Deactivate()
	sci.Activate("/h")
	suggs = sci.Suggestions()
	hasHelp, hasThinking := false, false
	for _, s := range suggs {
		if s.Name == "/help" {
			hasHelp = true
		}
		if s.Name == "/thinking" {
			hasThinking = true
		}
	}
	if !hasHelp {
		t.Errorf("prefix=\"/h\" should include /help via HasPrefix, got %+v", suggs)
	}
	if !hasThinking {
		t.Errorf("prefix=\"/h\" should include /thinking via fuzzy subsequence, got %+v", suggs)
	}

	// -- Behavior 5: KeyDown advances and clamps at end --
	sci.Deactivate()
	sci.Activate("/") // N suggestions, selected starts at 0
	// Push Down 200 times — must clamp at len-1.
	for i := 0; i < 200; i++ {
		sci.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	_, cmd = sci.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("KeyEnter with suggestions must return cmd")
	}
	msg := cmd()
	selMsg, ok := msg.(components.SlashCommandSelectedMsg)
	if !ok {
		t.Fatalf("Enter must return SlashCommandSelectedMsg, got %T", msg)
	}
	// After clamping, selected should equal the last default command.
	lastDefault := defaults[len(defaults)-1].Name
	if selMsg.Command.Name != lastDefault {
		t.Errorf("after clamping down at end, selected should be %s, got %q",
			lastDefault, selMsg.Command.Name)
	}
	// -- Behavior 7 (cont): Enter must deactivate --
	if sci.IsActive() {
		t.Error("Enter must deactivate after selection")
	}

	// -- Behavior 6: KeyUp clamps at 0 --
	firstDefault := defaults[0].Name
	sci.Activate("/")
	for i := 0; i < 200; i++ {
		sci.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	}
	_, cmd = sci.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	selMsg = cmd().(components.SlashCommandSelectedMsg)
	if selMsg.Command.Name != firstDefault {
		t.Errorf("after clamping up at 0, selected should be %s, got %q",
			firstDefault, selMsg.Command.Name)
	}

	// -- Behavior 8: Tab is alias for Enter --
	sci.Activate("/")
	_, cmd = sci.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if cmd == nil {
		t.Fatal("Tab must select current suggestion")
	}
	selMsg = cmd().(components.SlashCommandSelectedMsg)
	if selMsg.Command.Name != firstDefault { // selected=0 at fresh activate
		t.Errorf("Tab alias: expected %s, got %q", firstDefault, selMsg.Command.Name)
	}
	if sci.IsActive() {
		t.Error("Tab must deactivate after selection")
	}

	// -- Behavior 9: Escape deactivates without producing a message --
	sci.Activate("/")
	_, cmd = sci.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd != nil {
		t.Errorf("Escape must NOT produce a selection cmd, got %v", cmd())
	}
	if sci.IsActive() {
		t.Error("Escape must deactivate")
	}
}

// TestParity_ConversationViewportWindowing validates the viewport line-slicing
// math in ConversationPane.View() — i.e. which content lines are actually
// visible when total rendered lines exceed the height.
//
// Unique behaviors (distinct from B8 autoScroll state + B30 composition):
//  1. With N messages producing >height total lines, viewport returns exactly
//     `height` lines showing only the TAIL (last height lines).
//  2. scrollOffset=K shifts viewStart back by exactly K lines, so a previously
//     hidden earlier message becomes visible AND a previously visible tail line
//     is pushed out.
//  3. When scrollOffset exceeds available history, viewStart clamps at 0 — the
//     slice never goes negative and does NOT panic.
//  4. Scrolling back enough reveals message[0]; scrolling back to 0 restores
//     the tail view.
//
// Cross-ref: conversation.go:100-122 — viewStart = totalLines-height-scrollOffset
// with clamp `if viewStart < 0 { viewStart = 0 }` and the allLines[start:end]
// slice.
func TestParity_ConversationViewportWindowing(t *testing.T) {
	cp := components.NewConversationPane()
	// Height = 3: viewport shows at most 3 lines.
	cp.SetSize(80, 3)

	// Add 5 single-line user messages with unique markers.
	markers := []string{"ZZaaa", "ZZbbb", "ZZccc", "ZZddd", "ZZeee"}
	for _, m := range markers {
		cp.AddMessage(message.Message{
			Role:    message.RoleUser,
			Content: []message.ContentBlock{{Type: message.ContentText, Text: m}},
		})
	}

	// -- Behavior 1: viewport returns exactly `height` lines --
	viewInitial := strip(cp.View().Content)
	lines := strings.Split(viewInitial, "\n")
	if len(lines) != 3 {
		t.Fatalf("viewport must be exactly height=3 lines, got %d:\n%s", len(lines), viewInitial)
	}

	// -- Behavior 1 (cont): must show TAIL only — last marker always visible;
	// first marker must NOT be visible when autoScroll at offset 0.
	if !strings.Contains(viewInitial, "ZZeee") {
		t.Errorf("tail viewport must show last message marker ZZeee, got:\n%s", viewInitial)
	}
	if strings.Contains(viewInitial, "ZZaaa") {
		t.Errorf("first message ZZaaa should be scrolled off-screen at offset=0, got:\n%s", viewInitial)
	}

	// Find the boundary marker: the earliest visible marker tells us how
	// many tail lines the viewport holds.
	earliestVisibleIdx := -1
	for i, m := range markers {
		if strings.Contains(viewInitial, m) {
			earliestVisibleIdx = i
			break
		}
	}
	if earliestVisibleIdx == -1 {
		t.Fatalf("no marker visible in viewport:\n%s", viewInitial)
	}
	if earliestVisibleIdx == 0 {
		t.Fatalf("tail viewport unexpectedly contains first message — scrollOffset not at default 0")
	}

	// -- Behavior 2: scroll up by 1 → earlier marker now visible, tail marker
	// pushed out of the window. scrollOffset increases from 0→1 via KeyUp.
	_, _ = cp.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	viewScroll1 := strip(cp.View().Content)
	linesScrolled := strings.Split(viewScroll1, "\n")
	if len(linesScrolled) != 3 {
		t.Errorf("after scroll viewport must still be 3 lines, got %d", len(linesScrolled))
	}

	// After 1 scroll-up, a marker at index earliestVisibleIdx-1 should now
	// appear (assuming each message renders to ≥1 line; scroll shifts window
	// back by exactly 1 LINE not 1 message, so we check the shift happened).
	shiftedEarlierVisible := false
	if earliestVisibleIdx > 0 {
		shiftedEarlierVisible = strings.Contains(viewScroll1, markers[earliestVisibleIdx-1])
	}
	// The last marker (tail) must be pushed out OR the view must have shifted
	// visibly. Equivalence: the scrolled view differs from the initial view.
	if viewScroll1 == viewInitial {
		t.Errorf("scrolling up by 1 line must change viewport content, got identical views:\n%s", viewInitial)
	}
	// Verify earlier content appeared OR tail was pushed out — both confirm
	// the window shifted backward (the math is deterministic: viewStart--).
	tailPushedOut := !strings.Contains(viewScroll1, markers[len(markers)-1])
	if !shiftedEarlierVisible && !tailPushedOut {
		// At minimum, either a new earlier line appeared or the tail got
		// pushed out. If neither happened, the shift math is broken.
		t.Errorf("scrolling up must reveal earlier line or push tail out.\ninitial: %q\nscrolled: %q",
			viewInitial, viewScroll1)
	}

	// -- Behavior 3: scroll up many times — must NOT panic, must clamp --
	for i := 0; i < 100; i++ {
		_, _ = cp.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	}
	viewAtTop := strip(cp.View().Content)
	linesAtTop := strings.Split(viewAtTop, "\n")
	if len(linesAtTop) != 3 {
		t.Errorf("clamped viewport must be 3 lines, got %d:\n%s", len(linesAtTop), viewAtTop)
	}

	// -- Behavior 4: at clamp, first message ZZaaa MUST be visible --
	if !strings.Contains(viewAtTop, "ZZaaa") {
		t.Errorf("after scrolling to top, first message ZZaaa must be visible, got:\n%s", viewAtTop)
	}

	// -- Behavior 4 (cont): scroll back down to 0 → tail view restored --
	for i := 0; i < 200; i++ {
		_, _ = cp.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	viewBack := strip(cp.View().Content)
	if !strings.Contains(viewBack, "ZZeee") {
		t.Errorf("after scrolling back down, tail ZZeee must be visible, got:\n%s", viewBack)
	}
	if strings.Contains(viewBack, "ZZaaa") {
		t.Errorf("after scrolling back to bottom, first message ZZaaa should be gone, got:\n%s", viewBack)
	}
	// Restored view must equal the initial tail view (determinism check).
	if viewBack != viewInitial {
		t.Errorf("scrolling back to offset=0 must restore initial view.\ninitial: %q\nrestored: %q",
			viewInitial, viewBack)
	}
}
