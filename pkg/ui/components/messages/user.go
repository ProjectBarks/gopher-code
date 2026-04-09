package messages

// Source: components/messages/User*.tsx, UserToolResultMessage/*.tsx
//
// Renderers for user message content blocks. There are 14 variants in TS;
// in Go we handle the core types: prompt text, tool results (success/error/
// reject/cancel), command output, images, and system messages.

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// UserBlockOptions controls how a user content block renders.
type UserBlockOptions struct {
	Width       int
	Verbose     bool
	ShowDot     bool
	ToolName    string // for tool_result: which tool produced this
	IsContinuation bool // previous message was also user
}

// RenderUserBlock renders a single user content block.
func RenderUserBlock(block message.ContentBlock, opts UserBlockOptions) string {
	switch block.Type {
	case message.ContentText:
		return renderUserText(block, opts)
	case message.ContentToolResult:
		return renderUserToolResult(block, opts)
	default:
		return ""
	}
}

// renderUserText renders a user text input (prompt).
func renderUserText(block message.ContentBlock, opts UserBlockOptions) string {
	text := block.Text
	if text == "" {
		return ""
	}

	colors := theme.Current().Colors()
	promptStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Info)).Bold(true)

	var b strings.Builder

	// Truncate very long inputs (piped files)
	if len(text) > maxDisplayChars {
		text = truncateHeadTail(text, truncateHeadChars, truncateTailChars)
	}

	if !opts.IsContinuation {
		b.WriteString(promptStyle.Render("❯ "))
	} else {
		b.WriteString("  ")
	}
	b.WriteString(text)

	return b.String()
}

// renderUserToolResult renders a tool result (success, error, reject, cancel).
func renderUserToolResult(block message.ContentBlock, opts UserBlockOptions) string {
	colors := theme.Current().Colors()
	connectorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.TextSecondary))
	dimStyle := lipgloss.NewStyle().Faint(true)

	var b strings.Builder

	if block.IsError {
		return renderToolError(block, opts, colors)
	}

	// Success result
	content := block.Content
	if content == "" {
		return ""
	}

	// Determine display based on tool name
	toolName := opts.ToolName
	if toolName == "" {
		toolName = "tool"
	}

	b.WriteString(connectorStyle.Render(ResponseConnector))

	// Truncate long results
	if len(content) > 500 && !opts.Verbose {
		lines := strings.Split(content, "\n")
		if len(lines) > 10 {
			preview := strings.Join(lines[:5], "\n")
			b.WriteString(dimStyle.Render(preview))
			b.WriteString("\n")
			b.WriteString(dimStyle.Render(fmt.Sprintf("  … (%d lines)", len(lines))))
		} else {
			b.WriteString(dimStyle.Render(truncateContent(content, 500)))
		}
	} else {
		b.WriteString(dimStyle.Render(content))
	}

	return b.String()
}

// renderToolError renders an error tool result.
func renderToolError(block message.ContentBlock, _ UserBlockOptions, colors theme.ColorScheme) string {
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Error))
	connectorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.TextSecondary))

	var b strings.Builder
	b.WriteString(connectorStyle.Render(ResponseConnector))
	b.WriteString(errStyle.Render("Error: "))

	content := block.Content
	if len(content) > 500 {
		content = content[:500] + "…"
	}
	b.WriteString(errStyle.Render(content))

	return b.String()
}

// ---------------------------------------------------------------------------
// Tool result variants — Source: UserToolResultMessage/*.tsx
// ---------------------------------------------------------------------------

// RenderToolReject renders a rejected tool use result.
func RenderToolReject(toolName, reason string) string {
	colors := theme.Current().Colors()
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Warning))
	dimStyle := lipgloss.NewStyle().Faint(true)

	var b strings.Builder
	b.WriteString(warnStyle.Render("✗ " + toolName + " denied"))
	if reason != "" {
		b.WriteString(dimStyle.Render(" — " + reason))
	}
	return b.String()
}

// RenderToolCancel renders a cancelled tool use result.
func RenderToolCancel(toolName string) string {
	dimStyle := lipgloss.NewStyle().Faint(true)
	return dimStyle.Render("⏸ " + toolName + " cancelled")
}

// RenderPlanRejected renders a rejected plan approval.
func RenderPlanRejected() string {
	colors := theme.Current().Colors()
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Warning))
	return warnStyle.Render("✗ Plan rejected by user")
}

// ---------------------------------------------------------------------------
// Special user message types
// ---------------------------------------------------------------------------

// RenderUserImage renders an image attachment indicator.
func RenderUserImage(filename string, width, height int) string {
	colors := theme.Current().Colors()
	dimStyle := lipgloss.NewStyle().Faint(true)
	iconStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Info))

	var b strings.Builder
	b.WriteString(iconStyle.Render("🖼 "))
	b.WriteString(filename)
	if width > 0 && height > 0 {
		b.WriteString(dimStyle.Render(fmt.Sprintf(" (%dx%d)", width, height)))
	}
	return b.String()
}

// RenderCommandOutput renders output from a local command.
func RenderCommandOutput(command, output string) string {
	colors := theme.Current().Colors()
	cmdStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.ToolName)).Bold(true)
	dimStyle := lipgloss.NewStyle().Faint(true)

	var b strings.Builder
	b.WriteString(cmdStyle.Render("$ " + command))
	if output != "" {
		b.WriteString("\n")
		// Truncate long output
		lines := strings.Split(output, "\n")
		maxLines := 20
		for i, line := range lines {
			if i >= maxLines {
				b.WriteString(dimStyle.Render(fmt.Sprintf("  … (%d more lines)", len(lines)-maxLines)))
				break
			}
			b.WriteString(dimStyle.Render("  " + line))
			if i < len(lines)-1 && i < maxLines-1 {
				b.WriteString("\n")
			}
		}
	}
	return b.String()
}

// RenderUserMessage renders a complete user message (all blocks).
func RenderUserMessage(msg message.Message, opts UserBlockOptions) string {
	var parts []string
	for _, block := range msg.Content {
		rendered := RenderUserBlock(block, opts)
		if rendered != "" {
			parts = append(parts, rendered)
		}
	}
	return strings.Join(parts, "\n")
}

// ---------------------------------------------------------------------------
// Constants and helpers
// ---------------------------------------------------------------------------

const (
	maxDisplayChars  = 10_000
	truncateHeadChars = 2_500
	truncateTailChars = 2_500
)

// truncateHeadTail keeps the first headN and last tailN characters.
func truncateHeadTail(s string, headN, tailN int) string {
	if len(s) <= headN+tailN {
		return s
	}
	omitted := len(s) - headN - tailN
	return s[:headN] + fmt.Sprintf("\n… (%d characters omitted) …\n", omitted) + s[len(s)-tailN:]
}
