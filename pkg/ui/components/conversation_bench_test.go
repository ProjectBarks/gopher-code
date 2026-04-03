package components

import (
	"testing"

	"github.com/projectbarks/gopher-code/pkg/message"
)

// BenchmarkConversationPaneCreate measures the time to create a ConversationPane.
// Target: <1ms per ConversationPane creation
func BenchmarkConversationPaneCreate(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewConversationPane()
	}
}

// BenchmarkConversationPaneSetSize measures the time to set size on a ConversationPane.
// Target: <100µs per SetSize call
func BenchmarkConversationPaneSetSize(b *testing.B) {
	pane := NewConversationPane()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pane.SetSize(80, 20)
	}
}

// BenchmarkConversationPaneAddMessage measures the time to add a single message.
// Target: <1ms per message (including rendering)
func BenchmarkConversationPaneAddMessage(b *testing.B) {
	pane := NewConversationPane()
	pane.SetSize(80, 20)

	msg := message.UserMessage("Hello, world!")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pane.AddMessage(msg)
	}
}

// BenchmarkConversationPaneViewEmpty measures rendering an empty pane.
// Target: <100µs per View call
func BenchmarkConversationPaneViewEmpty(b *testing.B) {
	pane := NewConversationPane()
	pane.SetSize(80, 20)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pane.View()
	}
}

// BenchmarkConversationPaneViewWithMessages measures rendering with 100 messages.
// Target: <5ms per View call (virtual scrolling should keep this fast)
func BenchmarkConversationPaneViewWithMessages(b *testing.B) {
	pane := NewConversationPane()
	pane.SetSize(80, 20)

	// Add 100 messages
	for i := 0; i < 100; i++ {
		if i%2 == 0 {
			pane.AddMessage(message.UserMessage("User message"))
		} else {
			assistantMsg := message.Message{
				Role: message.RoleAssistant,
				Content: []message.ContentBlock{
					{Type: message.ContentText, Text: "Assistant response"},
				},
			}
			pane.AddMessage(assistantMsg)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pane.View()
	}
}

// BenchmarkConversationPaneViewWithLongMessages measures rendering with long text.
// Target: <5ms per View call (text wrapping should be efficient)
func BenchmarkConversationPaneViewWithLongMessages(b *testing.B) {
	pane := NewConversationPane()
	pane.SetSize(80, 20)

	// Add messages with long text that requires wrapping
	longText := "This is a very long message that will require text wrapping. " +
		"It contains multiple sentences and should be wrapped correctly to fit " +
		"within the terminal width. The wrapping algorithm should be efficient."

	for i := 0; i < 20; i++ {
		pane.AddMessage(message.UserMessage(longText))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pane.View()
	}
}

// BenchmarkConversationPaneUpdate measures processing keyboard input.
// Target: <100µs per Update call
func BenchmarkConversationPaneUpdate(b *testing.B) {
	pane := NewConversationPane()
	pane.SetSize(80, 20)

	// Add some messages
	for i := 0; i < 20; i++ {
		pane.AddMessage(message.UserMessage("Test message"))
	}

	// Simulate adding a new message
	msg := AddMessageMsg{Message: message.UserMessage("x")}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pane.Update(msg)
	}
}
