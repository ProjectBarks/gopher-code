package messages

// Source: components/messages/AssistantTextMessage.tsx,
//         AssistantToolUseMessage.tsx, AssistantThinkingMessage.tsx,
//         AssistantRedactedThinkingMessage.tsx
//
// Renderers for the four assistant message content block types:
// text (with markdown, error detection), tool use (with progress),
// thinking (collapsible), and redacted thinking.

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// ResponseConnector is the ⎿ character used before tool results.
const ResponseConnector = "⎿ "

// AssistantBlockOptions controls how an assistant content block renders.
type AssistantBlockOptions struct {
	Verbose        bool
	ShowDot        bool // show the ● role indicator
	Width          int
	IsInProgress   bool
	IsStreaming    bool
	ShowTimestamp  bool
	ToolCallCount int // number of concurrent tool calls
}

// RenderAssistantBlock renders a single assistant content block.
func RenderAssistantBlock(block message.ContentBlock, opts AssistantBlockOptions) string {
	switch block.Type {
	case message.ContentText:
		return renderAssistantText(block, opts)
	case message.ContentToolUse:
		return renderAssistantToolUse(block, opts)
	case message.ContentThinking:
		return renderAssistantThinking(block, opts)
	case message.ContentRedactedThinking:
		return renderAssistantRedactedThinking(opts)
	default:
		return ""
	}
}

// renderAssistantText renders a text content block.
// Handles: normal text, API errors, rate limits, interrupts.
func renderAssistantText(block message.ContentBlock, opts AssistantBlockOptions) string {
	text := block.Text
	if text == "" || isEmptyText(text) {
		return ""
	}

	colors := theme.Current().Colors()

	// Detect special message types
	if isAPIError(text) {
		return renderAPIError(text, colors)
	}
	if isRateLimitError(text) {
		return renderRateLimitError(text, colors)
	}
	if isInterruptedMessage(text) {
		return renderInterrupted(colors)
	}

	// Normal text — render with markdown-style formatting
	var b strings.Builder
	if opts.ShowDot {
		dotStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Accent))
		b.WriteString(dotStyle.Render("● "))
	}
	b.WriteString(text)
	return b.String()
}

// renderAssistantToolUse renders a tool_use content block.
func renderAssistantToolUse(block message.ContentBlock, opts AssistantBlockOptions) string {
	colors := theme.Current().Colors()
	connectorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.TextSecondary))
	toolStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.ToolName)).Bold(true)
	dimStyle := lipgloss.NewStyle().Faint(true)

	var b strings.Builder

	if opts.ShowDot {
		dotStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Accent))
		b.WriteString(dotStyle.Render("● "))
	}

	b.WriteString(connectorStyle.Render(ResponseConnector))
	b.WriteString(toolStyle.Render(block.Name))

	// Show loading state
	if block.IsLoading || opts.IsInProgress {
		b.WriteString(dimStyle.Render(" …"))
		if opts.ToolCallCount > 1 {
			b.WriteString(dimStyle.Render(fmt.Sprintf(" (%d concurrent)", opts.ToolCallCount)))
		}
	}

	return b.String()
}

// renderAssistantThinking renders a thinking content block.
func renderAssistantThinking(block message.ContentBlock, opts AssistantBlockOptions) string {
	if !opts.Verbose {
		return "" // hidden in non-verbose mode
	}
	if block.Thinking == "" {
		return ""
	}

	colors := theme.Current().Colors()
	thinkStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.TextMuted)).
		Italic(true)

	text := block.Thinking
	if len(text) > 500 {
		text = text[:500] + "…"
	}

	return thinkStyle.Render("💭 " + text)
}

// renderAssistantRedactedThinking renders a redacted thinking indicator.
func renderAssistantRedactedThinking(opts AssistantBlockOptions) string {
	if !opts.Verbose {
		return ""
	}
	colors := theme.Current().Colors()
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.TextMuted)).Italic(true)
	return style.Render("💭 [thinking hidden]")
}

// ---------------------------------------------------------------------------
// Special message detection — Source: AssistantTextMessage.tsx
// ---------------------------------------------------------------------------

// Common API error prefixes
const (
	apiErrorPrefix   = "API Error:"
	apiTimeoutPrefix = "API request timed out"
	promptTooLong    = "Prompt too long"
	invalidAPIKey    = "Invalid API key"
	creditTooLow     = "Insufficient credits"
	orgDisabled      = "Organization disabled"
	interruptedMsg   = "interrupted by user"
	noResponseMsg    = "[no response]"
)

func isEmptyText(text string) bool {
	return strings.TrimSpace(text) == "" || text == noResponseMsg
}

func isAPIError(text string) bool {
	return strings.HasPrefix(text, apiErrorPrefix) ||
		strings.HasPrefix(text, apiTimeoutPrefix) ||
		strings.Contains(text, invalidAPIKey) ||
		strings.Contains(text, creditTooLow) ||
		strings.Contains(text, orgDisabled)
}

func isRateLimitError(text string) bool {
	return strings.Contains(text, "rate limit") || strings.Contains(text, "Rate limit")
}

func isInterruptedMessage(text string) bool {
	return strings.Contains(strings.ToLower(text), interruptedMsg)
}

func renderAPIError(text string, colors theme.ColorScheme) string {
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Error))
	// Truncate long error messages
	if len(text) > 1000 {
		text = text[:1000] + "…"
	}
	return errStyle.Render("⚠ " + text)
}

func renderRateLimitError(text string, colors theme.ColorScheme) string {
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Warning))
	return warnStyle.Render("⏳ " + text)
}

func renderInterrupted(colors theme.ColorScheme) string {
	dimStyle := lipgloss.NewStyle().Faint(true)
	return dimStyle.Render("⏸ Interrupted by user")
}

// RenderAssistantMessage renders a complete assistant message (all blocks).
func RenderAssistantMessage(msg message.Message, opts AssistantBlockOptions) string {
	var parts []string
	for _, block := range msg.Content {
		rendered := RenderAssistantBlock(block, opts)
		if rendered != "" {
			parts = append(parts, rendered)
		}
	}
	result := strings.Join(parts, "\n")
	if opts.IsStreaming {
		accentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Current().Colors().Accent))
		result += accentStyle.Render("▌")
	}
	return result
}
