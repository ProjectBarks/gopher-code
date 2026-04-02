package cli

import (
	"fmt"

	"github.com/projectbarks/gopher-code/pkg/query"
)

// PrintEvent renders a QueryEvent to stdout with ANSI colors.
func PrintEvent(evt query.QueryEvent) {
	switch evt.Type {
	case query.QEventTextDelta:
		fmt.Print(evt.Text) // Stream text as it arrives

	case query.QEventToolUseStart:
		fmt.Printf("\n\033[36m⚙ %s\033[0m\n", evt.ToolName) // Cyan tool name

	case query.QEventToolResult:
		if evt.IsError {
			fmt.Printf("\033[31m✗ Error: %s\033[0m\n", truncate(evt.Content, 200))
		} else {
			fmt.Printf("\033[32m✓ %s\033[0m\n", truncate(evt.Content, 200))
		}

	case query.QEventTurnComplete:
		// Nothing needed

	case query.QEventUsage:
		// Optionally show usage — silent by default
	}
}

// truncate shortens a string to maxLen, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
