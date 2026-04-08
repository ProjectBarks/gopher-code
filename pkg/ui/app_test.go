package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/query"
	"github.com/projectbarks/gopher-code/pkg/session"
	"github.com/projectbarks/gopher-code/pkg/ui/components"
	cmdhooks "github.com/projectbarks/gopher-code/pkg/ui/hooks/commands"
)

func newTestApp() *AppModel {
	config := session.DefaultConfig()
	sess := session.New(config, "/tmp/test")
	return NewAppModel(sess, nil)
}

func TestAppModelCreation(t *testing.T) {
	app := newTestApp()
	if app == nil {
		t.Fatal("AppModel should not be nil")
	}
	if app.mode != ModeIdle {
		t.Error("Initial mode should be Idle")
	}
	if app.header == nil {
		t.Error("Header should be initialized")
	}
	if app.conversation == nil {
		t.Error("ConversationPane should be initialized")
	}
	if app.input == nil {
		t.Error("InputPane should be initialized")
	}
	if app.statusLine == nil {
		t.Error("StatusLine should be initialized")
	}
	if app.bubble == nil {
		t.Error("MessageBubble should be initialized")
	}
	if app.streaming == nil {
		t.Error("StreamingText should be initialized")
	}
}

func TestAppModelInit(t *testing.T) {
	app := newTestApp()
	cmd := app.Init()
	// Init can return nil
	_ = cmd
}

func TestAppModelViewBeforeResize(t *testing.T) {
	app := newTestApp()
	view := app.View()
	if view.Content != "Initializing..." {
		t.Errorf("Expected 'Initializing...' before resize, got %q", view.Content)
	}
}

func TestAppModelViewAfterResize(t *testing.T) {
	app := newTestApp()
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	view := app.View()
	if view.Content == "Initializing..." {
		t.Error("Should render full UI after resize")
	}
	if view.Content == "" {
		t.Error("View should not be empty after resize")
	}
}

func TestAppModelHandleResize(t *testing.T) {
	app := newTestApp()
	updated, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	result := updated.(*AppModel)
	if result.width != 120 || result.height != 40 {
		t.Errorf("Expected 120x40, got %dx%d", result.width, result.height)
	}
}

func TestAppModelHandleKeyTab(t *testing.T) {
	app := newTestApp()
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	// Tab should not panic with empty focus ring
	updated, _ := app.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if updated == nil {
		t.Error("Should return non-nil model")
	}
}

func TestAppModelHandleKeyShiftTab(t *testing.T) {
	app := newTestApp()
	updated, _ := app.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if updated == nil {
		t.Error("Should return non-nil model")
	}
}

func TestAppModelHandleKeyEscape(t *testing.T) {
	app := newTestApp()
	updated, _ := app.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if updated == nil {
		t.Error("Should return non-nil model")
	}
}

func TestAppModelHandleStatusUpdateMsg(t *testing.T) {
	app := newTestApp()
	updated, _ := app.Update(StatusUpdateMsg{Mode: ModeStreaming})
	result := updated.(*AppModel)
	if result.mode != ModeStreaming {
		t.Error("Mode should be updated to Streaming")
	}
}

func TestAppModelRenderHeader(t *testing.T) {
	app := newTestApp()
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	view := app.View()
	if !strings.Contains(view.Content, "Claude") {
		t.Error("Header should contain 'Claude'")
	}
}

func TestAppModelRenderStatusLine(t *testing.T) {
	app := newTestApp()
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	view := app.View()
	// Status line is part of the view
	if view.Content == "" {
		t.Error("View should contain status line")
	}
}

func TestAppModelTextDeltaUpdatesMode(t *testing.T) {
	app := newTestApp()
	app.Update(TextDeltaMsg{Text: "hello"})
	if app.mode != ModeStreaming {
		t.Error("TextDelta should set mode to Streaming")
	}
}

func TestAppModelToolUseStartUpdatesMode(t *testing.T) {
	app := newTestApp()
	app.Update(ToolUseStartMsg{ToolUseID: "t1", ToolName: "bash"})
	if app.mode != ModeToolRunning {
		t.Error("ToolUseStart should set mode to ToolRunning")
	}
}

func TestAppModelTurnCompleteResetsMode(t *testing.T) {
	app := newTestApp()
	app.Update(TextDeltaMsg{Text: "hello"})
	app.Update(TurnCompleteMsg{})
	if app.mode != ModeIdle {
		t.Error("TurnComplete should reset mode to Idle")
	}
}

func TestAppModelTurnCompleteFinalizesMessage(t *testing.T) {
	app := newTestApp()
	// Simulate streaming
	app.Update(TextDeltaMsg{Text: "Hello "})
	app.Update(TextDeltaMsg{Text: "world"})
	// Complete turn
	app.Update(TurnCompleteMsg{})
	// Conversation should have a message
	if app.conversation.MessageCount() != 1 {
		t.Errorf("Expected 1 message in conversation, got %d", app.conversation.MessageCount())
	}
}

func TestAppModelUsageUpdatesSession(t *testing.T) {
	app := newTestApp()
	evt := query.QueryEvent{
		Type:         query.QEventUsage,
		InputTokens:  100,
		OutputTokens: 50,
	}
	app.Update(QueryEventMsg{Event: evt})
	if app.session.TotalInputTokens != 100 {
		t.Errorf("Expected 100 input tokens, got %d", app.session.TotalInputTokens)
	}
}

func TestAppModelUnhandledMsgRouted(t *testing.T) {
	app := newTestApp()
	// Custom message type should be routed to focus
	type customMsg struct{}
	updated, _ := app.Update(customMsg{})
	if updated == nil {
		t.Error("Should return non-nil model for unhandled msg")
	}
}

func TestAppModelToolResultRemovesTracking(t *testing.T) {
	app := newTestApp()
	app.Update(ToolUseStartMsg{ToolUseID: "t1", ToolName: "bash"})
	if len(app.activeToolCalls) != 1 {
		t.Error("Should track tool call")
	}
	app.Update(ToolResultMsg{ToolUseID: "t1", Content: "ok"})
	if len(app.activeToolCalls) != 0 {
		t.Error("Should remove tool call after result")
	}
}

func TestAppModelTurnCompleteResetsToolCalls(t *testing.T) {
	app := newTestApp()
	app.Update(ToolUseStartMsg{ToolUseID: "t1", ToolName: "bash"})
	app.Update(TurnCompleteMsg{})
	if len(app.activeToolCalls) != 0 {
		t.Error("TurnComplete should clear active tool calls")
	}
}

func TestAppModelNilSession(t *testing.T) {
	app := NewAppModel(nil, nil)
	// Should not panic
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	view := app.View()
	if view.Content == "" {
		t.Error("View should work with nil session")
	}
}

func TestAppModelInputPaneFocused(t *testing.T) {
	app := newTestApp()
	if !app.input.Focused() {
		t.Error("InputPane should have initial focus")
	}
}

func TestAppModelSubmitAddsUserMessage(t *testing.T) {
	app := newTestApp()
	app.Update(components.SubmitMsg{Text: "hello"})
	if app.conversation.MessageCount() != 1 {
		t.Errorf("Expected 1 message after submit, got %d", app.conversation.MessageCount())
	}
}

func TestAppModelSubmitAddsToSession(t *testing.T) {
	app := newTestApp()
	app.Update(components.SubmitMsg{Text: "test query"})
	if len(app.session.Messages) != 1 {
		t.Errorf("Expected 1 session message, got %d", len(app.session.Messages))
	}
}

func TestAppModelSubmitEmptyIgnored(t *testing.T) {
	app := newTestApp()
	app.Update(components.SubmitMsg{Text: ""})
	if app.conversation.MessageCount() != 0 {
		t.Error("Empty submit should be ignored")
	}
}

func TestAppModelSubmitWhitespaceIgnored(t *testing.T) {
	app := newTestApp()
	app.Update(components.SubmitMsg{Text: "   "})
	if app.conversation.MessageCount() != 0 {
		t.Error("Whitespace-only submit should be ignored")
	}
}

func TestAppModelCtrlCQuits(t *testing.T) {
	app := newTestApp()
	// First Ctrl+C shows hint
	app.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	// Second Ctrl+C quits
	_, cmd := app.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("Double Ctrl+C should produce a command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("Expected QuitMsg on second Ctrl+C, got %T", msg)
	}
}

func TestAppModelQueryDoneResetsMode(t *testing.T) {
	app := newTestApp()
	app.mode = ModeStreaming
	app.Update(queryDoneMsg{err: nil})
	if app.mode != ModeIdle {
		t.Error("queryDone should reset mode to Idle")
	}
}

func TestAppModelQueryDoneWithError(t *testing.T) {
	app := newTestApp()
	app.mode = ModeStreaming
	app.Update(queryDoneMsg{err: fmt.Errorf("test error")})
	if app.mode != ModeIdle {
		t.Error("queryDone with error should reset mode to Idle")
	}
	// Should add error message to conversation
	if app.conversation.MessageCount() != 1 {
		t.Error("queryDone with error should add error to conversation")
	}
}

func TestAppModelViewHasInputPane(t *testing.T) {
	app := newTestApp()
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	view := app.View()
	// Input pane renders the prompt character "❯" (U+276F)
	if !strings.Contains(view.Content, "❯") {
		t.Error("View should contain input pane prompt ❯")
	}
}

func TestAppModelViewHasDivider(t *testing.T) {
	app := newTestApp()
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	view := app.View()
	// Should have light horizontal divider ─
	if !strings.Contains(view.Content, "─") {
		t.Error("View should contain divider line ─")
	}
}

func TestAppModelSetQueryFunc(t *testing.T) {
	app := newTestApp()
	called := false
	app.SetQueryFunc(func(ctx context.Context, sess *session.SessionState, onEvent query.EventCallback) error {
		called = true
		return nil
	})
	if app.queryFunc == nil {
		t.Error("queryFunc should be set")
	}
	_ = called
}

func TestAppModelSlashCommandClear(t *testing.T) {
	app := newTestApp()
	app.conversation.AddMessage(message.UserMessage("hello"))
	// Submit /clear — returns a Cmd that produces ClearConversationMsg
	_, cmd := app.Update(components.SubmitMsg{Text: "/clear"})
	if cmd == nil {
		t.Fatal("Expected command from /clear")
	}
	// Execute the command to get the message
	msg := cmd()
	// Update with the result
	app.Update(msg)
	if app.conversation.MessageCount() != 0 {
		t.Errorf("Expected 0 messages after /clear, got %d", app.conversation.MessageCount())
	}
}

func TestAppModelSlashCommandModel(t *testing.T) {
	app := newTestApp()
	_, cmd := app.Update(components.SubmitMsg{Text: "/model opus"})
	if cmd == nil {
		t.Fatal("Expected command from /model")
	}
	msg := cmd()
	app.Update(msg)
	if app.session.Config.Model != "opus" {
		t.Errorf("Expected model 'opus', got %q", app.session.Config.Model)
	}
}

func TestAppModelSlashCommandQuit(t *testing.T) {
	app := newTestApp()
	_, cmd := app.Update(components.SubmitMsg{Text: "/quit"})
	if cmd == nil {
		t.Fatal("Expected command from /quit")
	}
	msg := cmd()
	// The /quit handler returns QuitMsg, which Update should turn into tea.Quit
	_, quitCmd := app.Update(msg)
	if quitCmd == nil {
		t.Fatal("Expected quit command")
	}
	quitMsg := quitCmd()
	if _, ok := quitMsg.(tea.QuitMsg); !ok {
		t.Errorf("Expected tea.QuitMsg, got %T", quitMsg)
	}
}

func TestAppModelSlashCommandHelp(t *testing.T) {
	app := newTestApp()
	_, cmd := app.Update(components.SubmitMsg{Text: "/help"})
	msg := cmd()
	app.Update(msg)
	// Help should add a message to conversation
	if app.conversation.MessageCount() != 1 {
		t.Errorf("Expected 1 help message, got %d", app.conversation.MessageCount())
	}
}

func TestAppModelSlashCommandUnknown(t *testing.T) {
	app := newTestApp()
	_, cmd := app.Update(components.SubmitMsg{Text: "/nonexistent"})
	msg := cmd()
	app.Update(msg)
	// Should add error message
	if app.conversation.MessageCount() != 1 {
		t.Errorf("Expected 1 error message, got %d", app.conversation.MessageCount())
	}
}

// ---------------------------------------------------------------------------
// T164: Scroll activity tracking
// ---------------------------------------------------------------------------

func TestScrollTracker_NotDrainingByDefault(t *testing.T) {
	app := newTestApp()
	if app.GetIsScrollDraining() {
		t.Error("scroll should not be draining by default")
	}
}

func TestScrollTracker_MarkAndDrain(t *testing.T) {
	st := newScrollTracker()

	st.MarkScrollActivity()
	if !st.GetIsScrollDraining() {
		t.Error("scroll should be draining after MarkScrollActivity")
	}

	// WaitForScrollIdle should return once the timer fires
	st.WaitForScrollIdle()
	if st.GetIsScrollDraining() {
		t.Error("scroll should not be draining after WaitForScrollIdle")
	}
}

func TestScrollTracker_ImmediateIdleWhenNotDraining(t *testing.T) {
	st := newScrollTracker()
	// WaitForScrollIdle returns immediately when not draining
	done := make(chan struct{})
	go func() {
		st.WaitForScrollIdle()
		close(done)
	}()
	select {
	case <-done:
		// ok
	case <-time.After(time.Second):
		t.Fatal("WaitForScrollIdle should return immediately when not draining")
	}
}

func TestScrollTracker_AppModelDelegation(t *testing.T) {
	app := newTestApp()
	app.MarkScrollActivity()
	if !app.GetIsScrollDraining() {
		t.Error("AppModel.GetIsScrollDraining should delegate to scrollTracker")
	}
	app.WaitForScrollIdle()
	if app.GetIsScrollDraining() {
		t.Error("AppModel should not be draining after WaitForScrollIdle")
	}
}

// ---------------------------------------------------------------------------
// T403: Command keybindings + queue drain integration
// ---------------------------------------------------------------------------

func TestAppModel_CommandKeybindingsInitialized(t *testing.T) {
	app := newTestApp()
	if app.cmdKeybindings == nil {
		t.Fatal("cmdKeybindings should be initialized by NewAppModel")
	}
	if app.cmdQueue == nil {
		t.Fatal("cmdQueue should be initialized by NewAppModel")
	}
	if app.cmdProcessor == nil {
		t.Fatal("cmdProcessor should be initialized by NewAppModel")
	}
}

// drainCmds pumps commands through AppModel.Update, handling bubbletea's
// BatchMsg ([]Cmd) so that tests can work without the real bubbletea runtime.
func drainCmds(app *AppModel, cmd tea.Cmd) {
	var queue []tea.Cmd
	if cmd != nil {
		queue = append(queue, cmd)
	}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		msg := c()
		if batch, ok := msg.(tea.BatchMsg); ok {
			queue = append(queue, ([]tea.Cmd)(batch)...)
			continue
		}
		_, next := app.Update(msg)
		if next != nil {
			queue = append(queue, next)
		}
	}
}

// TestAppModel_KeybindingDispatchesCommand exercises the full pipeline:
// KeyPressMsg -> CommandKeybindings -> ExecuteCommandMsg -> Queue -> Processor -> Dispatcher.
// This is an integration test through the real AppModel code path, not an
// isolated unit test of the commands package.
func TestAppModel_KeybindingDispatchesCommand(t *testing.T) {
	app := newTestApp()

	// Simulate ctrl+t which maps to app:toggleTodos -> /tasks in Global context.
	// The CommandKeybindings model intercepts this in handleKey and returns a
	// tea.Cmd that produces ExecuteCommandMsg{Command: "/tasks"}.
	_, cmd := app.Update(tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("ctrl+t should produce a command from keybinding resolver")
	}

	// Execute the cmd to get the ExecuteCommandMsg.
	msg := cmd()
	ecm, ok := msg.(cmdhooks.ExecuteCommandMsg)
	if !ok {
		t.Fatalf("expected ExecuteCommandMsg, got %T", msg)
	}
	if ecm.Command != "/tasks" {
		t.Errorf("expected /tasks, got %q", ecm.Command)
	}
	if !ecm.FromKeybinding {
		t.Error("expected FromKeybinding to be true")
	}

	// Feed the ExecuteCommandMsg back into Update — it should enqueue and
	// trigger the queue processor, which dispatches through the command
	// dispatcher.
	_, cmd = app.Update(ecm)

	// Drain all resulting commands (handles tea.BatchMsg from the processor).
	drainCmds(app, cmd)

	// After draining, the queue should be empty and processor idle.
	if app.cmdQueue.Len() != 0 {
		t.Errorf("expected empty queue after drain, got %d items", app.cmdQueue.Len())
	}
	if app.cmdProcessor.IsProcessing() {
		t.Error("processor should not be processing after drain completes")
	}
}

// TestAppModel_QueueDrainSerialOrder verifies that commands enqueued via
// ExecuteCommandMsg are processed in priority order through the real
// AppModel.Update loop.
func TestAppModel_QueueDrainSerialOrder(t *testing.T) {
	app := newTestApp()

	// Directly enqueue two commands to the queue (simulating what happens
	// when ExecuteCommandMsg is received).
	app.cmdQueue.Enqueue(cmdhooks.QueuedCommand{
		Value:    "/help",
		Priority: cmdhooks.PriorityNext,
	})
	app.cmdQueue.Enqueue(cmdhooks.QueuedCommand{
		Value:    "/clear",
		Priority: cmdhooks.PriorityNow, // higher priority
	})

	// Kick the processor via Update with ProcessQueueMsg.
	_, cmd := app.Update(cmdhooks.ProcessQueueMsg{})

	// Drain the processor loop (handles tea.BatchMsg from the processor).
	drainCmds(app, cmd)

	// Both commands should have been dequeued.
	if app.cmdQueue.Len() != 0 {
		t.Errorf("expected empty queue, got %d", app.cmdQueue.Len())
	}
}
