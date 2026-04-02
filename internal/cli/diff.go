package cli

import (
	"fmt"
	"strings"
)

// Source: components — structured diffs render inline

// RenderDiff formats a unified diff with ANSI colors for terminal display.
func RenderDiff(oldContent, newContent, filePath string) string {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\033[1m--- a/%s\033[0m\n", filePath))
	sb.WriteString(fmt.Sprintf("\033[1m+++ b/%s\033[0m\n", filePath))

	// Simple line-by-line diff (not a real unified diff algorithm,
	// but shows additions and removals for display purposes)
	maxLines := len(oldLines)
	if len(newLines) > maxLines {
		maxLines = len(newLines)
	}

	for i := 0; i < maxLines; i++ {
		var oldLine, newLine string
		if i < len(oldLines) {
			oldLine = oldLines[i]
		}
		if i < len(newLines) {
			newLine = newLines[i]
		}

		if i >= len(oldLines) {
			// Added line
			sb.WriteString(fmt.Sprintf("\033[32m+ %s\033[0m\n", newLine))
		} else if i >= len(newLines) {
			// Removed line
			sb.WriteString(fmt.Sprintf("\033[31m- %s\033[0m\n", oldLine))
		} else if oldLine != newLine {
			// Changed line
			sb.WriteString(fmt.Sprintf("\033[31m- %s\033[0m\n", oldLine))
			sb.WriteString(fmt.Sprintf("\033[32m+ %s\033[0m\n", newLine))
		} else {
			// Context line
			sb.WriteString(fmt.Sprintf("  %s\n", oldLine))
		}
	}

	return sb.String()
}

// FormatFileChange formats a single file change summary.
func FormatFileChange(path, changeType string) string {
	var color string
	switch changeType {
	case "added":
		color = "\033[32m" // Green
	case "deleted":
		color = "\033[31m" // Red
	case "modified":
		color = "\033[33m" // Yellow
	default:
		color = "\033[0m"
	}
	return fmt.Sprintf("%s%s %s\033[0m", color, changeType, path)
}
