package components

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/message"
)

func TestConversationPaneCreation(t *testing.T) {
	cp := NewConversationPane()
	if cp == nil {
		t.Fatal("ConversationPane should not be nil")
	}
	if cp.MessageCount() != 0 {
		t.Error("Should have 0 messages initially")
	}
}

func TestConversationPaneInit(t *testing.T) {
	cp := NewConversationPane()
	cmd := cp.Init()
	_ = cmd // nil is valid
}

func TestConversationPaneAddMessage(t *testing.T) {
	cp := NewConversationPane()
	cp.SetSize(80, 20)
	cp.AddMessage(message.UserMessage("Hello"))
	if cp.MessageCount() != 1 {
		t.Errorf("Expected 1 message, got %d", cp.MessageCount())
	}
}

func TestConversationPaneViewEmpty(t *testing.T) {
	cp := NewConversationPane()
	cp.SetSize(80, 20)
	view := cp.View()
	plain := stripANSI(view.Content)
	if !strings.Contains(plain, "No messages") {
		t.Errorf("Expected 'No messages' placeholder, got %q", plain)
	}
}

func TestConversationPaneViewWithMessage(t *testing.T) {
	cp := NewConversationPane()
	cp.SetSize(80, 20)
	cp.AddMessage(message.UserMessage("Hello world"))
	view := cp.View()
	plain := stripANSI(view.Content)
	if !strings.Contains(plain, "Hello world") {
		t.Errorf("Expected message text, got %q", plain)
	}
}

func TestConversationPaneViewBeforeSetSize(t *testing.T) {
	cp := NewConversationPane()
	view := cp.View()
	if view.Content != "" {
		t.Error("View before SetSize should be empty")
	}
}

func TestConversationPaneSetStreamingText(t *testing.T) {
	cp := NewConversationPane()
	cp.SetSize(80, 20)
	cp.SetStreamingText("streaming...")
	view := cp.View()
	if !strings.Contains(view.Content, "streaming...") {
		t.Error("Expected streaming text in view")
	}
}

func TestConversationPaneClearStreamingText(t *testing.T) {
	cp := NewConversationPane()
	cp.SetSize(80, 20)
	cp.SetStreamingText("temp")
	cp.ClearStreamingText()
	view := cp.View()
	if strings.Contains(view.Content, "temp") {
		t.Error("Streaming text should be cleared")
	}
}

func TestConversationPaneFocus(t *testing.T) {
	cp := NewConversationPane()
	if cp.Focused() {
		t.Error("Should not be focused initially")
	}
	cp.Focus()
	if !cp.Focused() {
		t.Error("Should be focused after Focus()")
	}
	cp.Blur()
	if cp.Focused() {
		t.Error("Should not be focused after Blur()")
	}
}

func TestConversationPaneScrolling(t *testing.T) {
	cp := NewConversationPane()
	cp.SetSize(80, 5)

	// Add many messages to overflow viewport
	for i := 0; i < 20; i++ {
		cp.AddMessage(message.UserMessage("message"))
	}

	// Scroll up
	cp.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	// Scroll down
	cp.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	// Page up
	cp.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
	// Page down
	cp.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
}

func TestConversationPaneUpdateAddMessageMsg(t *testing.T) {
	cp := NewConversationPane()
	cp.SetSize(80, 20)
	cp.Update(AddMessageMsg{Message: message.UserMessage("via update")})
	if cp.MessageCount() != 1 {
		t.Error("AddMessageMsg should add message")
	}
}

func TestConversationPaneUpdateClearMsg(t *testing.T) {
	cp := NewConversationPane()
	cp.SetSize(80, 20)
	cp.AddMessage(message.UserMessage("test"))
	cp.Update(ClearMessagesMsg{})
	if cp.MessageCount() != 0 {
		t.Error("ClearMessagesMsg should clear all messages")
	}
}

func TestConversationPaneUpdateWindowSize(t *testing.T) {
	cp := NewConversationPane()
	cp.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	// Should not panic and should update size
}

func TestConversationPaneMultipleMessages(t *testing.T) {
	cp := NewConversationPane()
	cp.SetSize(80, 20)
	cp.AddMessage(message.UserMessage("first"))
	cp.AddMessage(message.Message{
		Role:    message.RoleAssistant,
		Content: []message.ContentBlock{{Type: message.ContentText, Text: "response"}},
	})
	if cp.MessageCount() != 2 {
		t.Errorf("Expected 2 messages, got %d", cp.MessageCount())
	}
}
