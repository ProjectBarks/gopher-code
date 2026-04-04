package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/query"
	"github.com/projectbarks/gopher-code/pkg/session"
	"github.com/projectbarks/gopher-code/pkg/ui/components"
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
	_, cmd := app.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("Ctrl+C should produce a command")
	}
	// Should be a quit command
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("Expected QuitMsg, got %T", msg)
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
	// Input pane renders the prompt character "›" (U+203A)
	if !strings.Contains(view.Content, "›") {
		t.Error("View should contain input pane prompt ›")
	}
}

func TestAppModelViewHasDivider(t *testing.T) {
	app := newTestApp()
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	view := app.View()
	// Should have heavy horizontal divider ━
	if !strings.Contains(view.Content, "━") {
		t.Error("View should contain divider line ━")
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
