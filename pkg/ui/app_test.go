package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/query"
	"github.com/projectbarks/gopher-code/pkg/session"
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
	if app.conversation == nil {
		t.Error("ConversationPane should be initialized")
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
	if !strings.Contains(view.Content, "Gopher") {
		t.Error("Header should contain 'Gopher'")
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
