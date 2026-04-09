// Package tool provides tool result message rendering for the TUI.
// Source: components/messages/UserToolResultMessage/
//
// Renders tool_result blocks as success, error, rejected, or canceled,
// with collapsible content and tool-specific formatting.
package tool

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// ResultType classifies a tool result for rendering.
type ResultType int

const (
	ResultSuccess  ResultType = iota
	ResultError
	ResultRejected
	ResultCanceled
)

// CancelMessage is the prefix that identifies canceled tool results.
// Source: utils/messages.ts — CANCEL_MESSAGE
const CancelMessage = "Tool execution was canceled by the user"

// RejectMessage is the prefix for permission-rejected results.
const RejectMessage = "The user rejected this tool"

// InterruptMessage is injected when the user interrupts during tool execution.
const InterruptMessage = "User interrupted tool execution"

// Result holds the data needed to render a tool result message.
type Result struct {
	ToolName  string
	ToolUseID string
	Content   string
	IsError   bool
	Type      ResultType
	Width     int
}

// ClassifyResult determines the result type from the content string.
// Source: UserToolResultMessage.tsx:40-60
func ClassifyResult(content string, isError bool) ResultType {
	if strings.HasPrefix(content, CancelMessage) {
		return ResultCanceled
	}
	if strings.HasPrefix(content, RejectMessage) || content == InterruptMessage {
		return ResultRejected
	}
	if isError {
		return ResultError
	}
	return ResultSuccess
}

// Render returns a styled string for terminal display.
// Source: UserToolResultMessage/ — dispatches to sub-renderers
func Render(r Result) string {
	switch r.Type {
	case ResultCanceled:
		return renderCanceled()
	case ResultRejected:
		return renderRejected(r)
	case ResultError:
		return renderError(r)
	default:
		return renderSuccess(r)
	}
}

func renderCanceled() string {
	style := lipgloss.NewStyle().Faint(true)
	return style.Render("⎿ Tool execution canceled")
}

func renderRejected(r Result) string {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("11")) // yellow
	msg := "⎿ Permission denied"
	if r.Content != RejectMessage && r.Content != InterruptMessage {
		// Strip prefix to show the reason
		reason := strings.TrimPrefix(r.Content, RejectMessage+": ")
		if reason != r.Content {
			msg += ": " + reason
		}
	}
	return style.Render(msg)
}

func renderError(r Result) string {
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	content := r.Content
	if len(content) > 200 {
		content = content[:200] + "..."
	}
	return errStyle.Render(fmt.Sprintf("⎿ Error: %s", content))
}

func renderSuccess(r Result) string {
	content := r.Content
	lines := strings.Split(content, "\n")

	// Truncate long results
	maxLines := 10
	if len(lines) > maxLines {
		shown := strings.Join(lines[:maxLines], "\n")
		omitted := len(lines) - maxLines
		content = shown + fmt.Sprintf("\n  ... (%d more lines)", omitted)
	}

	// Indent with the response prefix
	indented := "  " + strings.ReplaceAll(content, "\n", "\n  ")
	return "⎿ " + r.ToolName + "\n" + indented
}

// RenderCompact returns a one-line summary for collapsed display.
func RenderCompact(r Result) string {
	switch r.Type {
	case ResultCanceled:
		return "canceled"
	case ResultRejected:
		return "rejected"
	case ResultError:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("error")
	default:
		// First line of content, truncated
		first := strings.SplitN(r.Content, "\n", 2)[0]
		if len(first) > 60 {
			first = first[:57] + "..."
		}
		return first
	}
}
