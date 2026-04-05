package components

import (
	"fmt"
	"strings"

	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/tools"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// MessageBubble renders a single message (user or assistant) with
// role-based styling. It's a pure rendering helper, not a tea.Model.
type MessageBubble struct {
	theme    theme.Theme
	width    int
	renderer *glamour.TermRenderer
}

// NewMessageBubble creates a new message bubble renderer.
func NewMessageBubble(t theme.Theme, width int) *MessageBubble {
	mb := &MessageBubble{
		theme: t,
		width: width,
	}
	mb.initRenderer()
	return mb
}

// initRenderer sets up the Glamour markdown renderer.
func (mb *MessageBubble) initRenderer() {
	wordWrap := mb.width
	if wordWrap < 20 {
		wordWrap = 80
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithEnvironmentConfig(),
		glamour.WithWordWrap(wordWrap),
	)
	if err == nil {
		mb.renderer = r
	}
}

// SetWidth updates the rendering width and recreates the markdown renderer.
func (mb *MessageBubble) SetWidth(width int) {
	if width != mb.width {
		mb.width = width
		mb.initRenderer()
	}
}

// Render renders a complete message with role-based styling.
// Returns the styled string representation of the message.
func (mb *MessageBubble) Render(msg *message.Message) string {
	if msg == nil {
		return ""
	}

	switch msg.Role {
	case message.RoleUser:
		return mb.renderUserMessage(msg)
	case message.RoleAssistant:
		return mb.renderAssistantMessage(msg)
	default:
		return mb.renderGenericMessage(msg)
	}
}

// RenderContent renders a single content block with appropriate styling.
func (mb *MessageBubble) RenderContent(block message.ContentBlock) string {
	switch block.Type {
	case message.ContentText:
		return mb.renderTextBlock(block.Text)
	case message.ContentToolUse:
		return mb.renderToolUseBlock(block)
	case message.ContentToolResult:
		return mb.renderToolResultBlock(block)
	case message.ContentThinking:
		return mb.renderThinkingBlock(block.Thinking)
	default:
		return ""
	}
}

// --- User message rendering ---

func (mb *MessageBubble) renderUserMessage(msg *message.Message) string {
	cs := mb.theme.Colors()
	var parts []string

	// User messages: bold primary text on subtle background (matching Claude Code)
	userStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.TextPrimary)).
		Bold(true)
	promptStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.Accent)).
		Bold(true)
	// Full-width background row
	rowStyle := lipgloss.NewStyle().
		Background(lipgloss.Color(cs.Surface))

	for _, block := range msg.Content {
		switch block.Type {
		case message.ContentText:
			text := block.Text
			if mb.width > 4 {
				text = wrapText(text, mb.width-4)
			}
			// User messages: "› " prefix with accent color
			lines := strings.Split(text, "\n")
			for i, line := range lines {
				var styledLine string
				if i == 0 {
					styledLine = promptStyle.Render(PromptPrefix) + userStyle.Render(line)
				} else {
					styledLine = promptStyle.Render("  ") + userStyle.Render(line)
				}
				// Apply background to full width
				if mb.width > 0 {
					lines[i] = rowStyle.Width(mb.width).Render(styledLine)
				} else {
					lines[i] = styledLine
				}
			}
			parts = append(parts, strings.Join(lines, "\n"))

		case message.ContentToolResult:
			parts = append(parts, mb.renderToolResultBlock(block))
		}
	}

	return strings.Join(parts, "\n")
}

// --- Assistant message rendering ---

func (mb *MessageBubble) renderAssistantMessage(msg *message.Message) string {
	cs := mb.theme.Colors()
	prefixStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.Accent))
	var parts []string
	firstText := true

	for _, block := range msg.Content {
		rendered := mb.RenderContent(block)
		if rendered != "" {
			// Claude Code prefixes the first text block with ⏺ (U+23FA)
			if firstText && block.Type == message.ContentText {
				rendered = prefixStyle.Render("⏺") + " " + rendered
				firstText = false
			}
			parts = append(parts, rendered)
		}
	}

	// Metadata footer
	footer := mb.renderMetadata(msg)
	if footer != "" {
		parts = append(parts, footer)
	}

	return strings.Join(parts, "\n")
}

// --- Content block renderers ---

func (mb *MessageBubble) renderTextBlock(text string) string {
	if text == "" {
		return ""
	}

	// Try to render as markdown via Glamour
	if mb.renderer != nil {
		rendered, err := mb.renderer.Render(text)
		if err == nil {
			return strings.TrimRight(rendered, "\n")
		}
	}

	// Fallback: plain text with word wrap
	if mb.width > 0 {
		return wrapText(text, mb.width)
	}
	return text
}

func (mb *MessageBubble) renderToolUseBlock(block message.ContentBlock) string {
	cs := mb.theme.Colors()

	toolStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.ToolName)).
		Bold(true)
	iconStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.Spinner))

	// Tool name header — Claude uses ⏺ (U+23FA) prefix for tool use too
	header := fmt.Sprintf("%s %s",
		iconStyle.Render("⏺"),
		toolStyle.Render(block.Name),
	)

	// Show input if available and short
	if len(block.Input) > 0 && len(block.Input) < 200 {
		inputStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(cs.TextSecondary))
		return header + "\n" + inputStyle.Render("  "+string(block.Input))
	}

	return header
}

func (mb *MessageBubble) renderToolResultBlock(block message.ContentBlock) string {
	cs := mb.theme.Colors()
	content := block.Content
	if content == "" {
		content = block.Text
	}

	connectorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.TextSecondary))

	if block.IsError {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(cs.Error))

		errMsg := content
		if len(errMsg) > 300 {
			errMsg = errMsg[:300] + "…"
		}
		return connectorStyle.Render(ResponseConnector) + errorStyle.Render(errMsg)
	}

	// If the tool attached a structured display payload, dispatch on its type.
	if block.Display != nil {
		switch d := block.Display.(type) {
		case tools.DiffDisplay:
			return mb.renderDiffDisplay(content, d, cs)
		}
	}

	// Successful result: show with "  └ " connector
	resultStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.TextSecondary))

	result := content
	// Truncate long results
	lines := strings.Split(result, "\n")
	if len(lines) > 10 {
		result = strings.Join(lines[:10], "\n") + "\n…[truncated]"
	}
	if len(result) > 500 {
		result = result[:500] + "…"
	}

	if result == "" {
		return connectorStyle.Render(ResponseConnector) + resultStyle.Render("(no content)")
	}

	// Indent each line with connector or continuation
	resultLines := strings.Split(result, "\n")
	for i, line := range resultLines {
		if i == 0 {
			resultLines[i] = connectorStyle.Render(ResponseConnector) + resultStyle.Render(line)
		} else {
			resultLines[i] = connectorStyle.Render(ResponseContinuation) + resultStyle.Render(line)
		}
	}

	return strings.Join(resultLines, "\n")
}

// renderDiffResult renders a colored unified-diff block from an edit/write
// tool result. `header` is the summary line (e.g. "Edited /path"); `disp`
// is the structured patch attached by the tool.
func (mb *MessageBubble) renderDiffDisplay(header string, disp tools.DiffDisplay, cs theme.ColorScheme) string {
	connectorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(cs.TextSecondary))
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(cs.TextSecondary))
	addedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(cs.DiffAdded))
	removedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(cs.DiffRemoved))
	hunkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(cs.Info))
	contextStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(cs.TextSecondary))

	// Count additions/removals across all hunks for the summary badge.
	var added, removed int
	for _, h := range disp.Hunks {
		for _, line := range h.Lines {
			if len(line) == 0 {
				continue
			}
			switch line[0] {
			case '+':
				added++
			case '-':
				removed++
			}
		}
	}

	var out []string
	if header != "" {
		line := connectorStyle.Render(ResponseConnector) + headerStyle.Render(header)
		if added > 0 || removed > 0 {
			line += contextStyle.Render(fmt.Sprintf(" (%s%d %s%d)",
				addedStyle.Render("+"), added,
				removedStyle.Render("-"), removed))
		}
		out = append(out, line)
	}

	for _, h := range disp.Hunks {
		out = append(out, connectorStyle.Render(ResponseContinuation)+
			hunkStyle.Render(fmt.Sprintf("@@ -%d,%d +%d,%d @@",
				h.OldStart, h.OldLines, h.NewStart, h.NewLines)))
		oldLn, newLn := h.OldStart, h.NewStart
		for _, line := range h.Lines {
			if len(line) == 0 {
				continue
			}
			marker, body := line[0], line[1:]
			var rendered string
			switch marker {
			case '+':
				rendered = addedStyle.Render(fmt.Sprintf("%4d + %s", newLn, body))
				newLn++
			case '-':
				rendered = removedStyle.Render(fmt.Sprintf("%4d - %s", oldLn, body))
				oldLn++
			default:
				rendered = contextStyle.Render(fmt.Sprintf("%4d   %s", newLn, body))
				oldLn++
				newLn++
			}
			out = append(out, connectorStyle.Render(ResponseContinuation)+rendered)
		}
	}
	return strings.Join(out, "\n")
}

func (mb *MessageBubble) renderThinkingBlock(thinking string) string {
	if thinking == "" {
		return ""
	}

	cs := mb.theme.Colors()
	thinkStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.TextSecondary)).
		Italic(true)
	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.Info)).
		Bold(true)

	// Truncate long thinking blocks
	if len(thinking) > 200 {
		thinking = thinking[:200] + "…"
	}

	return labelStyle.Render("💭 Thinking: ") + thinkStyle.Render(thinking)
}

// --- Metadata ---

func (mb *MessageBubble) renderMetadata(msg *message.Message) string {
	// For now, metadata is minimal — tokens/cost can be added when available
	return ""
}

// --- Generic ---

func (mb *MessageBubble) renderGenericMessage(msg *message.Message) string {
	var parts []string
	for _, block := range msg.Content {
		if block.Type == message.ContentText {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n")
}
