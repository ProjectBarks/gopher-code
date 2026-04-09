// Package messages provides the conversation message rendering system.
//
// Source: components/Messages.tsx, MessageRow.tsx, Message.tsx
//
// Renders a list of conversation messages with role indicators, timestamps,
// tool use/result display, thinking blocks, and system messages. This is
// the core message container that T319 and T320 build on.
package messages

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// RenderableMessage wraps a message with display metadata.
type RenderableMessage struct {
	Type      string          // "user", "assistant", "system", "tool_result", "collapsed"
	Message   message.Message
	Timestamp time.Time
	// UI state
	IsStreaming   bool   // currently being streamed
	IsCollapsed   bool   // collapsed tool use group
	CollapsedText string // text for collapsed groups
}

// RenderOptions controls message rendering behavior.
type RenderOptions struct {
	Width        int
	Verbose      bool
	ShowTimestamps bool
	IsLoading    bool // model is currently generating
}

// RenderConversation renders a full conversation as a string.
func RenderConversation(msgs []RenderableMessage, opts RenderOptions) string {
	if len(msgs) == 0 {
		return ""
	}

	var b strings.Builder
	for i, rm := range msgs {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(RenderMessage(rm, opts))
	}
	return b.String()
}

// RenderMessage renders a single message with role indicator and content.
func RenderMessage(rm RenderableMessage, opts RenderOptions) string {
	colors := theme.Current().Colors()

	var b strings.Builder

	// Role indicator
	switch rm.Type {
	case "user":
		b.WriteString(renderUserMessage(rm, opts, colors))
	case "assistant":
		b.WriteString(renderAssistantMessage(rm, opts, colors))
	case "system":
		b.WriteString(renderSystemMessage(rm, colors))
	case "collapsed":
		b.WriteString(renderCollapsedGroup(rm, colors))
	default:
		b.WriteString(renderGenericMessage(rm, colors))
	}

	return b.String()
}

func renderUserMessage(rm RenderableMessage, opts RenderOptions, colors theme.ColorScheme) string {
	roleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Info)).Bold(true)
	dimStyle := lipgloss.NewStyle().Faint(true)

	var b strings.Builder
	b.WriteString(roleStyle.Render("❯ "))

	// Extract text content
	for _, block := range rm.Message.Content {
		switch block.Type {
		case message.ContentText:
			b.WriteString(block.Text)
		case message.ContentToolResult:
			if block.IsError {
				errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Error))
				b.WriteString(errStyle.Render("Error: " + block.Content))
			} else {
				b.WriteString(dimStyle.Render(truncateContent(block.Content, 200)))
			}
		}
	}

	if opts.ShowTimestamps && !rm.Timestamp.IsZero() {
		b.WriteString(dimStyle.Render(" " + formatTimestamp(rm.Timestamp)))
	}

	return b.String()
}

func renderAssistantMessage(rm RenderableMessage, opts RenderOptions, colors theme.ColorScheme) string {
	accentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Accent))
	dimStyle := lipgloss.NewStyle().Faint(true)
	thinkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.TextMuted)).Italic(true)

	var b strings.Builder

	for i, block := range rm.Message.Content {
		if i > 0 {
			b.WriteString("\n")
		}

		switch block.Type {
		case message.ContentText:
			if block.Text != "" {
				b.WriteString(block.Text)
			}

		case message.ContentThinking:
			if opts.Verbose && block.Thinking != "" {
				b.WriteString(thinkStyle.Render("💭 " + truncateContent(block.Thinking, 500)))
			}

		case message.ContentToolUse:
			toolStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.ToolName)).Bold(true)
			b.WriteString(accentStyle.Render("⎿ "))
			b.WriteString(toolStyle.Render(block.Name))
			if block.IsLoading {
				b.WriteString(dimStyle.Render(" …"))
			}

		case message.ContentRedactedThinking:
			if opts.Verbose {
				b.WriteString(thinkStyle.Render("💭 [redacted thinking]"))
			}
		}
	}

	if rm.IsStreaming {
		b.WriteString(accentStyle.Render("▌"))
	}

	return b.String()
}

func renderSystemMessage(rm RenderableMessage, colors theme.ColorScheme) string {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.TextMuted)).Italic(true)
	text := ""
	for _, block := range rm.Message.Content {
		if block.Type == message.ContentText {
			text = block.Text
			break
		}
	}
	return style.Render("ℹ " + text)
}

func renderCollapsedGroup(rm RenderableMessage, colors theme.ColorScheme) string {
	dimStyle := lipgloss.NewStyle().Faint(true)
	if rm.CollapsedText != "" {
		return dimStyle.Render("  " + rm.CollapsedText)
	}
	return dimStyle.Render("  [collapsed]")
}

func renderGenericMessage(rm RenderableMessage, colors theme.ColorScheme) string {
	dimStyle := lipgloss.NewStyle().Faint(true)
	text := ""
	for _, block := range rm.Message.Content {
		if block.Type == message.ContentText {
			text = block.Text
			break
		}
	}
	return dimStyle.Render(text)
}

// ---------------------------------------------------------------------------
// Message list utilities
// ---------------------------------------------------------------------------

// CountByRole returns counts of user and assistant messages.
func CountByRole(msgs []RenderableMessage) (user, assistant int) {
	for _, m := range msgs {
		switch m.Type {
		case "user":
			user++
		case "assistant":
			assistant++
		}
	}
	return
}

// LastAssistantText returns the text content of the last assistant message.
func LastAssistantText(msgs []RenderableMessage) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Type == "assistant" {
			for _, block := range msgs[i].Message.Content {
				if block.Type == message.ContentText && block.Text != "" {
					return block.Text
				}
			}
		}
	}
	return ""
}

// HasToolInProgress returns true if any tool use is currently loading.
func HasToolInProgress(msgs []RenderableMessage) bool {
	for _, m := range msgs {
		for _, block := range m.Message.Content {
			if block.Type == message.ContentToolUse && block.IsLoading {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func truncateContent(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}

func formatTimestamp(t time.Time) string {
	now := time.Now()
	if t.Day() == now.Day() && t.Month() == now.Month() && t.Year() == now.Year() {
		return t.Format("15:04")
	}
	return t.Format("Jan 2 15:04")
}

// MessageToRenderable converts a raw message to a renderable one.
func MessageToRenderable(msg message.Message, msgType string) RenderableMessage {
	return RenderableMessage{
		Type:      msgType,
		Message:   msg,
		Timestamp: time.Now(),
	}
}

// ConversationToRenderable converts a conversation to renderable messages.
func ConversationToRenderable(msgs []message.Message) []RenderableMessage {
	result := make([]RenderableMessage, len(msgs))
	for i, msg := range msgs {
		msgType := string(msg.Role)
		result[i] = RenderableMessage{
			Type:    msgType,
			Message: msg,
		}
	}
	return result
}

// RenderTurnSeparator renders a separator between conversation turns.
func RenderTurnSeparator(width int) string {
	colors := theme.Current().Colors()
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.BorderSubtle))
	return style.Render(strings.Repeat("─", width))
}

// RenderNewMessagesDivider renders the "N new messages" divider.
func RenderNewMessagesDivider(count int, width int) string {
	colors := theme.Current().Colors()
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Info))

	label := "new messages"
	if count == 1 {
		label = "new message"
	}
	text := fmt.Sprintf("── %d %s ──", count, label)
	padding := (width - len(text)) / 2
	if padding < 0 {
		padding = 0
	}
	return style.Render(strings.Repeat("─", padding) + text + strings.Repeat("─", padding))
}
